package process

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"runtime"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/phy0hk/devproxy/internal/config"
	"github.com/phy0hk/devproxy/internal/event"
	"github.com/phy0hk/devproxy/internal/logstore"
)

type State string

const (
	StateConfigured State = "configured"
	StateRunning    State = "running"
	StateStopping   State = "stopping"
	StateExited     State = "exited"
	StateFailed     State = "failed"
)

type Status struct {
	Name       string     `json:"name"`
	Command    string     `json:"command"`
	WorkingDir string     `json:"working_dir,omitempty"`
	State      State      `json:"state"`
	RunID      string     `json:"run_id,omitempty"`
	PID        int        `json:"pid,omitempty"`
	StartedAt  *time.Time `json:"started_at,omitempty"`
	ExitedAt   *time.Time `json:"exited_at,omitempty"`
	ExitCode   *int       `json:"exit_code,omitempty"`
	LastError  string     `json:"last_error,omitempty"`
}

// Options bounds in-memory log history. Zero values use logstore defaults.
type Options struct {
	LogCapacity int
	MaxRuns     int
}

type Manager struct {
	configs []config.ProcessConfig
	bus     *event.Bus
	logs    *logstore.Store

	mu       sync.Mutex
	outputMu sync.Mutex
	running  map[string]*runningProcess
	statuses map[string]Status
	runSeq   uint64
}

type runningProcess struct {
	config config.ProcessConfig
	runID  string
	cmd    *exec.Cmd
	cancel context.CancelFunc
	done   chan struct{}
}

func NewManager(configs []config.ProcessConfig, bus *event.Bus) *Manager {
	return NewManagerWithOptions(configs, bus, Options{})
}

// NewManagerWithOptions creates a manager with explicit log retention bounds.
func NewManagerWithOptions(
	configs []config.ProcessConfig,
	bus *event.Bus,
	options Options,
) *Manager {
	manager := &Manager{
		configs:  configs,
		bus:      bus,
		logs:     logstore.New(logstore.Options{Capacity: options.LogCapacity, MaxRuns: options.MaxRuns}),
		running:  make(map[string]*runningProcess),
		statuses: make(map[string]Status, len(configs)),
	}

	for _, processConfig := range configs {
		manager.statuses[processConfig.Name] = Status{
			Name:       processConfig.Name,
			Command:    processConfig.Command,
			WorkingDir: processConfig.WorkingDir,
			State:      StateConfigured,
		}
	}

	return manager
}

func (m *Manager) StartAll(ctx context.Context) error {
	for _, processConfig := range m.configs {
		if err := m.Start(ctx, processConfig.Name); err != nil {
			_ = m.StopAll(context.Background())
			return err
		}
	}

	return nil
}

func (m *Manager) Start(ctx context.Context, name string) error {
	processConfig, ok := m.findConfig(name)
	if !ok {
		return fmt.Errorf("process %q not found", name)
	}

	m.mu.Lock()
	if _, exists := m.running[name]; exists {
		m.mu.Unlock()
		return fmt.Errorf("process %q is already running", name)
	}
	m.mu.Unlock()

	runID := fmt.Sprintf("run-%d", atomic.AddUint64(&m.runSeq, 1))
	startedAt := time.Now()
	m.logs.BeginRun(name, runID, startedAt)

	// Process lifetime must not be tied to the HTTP request that started it.
	// The manager owns cancellation through Stop and StopAll.
	processCtx, cancel := context.WithCancel(context.Background())
	cmd := shellCommand(processCtx, processConfig.Command)
	configureProcessGroup(cmd)
	cmd.Dir = processConfig.WorkingDir
	cmd.Env = processEnv(processConfig.Env)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		m.markFailed(name, runID, err)
		return fmt.Errorf("create stdout pipe for %q: %w", name, err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		cancel()
		m.markFailed(name, runID, err)
		return fmt.Errorf("create stderr pipe for %q: %w", name, err)
	}

	if err := cmd.Start(); err != nil {
		cancel()
		m.markFailed(name, runID, err)
		return fmt.Errorf("start process %q: %w", name, err)
	}

	running := &runningProcess{
		config: processConfig,
		runID:  runID,
		cmd:    cmd,
		cancel: cancel,
		done:   make(chan struct{}),
	}

	m.mu.Lock()
	m.running[name] = running
	m.statuses[name] = Status{
		Name:       processConfig.Name,
		Command:    processConfig.Command,
		WorkingDir: processConfig.WorkingDir,
		State:      StateRunning,
		RunID:      runID,
		PID:        cmd.Process.Pid,
		StartedAt:  &startedAt,
	}
	m.mu.Unlock()

	m.publish(event.ProcessEvent{
		EventType: event.TypeProcessStarted,
		Timestamp: startedAt,
		Process:   name,
		RunID:     runID,
		Message:   processConfig.Command,
	})

	log.Printf("process %q started: %s", name, processConfig.Command)

	go m.captureOutput(name, runID, "stdout", stdout)
	go m.captureOutput(name, runID, "stderr", stderr)
	go m.wait(name, running)

	return nil
}

