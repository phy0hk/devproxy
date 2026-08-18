package event

import "time"

type RequestEvent struct {
	ID           string    `json:"id"`
	Timestamp    time.Time `json:"timestamp"`
	Method       string    `json:"method"`
	Path         string    `json:"path"`
	Route        string    `json:"route"`
	Upstream     string    `json:"upstream"`
	Status       int       `json:"status"`
	DurationMS   int64     `json:"duration_ms"`
	RequestSize  int64     `json:"request_size"`
	ResponseSize int64     `json:"response_size"`
}
