package process

import (
	"context"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/phy0hk/devproxy/internal/config"
	"github.com/phy0hk/devproxy/internal/event"
)

func TestManagerInitialStatuses(t *testing.T) {
	manager := NewManager([]config.ProcessConfig{
		{
			Name:       "frontend",
			Command:    "pnpm dev",
			WorkingDir: "./frontend",
		},
		{
			Name:    "backend",
			Command: "pnpm start:dev",
		},
	}, nil)

	statuses := manager.Statuses()
	if len(statuses) != 2 {
		t.Fatalf("got %d statuses, want 2", len(statuses))
	}

	if statuses[0].Name != "backend" {
		t.Fatalf("first status name = %q, want backend", statuses[0].Name)
	}

	if statuses[0].State != StateConfigured {
		t.Fatalf("backend state = %q, want %q", statuses[0].State, StateConfigured)
	}

	if statuses[1].Name != "frontend" {
		t.Fatalf("second status name = %q, want frontend", statuses[1].Name)
	}

	if statuses[1].WorkingDir != "./frontend" {
		t.Fatalf("frontend working dir = %q, want ./frontend", statuses[1].WorkingDir)
	}
}

func TestManagerStopUpdatesStatus(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses POSIX sleep command")
	}

	manager := NewManager([]config.ProcessConfig{
		{
			Name:    "worker",
			Command: "sleep 10",
		},
	}, nil)

	if err := manager.Start(context.Background(), "worker"); err != nil {
		t.Fatalf("start process: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if err := manager.Stop(ctx, "worker"); err != nil {
		t.Fatalf("stop process: %v", err)
	}

	status := manager.Statuses()[0]
	if status.State != StateExited {
		t.Fatalf("state = %q, want %q", status.State, StateExited)
	}

	if status.ExitCode == nil {
		t.Fatal("expected exit code to be set")
	}
}

func TestManagerStartOutlivesCallerContext(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses POSIX sleep command")
	}

	manager := NewManager([]config.ProcessConfig{{Name: "worker", Command: "sleep 1"}}, nil)
	ctx, cancel := context.WithCancel(context.Background())
	if err := manager.Start(ctx, "worker"); err != nil {
		t.Fatalf("start process: %v", err)
	}
	cancel()

	time.Sleep(50 * time.Millisecond)
	if status := manager.Statuses()[0]; status.State != StateRunning {
		t.Fatalf("state after caller cancellation = %q, want %q", status.State, StateRunning)
	}
	if err := manager.StopAll(context.Background()); err != nil {
		t.Fatalf("stop process: %v", err)
	}
}

func TestManagerRestartAfterNaturalExit(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses POSIX shell command")
	}

	manager := NewManager([]config.ProcessConfig{{Name: "worker", Command: "exit 0"}}, nil)
	if err := manager.Start(context.Background(), "worker"); err != nil {
		t.Fatalf("start process: %v", err)
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if manager.Statuses()[0].State == StateExited {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if status := manager.Statuses()[0]; status.State != StateExited {
		t.Fatalf("state after natural exit = %q, want %q", status.State, StateExited)
	}
	if err := manager.Restart(context.Background(), "worker"); err != nil {
		t.Fatalf("restart after natural exit: %v", err)
	}
	defer manager.StopAll(context.Background())
	if status := manager.Statuses()[0]; status.State != StateRunning {
		t.Fatalf("state after restart = %q, want %q", status.State, StateRunning)
	}
}

func TestManagerRestartStartsRunningProcessAgain(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses POSIX sleep command")
	}

	manager := NewManager([]config.ProcessConfig{
		{
			Name:    "worker",
			Command: "sleep 10",
		},
	}, nil)

	if err := manager.Start(context.Background(), "worker"); err != nil {
		t.Fatalf("start process: %v", err)
	}

	firstPID := manager.Statuses()[0].PID

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if err := manager.Restart(ctx, "worker"); err != nil {
		t.Fatalf("restart process: %v", err)
	}
	defer manager.StopAll(context.Background())

	status := manager.Statuses()[0]
	if status.State != StateRunning {
		t.Fatalf("state = %q, want %q", status.State, StateRunning)
	}

	if status.PID == 0 {
		t.Fatal("expected restarted process pid")
	}

	if status.PID == firstPID {
		t.Fatalf("pid = %d, want a new pid", status.PID)
	}
}

