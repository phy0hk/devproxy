package server

import (
	"bufio"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/phy0hk/devproxy/internal/event"
)

func TestHandleEventsResumesAfterLastEventID(t *testing.T) {
	bus := event.NewBusWithHistory(100)
	for i := uint64(1); i <= 4; i++ {
		bus.Publish(event.ProcessEvent{
			EventType: event.TypeProcessOutput,
			Timestamp: time.Now(),
			Process:   "worker",
			Sequence:  i,
			Stream:    "stdout",
			Message:   "line",
		})
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handleEvents(w, r, bus)
	}))
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, server.URL+"/events", nil)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	request.Header.Set("Last-Event-ID", "2")

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer response.Body.Close()

	frames := readSSEFrames(t, response.Body, 1)
	if len(frames) != 1 {
		t.Fatalf("got %d frames, want 1 batched frame", len(frames))
	}
	if got := frameSequences(t, frames[0]); strings.Join(got, ",") != "3,4" {
		t.Fatalf("resumed sequences = %v, want [3 4]", got)
	}
	if frames[0].id != "4" {
		t.Fatalf("frame id = %q, want 4", frames[0].id)
	}
}

func TestHandleEventsResumesWithoutGapsOrDuplicates(t *testing.T) {
	bus := event.NewBusWithHistory(200)
	for i := uint64(1); i <= 50; i++ {
		bus.Publish(event.ProcessEvent{
			EventType: event.TypeProcessOutput,
			Timestamp: time.Now(),
			Process:   "worker",
			Sequence:  i,
			Stream:    "stdout",
			Message:   "line",
		})
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handleEvents(w, r, bus)
	}))
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, server.URL+"/events", nil)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	request.Header.Set("Last-Event-ID", "17")

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer response.Body.Close()

	frame := readSSEFrames(t, response.Body, 1)[0]
	got := frameSequences(t, frame)

	seen := make(map[string]bool, len(got))
	index := 0
	for sequence := uint64(18); sequence <= 50; sequence++ {
		if index >= len(got) {
			t.Fatalf("resume stopped after %d of 33 expected sequences", len(got))
		}
		want := strconvFormat(sequence)
		if got[index] != want {
			t.Fatalf("sequence[%d] = %s, want %s", index, got[index], want)
		}
		if seen[got[index]] {
			t.Fatalf("duplicate sequence %s", got[index])
		}
		seen[got[index]] = true
		index++
	}
	if index != len(got) {
		t.Fatalf("got %d sequences, want 33: %v", len(got), got)
	}
}

func TestHandleEventsRejectsInvalidLastEventID(t *testing.T) {
	bus := event.NewBusWithHistory(10)
	request := httptest.NewRequest(http.MethodGet, "/events", nil)
	request.Header.Set("Last-Event-ID", "not-a-number")
	response := httptest.NewRecorder()

	handleEvents(response, request, bus)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", response.Code)
	}
}

func TestHandleEventsCoalescesFlushes(t *testing.T) {
	bus := event.NewBusWithHistory(1000)
	response := &flushRecorder{ResponseRecorder: httptest.NewRecorder()}

	ctx, cancel := context.WithCancel(context.Background())
	request := httptest.NewRequest(http.MethodGet, "/events", nil).WithContext(ctx)

	done := make(chan struct{})
	go func() {
		handleEvents(response, request, bus)
		close(done)
	}()

	// Let the handler subscribe before the burst.
	time.Sleep(20 * time.Millisecond)
	for i := uint64(1); i <= 200; i++ {
		bus.Publish(event.ProcessEvent{
			EventType: event.TypeProcessOutput,
			Timestamp: time.Now(),
			Process:   "worker",
			Sequence:  i,
			Stream:    "stdout",
			Message:   "line",
		})
	}

	time.Sleep(260 * time.Millisecond)
	cancel()
	<-done

	if response.flushes > 5 {
		t.Fatalf("flushes = %d, want a small number of batches", response.flushes)
	}
	frames := parseSSEFrames(t, response.Body.String())
	if len(frames) > 5 {
		t.Fatalf("frames = %d, want at most one per flush", len(frames))
	}

	var sequences []string
	for _, frame := range frames {
		sequences = append(sequences, frameSequences(t, frame)...)
	}
	if len(sequences) != 200 {
		t.Fatalf("wrote %d events, want 200", len(sequences))
	}
	seen := make(map[string]bool, len(sequences))
	for _, sequence := range sequences {
		if seen[sequence] {
			t.Fatalf("duplicate sequence %s", sequence)
		}
		seen[sequence] = true
	}
}

type sseFrame struct {
	id   string
	data string
}

func readSSEFrames(t *testing.T, body interface{ Read([]byte) (int, error) }, count int) []sseFrame {
	t.Helper()

	reader := bufio.NewReader(body)
	var frames []sseFrame
	var id, data []string

	for len(frames) < count {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read after %d frames: %v", len(frames), err)
		}
		line = strings.TrimRight(line, "\r\n")
		switch {
		case strings.HasPrefix(line, "id: "):
			id = append(id, strings.TrimSpace(strings.TrimPrefix(line, "id: ")))
		case strings.HasPrefix(line, "data: "):
			data = append(data, strings.TrimPrefix(line, "data: "))
		case line == "":
			if len(data) == 0 {
				continue
			}
			frames = append(frames, sseFrame{id: strings.Join(id, ","), data: strings.Join(data, "\n")})
			id, data = nil, nil
		}
	}

	return frames
}

func parseSSEFrames(t *testing.T, body string) []sseFrame {
	t.Helper()

	var frames []sseFrame
	var id, data []string
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimRight(line, "\r")
		switch {
		case strings.HasPrefix(line, "id: "):
			id = append(id, strings.TrimSpace(strings.TrimPrefix(line, "id: ")))
		case strings.HasPrefix(line, "data: "):
			data = append(data, strings.TrimPrefix(line, "data: "))
		case line == "":
			if len(data) == 0 {
				continue
			}
			frames = append(frames, sseFrame{id: strings.Join(id, ","), data: strings.Join(data, "\n")})
			id, data = nil, nil
		}
	}
	return frames
}

func frameSequences(t *testing.T, frame sseFrame) []string {
	t.Helper()

	var events []event.ProcessEvent
	if err := json.Unmarshal([]byte(frame.data), &events); err != nil {
		t.Fatalf("decode batch %q: %v", frame.data, err)
	}

	sequences := make([]string, 0, len(events))
	for _, item := range events {
		sequences = append(sequences, strconvFormat(item.Sequence))
	}
	return sequences
}

func strconvFormat(value uint64) string {
	if value == 0 {
		return "0"
	}

	var digits [20]byte
	index := len(digits)
	for value > 0 {
		index--
		digits[index] = byte('0' + value%10)
		value /= 10
	}
	return string(digits[index:])
}

type flushRecorder struct {
	*httptest.ResponseRecorder
	flushes int
}

func (r *flushRecorder) Flush() {
	r.flushes++
	r.ResponseRecorder.Flush()
}
