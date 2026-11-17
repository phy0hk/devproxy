package tui

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/phy0hk/devproxy/internal/event"
	"github.com/phy0hk/devproxy/internal/logstore"
	"github.com/phy0hk/devproxy/internal/process"
)

func readyModel(t *testing.T) Model {
	t.Helper()
	model := InitialModel(nil, nil, nil)
	updated, _ := model.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	ready, ok := updated.(Model)
	if !ok {
		t.Fatalf("Update returned %T, want Model", updated)
	}
	return ready
}

func keyMsg(key tea.KeyType, runes ...rune) tea.KeyMsg {
	return tea.KeyMsg{Type: key, Runes: runes}
}

func runeKey(value rune) tea.KeyMsg {
	return keyMsg(tea.KeyRunes, value)
}

func typeKeys(model Model, text string) Model {
	for _, value := range text {
		updated, _ := model.Update(runeKey(value))
		model = updated.(Model)
	}
	return model
}

func TestModelStartsUnreadyAndLoadsOnResize(t *testing.T) {
	model := InitialModel(nil, nil, nil)
	if model.ready {
		t.Fatal("model should not be ready before the first window size")
	}
	if got := model.View(); !strings.Contains(got, "Loading") {
		t.Fatalf("initial View() = %q, want loading text", got)
	}

	updated, _ := model.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	ready := updated.(Model)
	if !ready.ready {
		t.Fatal("model should be ready after WindowSizeMsg")
	}
	if ready.layout.LogWidth == 0 || ready.layout.LogViewportHeight == 0 {
		t.Fatalf("layout not applied: %+v", ready.layout)
	}
}

func TestViewFitsTerminalWidth(t *testing.T) {
	sizes := []struct {
		width  int
		height int
	}{
		{80, 24},
		{100, 30},
		{120, 40},
	}

	for _, size := range sizes {
		updated, _ := InitialModel(nil, nil, nil).Update(
			tea.WindowSizeMsg{Width: size.width, Height: size.height},
		)
		model := updated.(Model)
		model.processes = []process.Status{
			{Name: "a-very-long-process-name-that-should-truncate", State: process.StateRunning},
			{Name: "api", State: process.StateConfigured},
		}
		model.logs = []LogEntry{{
			Process: "a-very-long-process-name-that-should-truncate",
			Stream:  "stdout",
			Message: strings.Repeat("long output ", 40),
		}}

		lines := strings.Split(model.View(), "\n")
		if len(lines) != size.height {
			t.Fatalf("%dx%d: View has %d lines, want %d", size.width, size.height, len(lines), size.height)
		}
		wantWidth := size.width - 1
		for index, line := range lines {
			if width := lipgloss.Width(line); width != wantWidth {
				t.Fatalf("%dx%d: line %d is %d cells wide, want %d: %q", size.width, size.height, index, width, wantWidth, line)
			}
		}
	}
}

func TestProcessListReplacesStatusRows(t *testing.T) {
	model := readyModel(t)
	model.processes = []process.Status{{Name: "noisy", State: process.StateRunning}}
	model.rebuildLogContent()
	if running := model.View(); !strings.Contains(running, "[running]") {
		t.Fatalf("running view = %q", running)
	}

	updated, _ := model.Update(statusMsg([]process.Status{{Name: "noisy", State: process.StateExited}}))
	model = updated.(Model)
	view := model.View()
	if !strings.Contains(view, "[exited]") {
		t.Fatalf("exited view = %q", view)
	}
	if strings.Contains(view, "[running]") {
		t.Fatalf("exited view still renders a running row: %q", view)
	}
}

func TestViewRendersProcessListAndLogs(t *testing.T) {
	model := readyModel(t)
	model.processes = []process.Status{{Name: "web", State: process.StateRunning}}
	model.logs = []LogEntry{{
		Process:   "web",
		Stream:    "stdout",
		Timestamp: time.Now(),
		Message:   "hello from the child",
	}}
	model.rebuildLogContent()

	view := model.View()
	for _, want := range []string{"web", "hello from the child"} {
		if !strings.Contains(view, want) {
			t.Fatalf("View() is missing %q:\n%s", want, view)
		}
	}
}

func TestModelTooSmallView(t *testing.T) {
	updated, _ := InitialModel(nil, nil, nil).Update(tea.WindowSizeMsg{Width: 50, Height: 12})
	model := updated.(Model)
	view := model.View()
	if !strings.Contains(view, "Terminal too small") {
		t.Fatalf("View() = %q, want too-small message", view)
	}
	if !strings.Contains(view, "Resize") {
		t.Fatalf("View() = %q, want resize hint", view)
	}
}

