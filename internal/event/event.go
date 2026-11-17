package event

import "time"

// ProcessEvent represents a process lifecycle or output event.
type ProcessEvent struct {
	EventType EventType `json:"type"`
	Timestamp time.Time `json:"timestamp"`
	Process   string    `json:"process"`
	RunID     string    `json:"run_id,omitempty"`
	Sequence  uint64    `json:"sequence,omitempty"`
	Stream    string    `json:"stream,omitempty"`
	Message   string    `json:"message,omitempty"`
	ExitCode  int       `json:"exit_code,omitempty"`
	Error     string    `json:"error,omitempty"`
}

// Type returns the event type for filtering.
func (e ProcessEvent) Type() EventType {
	return e.EventType
}

// SequenceNumber returns the stream sequence used for SSE resume.
func (e ProcessEvent) SequenceNumber() uint64 {
	return e.Sequence
}

// RequestEvent represents a proxy request event.
type RequestEvent struct {
	EventType    EventType `json:"type"`
	ID           string    `json:"id"`
	Timestamp    time.Time `json:"timestamp"`
	Sequence     uint64    `json:"sequence,omitempty"`
	Method       string    `json:"method"`
	Path         string    `json:"path"`
	Route        string    `json:"route"`
	Upstream     string    `json:"upstream"`
	Status       int       `json:"status"`
	DurationMS   int64     `json:"duration_ms"`
	RequestSize  int64     `json:"request_size"`
	ResponseSize int64     `json:"response_size"`
}

// Type returns the event type for filtering.
func (e RequestEvent) Type() EventType {
	return e.EventType
}

// SequenceNumber returns the stream sequence used for SSE resume.
func (e RequestEvent) SequenceNumber() uint64 {
	return e.Sequence
}

// sequenced is implemented by events that carry a stream sequence.
type sequenced interface {
	SequenceNumber() uint64
}

// SequenceOf returns the event's stream sequence, or zero when it has none.
func SequenceOf(event Event) uint64 {
	if event == nil {
		return 0
	}
	if sequencedEvent, ok := event.(sequenced); ok {
		return sequencedEvent.SequenceNumber()
	}
	return 0
}