func (m *Manager) Stop(ctx context.Context, name string) error {
	if _, ok := m.findConfig(name); !ok {
		return fmt.Errorf("process %q not found", name)
	}

	m.mu.Lock()
	running, exists := m.running[name]
	m.mu.Unlock()
	if !exists {
		return fmt.Errorf("process %q is not running", name)
	}

	return m.stop(ctx, running)
}

func (m *Manager) Restart(ctx context.Context, name string) error {
	if _, ok := m.findConfig(name); !ok {
		return fmt.Errorf("process %q not found", name)
	}

	m.mu.Lock()
	running, exists := m.running[name]
	m.mu.Unlock()

	if exists {
		if err := m.stop(ctx, running); err != nil {
			return err
		}
	}

	// A process that exited naturally is already removed from running; restart
	// should still start a fresh instance rather than requiring a manual reset.
	return m.Start(ctx, name)
}

func (m *Manager) StopAll(ctx context.Context) error {
	m.mu.Lock()
	running := make([]*runningProcess, 0, len(m.running))
	for _, process := range m.running {
		running = append(running, process)
	}
	m.mu.Unlock()

	var errs []error
	for _, process := range running {
		if err := m.stop(ctx, process); err != nil {
			errs = append(errs, err)
		}
	}

	return errors.Join(errs...)
}

func (m *Manager) Statuses() []Status {
	m.mu.Lock()
	defer m.mu.Unlock()

	statuses := make([]Status, 0, len(m.statuses))
	for _, status := range m.statuses {
		statuses = append(statuses, status)
	}

	sort.Slice(statuses, func(i, j int) bool {
		return statuses[i].Name < statuses[j].Name
	})

	return statuses
}

// Logs returns bounded, run-aware output history for a configured process.
func (m *Manager) Logs(
	name string,
	run string,
	limit int,
	before uint64,
) (logstore.Result, error) {
	if _, ok := m.findConfig(name); !ok {
		return logstore.Result{}, fmt.Errorf("process %q not found", name)
	}

	return m.logs.Query(name, run, limit, before)
}

