package tui

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/phy0hk/devproxy/internal/config"
	"github.com/phy0hk/devproxy/internal/event"
	"github.com/phy0hk/devproxy/internal/logstore"
	"github.com/phy0hk/devproxy/internal/process"
)

const (
	maxLogLines = 2000
	logLimit    = 2000
)

// LogEntry is one locally retained line for the terminal console.
type LogEntry struct {
	RunID     string
	Sequence  uint64
	Timestamp time.Time
	Process   string
	Stream    string // stdout, stderr, lifecycle, proxy
	Message   string
	Type      string
	Error     string
}

type eventMsg struct {
	Event event.Event
}

type statusMsg []process.Status

type historyMsg struct {
	Token     int
	Process   string
	Run       string
	Follow    bool
	Lines     []logstore.Line
	Runs      []logstore.Run
	ActiveRun string
	Err       error
}

type actionDoneMsg struct {
	Action string
	Name   string
	Err    error
}

type tickMsg time.Time

// Model is the Bubble Tea model. All mutation happens in Update, including
// event-driven updates delivered by Program.Send.
type Model struct {
	config  *config.Config
	manager *process.Manager
	bus     *event.Bus

	keys KeyMap
	help help.Model

	width  int
	height int
	layout Layout
	ready  bool

	processes []process.Status
	selected  int

	logs          []LogEntry
	maxSeq        map[string]uint64
	views         []string
	viewIndex     int
	runs          []logstore.Run
	followCurrent bool
	selectedRun   string
	activeRun     string

	follow    bool
	filter    string
	filtering bool
	showHelp  bool

	statusMsg  string
	statusTime time.Time
	err        error
	quitting   bool

	historyToken   int
	historyProcess string

	logVP  viewport.Model
	listVP viewport.Model
}

func InitialModel(cfg *config.Config, manager *process.Manager, bus *event.Bus) Model {
	model := Model{
		config:        cfg,
		manager:       manager,
		bus:           bus,
		keys:          DefaultKeyMap,
		help:          help.New(),
		views:         []string{"stdout", "stderr", "lifecycle", "proxy"},
		follow:        true,
		followCurrent: true,
		maxSeq:        make(map[string]uint64),
		logVP:         viewport.New(0, 0),
		listVP:        viewport.New(0, 0),
	}
	model.help.ShowAll = false
	model.help.Styles.ShortKey = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	model.help.Styles.ShortDesc = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	model.help.Styles.FullKey = lipgloss.NewStyle().Foreground(lipgloss.Color("229"))
	model.help.Styles.FullDesc = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	model.help.Styles.FullSeparator = lipgloss.NewStyle().Foreground(lipgloss.Color("62"))
	return model
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(m.tick(), statusesCmd(m.manager), m.requestHistory())
}

func (m Model) tick() tea.Cmd {
	return tea.Tick(time.Second, func(at time.Time) tea.Msg {
		return tickMsg(at)
	})
}

func statusesCmd(manager *process.Manager) tea.Cmd {
	if manager == nil {
		return nil
	}
	return func() tea.Msg {
		return statusMsg(manager.Statuses())
	}
}

func actionCmd(manager *process.Manager, name string, action string) tea.Cmd {
	if manager == nil || name == "" {
		return nil
	}
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		var err error
		switch action {
		case "start":
			err = manager.Start(ctx, name)
		case "stop":
			err = manager.Stop(ctx, name)
		case "restart":
			err = manager.Restart(ctx, name)
		default:
			err = fmt.Errorf("unknown action %q", action)
		}
		return actionDoneMsg{Action: action, Name: name, Err: err}
	}
}

