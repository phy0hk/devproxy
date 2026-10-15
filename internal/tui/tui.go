package tui

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/phy0hk/devproxy/internal/config"
	"github.com/phy0hk/devproxy/internal/event"
	"github.com/phy0hk/devproxy/internal/process"
)

// LogEntry represents a single log line in the TUI
type LogEntry struct {
	Timestamp time.Time
	Process   string
	Stream    string // stdout, stderr, lifecycle, proxy
	Message   string
	Type      string // for lifecycle events
	Error     string
}

// Model is the main TUI model
type Model struct {
	// Configuration
	config *config.Config

	// Process management
	processManager *process.Manager
	eventBus       *event.Bus

	// UI state
	keys      KeyMap
	help      help.Model
	width     int
	height    int
	ready     bool
	listWidth int
	logWidth  int

	// Process list
	processes     []process.Status
	selectedIndex int
	processListVP viewport.Model

	// Log panel
	logs        []LogEntry
	logVP       viewport.Model
	currentView string // stdout, stderr, lifecycle, proxy
	views       []string
	atBottom    bool // track if user is at bottom for auto-scroll

	// State
	showHelp   bool
	err        error
	quitting   bool
	statusMsg  string
	statusTime time.Time
	mu         *sync.Mutex

	// Mouse support
	mouseEnabled bool
}

// InitialModel creates a new TUI model
func InitialModel(cfg *config.Config, pm *process.Manager, bus *event.Bus) Model {
	m := Model{
		config:         cfg,
		processManager: pm,
		eventBus:       bus,
		keys:           DefaultKeyMap,
		help:           help.New(),
		processes:      []process.Status{},
		selectedIndex:  0,
		logs:           []LogEntry{},
		currentView:    "stdout",
		views:          []string{"stdout", "stderr", "lifecycle", "proxy"},
		mouseEnabled:   true,
	}

	m.help.ShowAll = false
	m.help.Styles.ShortKey = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	m.help.Styles.ShortDesc = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	m.help.Styles.FullKey = lipgloss.NewStyle().Foreground(lipgloss.Color("229"))
	m.help.Styles.FullDesc = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	m.help.Styles.FullSeparator = lipgloss.NewStyle().Foreground(lipgloss.Color("62"))

	return m
}

// Init initializes the TUI
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.tick(),
		m.listenEvents(),
		tea.EnableMouseCellMotion,
		tea.EnterAltScreen,
	)
}

// tick returns a command that ticks every second for status updates
func (m Model) tick() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return TickMsg(t)
	})
}

// TickMsg is sent on each tick
type TickMsg time.Time

// listenEvents subscribes to the event bus
func (m Model) listenEvents() tea.Cmd {
	return func() tea.Msg {
		ch := m.eventBus.Subscribe()
		for evt := range ch {
			switch e := evt.(type) {
			case event.ProcessEvent:
				m.handleProcessEvent(e)
			case event.RequestEvent:
				m.handleRequestEvent(e)
			}
		}
		return nil
	}
}

// handleProcessEvent processes process events
func (m *Model) handleProcessEvent(evt event.ProcessEvent) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Update process statuses
	m.processes = m.processManager.Statuses()

	// Add to logs
	logEntry := LogEntry{
		Timestamp: evt.Timestamp,
		Process:   evt.Process,
		Stream:    evt.Stream,
		Message:   evt.Message,
		Type:      string(evt.Type()),
		Error:     evt.Error,
	}
	m.logs = append(m.logs, logEntry)

	// Keep only last 2000 logs
	if len(m.logs) > 2000 {
		m.logs = m.logs[len(m.logs)-2000:]
	}

	// Update status message
	switch evt.Type() {
	case event.TypeProcessStarted:
		m.statusMsg = fmt.Sprintf("Process %q started", evt.Process)
	case event.TypeProcessStopping:
		m.statusMsg = fmt.Sprintf("Process %q stopping...", evt.Process)
	case event.TypeProcessExited:
		m.statusMsg = fmt.Sprintf("Process %q exited with code %d", evt.Process, evt.ExitCode)
	case event.TypeProcessFailed:
		m.statusMsg = fmt.Sprintf("Process %q failed: %s", evt.Process, evt.Error)
	}
	m.statusTime = time.Now()
}

// handleRequestEvent processes proxy request events
func (m *Model) handleRequestEvent(evt event.RequestEvent) {
	m.mu.Lock()
	defer m.mu.Unlock()

	logEntry := LogEntry{
		Timestamp: evt.Timestamp,
		Process:   "proxy",
		Stream:    "proxy",
		Message:   fmt.Sprintf("%s %s → %s %d %dms", evt.Method, evt.Path, evt.Upstream, evt.Status, evt.DurationMS),
	}
	m.logs = append(m.logs, logEntry)

	if len(m.logs) > 2000 {
		m.logs = m.logs[len(m.logs)-2000:]
	}
}

