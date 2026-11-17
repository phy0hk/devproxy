//go:build !windows

package process

import (
	"errors"
	"os"
	"os/exec"
	"syscall"
)

// terminationSignal selects how a managed process tree is stopped.
type terminationSignal int

const (
	terminateGraceful terminationSignal = iota
	terminateForce
)

// configureProcessGroup puts a managed command in its own process group. The
// group leader is the command itself, so its PID is also the group ID.
func configureProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

// terminateProcessGroup signals every process in the group, including
// grandchildren spawned by shell wrappers such as "npm run dev".
func terminateProcessGroup(proc *os.Process, signal terminationSignal) error {
	sig := syscall.SIGTERM
	if signal == terminateForce {
		sig = syscall.SIGKILL
	}

	err := syscall.Kill(-proc.Pid, sig)
	if err == nil || errors.Is(err, syscall.ESRCH) {
		return nil
	}

	// Fall back to the direct child if group signalling is not permitted.
	if killErr := proc.Kill(); killErr != nil && !errors.Is(killErr, os.ErrProcessDone) {
		return killErr
	}
	return nil
}