func TestQuitKeys(t *testing.T) {
	cases := []struct {
		name string
		key  tea.KeyMsg
	}{
		{"q", runeKey('q')},
		{"ctrl+c", keyMsg(tea.KeyCtrlC)},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			model := readyModel(t)
			updated, cmd := model.Update(testCase.key)
			if !updated.(Model).quitting {
				t.Fatal("model should be quitting")
			}
			if cmd == nil {
				t.Fatal("quit key should return a command")
			}
		})
	}
}

func TestSwitchViewCycles(t *testing.T) {
	model := readyModel(t)
	want := []int{1, 2, 3, 0, 1}

	for index, expected := range want {
		updated, _ := model.Update(runeKey('v'))
		model = updated.(Model)
		if model.viewIndex != expected {
			t.Fatalf("after %d presses viewIndex = %d, want %d", index+1, model.viewIndex, expected)
		}
	}
}

func TestFilterModeEditsAndExits(t *testing.T) {
	model := readyModel(t)

	updated, _ := model.Update(runeKey('/'))
	model = updated.(Model)
	if !model.filtering {
		t.Fatal("/ should enter filter mode")
	}
	if !strings.Contains(model.renderHelp(), "Type to filter") {
		t.Fatalf("filter help = %q", model.renderHelp())
	}

	model = typeKeys(model, "abc")
	if model.filter != "abc" {
		t.Fatalf("filter = %q, want abc", model.filter)
	}

	updated, _ = model.Update(keyMsg(tea.KeyBackspace))
	model = updated.(Model)
	if model.filter != "ab" {
		t.Fatalf("filter after backspace = %q, want ab", model.filter)
	}

	updated, _ = model.Update(keyMsg(tea.KeyCtrlU))
	model = updated.(Model)
	if model.filter != "" {
		t.Fatalf("filter after ctrl+u = %q, want empty", model.filter)
	}

	model = typeKeys(model, "x")
	updated, _ = model.Update(keyMsg(tea.KeyEnter))
	model = updated.(Model)
	if model.filtering {
		t.Fatal("enter should leave filter mode")
	}
	if model.filter != "x" {
		t.Fatalf("filter after enter = %q, want x", model.filter)
	}

	updated, _ = model.Update(runeKey('/'))
	model = updated.(Model)
	updated, _ = model.Update(keyMsg(tea.KeyEsc))
	model = updated.(Model)
	if model.filtering {
		t.Fatal("esc should leave filter mode")
	}
}

func TestProcessSelectionKeys(t *testing.T) {
	model := readyModel(t)
	model.processes = []process.Status{
		{Name: "web", State: process.StateRunning},
		{Name: "api", State: process.StateConfigured},
	}

	updated, _ := model.Update(runeKey('j'))
	model = updated.(Model)
	if model.selected != 1 {
		t.Fatalf("selected = %d, want 1", model.selected)
	}

	updated, _ = model.Update(runeKey('j'))
	model = updated.(Model)
	if model.selected != 1 {
		t.Fatalf("selected should stay clamped at 1, got %d", model.selected)
	}

	updated, _ = model.Update(runeKey('k'))
	model = updated.(Model)
	if model.selected != 0 {
		t.Fatalf("selected = %d, want 0", model.selected)
	}
}

func TestStatusMsgClampsSelection(t *testing.T) {
	model := readyModel(t)
	model.selected = 5

	updated, _ := model.Update(statusMsg([]process.Status{
		{Name: "web", State: process.StateRunning},
	}))
	model = updated.(Model)
	if model.selected != 0 {
		t.Fatalf("selected = %d, want 0", model.selected)
	}

	updated, _ = model.Update(statusMsg(nil))
	model = updated.(Model)
	if model.selected != 0 {
		t.Fatalf("selected = %d after empty statuses, want 0", model.selected)
	}
}