func historyCmd(
	manager *process.Manager,
	name string,
	follow bool,
	run string,
	token int,
) tea.Cmd {
	if manager == nil || name == "" {
		return nil
	}

	return func() tea.Msg {
		meta, err := manager.Logs(name, "current", 1, 0)
		if err != nil {
			return historyMsg{Token: token, Process: name, Err: err}
		}

		if follow {
			current, err := manager.Logs(name, "current", logLimit, 0)
			if err != nil {
				return historyMsg{Token: token, Process: name, Err: err}
			}

			lines := append([]logstore.Line(nil), current.Lines...)
			if len(meta.Runs) > 1 {
				previous, err := manager.Logs(name, "previous", logLimit, 0)
				if err == nil {
					lines = append(previous.Lines, lines...)
				}
			}

			return historyMsg{
				Token:     token,
				Process:   name,
				Follow:    true,
				Lines:     lines,
				Runs:      meta.Runs,
				ActiveRun: current.Run,
			}
		}

		result, err := manager.Logs(name, run, logLimit, 0)
		if err != nil {
			return historyMsg{Token: token, Process: name, Err: err}
		}
		return historyMsg{
			Token:   token,
			Process: name,
			Run:     result.Run,
			Lines:   result.Lines,
			Runs:    result.Runs,
		}
	}
}

func (m *Model) requestHistory() tea.Cmd {
	name := m.selectedName()
	if name == "" || m.manager == nil {
		return nil
	}
	m.historyToken++
	m.historyProcess = name
	return historyCmd(m.manager, name, m.followCurrent, m.selectedRun, m.historyToken)
}

func (m Model) selectedName() string {
	if m.selected >= 0 && m.selected < len(m.processes) {
		return m.processes[m.selected].Name
	}
	return ""
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.layout = ComputeLayout(m.renderWidth(), m.height, m.showHelp)
		m.ready = true
		m.applyLayout()
		m.rebuildLogContent()
		return m, nil

	case tickMsg:
		return m, tea.Batch(m.tick(), statusesCmd(m.manager))

	case statusMsg:
		m.processes = []process.Status(msg)
		if len(m.processes) == 0 {
			m.selected = 0
		} else if m.selected >= len(m.processes) {
			m.selected = len(m.processes) - 1
		}

		var cmds []tea.Cmd
		name := m.selectedName()
		if name != "" && m.historyProcess != name {
			cmds = append(cmds, m.requestHistory())
		}
		m.rebuildLogContent()
		return m, tea.Batch(cmds...)

	case eventMsg:
		return m.handleEvent(msg.Event)

	case historyMsg:
		if msg.Token != m.historyToken || msg.Process != m.selectedName() {
			return m, nil
		}
		if msg.Err != nil {
			m.statusMsg = fmt.Sprintf("History: %v", msg.Err)
			m.statusTime = time.Now()
			return m, nil
		}
		m.applyHistory(msg)
		return m, nil

	case actionDoneMsg:
		if msg.Err != nil {
			m.statusMsg = fmt.Sprintf("%s %s: %v", msg.Action, msg.Name, msg.Err)
			m.statusTime = time.Now()
		} else {
			m.statusMsg = fmt.Sprintf("%s %s", msg.Action, msg.Name)
			m.statusTime = time.Now()
		}

		cmds := []tea.Cmd{statusesCmd(m.manager)}
		if msg.Err == nil && (msg.Action == "start" || msg.Action == "restart") &&
			msg.Name == m.selectedName() {
			cmds = append(cmds, m.requestHistory())
		}
		return m, tea.Batch(cmds...)

	case tea.KeyMsg:
		return m.handleKey(msg)

	case error:
		m.err = msg
	}

	return m, nil
}

func (m Model) handleEvent(item event.Event) (tea.Model, tea.Cmd) {
	switch evt := item.(type) {
	case event.ProcessEvent:
		return m.handleProcessEvent(evt)
	case event.RequestEvent:
		m.appendLog(LogEntry{
			Sequence:  evt.Sequence,
			Timestamp: evt.Timestamp,
			Process:   "proxy",
			Stream:    "proxy",
			Message: fmt.Sprintf(
				"%s %s → %s %d %dms",
				evt.Method,
				evt.Path,
				evt.Upstream,
				evt.Status,
				evt.DurationMS,
			),
		})
		m.rebuildLogContent()
	}
	return m, nil
}