func TestManagerRestartCycleKeepsStatusConsistent(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses POSIX shell command")
	}

	manager := NewManager([]config.ProcessConfig{{Name: "worker", Command: "sleep 5"}}, nil)
	if err := manager.Start(context.Background(), "worker"); err != nil {
		t.Fatalf("start process: %v", err)
	}
	firstPID := manager.Statuses()[0].PID

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := manager.Restart(ctx, "worker"); err != nil {
		t.Fatalf("restart process: %v", err)
	}
	defer manager.StopAll(context.Background())

	status := manager.Statuses()[0]
	if status.State != StateRunning {
		t.Fatalf("state = %q, want %q", status.State, StateRunning)
	}
	if status.PID == 0 || status.PID == firstPID {
		t.Fatalf("pid = %d, want a new pid different from %d", status.PID, firstPID)
	}
	if status.ExitedAt != nil {
		t.Fatalf("exited_at = %v, want nil for a running process", status.ExitedAt)
	}
	if status.LastError != "" {
		t.Fatalf("last_error = %q, want empty for a running process", status.LastError)
	}
}

func TestManagerStartFailureUpdatesStatus(t *testing.T) {
	manager := NewManager([]config.ProcessConfig{
		{
			Name:       "missing-dir",
			Command:    "echo test",
			WorkingDir: "/devproxy/this/directory/does/not/exist",
		},
	}, nil)

	err := manager.Start(context.Background(), "missing-dir")
	if err == nil {
		t.Fatal("expected start error")
	}

	statuses := manager.Statuses()
	if len(statuses) != 1 {
		t.Fatalf("got %d statuses, want 1", len(statuses))
	}

	status := statuses[0]
	if status.State != StateFailed {
		t.Fatalf("state = %q, want %q", status.State, StateFailed)
	}

	if status.LastError == "" {
		t.Fatal("expected last error to be set")
	}

	if !strings.Contains(err.Error(), "start process") {
		t.Fatalf("error = %q, want start process context", err.Error())
	}
}

func TestManagerRunIDsChangeAcrossRestart(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses POSIX shell command")
	}

	manager := NewManager([]config.ProcessConfig{{
		Name:    "worker",
		Command: "echo first; sleep 10",
	}}, nil)

	if err := manager.Start(context.Background(), "worker"); err != nil {
		t.Fatalf("start process: %v", err)
	}

	waitForLogLine(t, manager, "worker", "first")
	firstRunID := manager.Statuses()[0].RunID
	if firstRunID == "" {
		t.Fatal("expected first run ID")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := manager.Restart(ctx, "worker"); err != nil {
		t.Fatalf("restart process: %v", err)
	}
	defer manager.StopAll(context.Background())

	secondRunID := manager.Statuses()[0].RunID
	if secondRunID == "" || secondRunID == firstRunID {
		t.Fatalf("second run ID = %q, want a new ID different from %q", secondRunID, firstRunID)
	}

	previous, err := manager.Logs("worker", "previous", 10, 0)
	if err != nil {
		t.Fatalf("previous logs: %v", err)
	}
	if previous.Run != firstRunID {
		t.Fatalf("previous run = %q, want %q", previous.Run, firstRunID)
	}
	if len(previous.Lines) == 0 || previous.Lines[0].Message != "first" {
		t.Fatalf("previous lines = %v, want first output line", previous.Lines)
	}
	if previous.Lines[0].RunID != firstRunID {
		t.Fatalf("line run ID = %q, want %q", previous.Lines[0].RunID, firstRunID)
	}
}

