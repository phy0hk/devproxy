package event

import (
	"runtime"
	"sync"
	"testing"
	"time"
)

func TestBusPublish(t *testing.T) {
	bus := NewBus()

	sub := bus.Subscribe()
	defer bus.Unsubscribe(sub)

	expected := RequestEvent{
		ID:        "test-1",
		Method:    "GET",
		Path:      "/api/users",
		Status:    200,
		Timestamp: time.Now(),
	}

	bus.Publish(expected)

	select {
	case event := <-sub:
		got, ok := event.(RequestEvent)
		if !ok {
			t.Fatalf("got event type %T, want RequestEvent", event)
		}

		if got.ID != expected.ID {
			t.Fatalf(
				"got ID %q, want %q",
				got.ID,
				expected.ID,
			)
		}

		if got.Path != expected.Path {
			t.Fatalf(
				"got path %q, want %q",
				got.Path,
				expected.Path,
			)
		}

	case <-time.After(time.Second):
		t.Fatal("timed out waiting for event")
	}
}

func TestBusPublishAfterUnsubscribeDoesNotPanic(t *testing.T) {
	bus := NewBus()
	sub := bus.Subscribe()
	bus.Unsubscribe(sub)

	// Publishing after a subscriber is removed must never send on a closed
	// channel. This used to panic inside Publish.
	bus.Publish(RequestEvent{ID: "after-close", Timestamp: time.Now()})
}

func TestBusUnsubscribeDoesNotLeakGoroutines(t *testing.T) {
	bus := NewBusWithHistory(256)
	before := runtime.NumGoroutine()

	for i := 0; i < 200; i++ {
		sub := bus.Subscribe()
		bus.Publish(ProcessEvent{
			EventType: TypeProcessOutput,
			Timestamp: time.Now(),
			Process:   "worker",
			Sequence:  NextSequence(),
			Message:   "line",
		})
		bus.Unsubscribe(sub)
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		runtime.GC()
		if runtime.NumGoroutine() <= before+2 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}

	t.Fatalf(
		"goroutine count grew from %d to %d after subscribe/unsubscribe cycles",
		before,
		runtime.NumGoroutine(),
	)
}

func TestBusReplaysHistoryInSequenceOrder(t *testing.T) {
	bus := NewBusWithHistory(16)
	for i := 1; i <= 5; i++ {
		bus.Publish(ProcessEvent{
			EventType: TypeProcessOutput,
			Timestamp: time.Now(),
			Process:   "worker",
			Sequence:  NextSequence(),
			Message:   "line",
		})
	}

	sub := bus.Subscribe()
	defer bus.Unsubscribe(sub)

	var got []uint64
	for i := 0; i < 5; i++ {
		select {
		case event := <-sub:
			got = append(got, SequenceOf(event))
		case <-time.After(time.Second):
			t.Fatalf("timed out after %d replayed events", len(got))
		}
	}

	for i := 1; i < len(got); i++ {
		if got[i] <= got[i-1] {
			t.Fatalf("replayed sequences not increasing: %v", got)
		}
	}
}

func TestBusSubscribeAfterSequenceSkipsOlderHistory(t *testing.T) {
	bus := NewBusWithHistory(16)

	var sequences []uint64
	for i := 0; i < 5; i++ {
		seq := NextSequence()
		sequences = append(sequences, seq)
		bus.Publish(ProcessEvent{
			EventType: TypeProcessOutput,
			Timestamp: time.Now(),
			Process:   "worker",
			Sequence:  seq,
			Message:   "line",
		})
	}

	sub := bus.SubscribeFilteredFrom(nil, sequences[2])
	defer bus.Unsubscribe(sub)

	for i := 3; i < 5; i++ {
		select {
		case event := <-sub:
			if got := SequenceOf(event); got != sequences[i] {
				t.Fatalf("replayed sequence = %d, want %d", got, sequences[i])
			}
		case <-time.After(time.Second):
			t.Fatalf("timed out waiting for sequence %d", sequences[i])
		}
	}
}