func (m Model) handleProcessEvent(evt event.ProcessEvent) (tea.Model, tea.Cmd) {
	if evt.Process == m.selectedName() {
		switch evt.Type() {
		case event.TypeProcessStarted:
			m.statusMsg = fmt.Sprintf("Started %s", evt.Process)
			m.statusTime = time.Now()
		case event.TypeProcessStopping:
			m.statusMsg = fmt.Sprintf("Stopping %s", evt.Process)
			m.statusTime = time.Now()
		case event.TypeProcessExited:
			m.statusMsg = fmt.Sprintf("%s exited (%d)", evt.Process, evt.ExitCode)
			m.statusTime = time.Now()
		case event.TypeProcessFailed:
			m.statusMsg = fmt.Sprintf("%s failed: %s", evt.Process, evt.Error)
			m.statusTime = time.Now()
		}
	}

	switch evt.Type() {
	case event.TypeProcessOutput:
		if evt.Process != m.selectedName() {
			return m, nil
		}
		if m.followCurrent {
			if m.activeRun != "" && evt.RunID != "" && evt.RunID != m.activeRun {
				return m, nil
			}
		} else if m.selectedRun != "" && evt.RunID != "" && evt.RunID != m.selectedRun {
			return m, nil
		}

		m.appendLog(LogEntry{
			RunID:     evt.RunID,
			Sequence:  evt.Sequence,
			Timestamp: evt.Timestamp,
			Process:   evt.Process,
			Stream:    evt.Stream,
			Message:   evt.Message,
			Type:      string(evt.Type()),
		})
		m.rebuildLogContent()

	case event.TypeProcessStarted:
		var cmds []tea.Cmd
		if evt.Process == m.selectedName() {
			cmds = append(cmds, m.requestHistory())
		}
		cmds = append(cmds, statusesCmd(m.manager))
		return m, tea.Batch(cmds...)

	case event.TypeProcessStopping, event.TypeProcessExited, event.TypeProcessFailed:
		m.appendLog(LogEntry{
			RunID:     evt.RunID,
			Sequence:  evt.Sequence,
			Timestamp: evt.Timestamp,
			Process:   evt.Process,
			Stream:    "lifecycle",
			Type:      string(evt.Type()),
			Message:   evt.Message,
			Error:     evt.Error,
		})
		m.rebuildLogContent()
		return m, statusesCmd(m.manager)
	}

	return m, nil
}

func (m *Model) appendLog(entry LogEntry) {
	if entry.Sequence != 0 {
		if entry.Sequence <= m.maxSeq[entry.Process] {
			return
		}
		m.maxSeq[entry.Process] = entry.Sequence
	}

	m.logs = append(m.logs, entry)
	if len(m.logs) > maxLogLines {
		m.logs = append([]LogEntry(nil), m.logs[len(m.logs)-maxLogLines:]...)
	}
}

