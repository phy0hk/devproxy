package event

import (
	"sync"
	"sync/atomic"
)

// EventType identifies the type of an event for filtering.
type EventType string

const (
	// Process event types
	TypeProcessStarted   EventType = "process.started"
	TypeProcessStopping  EventType = "process.stopping"
	TypeProcessExited    EventType = "process.exited"
	TypeProcessFailed    EventType = "process.failed"
	TypeProcessOutput    EventType = "process.output"
	TypeProcessOutputErr EventType = "process.output_error"

	// Proxy event types
	TypeProxyRequest EventType = "proxy.request"
	TypeProxyError   EventType = "proxy.error"
)

// Event is the interface all events must implement.
type Event interface {
	Type() EventType
}

// Subscriber receives events from the bus.
type Subscriber chan Event

// subscription holds a subscriber and its filter.
type subscription struct {
	ch       Subscriber
	filter   map[EventType]struct{}
	noFilter bool // true means receive all events
}

// Bus publishes events to subscribers with optional type filtering.
type Bus struct {
	// Atomic pointer to the current subscriptions snapshot for lock-free reads.
	subscriptions atomic.Pointer[subscriptionsSnapshot]

	// Mutex protects modifications to the subscription list.
	mu sync.Mutex

	// Ring buffer for recent event history (for late-joining subscribers).
	history *ringBuffer
}

// subscriptionsSnapshot is an immutable snapshot of subscriptions for lock-free publishing.
type subscriptionsSnapshot struct {
	all    []*subscription
	byType map[EventType][]*subscription
}

// ringBuffer stores recent events for replay to new subscribers.
type ringBuffer struct {
	mu       sync.Mutex
	events   []Event
	capacity int
	size     int
	head     int
}

func newRingBuffer(capacity int) *ringBuffer {
	return &ringBuffer{
		events:   make([]Event, capacity),
		capacity: capacity,
	}
}

func (rb *ringBuffer) push(event Event) {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	rb.events[rb.head] = event
	rb.head = (rb.head + 1) % rb.capacity
	if rb.size < rb.capacity {
		rb.size++
	}
}

func (rb *ringBuffer) snapshot() []Event {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	if rb.size == 0 {
		return nil
	}

	result := make([]Event, rb.size)
	if rb.size < rb.capacity {
		copy(result, rb.events[:rb.size])
	} else {
		copy(result, rb.events[rb.head:])
		copy(result[rb.capacity-rb.head:], rb.events[:rb.head])
	}
	return result
}

func (rb *ringBuffer) snapshotFiltered(filter map[EventType]struct{}) []Event {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	if rb.size == 0 {
		return nil
	}

	var result []Event
	iterate := func(i int) {
		event := rb.events[i]
		if filter == nil {
			result = append(result, event)
		} else {
			if _, ok := filter[event.Type()]; ok {
				result = append(result, event)
			}
		}
	}

	if rb.size < rb.capacity {
		for i := 0; i < rb.size; i++ {
			iterate(i)
		}
	} else {
		for i := rb.head; i < rb.capacity; i++ {
			iterate(i)
		}
		for i := 0; i < rb.head; i++ {
			iterate(i)
		}
	}
	return result
}

// NewBus creates a new event bus with a default history capacity of 1000 events.
func NewBus() *Bus {
	return NewBusWithHistory(1000)
}

// NewBusWithHistory creates a new event bus with the specified history capacity.
func NewBusWithHistory(historyCapacity int) *Bus {
	b := &Bus{
		history: newRingBuffer(historyCapacity),
	}
	// Initialize with empty snapshot
	b.subscriptions.Store(&subscriptionsSnapshot{
		all:    nil,
		byType: make(map[EventType][]*subscription),
	})
	return b
}

// Subscribe creates a new subscriber that receives all events.
func (b *Bus) Subscribe() Subscriber {
	return b.SubscribeFiltered(nil)
}