func TestBusHistoryAfterFiltersByTypeAndSequence(t *testing.T) {
	bus := NewBusWithHistory(16)

	output := ProcessEvent{
		EventType: TypeProcessOutput,
		Timestamp: time.Now(),
		Process:   "worker",
		Sequence:  NextSequence(),
		Message:   "out",
	}
	request := RequestEvent{
		EventType: TypeProxyRequest,
		Timestamp: time.Now(),
		ID:        "req",
		Sequence:  NextSequence(),
	}
	latest := ProcessEvent{
		EventType: TypeProcessOutput,
		Timestamp: time.Now(),
		Process:   "worker",
		Sequence:  NextSequence(),
		Message:   "latest",
	}

	bus.Publish(output)
	bus.Publish(request)
	bus.Publish(latest)

	history := bus.HistoryAfter([]EventType{TypeProcessOutput}, output.Sequence)
	if len(history) != 1 {
		t.Fatalf("history length = %d, want 1", len(history))
	}
	if got := SequenceOf(history[0]); got != latest.Sequence {
		t.Fatalf("history sequence = %d, want %d", got, latest.Sequence)
	}
}

func TestBusConcurrentSubscribeDeliversEachSequenceOnce(t *testing.T) {
	bus := NewBusWithHistory(4096)

	for sequence := uint64(1); sequence <= 500; sequence++ {
		bus.Publish(RequestEvent{
			EventType: TypeProxyRequest,
			Timestamp: time.Now(),
			Sequence:  sequence,
		})
	}

	var publisher sync.WaitGroup
	publisher.Add(1)
	go func() {
		defer publisher.Done()
		for sequence := uint64(501); sequence <= 800; sequence++ {
			bus.Publish(RequestEvent{
				EventType: TypeProxyRequest,
				Timestamp: time.Now(),
				Sequence:  sequence,
			})
		}
	}()

	subscriber := bus.SubscribeFilteredFrom(nil, 500)
	defer bus.Unsubscribe(subscriber)

	seen := make(map[uint64]int, 300)
	timeout := time.After(5 * time.Second)
	for len(seen) < 300 {
		select {
		case item, open := <-subscriber:
			if !open {
				t.Fatalf("subscriber closed with %d events seen", len(seen))
			}
			seen[SequenceOf(item)]++
		case <-timeout:
			t.Fatalf("timed out with %d events seen", len(seen))
		}
	}

	publisher.Wait()

	for sequence, count := range seen {
		if count != 1 {
			t.Fatalf("sequence %d delivered %d times", sequence, count)
		}
	}
	for sequence := uint64(501); sequence <= 800; sequence++ {
		if seen[sequence] != 1 {
			t.Fatalf("sequence %d missing (seen %d times)", sequence, seen[sequence])
		}
	}
}

func TestBusReplayDeliversEntireBoundedHistory(t *testing.T) {
	const capacity = 5000
	bus := NewBusWithHistory(capacity)

	for sequence := uint64(1); sequence <= capacity; sequence++ {
		bus.Publish(RequestEvent{
			EventType: TypeProxyRequest,
			Timestamp: time.Now(),
			Sequence:  sequence,
		})
	}

	subscriber := bus.SubscribeFilteredFrom(nil, 0)
	defer bus.Unsubscribe(subscriber)

	seen := make([]uint64, 0, capacity)
	timeout := time.After(5 * time.Second)
	for len(seen) < capacity {
		select {
		case item, open := <-subscriber:
			if !open {
				t.Fatalf("subscriber closed after %d events", len(seen))
			}
			seen = append(seen, SequenceOf(item))
		case <-timeout:
			t.Fatalf("timed out after %d events", len(seen))
		}
	}

	for index, sequence := range seen {
		if want := uint64(index + 1); sequence != want {
			t.Fatalf("event %d has sequence %d, want %d", index, sequence, want)
		}
	}
}
