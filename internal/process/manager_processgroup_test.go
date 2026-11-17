//go:build !windows

package process

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/phy0hk/devproxy/internal/config"
)

// A command wrapper (for example "npm run dev") spawns the real dev server as
// a grandchild. Stopping must terminate the whole process tree, otherwise the
// dev server is orphaned and keeps holding its port after devproxy exits.
func TestManagerStopAllKillsGrandchildren(t *testing.T) {
	dir := t.TempDir()
	pidFile := filepath.Join(dir, "child.pid")
	command := fmt.Sprintf("sleep 60 & echo $! > %s; wait", pidFile)

	manager := NewManager([]config.ProcessConfig{{
		Name:    "worker",
		Command: command,
	}}, nil)

	if err := manager.Start(context.Background(), "worker"); err != nil {
		t.Fatalf("start process: %v", err)
	}
	defer manager.StopAll(context.Background())

	grandchildPID := waitForPIDFile(t, pidFile)
	t.Cleanup(func() { _ = syscall.Kill(grandchildPID, syscall.SIGKILL) })

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := manager.StopAll(ctx); err != nil {
		t.Fatalf("stop all: %v", err)
	}

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if !processAlive(grandchildPID) {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("grandchild process %d is still alive after StopAll", grandchildPID)
}

func waitForPIDFile(t *testing.T, path string) int {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		data, err := os.ReadFile(path)
		if err == nil {
			pid, parseErr := strconv.Atoi(strings.TrimSpace(string(data)))
			if parseErr == nil && pid > 0 {
				return pid
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for pid file %s", path)
	return 0
}

func processAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	err := syscall.Kill(pid, 0)
	if errors.Is(err, syscall.ESRCH) {
		return false
	}
	if err != nil {
		return true
	}
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
	if err == nil {
		fields := strings.Fields(string(data))
		if len(fields) > 2 && fields[2] == "Z" {
			return false
		}
	}
	return true
}