// Update handles messages and updates the model
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true
		m.recalculateLayout()
		cmds = append(cmds, m.updateViewports())

	case TickMsg:
		m.processes = m.processManager.Statuses()
		// Clamp selected index
		if m.selectedIndex >= len(m.processes) && len(m.processes) > 0 {
			m.selectedIndex = len(m.processes) - 1
		}
		cmds = append(cmds, m.tick(), m.updateViewports())

	case tea.KeyMsg:
		return m.handleKeyMsg(msg)

	case tea.MouseMsg:
		if m.mouseEnabled {
			return m.handleMouseMsg(msg)
		}

	case error:
		m.err = msg
	}

	return m, tea.Batch(cmds...)
}

// recalculateLayout calculates panel dimensions based on current window size
func (m *Model) recalculateLayout() {
	if !m.ready {
		return
	}

	// Layout: header(1) + process list + status bar(1) + help(1 or more) = height
	// Process list and log panel share the middle area
	// Reserve space for: header(1), status bar(1), help(1 min), borders(2)
	reservedHeight := 5
	if m.showHelp {
		// Estimate help height based on number of key binding groups
		reservedHeight += 8
	}

	listHeight := m.height - reservedHeight
	logHeight := m.height - reservedHeight

	// List takes 1/3, logs take 2/3
	m.listWidth = m.width / 3
	if m.listWidth < 30 {
		m.listWidth = 30
	}
	if m.listWidth > m.width-40 {
		m.listWidth = m.width - 40
	}
	m.logWidth = m.width - m.listWidth - 2 // -2 for gap

	m.processListVP = viewport.New(m.listWidth-2, listHeight) // -2 for border padding
	m.logVP = viewport.New(m.logWidth-2, logHeight)           // -2 for border padding
}

// updateViewports updates the viewport content
func (m *Model) updateViewports() tea.Cmd {
	m.updateProcessList()
	m.updateLogPanel()
	return nil
}

// handleKeyMsg handles keyboard input
func (m Model) handleKeyMsg(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Quit):
		m.quitting = true
		return m, tea.Quit

	case key.Matches(msg, m.keys.Help):
		m.showHelp = !m.showHelp
		m.help.ShowAll = m.showHelp
		m.recalculateLayout()
		return m, m.updateViewports()

	case key.Matches(msg, m.keys.Up):
		if m.selectedIndex > 0 {
			m.selectedIndex--
		}

	case key.Matches(msg, m.keys.Down):
		if m.selectedIndex < len(m.processes)-1 {
			m.selectedIndex++
		}

	case key.Matches(msg, m.keys.Left), key.Matches(msg, m.keys.ShiftTab):
		m.prevView()

	case key.Matches(msg, m.keys.Right), key.Matches(msg, m.keys.Tab):
		m.nextView()

	case key.Matches(msg, m.keys.SwitchView):
		m.cycleView()

	case key.Matches(msg, m.keys.Start):
		if len(m.processes) > 0 && m.selectedIndex < len(m.processes) {
			name := m.processes[m.selectedIndex].Name
			if m.processes[m.selectedIndex].State != "running" {
				go func() {
					if err := m.processManager.Start(context.Background(), name); err != nil {
						m.mu.Lock()
						m.statusMsg = fmt.Sprintf("Failed to start %q: %v", name, err)
						m.mu.Unlock()
					}
				}()
			}
		}

	case key.Matches(msg, m.keys.Stop):
		if len(m.processes) > 0 && m.selectedIndex < len(m.processes) {
			name := m.processes[m.selectedIndex].Name
			if m.processes[m.selectedIndex].State == "running" {
				go func() {
					if err := m.processManager.Stop(context.Background(), name); err != nil {
						m.mu.Lock()
						m.statusMsg = fmt.Sprintf("Failed to stop %q: %v", name, err)
						m.mu.Unlock()
					}
				}()
			}
		}

	case key.Matches(msg, m.keys.Restart):
		if len(m.processes) > 0 && m.selectedIndex < len(m.processes) {
			name := m.processes[m.selectedIndex].Name
			go func() {
				if err := m.processManager.Restart(context.Background(), name); err != nil {
					m.mu.Lock()
					m.statusMsg = fmt.Sprintf("Failed to restart %q: %v", name, err)
					m.mu.Unlock()
				}
			}()
		}

	case key.Matches(msg, m.keys.Refresh):
		m.processes = m.processManager.Statuses()
		m.statusMsg = "Refreshed"
		m.statusTime = time.Now()

	case key.Matches(msg, m.keys.Clear):
		m.mu.Lock()
		m.logs = []LogEntry{}
		m.mu.Unlock()
		m.statusMsg = "Logs cleared"
		m.statusTime = time.Now()

	case key.Matches(msg, m.keys.ScrollUp):
		m.logVP.LineUp(5)

	case key.Matches(msg, m.keys.ScrollDown):
		m.logVP.LineDown(5)

	case key.Matches(msg, m.keys.PageUp):
		m.logVP.PageUp()

	case key.Matches(msg, m.keys.PageDown):
		m.logVP.PageDown()

	case key.Matches(msg, m.keys.Home):
		m.logVP.GotoTop()

	case key.Matches(msg, m.keys.End):
		m.logVP.GotoBottom()
		m.atBottom = true
	}

	return m, m.updateViewports()
}