func TestManagerKeepsStdoutAndStderrDistinct(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses POSIX shell command")
	}

	manager := NewManager([]config.ProcessConfig{{
		Name:    "worker",
		Command: "echo out; echo err >&2",
	}}, nil)

	if err := manager.Start(context.Background(), "worker"); err != nil {
		t.Fatalf("start process: %v", err)
	}
	defer manager.StopAll(context.Background())

	waitForLogCount(t, manager, "worker", 2)
	result, err := manager.Logs("worker", "current", 10, 0)
	if err != nil {
		t.Fatalf("logs: %v", err)
	}

	streams := map[string]string{}
	for _, line := range result.Lines {
		streams[line.Message] = line.Stream
	}
	if streams["out"] != "stdout" {
		t.Fatalf("out stream = %q, want stdout", streams["out"])
	}
	if streams["err"] != "stderr" {
		t.Fatalf("err stream = %q, want stderr", streams["err"])
	}
}

func TestManagerOutputSequencesIncrease(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses POSIX shell command")
	}

	manager := NewManager([]config.ProcessConfig{{
		Name:    "worker",
		Command: "echo one; echo two; echo three; echo four",
	}}, nil)

	if err := manager.Start(context.Background(), "worker"); err != nil {
		t.Fatalf("start process: %v", err)
	}
	defer manager.StopAll(context.Background())

	waitForLogCount(t, manager, "worker", 4)
	result, err := manager.Logs("worker", "current", 10, 0)
	if err != nil {
		t.Fatalf("logs: %v", err)
	}

	for i := 1; i < len(result.Lines); i++ {
		if result.Lines[i].Sequence <= result.Lines[i-1].Sequence {
			t.Fatalf("sequences not increasing: %v", result.Lines)
		}
	}
}

func TestManagerStartEventsCarryRunAndSequence(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses POSIX shell command")
	}

	bus := event.NewBusWithHistory(100)
	manager := NewManager([]config.ProcessConfig{{
		Name:    "worker",
		Command: "echo hello",
	}}, bus)

	if err := manager.Start(context.Background(), "worker"); err != nil {
		t.Fatalf("start process: %v", err)
	}
	defer manager.StopAll(context.Background())

	waitForLogCount(t, manager, "worker", 1)

	sub := bus.SubscribeFiltered([]event.EventType{event.TypeProcessStarted, event.TypeProcessOutput})
	defer bus.Unsubscribe(sub)

	var started, output *event.ProcessEvent
	deadline := time.After(2 * time.Second)
	for started == nil || output == nil {
		select {
		case item := <-sub:
			processEvent, ok := item.(event.ProcessEvent)
			if !ok {
				continue
			}
			switch processEvent.Type() {
			case event.TypeProcessStarted:
				started = &processEvent
			case event.TypeProcessOutput:
				output = &processEvent
			}
		case <-deadline:
			t.Fatal("timed out waiting for events")
		}
	}

	if started.RunID == "" || started.Sequence == 0 {
		t.Fatalf("started event missing run/sequence: %+v", started)
	}
	if output.RunID != started.RunID {
		t.Fatalf("output run ID = %q, want %q", output.RunID, started.RunID)
	}
	if output.Sequence <= started.Sequence {
		t.Fatalf("output sequence = %d, want > started %d", output.Sequence, started.Sequence)
	}
}

func waitForLogLine(t *testing.T, manager *Manager, name, message string) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		result, err := manager.Logs(name, "current", 50, 0)
		if err == nil {
			for _, line := range result.Lines {
				if line.Message == message {
					return
				}
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for log line %q", message)
}

func waitForLogCount(t *testing.T, manager *Manager, name string, count int) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		result, err := manager.Logs(name, "current", 50, 0)
		if err == nil && len(result.Lines) >= count {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %d log lines", count)
}
