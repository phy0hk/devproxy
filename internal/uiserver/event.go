package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/phy0hk/devproxy/internal/event"
)

const (
	// eventBatchInterval coalesces live output into at most ~10 flushes/s.
	eventBatchInterval = 100 * time.Millisecond
	// maxEventBatch protects memory if a client stalls between ticks.
	maxEventBatch = 5000
)

func handleEvents(
	w http.ResponseWriter,
	r *http.Request,
	bus *event.Bus,
) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(
			w,
			"streaming unsupported",
			http.StatusInternalServerError,
		)
		return
	}

	afterSequence := uint64(0)
	if lastEventID := r.Header.Get("Last-Event-ID"); lastEventID != "" {
		parsed, err := strconv.ParseUint(strings.TrimSpace(lastEventID), 10, 64)
		if err != nil {
			http.Error(w, "invalid Last-Event-ID", http.StatusBadRequest)
			return
		}
		afterSequence = parsed
	}

	w.Header().Set(
		"Content-Type",
		"text/event-stream",
	)
	w.Header().Set(
		"Cache-Control",
		"no-cache",
	)
	w.Header().Set(
		"Connection",
		"keep-alive",
	)

	// Parse optional event type filter from query parameters
	// e.g., /events?types=process.output,proxy.request
	var filter []event.EventType
	if typesParam := r.URL.Query().Get("types"); typesParam != "" {
		for _, t := range strings.Split(typesParam, ",") {
			t = strings.TrimSpace(t)
			if t != "" {
				filter = append(filter, event.EventType(t))
			}
		}
	}

	subscriber := bus.SubscribeFilteredFrom(filter, afterSequence)
	defer bus.Unsubscribe(subscriber)

	if _, err := fmt.Fprint(w, ": connected\n\n"); err != nil {
		return
	}
	flusher.Flush()

	ticker := time.NewTicker(eventBatchInterval)
	defer ticker.Stop()

	batch := make([]event.Event, 0, 256)
	flush := func() bool {
		if len(batch) == 0 {
			return true
		}

		// One SSE frame per flush keeps browser work bounded: a client at
		// ~1,000 lines/s receives about ten JSON arrays per second instead of
		// one thousand EventSource callbacks.
		data, err := json.Marshal(batch)
		if err != nil {
			batch = batch[:0]
			return true
		}

		if sequence := event.SequenceOf(batch[len(batch)-1]); sequence > 0 {
			if _, err := fmt.Fprintf(w, "id: %d\n", sequence); err != nil {
				return false
			}
		}

		if _, err := fmt.Fprintf(w, "event: devproxy\ndata: %s\n\n", data); err != nil {
			return false
		}

		batch = batch[:0]
		flusher.Flush()
		return true
	}

	for {
		select {
		case <-r.Context().Done():
			flush()
			return

		case <-ticker.C:
			if !flush() {
				return
			}

		case entry, open := <-subscriber:
			if !open {
				return
			}

			batch = append(batch, entry)
			if len(batch) >= maxEventBatch && !flush() {
				return
			}
		}
	}
}
