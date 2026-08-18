package event

import "sync"

type Event interface{}

type Subscriber chan Event

type Bus struct {
	mu          sync.RWMutex
	subscribers map[Subscriber]struct{}
}

func NewBus() *Bus {
	return &Bus{
		subscribers: make(map[Subscriber]struct{}),
	}
}

func (b *Bus) Subscribe() Subscriber {
	b.mu.Lock()
	defer b.mu.Unlock()

	ch := make(Subscriber, 16)

	b.subscribers[ch] = struct{}{}

	return ch
}

func (b *Bus) Unsubscribe(ch Subscriber) {
	b.mu.Lock()
	defer b.mu.Unlock()

	delete(b.subscribers, ch)
	close(ch)
}

func (b *Bus) Publish(event Event) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	for subscriber := range b.subscribers {
		select {
		case subscriber <- event:
		default:
			// Don't allow a slow subscriber to block the proxy.
		}
	}
}
