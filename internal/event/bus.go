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

const subscriberBuffer = 1024

// subscription holds a subscriber and its filter.
type subscription struct {
	ch       Subscriber
	filter   map[EventType]struct{}
	noFilter bool // true means receive all events

	// mu prevents a publish/replay from sending to a channel while
	// Unsubscribe closes it.
	mu     sync.Mutex
	closed bool
}

// send performs a non-blocking send and reports whether the event was queued.
func (s *subscription) send(event Event) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return false
	}

	select {
	case s.ch <- event:
		return true
	default:
		return false
	}
}

// close closes the subscriber channel exactly once.
func (s *subscription) close() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return
	}

	s.closed = true
	close(s.ch)
}

// Bus publishes events to subscribers with optional type filtering.
type Bus struct {
	// Atomic pointer to the current subscriptions snapshot for lock-free reads.
	subscriptions atomic.Pointer[subscriptionsSnapshot]

	// Mutex protects modifications to the subscription list and serializes
	// history replay with publishes so a joining subscriber sees every event
	// exactly once. Publishes take the read lock and still run concurrently.
	mu sync.RWMutex

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
	if capacity < 1 {
		capacity = 1
	}

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

func (rb *ringBuffer) snapshotFiltered(
	filter map[EventType]struct{},
	afterSequence uint64,
) []Event {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	if rb.size == 0 {
		return nil
	}

	result := make([]Event, 0, rb.size)
	iterate := func(i int) {
		event := rb.events[i]
		if event == nil {
			return
		}

		if len(filter) > 0 {
			if _, ok := filter[event.Type()]; !ok {
				return
			}
		}

		if afterSequence > 0 && SequenceOf(event) <= afterSequence {
			return
		}

		result = append(result, event)
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

func (rb *ringBuffer) count() int {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	return rb.size
}

// NewBus creates a new event bus with a bounded history large enough to
// bridge short SSE reconnects.
func NewBus() *Bus {
	return NewBusWithHistory(20000)
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
	return b.SubscribeFilteredFrom(eventTypes, 0)
}

// SubscribeFilteredFrom creates a filtered subscriber and replays only events
// newer than afterSequence. Replay runs synchronously while the subscription
// is being registered, so no goroutine is left behind if the caller
// immediately unsubscribes.
func (b *Bus) SubscribeFilteredFrom(
	eventTypes []EventType,
	afterSequence uint64,
) Subscriber {
	b.mu.Lock()
	defer b.mu.Unlock()

	// Subscribers can buffer at least the whole bounded history so replaying
	// the retained window after Last-Event-ID never drops its oldest events.
	channelSize := subscriberBuffer
	if b.history.capacity > channelSize {
		channelSize = b.history.capacity
	}
	ch := make(Subscriber, channelSize)

	filter := make(map[EventType]struct{}, len(eventTypes))
	for _, t := range eventTypes {
		filter[t] = struct{}{}
	}

	sub := &subscription{
		ch:       ch,
		filter:   filter,
		noFilter: len(filter) == 0,
	}

	// Get current snapshot and create a new one with the added subscription.
	// Copy the slices so an old snapshot is never mutated in place.
	old := b.subscriptions.Load()
	newAll := make([]*subscription, 0, len(old.all)+1)
	newAll = append(newAll, old.all...)
	newAll = append(newAll, sub)

	newByType := make(map[EventType][]*subscription, len(old.byType)+len(filter))
	for k, v := range old.byType {
		newByType[k] = append([]*subscription(nil), v...)
	}

	if !sub.noFilter {
		for t := range filter {
			newByType[t] = append(newByType[t], sub)
		}
	}

	// Hold the subscription lock while registering and replaying. Publishers
	// targeting this subscription wait for replay to finish instead of
	// delivering a live event before its history.
	sub.mu.Lock()
	b.subscriptions.Store(&subscriptionsSnapshot{
		all:    newAll,
		byType: newByType,
	})
	b.replayHistoryLocked(sub, filter, afterSequence)
	sub.mu.Unlock()

	return ch
}

// replayHistoryLocked sends recent events newer than afterSequence. The caller
// must hold sub.mu.
func (b *Bus) replayHistoryLocked(
	sub *subscription,
	filter map[EventType]struct{},
	afterSequence uint64,
) {
	events := b.history.snapshotFiltered(filter, afterSequence)
	if len(events) == 0 {
		return
	}

	// Prefer the newest events when history exceeds the channel buffer.
	if len(events) > cap(sub.ch) {
		events = events[len(events)-cap(sub.ch):]
	}

	for _, event := range events {
		select {
		case sub.ch <- event:
		default:
			return
		}
	}
}

// Unsubscribe removes a subscriber from the bus and closes its channel.
func (b *Bus) Unsubscribe(ch Subscriber) {
	b.mu.Lock()
	defer b.mu.Unlock()

	old := b.subscriptions.Load()

	// Find and remove the subscription.
	var found *subscription
	newAll := make([]*subscription, 0, len(old.all))
	for _, sub := range old.all {
		if sub.ch == ch {
			found = sub
		} else {
			newAll = append(newAll, sub)
		}
	}

	if found == nil {
		return // Not found
	}

	// Rebuild byType map.
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

	found.close()
}

// Publish sends an event to all matching subscribers.
// This is lock-free for the read path.
func (b *Bus) Publish(event Event) {
	if event == nil {
		return
	}

	// Hold the read lock across history insertion and delivery. Subscribe
	// replays history while holding the write lock, so an event can never be
	// both missed by replay and skipped by the live snapshot.
	b.mu.RLock()
	defer b.mu.RUnlock()

	// Store in history for late joiners.
	b.history.push(event)

	snapshot := b.subscriptions.Load()
	if snapshot == nil {
		return
	}

	// Build the effective subscriber list without mutating any snapshot
	// slice in place.
	subs := make([]*subscription, 0, len(snapshot.all))
	seen := make(map[*subscription]struct{}, len(snapshot.all))

	for _, sub := range snapshot.byType[event.Type()] {
		subs = append(subs, sub)
		seen[sub] = struct{}{}
	}

	for _, sub := range snapshot.all {
		if !sub.noFilter {
			continue
		}
		if _, ok := seen[sub]; ok {
			continue
		}
		subs = append(subs, sub)
		seen[sub] = struct{}{}
	}

	for _, sub := range subs {
		// Don't allow a slow subscriber to block the publisher.
		// Event is dropped for this subscriber.
		sub.send(event)
	}
}

// HistoryAfter returns buffered events newer than afterSequence, optionally
// filtered by type. Results are ordered oldest to newest.
func (b *Bus) HistoryAfter(
	eventTypes []EventType,
	afterSequence uint64,
) []Event {
	filter := make(map[EventType]struct{}, len(eventTypes))
	for _, t := range eventTypes {
		filter[t] = struct{}{}
	}
	if len(filter) == 0 {
		filter = nil
	}

	return b.history.snapshotFiltered(filter, afterSequence)
}

// LatestSequence returns the sequence of the newest buffered event, or zero
// when history is empty. Consumers that load history separately can subscribe
// from this point to avoid replaying events twice.
func (b *Bus) LatestSequence() uint64 {
	events := b.history.snapshotFiltered(nil, 0)
	if len(events) == 0 {
		return 0
	}
	return SequenceOf(events[len(events)-1])
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
	return b.history.count()
}

// HistoryCapacity returns the maximum capacity of the history buffer.
func (b *Bus) HistoryCapacity() int {
	return b.history.capacity
}