// handleMouseMsg handles mouse events
func (m Model) handleMouseMsg(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.MouseLeft:
		// Check if click is in process list area (left panel)
		// Header is at y=0, process list starts at y=1
		if msg.X < m.listWidth && msg.Y >= 1 && msg.Y < m.height-2 {
			// Click in process list - account for header and viewport offset
			index := msg.Y - 1 + m.processListVP.YOffset
			if index >= 0 && index < len(m.processes) {
				m.selectedIndex = index
			}
		} else if msg.X >= m.listWidth+1 && msg.Y == 1 {
			// Click on tab bar (y=1, right panel)
			tabX := m.listWidth + 2 // account for gap and border
			for _, view := range m.views {
				tabWidth := len(view) + 4 // padding
				if msg.X >= tabX && msg.X < tabX+tabWidth {
					m.currentView = view
					break
				}
				tabX += tabWidth + 1
			}
		}

	case tea.MouseWheelUp:
		if msg.X >= m.listWidth+1 {
			m.logVP.LineUp(3)
			m.atBottom = false
		} else {
			m.processListVP.LineUp(3)
		}

	case tea.MouseWheelDown:
		if msg.X >= m.listWidth+1 {
			m.logVP.LineDown(3)
			// Check if we're at bottom after scrolling
			if m.logVP.YOffset >= m.logVP.TotalLineCount()-m.logVP.Height {
				m.atBottom = true
			}
		} else {
			m.processListVP.LineDown(3)
		}
	}

	return m, m.updateViewports()
}

// prevView switches to the previous view
func (m *Model) prevView() {
	for i, v := range m.views {
		if v == m.currentView {
			if i > 0 {
				m.currentView = m.views[i-1]
			} else {
				m.currentView = m.views[len(m.views)-1]
			}
			break
		}
	}
}

// nextView switches to the next view
func (m *Model) nextView() {
	for i, v := range m.views {
		if v == m.currentView {
			if i < len(m.views)-1 {
				m.currentView = m.views[i+1]
			} else {
				m.currentView = m.views[0]
			}
			break
		}
	}
}

// cycleView cycles through views
func (m *Model) cycleView() {
	m.nextView()
}

// updateProcessList updates the process list viewport
func (m *Model) updateProcessList() {
	if !m.ready {
		return
	}

	var lines []string

	// Header
	header := titleStyle.Render(" Instances ")
	lines = append(lines, header)

	for i, p := range m.processes {
		statusStyle := GetStatusStyle(string(p.State))
		status := statusStyle.Render(fmt.Sprintf(" [%s] ", p.State))

		pid := ""
		if p.PID > 0 {
			pid = fmt.Sprintf(" (pid: %d)", p.PID)
		}

		line := fmt.Sprintf(" %s%s%s", p.Name, pid, status)

		if i == m.selectedIndex {
			line = selectedProcessStyle.Width(m.processListVP.Width - 2).Render(line)
		} else {
			line = processItemStyle.Width(m.processListVP.Width - 2).Render(line)
		}

		lines = append(lines, line)
	}

	if len(m.processes) == 0 {
		lines = append(lines, " No processes configured")
	}

	m.processListVP.SetContent(strings.Join(lines, "\n"))
}