func TestProcessOutputAppendsAndDeduplicates(t *testing.T) {
	model := readyModel(t)
	model.processes = []process.Status{{Name: "web", State: process.StateRunning}}
	now := time.Now()

	message := func(sequence uint64, text string) eventMsg {
		return eventMsg{Event: event.ProcessEvent{
			EventType: event.TypeProcessOutput,
			Timestamp: now,
			Process:   "web",
			RunID:     "run-1",
			Sequence:  sequence,
			Stream:    "stdout",
			Message:   text,
		}}
	}

	updated, _ := model.Update(message(1, "first"))
	model = updated.(Model)
	updated, _ = model.Update(message(1, "duplicate"))
	model = updated.(Model)
	updated, _ = model.Update(message(2, "second"))
	model = updated.(Model)

	if len(model.logs) != 2 {
		t.Fatalf("logs = %d, want 2 (duplicate ignored)", len(model.logs))
	}
	view := strings.Join(model.visibleLogLines(), "\n")
	if !strings.Contains(view, "first") || !strings.Contains(view, "second") {
		t.Fatalf("visible log = %q", view)
	}
	if strings.Count(view, "first") != 1 {
		t.Fatalf("duplicate rendered: %q", view)
	}
	if !strings.Contains(view, "run-1") {
		t.Fatalf("run separator missing: %q", view)
	}
}

func TestProcessOutputIgnoresOtherProcessAndRuns(t *testing.T) {
	model := readyModel(t)
	model.processes = []process.Status{{Name: "web", State: process.StateRunning}}
	model.activeRun = "run-2"
	now := time.Now()

	otherProcess := eventMsg{Event: event.ProcessEvent{
		EventType: event.TypeProcessOutput,
		Timestamp: now,
		Process:   "api",
		RunID:     "run-2",
		Sequence:  1,
		Stream:    "stdout",
		Message:   "api output",
	}}
	otherRun := eventMsg{Event: event.ProcessEvent{
		EventType: event.TypeProcessOutput,
		Timestamp: now,
		Process:   "web",
		RunID:     "run-1",
		Sequence:  2,
		Stream:    "stdout",
		Message:   "old output",
	}}

	updated, _ := model.Update(otherProcess)
	model = updated.(Model)
	updated, _ = model.Update(otherRun)
	model = updated.(Model)

	if len(model.logs) != 0 {
		t.Fatalf("logs = %#v, want none", model.logs)
	}
}

func TestViewFilteringAndAnsiSanitising(t *testing.T) {
	model := readyModel(t)
	model.processes = []process.Status{{Name: "web", State: process.StateRunning}}
	model.logs = []LogEntry{
		{Process: "web", Stream: "stdout", Message: "\x1b[31malpha\x1b[0m"},
		{Process: "web", Stream: "stdout", Message: "beta"},
		{Process: "web", Stream: "stderr", Message: "stderr line"},
	}

	model.viewIndex = 0
	view := strings.Join(model.visibleLogLines(), "\n")
	if !strings.Contains(view, "alpha") || !strings.Contains(view, "beta") {
		t.Fatalf("stdout view = %q", view)
	}
	if strings.Contains(view, "stderr line") {
		t.Fatalf("stdout view leaked stderr: %q", view)
	}
	if strings.ContainsRune(view, '\x1b') {
		t.Fatalf("visible log contains raw escape bytes: %q", view)
	}

	model.filter = "beta"
	filtered := strings.Join(model.visibleLogLines(), "\n")
	if strings.Contains(filtered, "alpha") || !strings.Contains(filtered, "beta") {
		t.Fatalf("filtered view = %q", filtered)
	}

	model.filter = ""
	model.viewIndex = 1
	stderr := strings.Join(model.visibleLogLines(), "\n")
	if !strings.Contains(stderr, "stderr line") || strings.Contains(stderr, "alpha") {
		t.Fatalf("stderr view = %q", stderr)
	}
}

func TestScreenClearRendersSeparator(t *testing.T) {
	model := readyModel(t)
	model.processes = []process.Status{{Name: "web", State: process.StateRunning}}
	model.logs = []LogEntry{
		{Process: "web", Stream: "stdout", Message: "before\x1b[2Jafter"},
	}

	view := strings.Join(model.visibleLogLines(), "\n")
	if !strings.Contains(view, "screen cleared") {
		t.Fatalf("clear separator missing: %q", view)
	}
	if strings.ContainsRune(view, '\x1b') {
		t.Fatalf("visible log contains raw escape bytes: %q", view)
	}
}