func (m *Manager) stop(ctx context.Context, process *runningProcess) error {
	name := process.config.Name

	if process.cmd.Process == nil {
		return nil
	}

	// Release the lifetime context once this instance is fully stopped. The
	// group signal below owns termination of the whole process tree.
	defer process.cancel()

	now := time.Now()
	m.mu.Lock()
	owned := false
	if current, ok := m.running[name]; ok && current == process {
		owned = true
		status := m.statuses[name]
		status.State = StateStopping
		m.statuses[name] = status
	}
	m.mu.Unlock()

	if owned {
		m.publish(event.ProcessEvent{
			EventType: event.TypeProcessStopping,
			Timestamp: now,
			Process:   name,
			RunID:     process.runID,
		})
	}

	// The direct child is usually only a shell wrapper; the real dev server
	// runs as one of its descendants. Signal the whole process group so every
	// instance is stopped, not just the wrapper.
	if err := terminateProcessGroup(process.cmd.Process, terminateGraceful); err != nil {
		return fmt.Errorf("stop process group for %q: %w", name, err)
	}

	select {
	case <-process.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(5 * time.Second):
		if err := terminateProcessGroup(process.cmd.Process, terminateForce); err != nil {
			return fmt.Errorf("kill process group for %q: %w", name, err)
		}
	}

	select {
	case <-process.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (m *Manager) captureOutput(name string, runID string, stream string, reader io.Reader) {
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		line := scanner.Text()

		// Assign the sequence inside the lock so the stored order, the
		// published order, and the sequence order always agree when stdout
		// and stderr are read concurrently.
		m.outputMu.Lock()
		timestamp := time.Now()
		sequence := event.NextSequence()
		m.logs.Append(logstore.Line{
			Process:   name,
			RunID:     runID,
			Sequence:  sequence,
			Timestamp: timestamp,
			Stream:    stream,
			Message:   line,
		})
		m.publish(event.ProcessEvent{
			EventType: event.TypeProcessOutput,
			Timestamp: timestamp,
			Process:   name,
			RunID:     runID,
			Sequence:  sequence,
			Stream:    stream,
			Message:   line,
		})
		m.outputMu.Unlock()
	}

	if err := scanner.Err(); err != nil {
		m.publish(event.ProcessEvent{
			EventType: event.TypeProcessOutputErr,
			Timestamp: time.Now(),
			Process:   name,
			RunID:     runID,
			Stream:    stream,
			Error:     err.Error(),
		})
	}
}

func (m *Manager) wait(name string, process *runningProcess) {
	defer close(process.done)
	defer process.cancel()

	err := process.cmd.Wait()
	exitCode := 0
	if process.cmd.ProcessState != nil {
		exitCode = process.cmd.ProcessState.ExitCode()
	}

	now := time.Now()
	processEvent := event.ProcessEvent{
		EventType: event.TypeProcessExited,
		Timestamp: now,
		Process:   name,
		RunID:     process.runID,
		ExitCode:  exitCode,
	}

	m.mu.Lock()
	if current, ok := m.running[name]; ok && current == process {
		// Only the waiter that still owns the entry may update the status. A
		// late waiter from an older instance must not overwrite the status of
		// a newer instance started after a restart.
		delete(m.running, name)

		currentStatus := m.statuses[name]
		wasStopping := currentStatus.State == StateStopping
		currentStatus.PID = 0
		currentStatus.State = StateExited
		currentStatus.ExitedAt = &now
		currentStatus.ExitCode = &exitCode

		if err != nil {
			processEvent.Error = err.Error()
			currentStatus.LastError = err.Error()

			if !wasStopping {
				currentStatus.State = StateFailed
			}
		}

		m.statuses[name] = currentStatus
	}
	m.mu.Unlock()

	m.logs.EndRun(name, process.runID, now)
	m.publish(processEvent)
	log.Printf("process %q exited with code %d", name, exitCode)
}

func (m *Manager) findConfig(name string) (config.ProcessConfig, bool) {
	for _, processConfig := range m.configs {
		if processConfig.Name == name {
			return processConfig, true
		}
	}

	return config.ProcessConfig{}, false
}

func (m *Manager) markFailed(name string, runID string, err error) {
	now := time.Now()

	m.mu.Lock()
	status := m.statuses[name]
	status.State = StateFailed
	status.RunID = runID
	status.ExitedAt = &now
	status.LastError = err.Error()
	m.statuses[name] = status
	m.mu.Unlock()

	m.logs.EndRun(name, runID, now)
	m.publish(event.ProcessEvent{
		EventType: event.TypeProcessFailed,
		Timestamp: now,
		Process:   name,
		RunID:     runID,
		Error:     err.Error(),
	})
}

func (m *Manager) publish(processEvent event.ProcessEvent) {
	if processEvent.Sequence == 0 {
		processEvent.Sequence = event.NextSequence()
	}

	if m.bus != nil {
		m.bus.Publish(processEvent)
	}
}

func shellCommand(ctx context.Context, command string) *exec.Cmd {
	if runtime.GOOS == "windows" {
		return exec.CommandContext(ctx, "cmd", "/C", command)
	}

	return exec.CommandContext(ctx, "sh", "-c", command)
}

func processEnv(env map[string]string) []string {
	processEnv := os.Environ()
	for key, value := range env {
		processEnv = append(processEnv, fmt.Sprintf("%s=%s", key, value))
	}

	return processEnv
}
