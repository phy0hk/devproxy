//go:build windows

package process

import (
	"errors"
	"os"
	"os/exec"
)

// terminationSignal selects how a managed process tree is stopped.
type terminationSignal int

const (
	terminateGraceful terminationSignal = iota
	terminateForce
)

// configureProcessGroup is a no-op on Windows, which has no POSIX process
// groups. Stop falls back to terminating the direct child.
func configureProcessGroup(cmd *exec.Cmd) {}

// terminateProcessGroup terminates the direct child on Windows. Full tree
// termination would require a job object and is out of scope for local dev.
func terminateProcessGroup(proc *os.Process, signal terminationSignal) error {
	if err := proc.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
		return err
	}
	return nil
}