// SubscribeFiltered creates a new subscriber that only receives events matching the given types.
// Pass nil or empty slice to receive all events.
func (b *Bus) SubscribeFiltered(eventTypes []EventType) Subscriber {
	b.mu.Lock()
	defer b.mu.Unlock()

	ch := make(Subscriber, 64) // Larger buffer for burst handling

	filter := make(map[EventType]struct{}, len(eventTypes))
	for _, t := range eventTypes {
		filter[t] = struct{}{}
	}

	sub := &subscription{
		ch:       ch,
		filter:   filter,
		noFilter: len(filter) == 0,
	}

	// Get current snapshot and create a new one with the added subscription
	old := b.subscriptions.Load()
	newAll := append(old.all, sub)

	newByType := make(map[EventType][]*subscription, len(old.byType))
	for k, v := range old.byType {
		newByType[k] = append([]*subscription(nil), v...)
	}

	if !sub.noFilter {
		for t := range filter {
			newByType[t] = append(newByType[t], sub)
		}
	}

	b.subscriptions.Store(&subscriptionsSnapshot{
		all:    newAll,
		byType: newByType,
	})

	// Replay recent history to the new subscriber
	go b.replayHistory(ch, filter)

	return ch
}

// replayHistory sends recent events to a newly subscribed channel.
// It recovers from panics caused by sending to a closed channel (when Unsubscribe is called).
func (b *Bus) replayHistory(ch Subscriber, filter map[EventType]struct{}) {
	defer func() {
		// Recover from panic when sending to a closed channel.
		// This can happen if Unsubscribe is called while replay is in progress.
		recover()
	}()

	events := b.history.snapshotFiltered(filter)
	for _, event := range events {
		select {
		case ch <- event:
		default:
			// Subscriber not ready or buffer full, stop replay
			return
		}
	}
}

// Unsubscribe removes a subscriber from the bus.
func (b *Bus) Unsubscribe(ch Subscriber) {
	b.mu.Lock()
	defer b.mu.Unlock()

	old := b.subscriptions.Load()

	// Find and remove the subscription
	var found *subscription
	newAll := make([]*subscription, 0, len(old.all))
	for _, sub := range old.all {
		if sub.ch == ch {
			found = sub
			close(ch)
		} else {
			newAll = append(newAll, sub)
		}
	}

	if found == nil {
		return // Not found
	}

	// Rebuild byType map
	newByType := make(map[EventType][]*subscription, len(old.byType))
	for t, subs := range old.byType {
		newSubs := make([]*subscription, 0, len(subs))
		for _, sub := range subs {
			if sub != found {
				newSubs = append(newSubs, sub)
			}
		}
		if len(newSubs) > 0 {
			newByType[t] = newSubs
		}
	}

	b.subscriptions.Store(&subscriptionsSnapshot{
		all:    newAll,
		byType: newByType,
	})
}

// Publish sends an event to all matching subscribers.
// This is lock-free for the read path.
func (b *Bus) Publish(event Event) {
	// Store in history for late joiners
	b.history.push(event)

	snapshot := b.subscriptions.Load()
	if snapshot == nil {
		return
	}

	var subs []*subscription
	if event != nil {
		if typeSubs, ok := snapshot.byType[event.Type()]; ok {
			subs = typeSubs
		}
	}

	// Also send to subscribers with no filter (receive all events)
	if len(snapshot.all) > len(subs) {
		// There are subscribers with no filter
		allSubs := snapshot.all
		if len(subs) == 0 {
			subs = allSubs
		} else {
			// Merge: we need to send to both filtered and unfiltered subscribers
			// Create a combined list without duplicates
			seen := make(map[*subscription]struct{}, len(allSubs))
			for _, sub := range subs {
				seen[sub] = struct{}{}
			}
			for _, sub := range allSubs {
				if sub.noFilter {
					if _, ok := seen[sub]; !ok {
						subs = append(subs, sub)
					}
				}
			}
		}
	}

	for _, sub := range subs {
		select {
		case sub.ch <- event:
		default:
			// Don't allow a slow subscriber to block the publisher.
			// Event is dropped for this subscriber.
		}
	}
}

// SubscriberCount returns the current number of subscribers.
func (b *Bus) SubscriberCount() int {
	snapshot := b.subscriptions.Load()
	if snapshot == nil {
		return 0
	}
	return len(snapshot.all)
}

// HistorySize returns the number of events currently in the history buffer.
func (b *Bus) HistorySize() int {
	return b.history.size
}

// HistoryCapacity returns the maximum capacity of the history buffer.
func (b *Bus) HistoryCapacity() int {
	return b.history.capacity
}