func (m *Model) applyHistory(msg historyMsg) {
	m.runs = msg.Runs
	m.followCurrent = msg.Follow
	if msg.Follow {
		m.activeRun = msg.ActiveRun
	} else {
		m.activeRun = msg.Run
	}

	allowed := make(map[string]struct{})
	if msg.Follow {
		if msg.ActiveRun != "" {
			allowed[msg.ActiveRun] = struct{}{}
		}
		if len(msg.Runs) > 1 {
			allowed[msg.Runs[1].ID] = struct{}{}
		}
	} else if msg.Run != "" {
		allowed[msg.Run] = struct{}{}
	}

	merged := make(map[uint64]LogEntry)
	kept := make([]LogEntry, 0, len(m.logs)+len(msg.Lines))
	for _, entry := range m.logs {
		if entry.Process != msg.Process {
			kept = append(kept, entry)
			continue
		}
		if entry.Stream == "stdout" || entry.Stream == "stderr" {
			if len(allowed) > 0 && entry.RunID != "" {
				if _, ok := allowed[entry.RunID]; !ok {
					continue
				}
			}
			if entry.Sequence != 0 {
				merged[entry.Sequence] = entry
				continue
			}
		}
		kept = append(kept, entry)
	}

	for _, line := range msg.Lines {
		entry := LogEntry{
			RunID:     line.RunID,
			Sequence:  line.Sequence,
			Timestamp: line.Timestamp,
			Process:   line.Process,
			Stream:    line.Stream,
			Message:   line.Message,
		}
		merged[line.Sequence] = entry
		if line.Sequence > m.maxSeq[msg.Process] {
			m.maxSeq[msg.Process] = line.Sequence
		}
	}

	for _, entry := range merged {
		kept = append(kept, entry)
	}

	sort.SliceStable(kept, func(left, right int) bool {
		if kept[left].Timestamp.Equal(kept[right].Timestamp) {
			return kept[left].Sequence < kept[right].Sequence
		}
		return kept[left].Timestamp.Before(kept[right].Timestamp)
	})

	if len(kept) > maxLogLines {
		kept = kept[len(kept)-maxLogLines:]
	}
	m.logs = kept
	m.follow = true
	m.rebuildLogContent()
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.filtering {
		return m.handleFilterKey(msg)
	}

	switch {
	case key.Matches(msg, m.keys.Quit):
		m.quitting = true
		return m, tea.Quit

	case key.Matches(msg, m.keys.Help):
		m.showHelp = !m.showHelp
		m.help.ShowAll = m.showHelp
		m.layout = ComputeLayout(m.renderWidth(), m.height, m.showHelp)
		m.applyLayout()
		m.rebuildLogContent()

	case key.Matches(msg, m.keys.Up):
		if m.selected > 0 {
			m.selected--
			m.follow = true
			return m, m.selectChanged()
		}

	case key.Matches(msg, m.keys.Down):
		if m.selected < len(m.processes)-1 {
			m.selected++
			m.follow = true
			return m, m.selectChanged()
		}

	case key.Matches(msg, m.keys.Left), key.Matches(msg, m.keys.ShiftTab):
		m.viewIndex = (m.viewIndex + len(m.views) - 1) % len(m.views)
		m.follow = true
		m.rebuildLogContent()

	case key.Matches(msg, m.keys.Right), key.Matches(msg, m.keys.Tab),
		key.Matches(msg, m.keys.SwitchView):
		m.viewIndex = (m.viewIndex + 1) % len(m.views)
		m.follow = true
		m.rebuildLogContent()

	case key.Matches(msg, m.keys.Start):
		return m, actionCmd(m.manager, m.selectedName(), "start")

	case key.Matches(msg, m.keys.Stop):
		return m, actionCmd(m.manager, m.selectedName(), "stop")

	case key.Matches(msg, m.keys.Restart):
		return m, actionCmd(m.manager, m.selectedName(), "restart")

	case key.Matches(msg, m.keys.Refresh):
		m.statusMsg = "Refreshed"
		m.statusTime = time.Now()
		return m, tea.Batch(statusesCmd(m.manager), m.requestHistory())

	case key.Matches(msg, m.keys.Clear):
		m.clearCurrentView()
		m.statusMsg = "View cleared (server history kept)"
		m.statusTime = time.Now()
		m.rebuildLogContent()

	case key.Matches(msg, m.keys.PrevRun):
		return m.changeRun(-1)

	case key.Matches(msg, m.keys.NextRun):
		return m.changeRun(1)

	case key.Matches(msg, m.keys.Filter):
		m.filtering = true
		m.filter = ""
		m.rebuildLogContent()

	case key.Matches(msg, m.keys.ScrollUp):
		m.logVP.LineUp(5)
		m.follow = m.atBottom()

	case key.Matches(msg, m.keys.ScrollDown):
		m.logVP.LineDown(5)
		m.follow = m.atBottom()

	case key.Matches(msg, m.keys.PageUp):
		m.logVP.PageUp()
		m.follow = false

	case key.Matches(msg, m.keys.PageDown):
		m.logVP.PageDown()
		m.follow = m.atBottom()

	case key.Matches(msg, m.keys.Home):
		m.logVP.GotoTop()
		m.follow = false

	case key.Matches(msg, m.keys.End):
		m.logVP.GotoBottom()
		m.follow = true
	}

	return m, nil
}

func (m Model) handleFilterKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		m.quitting = true
		return m, tea.Quit
	case "esc", "enter":
		m.filtering = false
		m.rebuildLogContent()
	case "backspace":
		runes := []rune(m.filter)
		if len(runes) > 0 {
			m.filter = string(runes[:len(runes)-1])
		}
		m.rebuildLogContent()
	case "ctrl+u":
		m.filter = ""
		m.rebuildLogContent()
	default:
		if len(msg.Runes) > 0 {
			m.filter += string(msg.Runes)
			m.rebuildLogContent()
		}
	}
	return m, nil
}

func (m *Model) selectChanged() tea.Cmd {
	m.historyProcess = ""
	m.follow = true
	return m.requestHistory()
}

