package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/phy0hk/devproxy/internal/event"
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

	subscriber := bus.SubscribeFiltered(filter)
	defer bus.Unsubscribe(subscriber)

	fmt.Fprint(w, ": connected\n\n")
	flusher.Flush()

	for {
		select {
		case <-r.Context().Done():
			return

		case entry := <-subscriber:
			data, err := json.Marshal(entry)
			if err != nil {
				continue
			}

			fmt.Fprintf(
				w,
				"event: devproxy\n",
			)

			fmt.Fprintf(
				w,
				"data: %s\n\n",
				data,
			)

			flusher.Flush()
		}
	}
}
