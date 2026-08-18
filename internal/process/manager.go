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
	"time"

	"github.com/phy0hk/devproxy/internal/config"
	"github.com/phy0hk/devproxy/internal/event"
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
	PID        int        `json:"pid,omitempty"`
	StartedAt  *time.Time `json:"started_at,omitempty"`
	ExitedAt   *time.Time `json:"exited_at,omitempty"`
	ExitCode   *int       `json:"exit_code,omitempty"`
	LastError  string     `json:"last_error,omitempty"`
}

type Manager struct {
	configs []config.ProcessConfig
	bus     *event.Bus

	mu       sync.Mutex
	running  map[string]*runningProcess
	statuses map[string]Status
}

type runningProcess struct {
	config config.ProcessConfig
	cmd    *exec.Cmd
	cancel context.CancelFunc
	done   chan struct{}
}

func NewManager(configs []config.ProcessConfig, bus *event.Bus) *Manager {
	manager := &Manager{
		configs:  configs,
		bus:      bus,
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

	processCtx, cancel := context.WithCancel(ctx)
	cmd := shellCommand(processCtx, processConfig.Command)
	cmd.Dir = processConfig.WorkingDir
	cmd.Env = processEnv(processConfig.Env)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		m.markFailed(name, err)
		return fmt.Errorf("create stdout pipe for %q: %w", name, err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		cancel()
		m.markFailed(name, err)
		return fmt.Errorf("create stderr pipe for %q: %w", name, err)
	}

	if err := cmd.Start(); err != nil {
		cancel()
		m.markFailed(name, err)
		return fmt.Errorf("start process %q: %w", name, err)
	}

	running := &runningProcess{
		config: processConfig,
		cmd:    cmd,
		cancel: cancel,
		done:   make(chan struct{}),
	}

	now := time.Now()
	m.mu.Lock()
	m.running[name] = running
	m.statuses[name] = Status{
		Name:       processConfig.Name,
		Command:    processConfig.Command,
		WorkingDir: processConfig.WorkingDir,
		State:      StateRunning,
		PID:        cmd.Process.Pid,
		StartedAt:  &now,
	}
	m.mu.Unlock()

	m.publish(event.ProcessEvent{
		Type:      "process.started",
		Timestamp: now,
		Process:   name,
		Message:   processConfig.Command,
	})

	log.Printf("process %q started: %s", name, processConfig.Command)

	go m.captureOutput(name, "stdout", stdout)
	go m.captureOutput(name, "stderr", stderr)
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
	_, running := m.running[name]
	m.mu.Unlock()

	if running {
		if err := m.Stop(ctx, name); err != nil {
			return err
		}
	}

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

func (m *Manager) stop(ctx context.Context, process *runningProcess) error {
	name := process.config.Name

	if process.cmd.Process == nil {
		return nil
	}

	now := time.Now()
	m.mu.Lock()
	status := m.statuses[name]
	status.State = StateStopping
	m.statuses[name] = status
	m.mu.Unlock()

	m.publish(event.ProcessEvent{
		Type:      "process.stopping",
		Timestamp: now,
		Process:   name,
	})

	process.cancel()

	select {
	case <-process.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(5 * time.Second):
		if err := process.cmd.Process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
			return fmt.Errorf("kill process %q: %w", name, err)
		}
	}

	select {
	case <-process.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (m *Manager) captureOutput(name string, stream string, reader io.Reader) {
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		line := scanner.Text()

		m.publish(event.ProcessEvent{
			Type:      "process.output",
			Timestamp: time.Now(),
			Process:   name,
			Stream:    stream,
			Message:   line,
		})

		log.Printf("[%s:%s] %s", name, stream, line)
	}

	if err := scanner.Err(); err != nil {
		m.publish(event.ProcessEvent{
			Type:      "process.output_error",
			Timestamp: time.Now(),
			Process:   name,
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
		Type:      "process.exited",
		Timestamp: now,
		Process:   name,
		ExitCode:  exitCode,
	}

	m.mu.Lock()
	currentStatus := m.statuses[name]
	delete(m.running, name)

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
	m.mu.Unlock()

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

func (m *Manager) markFailed(name string, err error) {
	now := time.Now()

	m.mu.Lock()
	status := m.statuses[name]
	status.State = StateFailed
	status.ExitedAt = &now
	status.LastError = err.Error()
	m.statuses[name] = status
	m.mu.Unlock()

	m.publish(event.ProcessEvent{
		Type:      "process.failed",
		Timestamp: now,
		Process:   name,
		Error:     err.Error(),
	})
}

func (m *Manager) publish(processEvent event.ProcessEvent) {
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