func (m Model) changeRun(direction int) (tea.Model, tea.Cmd) {
	if len(m.runs) == 0 {
		return m, nil
	}

	if m.followCurrent {
		if direction > 0 || len(m.runs) < 2 {
			return m, nil
		}
		m.followCurrent = false
		m.selectedRun = m.runs[1].ID
	} else {
		index := 0
		for i, run := range m.runs {
			if run.ID == m.selectedRun {
				index = i
				break
			}
		}
		// Older runs have larger indexes because the API returns newest first.
		index -= direction
		if index <= 0 {
			m.followCurrent = true
			m.selectedRun = ""
		} else if index < len(m.runs) {
			m.selectedRun = m.runs[index].ID
		} else {
			return m, nil
		}
	}

	m.follow = true
	m.historyToken++
	m.historyProcess = m.selectedName()
	return m, historyCmd(m.manager, m.selectedName(), m.followCurrent, m.selectedRun, m.historyToken)
}

func (m *Model) clearCurrentView() {
	view := m.views[m.viewIndex]
	filtered := make([]LogEntry, 0, len(m.logs))
	for _, entry := range m.logs {
		matches := false
		switch view {
		case "proxy":
			matches = entry.Stream == "proxy"
		case "lifecycle":
			matches = entry.Stream == "lifecycle" && entry.Process == m.selectedName()
		default:
			matches = entry.Stream == view && entry.Process == m.selectedName()
		}
		if !matches {
			filtered = append(filtered, entry)
		}
	}
	m.logs = filtered
}

func (m Model) View() string {
	if !m.ready {
		return "Loading…"
	}
	if m.quitting {
		return ""
	}
	if m.layout.TooSmall {
		message := fmt.Sprintf(
			"Terminal too small\n%d×%d (minimum %d×%d)\nResize the terminal to continue.",
			m.width,
			m.height,
			MinWidth,
			MinHeight,
		)
		return lipgloss.Place(
			m.width,
			m.height,
			lipgloss.Center,
			lipgloss.Center,
			errorStyle.Render(message),
		)
	}

	renderWidth := m.renderWidth()
	header := headerStyle.Width(renderWidth).MaxWidth(renderWidth).Render(m.headerText())
	left := m.renderProcessPanel()
	right := m.renderLogPanel()
	content := lipgloss.JoinHorizontal(lipgloss.Top, left, " ", right)
	status := statusBarStyle.Width(renderWidth).Render(m.statusText())
	help := m.renderHelp()

	return lipgloss.NewStyle().
		MaxWidth(renderWidth).
		MaxHeight(m.height).
		Render(lipgloss.JoinVertical(lipgloss.Left, header, content, status, help))
}

// renderWidth leaves one cell free at the right edge. Full-width lines put
// terminals in a pending-wrap state, and Bubble Tea's line skipping expects the
// cursor to be at column zero on the next row.
func (m Model) renderWidth() int {
	return max(1, m.width-1)
}

func (m Model) headerText() string {
	running := 0
	for _, status := range m.processes {
		if status.State == process.StateRunning {
			running++
		}
	}
	text := fmt.Sprintf(
		" DevProxy TUI · %d managed · %d running · view %s ",
		len(m.processes),
		running,
		m.views[m.viewIndex],
	)
	if m.filtering {
		text += "· FILTER "
	}
	return truncate(text, m.renderWidth())
}

func (m Model) renderProcessPanel() string {
	title := titleStyle.Render(" Instances ")
	body := m.listVP.View()
	content := lipgloss.JoinVertical(lipgloss.Left, title, body)
	return processListStyle.
		Width(max(1, m.layout.ListWidth-2)).
		Height(max(1, m.layout.ContentHeight-2)).
		MaxWidth(m.layout.ListWidth).
		MaxHeight(m.layout.ContentHeight).
		Render(content)
}

func (m Model) renderLogPanel() string {
	tabs := make([]string, 0, len(m.views))
	for index, view := range m.views {
		if index == m.viewIndex {
			tabs = append(tabs, activeTabStyle.Render(" "+view+" "))
		} else {
			tabs = append(tabs, tabStyle.Render(" "+view+" "))
		}
	}
	tabBar := strings.Join(tabs, " ")
	content := lipgloss.JoinVertical(lipgloss.Left, tabBar, m.logVP.View())
	return logPanelStyle.
		Width(max(1, m.layout.LogWidth-2)).
		Height(max(1, m.layout.ContentHeight-2)).
		MaxWidth(m.layout.LogWidth).
		MaxHeight(m.layout.ContentHeight).
		Render(content)
}

