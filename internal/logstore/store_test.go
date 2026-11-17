package logstore

import (
	"errors"
	"fmt"
	"testing"
	"time"
)

func line(runID string, seq uint64, stream string) Line {
	return Line{
		Process:   "api",
		RunID:     runID,
		Sequence:  seq,
		Timestamp: time.Unix(int64(seq), 0).UTC(),
		Stream:    stream,
		Message:   fmt.Sprintf("line %d", seq),
	}
}

func TestStoreRingBufferKeepsNewestLinesInOrder(t *testing.T) {
	store := New(Options{Capacity: 3, MaxRuns: 2})
	store.BeginRun("api", "run-1", time.Now())

	for i := uint64(1); i <= 5; i++ {
		store.Append(line("run-1", i, "stdout"))
	}

	result, err := store.Query("api", "", 10, 0)
	if err != nil {
		t.Fatalf("query: %v", err)
	}

	if len(result.Lines) != 3 {
		t.Fatalf("got %d lines, want 3", len(result.Lines))
	}

	for i, want := range []uint64{3, 4, 5} {
		if result.Lines[i].Sequence != want {
			t.Fatalf("line %d sequence = %d, want %d", i, result.Lines[i].Sequence, want)
		}
	}
}

func TestStorePrunesRunsBeyondRetention(t *testing.T) {
	store := New(Options{Capacity: 100, MaxRuns: 2})

	store.BeginRun("api", "run-1", time.Now())
	store.Append(line("run-1", 1, "stdout"))
	store.EndRun("api", "run-1", time.Now())

	store.BeginRun("api", "run-2", time.Now())
	store.Append(line("run-2", 2, "stdout"))
	store.EndRun("api", "run-2", time.Now())

	store.BeginRun("api", "run-3", time.Now())
	store.Append(line("run-3", 3, "stdout"))

	runs, err := store.Runs("api")
	if err != nil {
		t.Fatalf("runs: %v", err)
	}
	if len(runs) != 2 {
		t.Fatalf("got %d retained runs, want 2", len(runs))
	}
	if runs[0].ID != "run-2" || runs[1].ID != "run-3" {
		t.Fatalf("retained runs = %q, want [run-2 run-3]", runIDs(runs))
	}

	if _, err := store.Query("api", "run-1", 10, 0); !errors.Is(err, ErrRunNotFound) {
		t.Fatalf("query pruned run error = %v, want ErrRunNotFound", err)
	}

	result, err := store.Query("api", "previous", 10, 0)
	if err != nil {
		t.Fatalf("query previous: %v", err)
	}
	if result.Run != "run-2" || len(result.Lines) != 1 || result.Lines[0].Sequence != 2 {
		t.Fatalf("previous result = run %q lines %v", result.Run, result.Lines)
	}
}

func TestStoreQueryPagesBeforeSequence(t *testing.T) {
	store := New(Options{Capacity: 100, MaxRuns: 1})
	store.BeginRun("api", "run-1", time.Now())

	for i := uint64(1); i <= 10; i++ {
		store.Append(line("run-1", i, "stdout"))
	}

	result, err := store.Query("api", "current", 3, 8)
	if err != nil {
		t.Fatalf("query: %v", err)
	}

	if len(result.Lines) != 3 {
		t.Fatalf("got %d lines, want 3", len(result.Lines))
	}
	for i, want := range []uint64{5, 6, 7} {
		if result.Lines[i].Sequence != want {
			t.Fatalf("line %d sequence = %d, want %d", i, result.Lines[i].Sequence, want)
		}
	}
	if !result.HasMore {
		t.Fatal("expected has_more for earlier lines")
	}
	if result.NextBefore != 5 {
		t.Fatalf("next_before = %d, want 5", result.NextBefore)
	}
}

func TestStoreQuerySortsConcurrentStreamsBySequence(t *testing.T) {
	store := New(Options{Capacity: 100, MaxRuns: 1})
	store.BeginRun("api", "run-1", time.Now())

	// stdout and stderr are read by separate goroutines, so append order can
	// differ from the sequence assigned before the append.
	for _, sequence := range []uint64{3, 1, 5, 2, 4} {
		stream := "stdout"
		if sequence%2 == 0 {
			stream = "stderr"
		}
		store.Append(line("run-1", sequence, stream))
	}

	result, err := store.Query("api", "current", 2, 5)
	if err != nil {
		t.Fatalf("query: %v", err)
	}

	for i, want := range []uint64{3, 4} {
		if result.Lines[i].Sequence != want {
			t.Fatalf("line %d sequence = %d, want %d", i, result.Lines[i].Sequence, want)
		}
	}
	if !result.HasMore || result.NextBefore != 3 {
		t.Fatalf("has_more=%v next_before=%d, want true/3", result.HasMore, result.NextBefore)
	}
}

func TestStoreKeepsStreamsDistinct(t *testing.T) {
	store := New(Options{Capacity: 10, MaxRuns: 1})
	store.BeginRun("api", "run-1", time.Now())
	store.Append(line("run-1", 1, "stdout"))
	store.Append(line("run-1", 2, "stderr"))

	result, err := store.Query("api", "", 10, 0)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if result.Lines[0].Stream != "stdout" || result.Lines[1].Stream != "stderr" {
		t.Fatalf("streams = %q,%q, want stdout,stderr", result.Lines[0].Stream, result.Lines[1].Stream)
	}
}

func TestStoreQueryCurrentRunDefaultsToLatest(t *testing.T) {
	store := New(Options{Capacity: 100, MaxRuns: 2})
	store.BeginRun("api", "run-1", time.Now())
	store.Append(line("run-1", 1, "stdout"))
	store.BeginRun("api", "run-2", time.Now())
	store.Append(line("run-2", 2, "stdout"))

	result, err := store.Query("api", "", 10, 0)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if result.Run != "run-2" || len(result.Lines) != 1 || result.Lines[0].Sequence != 2 {
		t.Fatalf("current result = run %q lines %v", result.Run, result.Lines)
	}
}

func runIDs(runs []Run) []string {
	ids := make([]string, len(runs))
	for i, run := range runs {
		ids[i] = run.ID
	}
	return ids
}