func TestClearCurrentViewOnlyClearsSelectedStream(t *testing.T) {
	model := readyModel(t)
	model.processes = []process.Status{{Name: "web", State: process.StateRunning}}
	model.logs = []LogEntry{
		{Process: "web", Stream: "stdout", Message: "out"},
		{Process: "web", Stream: "stderr", Message: "err"},
		{Process: "proxy", Stream: "proxy", Message: "request"},
	}
	model.viewIndex = 0

	model.clearCurrentView()
	if len(model.logs) != 2 {
		t.Fatalf("logs = %d, want 2", len(model.logs))
	}
	for _, entry := range model.logs {
		if entry.Stream == "stdout" && entry.Process == "web" {
			t.Fatalf("stdout entry survived clear: %#v", entry)
		}
	}
}

func TestApplyHistoryMergesRunsAndOrdering(t *testing.T) {
	model := readyModel(t)
	model.processes = []process.Status{{Name: "web", State: process.StateRunning}}
	now := time.Now()

	previousAt := now.Add(-time.Minute)
	currentAt := now
	model.logs = []LogEntry{
		{
			Process:   "web",
			RunID:     "run-2",
			Sequence:  10,
			Timestamp: currentAt,
			Stream:    "stdout",
			Message:   "live line",
		},
	}

	model.applyHistory(historyMsg{
		Token:   model.historyToken,
		Process: "web",
		Follow:  true,
		Runs: []logstore.Run{
			{ID: "run-2", StartedAt: currentAt},
			{ID: "run-1", StartedAt: previousAt},
		},
		ActiveRun: "run-2",
		Lines: []logstore.Line{
			{
				Process:   "web",
				RunID:     "run-1",
				Sequence:  3,
				Timestamp: previousAt,
				Stream:    "stdout",
				Message:   "previous line",
			},
			{
				Process:   "web",
				RunID:     "run-2",
				Sequence:  10,
				Timestamp: currentAt,
				Stream:    "stdout",
				Message:   "live line",
			},
		},
	})

	if len(model.logs) != 2 {
		t.Fatalf("logs = %d, want 2 (history/live deduplicated): %#v", len(model.logs), model.logs)
	}
	if model.logs[0].Message != "previous line" || model.logs[1].Message != "live line" {
		t.Fatalf("ordering = %#v", model.logs)
	}
	if model.activeRun != "run-2" {
		t.Fatalf("active run = %q, want run-2", model.activeRun)
	}

	updated, _ := model.Update(eventMsg{Event: event.ProcessEvent{
		EventType: event.TypeProcessOutput,
		Timestamp: currentAt,
		Process:   "web",
		RunID:     "run-2",
		Sequence:  3,
		Stream:    "stdout",
		Message:   "stale",
	}})
	model = updated.(Model)
	if len(model.logs) != 2 {
		t.Fatalf("stale sequence appended: %#v", model.logs)
	}
}

func TestChangeRunTogglesFollowMode(t *testing.T) {
	model := readyModel(t)
	model.manager = process.NewManager(nil, nil)
	model.processes = []process.Status{{Name: "web", State: process.StateRunning}}
	model.runs = []logstore.Run{
		{ID: "run-3", StartedAt: time.Now()},
		{ID: "run-2", StartedAt: time.Now().Add(-time.Minute)},
		{ID: "run-1", StartedAt: time.Now().Add(-2 * time.Minute)},
	}

	updated, cmd := model.changeRun(-1)
	model = updated.(Model)
	if model.followCurrent || model.selectedRun != "run-2" {
		t.Fatalf("older run state = followCurrent:%v selected:%q", model.followCurrent, model.selectedRun)
	}
	if cmd == nil {
		t.Fatal("run switch should request history")
	}

	updated, _ = model.changeRun(-1)
	model = updated.(Model)
	if model.selectedRun != "run-1" {
		t.Fatalf("selected run = %q, want run-1", model.selectedRun)
	}

	updated, _ = model.changeRun(1)
	model = updated.(Model)
	if model.selectedRun != "run-2" {
		t.Fatalf("selected run = %q, want run-2", model.selectedRun)
	}

	updated, _ = model.changeRun(1)
	model = updated.(Model)
	if !model.followCurrent || model.selectedRun != "" {
		t.Fatalf("newer run should return to live: followCurrent=%v selected=%q", model.followCurrent, model.selectedRun)
	}
}

func TestErrorMsgStored(t *testing.T) {
	model := readyModel(t)
	updated, _ := model.Update(errorString("boom"))
	model = updated.(Model)
	if model.err == nil || model.err.Error() != "boom" {
		t.Fatalf("err = %v, want boom", model.err)
	}
}

type errorString string

func (e errorString) Error() string { return string(e) }
