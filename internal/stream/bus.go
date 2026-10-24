// Package stream fans audit-log events out to connected browsers.
//
// Everything here is an accelerator. Every screen must be correct if the
// stream never connects (SPEC.md §12.4), so nothing in this package holds
// state any read depends on: a subscriber that misses a notification is told
// to refetch, and refetching is what the client does anyway.
package stream

import "sync"

// Notification is what goes down the wire: a sequence number, an event kind,
// an item identifier and an owning team, and nothing else. Kind "reset" tells
// the client to discard its state and refetch: the subscriber has fallen so
// far behind that replaying the gap is worse than starting again.
//
// Thin by rule (SPEC.md §12.3). Carrying entity contents would let the stream
// disclose data the recipient may not read, would break every time an entity's
// shape changed, and would make the stream a second path that renders data.
type Notification struct {
	Seq        int64  `json:"seq"`
	Kind       string `json:"kind"`
	Identifier string `json:"identifier,omitempty"`
	TeamID     int64  `json:"teamId,omitempty"`
}

// queue is how much a slow subscriber may fall behind before it is reset. A
// browser that cannot keep up with this is not going to be helped by a longer
// queue; it is going to be helped by one refetch.
const queue = 256

type subscriber struct {
	ch chan Notification
	// reset fires when the subscriber has fallen behind. It is separate from
	// ch so that the signal cannot itself be dropped by a full queue.
	reset chan struct{}
	over  bool
}

// Bus is the fan-out. It is a plain in-process hub because a single-node
// deployment is the whole architecture (SPEC.md §13.4); nothing about
// correctness depends on it.
type Bus struct {
	mu   sync.Mutex
	next int64
	subs map[int64]*subscriber
}

func NewBus() *Bus { return &Bus{subs: map[int64]*subscriber{}} }

// Subscribe returns a channel of notifications, a channel that fires when the
// subscriber has fallen too far behind to be caught up, and the function that
// unsubscribes. The caller must call cancel, once.
func (b *Bus) Subscribe() (<-chan Notification, <-chan struct{}, func()) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.next++
	id := b.next
	s := &subscriber{ch: make(chan Notification, queue), reset: make(chan struct{}, 1)}
	b.subs[id] = s
	return s.ch, s.reset, func() {
		b.mu.Lock()
		defer b.mu.Unlock()
		delete(b.subs, id)
	}
}

// Publish delivers to every subscriber, without blocking on any of them.
//
// A subscriber whose queue is full is sent a reset instead and dropped: a
// silent partial replay MUST NOT occur (SPEC.md §12.2), so a gap becomes an
// explicit instruction to refetch.
func (b *Bus) Publish(n Notification) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, s := range b.subs {
		if s.over {
			continue
		}
		select {
		case s.ch <- n:
		default:
			s.over = true
			select {
			case s.reset <- struct{}{}:
			default:
			}
		}
	}
}

// Subscribers is the current connection count, for the administration screen.
func (b *Bus) Subscribers() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.subs)
}
