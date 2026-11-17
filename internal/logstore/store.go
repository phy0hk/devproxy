// Package logstore keeps bounded, run-aware process output history in memory.
package logstore

import (
	"errors"
	"sort"
	"sync"
	"time"
)

const (
	// DefaultCapacity is the number of output lines retained per process.
	DefaultCapacity = 5000
	// DefaultMaxRuns keeps the current run plus the previous run.
	DefaultMaxRuns = 2
)

// ErrRunNotFound reports a run that is not retained.
var ErrRunNotFound = errors.New("run not found")

// Line is one captured stdout or stderr line.
type Line struct {
	Process   string    `json:"process"`
	RunID     string    `json:"run_id"`
	Sequence  uint64    `json:"sequence"`
	Timestamp time.Time `json:"timestamp"`
	Stream    string    `json:"stream"`
	Message   string    `json:"message"`
}

// Run describes one process start.
type Run struct {
	ID        string     `json:"id"`
	StartedAt time.Time  `json:"started_at"`
	EndedAt   *time.Time `json:"ended_at,omitempty"`
}

// Options bounds memory use.
type Options struct {
	Capacity int
	MaxRuns  int
}

// Result is one page of history.
type Result struct {
	Process    string `json:"process"`
	Run        string `json:"run"`
	Runs       []Run  `json:"runs"`
	Lines      []Line `json:"lines"`
	HasMore    bool   `json:"has_more"`
	NextBefore uint64 `json:"next_before,omitempty"`
}

// Store holds per-process ring buffers. All state is in memory.
type Store struct {
	mu       sync.Mutex
	capacity int
	maxRuns  int
	process  map[string]*processHistory
}

type processHistory struct {
	runs  []Run
	lines *lineRing
}

// New creates a store. Zero options fall back to the defaults.
func New(options Options) *Store {
	capacity := options.Capacity
	if capacity <= 0 {
		capacity = DefaultCapacity
	}

	maxRuns := options.MaxRuns
	if maxRuns <= 0 {
		maxRuns = DefaultMaxRuns
	}

	return &Store{
		capacity: capacity,
		maxRuns:  maxRuns,
		process:  make(map[string]*processHistory),
	}
}

// BeginRun records a new run boundary and prunes runs beyond retention.
func (s *Store) BeginRun(process, runID string, startedAt time.Time) {
	if runID == "" {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	history := s.historyLocked(process)
	for _, run := range history.runs {
		if run.ID == runID {
			return
		}
	}

	history.runs = append(history.runs, Run{
		ID:        runID,
		StartedAt: startedAt,
	})

	if len(history.runs) <= s.maxRuns {
		return
	}

	history.runs = append([]Run(nil), history.runs[len(history.runs)-s.maxRuns:]...)
	allowed := make(map[string]struct{}, len(history.runs))
	for _, run := range history.runs {
		allowed[run.ID] = struct{}{}
	}
	history.lines.retainRuns(allowed)
}

// EndRun records the end time of a retained run.
func (s *Store) EndRun(process, runID string, endedAt time.Time) {
	if runID == "" {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	history, ok := s.process[process]
	if !ok {
		return
	}

	for i := range history.runs {
		if history.runs[i].ID == runID {
			end := endedAt
			history.runs[i].EndedAt = &end
			return
		}
	}
}

// Append adds one line, dropping the oldest line when the ring is full.
func (s *Store) Append(line Line) {
	if line.Process == "" {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	history := s.historyLocked(line.Process)
	if line.RunID == "" && len(history.runs) > 0 {
		line.RunID = history.runs[len(history.runs)-1].ID
	}
	history.lines.push(line)
}

// Runs returns retained runs for a process, oldest first.
func (s *Store) Runs(process string) ([]Run, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	history, ok := s.process[process]
	if !ok {
		return []Run{}, nil
	}

	runs := make([]Run, len(history.runs))
	copy(runs, history.runs)
	return runs, nil
}

// Query returns a page of retained lines for a run.
//
// run may be "", "current", "previous", or an explicit run ID. When before is
// non-zero only lines with a smaller sequence are returned. The page keeps the
// newest matching lines up to limit, in ascending sequence order.
func (s *Store) Query(
	process string,
	run string,
	limit int,
	before uint64,
) (Result, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	result := Result{
		Process: process,
		Runs:    []Run{},
		Lines:   []Line{},
	}

	history, ok := s.process[process]
	if !ok {
		return result, nil
	}

	result.Runs = make([]Run, len(history.runs))
	for i, item := range history.runs {
		result.Runs[len(history.runs)-1-i] = item
	}

	runID, err := resolveRun(history.runs, run)
	if err != nil {
		return result, err
	}
	result.Run = runID
	if runID == "" {
		return result, nil
	}

	if limit <= 0 {
		limit = 1000
	}
	if limit > s.capacity {
		limit = s.capacity
	}

	matches := make([]Line, 0, limit)
	for _, entry := range history.lines.ordered() {
		if entry.RunID != runID {
			continue
		}
		if before > 0 && entry.Sequence >= before {
			continue
		}
		matches = append(matches, entry)
	}

	// stdout and stderr are captured concurrently, so insertion order can lag
	// sequence order by a few entries. Sort before paging so `before` and
	// `next_before` always describe the newest lines correctly.
	sort.SliceStable(matches, func(i, j int) bool {
		return matches[i].Sequence < matches[j].Sequence
	})

	if len(matches) > limit {
		result.HasMore = true
		matches = matches[len(matches)-limit:]
	}

	result.Lines = matches
	if result.HasMore && len(matches) > 0 {
		result.NextBefore = matches[0].Sequence
	}

	return result, nil
}

func (s *Store) historyLocked(process string) *processHistory {
	history, ok := s.process[process]
	if !ok {
		history = &processHistory{lines: newLineRing(s.capacity)}
		s.process[process] = history
	}
	return history
}

func resolveRun(runs []Run, requested string) (string, error) {
	if requested == "" || requested == "current" {
		if len(runs) == 0 {
			return "", nil
		}
		return runs[len(runs)-1].ID, nil
	}

	if requested == "previous" {
		if len(runs) < 2 {
			return "", nil
		}
		return runs[len(runs)-2].ID, nil
	}

	for _, run := range runs {
		if run.ID == requested {
			return requested, nil
		}
	}

	return "", ErrRunNotFound
}

type lineRing struct {
	buffer   []Line
	capacity int
	head     int
	size     int
}

func newLineRing(capacity int) *lineRing {
	if capacity < 1 {
		capacity = 1
	}
	return &lineRing{
		buffer:   make([]Line, capacity),
		capacity: capacity,
	}
}

func (r *lineRing) push(line Line) {
	r.buffer[r.head] = line
	r.head = (r.head + 1) % r.capacity
	if r.size < r.capacity {
		r.size++
	}
}

func (r *lineRing) ordered() []Line {
	if r.size == 0 {
		return nil
	}

	result := make([]Line, 0, r.size)
	if r.size < r.capacity {
		return append(result, r.buffer[:r.size]...)
	}

	result = append(result, r.buffer[r.head:]...)
	return append(result, r.buffer[:r.head]...)
}

func (r *lineRing) retainRuns(allowed map[string]struct{}) {
	lines := r.ordered()
	r.buffer = make([]Line, r.capacity)
	r.head = 0
	r.size = 0

	for _, line := range lines {
		if _, ok := allowed[line.RunID]; ok {
			r.push(line)
		}
	}
}