func (m Model) statusText() string {
	parts := []string{fmt.Sprintf("Selected: %s", orDash(m.selectedName()))}

	if m.selected < len(m.processes) {
		status := m.processes[m.selected]
		parts = append(parts, fmt.Sprintf("%s", status.State))
		parts = append(parts, "run: "+m.currentRunLabel())
	}
	if m.filtering || m.filter != "" {
		parts = append(parts, "filter: "+m.filter)
	}
	if m.statusMsg != "" && time.Since(m.statusTime) < 5*time.Second {
		parts = append(parts, SanitizeTerminalText(m.statusMsg))
	}

	text := strings.Join(parts, " │ ")
	return truncate(text, max(1, m.renderWidth()-2))
}

func (m Model) renderHelp() string {
	if m.showHelp {
		view := m.help.View(m.keys)
		lines := strings.Split(view, "\n")
		if len(lines) > m.layout.HelpHeight {
			lines = lines[:m.layout.HelpHeight]
		}
		return helpStyle.Width(m.renderWidth()).Render(strings.Join(lines, "\n"))
	}

	text := "↑↓ move · s start · x stop · r restart · v view · [ ] runs · / filter · c clear · q quit"
	if m.filtering {
		text = "Type to filter · enter/esc done · ctrl+u clear · ctrl+c quit"
	}
	return helpStyle.Width(m.renderWidth()).Render(truncate(text, max(1, m.renderWidth()-2)))
}

func (m Model) currentRunLabel() string {
	if m.followCurrent {
		if m.activeRun != "" {
			return "live " + m.activeRun
		}
		return "live"
	}
	if m.selectedRun == "" {
		return "live"
	}
	return m.runLabel(m.selectedRun)
}

func (m Model) runLabel(runID string) string {
	for _, run := range m.runs {
		if run.ID == runID {
			return fmt.Sprintf("%s · %s", run.ID, run.StartedAt.Format("15:04:05"))
		}
	}
	return runID
}

func (m *Model) applyLayout() {
	if !m.ready {
		return
	}
	m.logVP.Width = m.layout.LogViewportWidth
	m.logVP.Height = m.layout.LogViewportHeight
	m.listVP.Width = m.layout.ListViewportWidth
	m.listVP.Height = m.layout.ListViewportHeight
	m.help.Width = max(20, m.width-2)
}

func (m *Model) rebuildLogContent() {
	if !m.ready {
		return
	}

	m.updateProcessList()

	wasAtBottom := m.atBottom()
	lines := m.visibleLogLines()
	m.logVP.SetContent(strings.Join(lines, "\n"))
	if m.follow || wasAtBottom {
		m.logVP.GotoBottom()
		m.follow = true
	}
}

func (m *Model) updateProcessList() {
	lines := make([]string, 0, len(m.processes)+1)
	width := max(1, m.layout.ListViewportWidth)

	for index, status := range m.processes {
		name := truncate(SanitizeTerminalText(status.Name), max(1, width-14))
		state := GetStatusStyle(string(status.State)).Render("[" + string(status.State) + "]")
		prefix := "  "
		if index == m.selected {
			prefix = "▸ "
		}
		line := fmt.Sprintf("%s%s %s", prefix, name, state)
		line = truncate(line, width)
		if index == m.selected {
			line = selectedProcessStyle.Width(width).Render(line)
		} else {
			line = processItemStyle.Width(width).Render(line)
		}
		lines = append(lines, line)
	}

	if len(lines) == 0 {
		lines = append(lines, helpStyle.Render(" No processes configured"))
	}
	m.listVP.SetContent(strings.Join(lines, "\n"))
}

