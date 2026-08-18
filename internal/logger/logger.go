package logger

import (
	"log"
	"net/http"
	"time"

	"github.com/phy0hk/devproxy/internal/event"
)

type Logger struct {
	bus *event.Bus
}

func New(bus *event.Bus) *Logger {
	return &Logger{
		bus: bus,
	}
}

type responseWriter struct {
	http.ResponseWriter
	status       int
	responseSize int64
}

func (w *responseWriter) WriteHeader(status int) {
	w.status = status

	w.ResponseWriter.WriteHeader(status)
}

func (w *responseWriter) Write(data []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}

	n, err := w.ResponseWriter.Write(data)

	w.responseSize += int64(n)

	return n, err
}

func (l *Logger) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(
		w http.ResponseWriter,
		r *http.Request,
	) {
		start := time.Now()

		writer := &responseWriter{
			ResponseWriter: w,
		}

		next.ServeHTTP(writer, r)

		status := writer.status

		if status == 0 {
			status = http.StatusOK
		}

		entry := event.RequestEvent{
			Type:         "proxy.request",
			ID:           "",
			Timestamp:    start,
			Method:       r.Method,
			Path:         r.URL.RequestURI(),
			Route:        writer.Header().Get("X-DevProxy-Route"),
			Upstream:     writer.Header().Get("X-DevProxy-Upstream"),
			Status:       status,
			DurationMS:   time.Since(start).Milliseconds(),
			RequestSize:  r.ContentLength,
			ResponseSize: writer.responseSize,
		}

		if entry.Route == "" {
			log.Printf(
				"%s %s -> %d no-route (%dms)",
				entry.Method,
				entry.Path,
				entry.Status,
				entry.DurationMS,
			)
		} else {
			log.Printf(
				"%s %s -> %d route=%s upstream=%s (%dms)",
				entry.Method,
				entry.Path,
				entry.Status,
				entry.Route,
				entry.Upstream,
				entry.DurationMS,
			)
		}

		if l.bus != nil {
			l.bus.Publish(entry)
		}
	})
}
