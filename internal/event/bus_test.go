package event

import (
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
	case got := <-sub:
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