func (m Model) visibleLogLines() []string {
	view := m.views[m.viewIndex]
	selected := m.selectedName()
	allowed := m.allowedRuns(view)

	lines := make([]string, 0, len(m.logs)+4)
	lastRun := ""
	needle := strings.ToLower(m.filter)

	for _, entry := range m.logs {
		if !entryMatches(entry, view, selected, allowed) {
			continue
		}

		text, cleared := CleanTerminalLine(entryText(entry, view))
		if needle != "" && !strings.Contains(strings.ToLower(text), needle) {
			continue
		}

		if cleared {
			lines = append(lines, separatorStyle.Render("── screen cleared · server history kept ──"))
		}
		if (view == "stdout" || view == "stderr") && entry.RunID != "" && entry.RunID != lastRun {
			lines = append(lines, separatorStyle.Render("── "+m.runLabel(entry.RunID)+" ──"))
			lastRun = entry.RunID
		}
		if text == "" {
			continue
		}

		lines = append(lines, renderLogLine(entry, view, text))
	}

	if len(lines) == 0 {
		lines = append(lines, helpStyle.Render(" No logs in this view"))
	}
	if len(lines) > maxLogLines {
		lines = lines[len(lines)-maxLogLines:]
	}
	return lines
}

func (m Model) allowedRuns(view string) map[string]struct{} {
	if view != "stdout" && view != "stderr" && view != "lifecycle" {
		return nil
	}

	allowed := make(map[string]struct{})
	if m.followCurrent {
		if m.activeRun != "" {
			allowed[m.activeRun] = struct{}{}
		}
		if len(m.runs) > 1 {
			allowed[m.runs[1].ID] = struct{}{}
		}
	} else if m.selectedRun != "" {
		allowed[m.selectedRun] = struct{}{}
	}
	return allowed
}

func entryMatches(
	entry LogEntry,
	view string,
	selected string,
	allowed map[string]struct{},
) bool {
	switch view {
	case "proxy":
		return entry.Stream == "proxy"
	case "lifecycle":
		if entry.Process != selected || entry.Stream != "lifecycle" {
			return false
		}
	default:
		if entry.Process != selected || entry.Stream != view {
			return false
		}
	}

	if len(allowed) > 0 && entry.RunID != "" {
		if _, ok := allowed[entry.RunID]; !ok {
			return false
		}
	}
	return true
}

func entryText(entry LogEntry, view string) string {
	switch view {
	case "lifecycle":
		detail := entry.Error
		if detail == "" {
			detail = entry.Message
		}
		if detail == "" {
			return entry.Type
		}
		return entry.Type + " — " + detail
	default:
		return entry.Message
	}
}

func renderLogLine(entry LogEntry, view string, text string) string {
	prefix := fmt.Sprintf(
		"[%s] %s: ",
		entry.Timestamp.Format("15:04:05"),
		SanitizeTerminalText(entry.Process),
	)

	style := stdoutStyle
	switch view {
	case "stderr":
		style = stderrStyle
	case "lifecycle":
		style = lifecycleStyle
	case "proxy":
		style = proxyStyle
	}
	if entry.Error != "" {
		style = errorStyle
	}
	return style.Render(prefix + text)
}

func (m Model) atBottom() bool {
	total := m.logVP.TotalLineCount()
	height := m.logVP.Height
	return m.logVP.YOffset >= total-height
}

func truncate(text string, width int) string {
	if width <= 0 {
		return ""
	}
	runes := []rune(text)
	if len(runes) <= width {
		return text
	}
	if width == 1 {
		return string(runes[:1])
	}
	return string(runes[:width-1]) + "…"
}

func orDash(value string) string {
	if value == "" {
		return "—"
	}
	return value
}

// Run starts the TUI and bridges event-bus messages into the Bubble Tea update
// loop. Subscribing from the latest sequence avoids duplicating history that
// the model loads directly from the shared process manager.
func Run(cfg *config.Config, manager *process.Manager, bus *event.Bus) error {
	model := InitialModel(cfg, manager, bus)
	program := tea.NewProgram(model, tea.WithAltScreen())

	after := uint64(0)
	if bus != nil {
		after = bus.LatestSequence()
	}

	var subscriber event.Subscriber
	if bus != nil {
		subscriber = bus.SubscribeFilteredFrom(nil, after)
	}

	done := make(chan struct{})
	if subscriber != nil {
		go func() {
			defer close(done)
			for item := range subscriber {
				program.Send(eventMsg{Event: item})
			}
		}()
	} else {
		close(done)
	}

	_, err := program.Run()

	if bus != nil && subscriber != nil {
		bus.Unsubscribe(subscriber)
	}
	<-done

	return err
}
