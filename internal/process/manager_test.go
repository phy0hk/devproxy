package process

import (
	"context"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/phy0hk/devproxy/internal/config"
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
