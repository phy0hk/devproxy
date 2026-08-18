package event

import "time"

type ProcessEvent struct {
	Type      string    `json:"type"`
	Timestamp time.Time `json:"timestamp"`
	Process   string    `json:"process"`
	Stream    string    `json:"stream,omitempty"`
	Message   string    `json:"message,omitempty"`
	ExitCode  int       `json:"exit_code,omitempty"`
	Error     string    `json:"error,omitempty"`
}

type RequestEvent struct {
	Type         string    `json:"type"`
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
