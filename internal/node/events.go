package node

import "sync"

// Event is pushed to every connected UI over Server-Sent Events.
type Event struct {
	Type string `json:"type"`
	Data any    `json:"data,omitempty"`
}

// Bus is a tiny fan-out publisher. Slow subscribers drop events rather than
// blocking the node; the UI re-syncs from /api/state whenever it reconnects.
type Bus struct {
	mu   sync.Mutex
	subs map[chan Event]struct{}
}

func newBus() *Bus { return &Bus{subs: map[chan Event]struct{}{}} }

func (b *Bus) Subscribe() (<-chan Event, func()) {
	ch := make(chan Event, 64)
	b.mu.Lock()
	b.subs[ch] = struct{}{}
	b.mu.Unlock()
	return ch, func() {
		b.mu.Lock()
		if _, ok := b.subs[ch]; ok {
			delete(b.subs, ch)
			close(ch)
		}
		b.mu.Unlock()
	}
}

func (b *Bus) Publish(typ string, data any) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for ch := range b.subs {
		select {
		case ch <- Event{Type: typ, Data: data}:
		default:
		}
	}
}