// updateLogPanel updates the log panel viewport
func (m *Model) updateLogPanel() {
	if !m.ready {
		return
	}

	m.mu.Lock()
	logs := make([]LogEntry, len(m.logs))
	copy(logs, m.logs)
	m.mu.Unlock()

	// Filter logs by current view and selected process
	var filtered []LogEntry
	selectedProcess := ""
	if m.selectedIndex < len(m.processes) {
		selectedProcess = m.processes[m.selectedIndex].Name
	}

	for _, log := range logs {
		switch m.currentView {
		case "stdout":
			if log.Stream == "stdout" && (selectedProcess == "" || log.Process == selectedProcess) {
				filtered = append(filtered, log)
			}
		case "stderr":
			if log.Stream == "stderr" && (selectedProcess == "" || log.Process == selectedProcess) {
				filtered = append(filtered, log)
			}
		case "lifecycle":
			if log.Stream == "lifecycle" && (selectedProcess == "" || log.Process == selectedProcess) {
				filtered = append(filtered, log)
			}
		case "proxy":
			if log.Stream == "proxy" {
				filtered = append(filtered, log)
			}
		}
	}

	// Keep last 1000 lines
	if len(filtered) > 1000 {
		filtered = filtered[len(filtered)-1000:]
	}

	var lines []string

	// Log lines only (tabs are rendered separately in View())
	for _, log := range filtered {
		timeStr := log.Timestamp.Format("15:04:05")
		var style lipgloss.Style

		switch m.currentView {
		case "stdout":
			style = stdoutStyle
		case "stderr":
			style = stderrStyle
		case "lifecycle":
			style = lifecycleStyle
		case "proxy":
			style = proxyStyle
		}

		msg := log.Message
		if log.Error != "" {
			msg = log.Error
			style = errorStyle
		}

		line := fmt.Sprintf("[%s] %s: %s", timeStr, log.Process, msg)
		lines = append(lines, style.Render(line))
	}

	if len(lines) == 0 {
		lines = append(lines, " No logs in this view")
	}

	m.logVP.SetContent(strings.Join(lines, "\n"))

	// Auto-scroll to bottom only if user was already at bottom
	if m.atBottom {
		m.logVP.GotoBottom()
	}
}

// View renders the TUI
func (m Model) View() string {
	if !m.ready {
		return "Loading..."
	}

	if m.quitting {
		return "Goodbye!\n"
	}

	if m.err != nil {
		return errorStyle.Render(fmt.Sprintf("Error: %v", m.err))
	}

	// Build tab bar
	var tabs []string
	for _, v := range m.views {
		if v == m.currentView {
			tabs = append(tabs, activeTabStyle.Render(" "+v+" "))
		} else {
			tabs = append(tabs, tabStyle.Render(" "+v+" "))
		}
	}
	tabBar := strings.Join(tabs, " ")

	// Process list panel (left)
	listPanel := processListStyle.Width(m.listWidth).Height(m.height - 3).Render(m.processListVP.View())

	// Log panel (right) - tab bar + content
	logContent := lipgloss.JoinVertical(lipgloss.Left, tabBar, "", m.logVP.View())
	logPanel := logPanelStyle.Width(m.logWidth).Height(m.height - 3).Render(logContent)

	// Main content
	mainContent := lipgloss.JoinHorizontal(lipgloss.Top, listPanel, "  ", logPanel)

	// Header
	header := headerStyle.Width(m.width).Render(" DevProxy TUI ")

	// Status bar
	statusBar := m.renderStatusBar()

	// Help
	helpView := ""
	if m.showHelp {
		helpView = m.help.View(m.keys)
	} else {
		helpView = helpStyle.Render(m.help.ShortHelpView(m.keys.ShortHelp()))
	}

	return lipgloss.JoinVertical(lipgloss.Left, header, mainContent, statusBar, helpView)
}

// renderStatusBar renders the status bar
func (m Model) renderStatusBar() string {
	var parts []string

	// Process count
	running := 0
	for _, p := range m.processes {
		if p.State == "running" {
			running++
		}
	}
	parts = append(parts, fmt.Sprintf(" Processes: %d managed, %d running ", len(m.processes), running))

	// Selected process
	if m.selectedIndex < len(m.processes) {
		p := m.processes[m.selectedIndex]
		parts = append(parts, fmt.Sprintf(" Selected: %s [%s] ", p.Name, p.State))
	}

	// Current view
	parts = append(parts, fmt.Sprintf(" View: %s ", m.currentView))

	// Mouse indicator
	if m.mouseEnabled {
		parts = append(parts, " Mouse: ON ")
	}

	// Status message
	if m.statusMsg != "" && time.Since(m.statusTime) < 5*time.Second {
		parts = append(parts, fmt.Sprintf(" %s ", m.statusMsg))
	}

	statusText := strings.Join(parts, "│")
	return logPanelStyle.Width(m.width).Render(statusText)
}

// Run starts the TUI
func Run(cfg *config.Config, pm *process.Manager, bus *event.Bus) error {
	m := InitialModel(cfg, pm, bus)
	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())
	_, err := p.Run()
	return err
}
