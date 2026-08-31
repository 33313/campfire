package stream

import (
	"testing"
	"time"
)

func TestPublishReachesEverySubscriber(t *testing.T) {
	bus := NewBus()
	a, _, cancelA := bus.Subscribe()
	b, _, cancelB := bus.Subscribe()
	defer cancelA()
	defer cancelB()

	bus.Publish(Notification{Seq: 7, Kind: "item.created", Identifier: "SOL-1", TeamID: 1})
	for i, ch := range []<-chan Notification{a, b} {
		select {
		case n := <-ch:
			if n.Seq != 7 || n.Identifier != "SOL-1" {
				t.Fatalf("subscriber %d got %+v", i, n)
			}
		case <-time.After(time.Second):
			t.Fatalf("subscriber %d got nothing", i)
		}
	}
}

func TestCancelStopsDelivery(t *testing.T) {
	bus := NewBus()
	ch, _, cancel := bus.Subscribe()
	cancel()
	if bus.Subscribers() != 0 {
		t.Fatal("the subscriber was not removed")
	}
	bus.Publish(Notification{Seq: 1})
	select {
	case n, ok := <-ch:
		if ok {
			t.Fatalf("a cancelled subscriber received %+v", n)
		}
	default:
	}
	// Cancelling twice is not an error: the handler defers it and may also
	// have hit an error path.
	cancel()
}

// A subscriber that falls behind is reset rather than quietly skipped: a
// silent partial replay MUST NOT occur (SPEC.md §12.2).
func TestASlowSubscriberIsResetRatherThanSkipped(t *testing.T) {
	bus := NewBus()
	ch, reset, cancel := bus.Subscribe()
	defer cancel()

	for i := 0; i < queue+50; i++ {
		bus.Publish(Notification{Seq: int64(i + 1)})
	}
	select {
	case <-reset:
	default:
		t.Fatal("an overrun subscriber was not told to reset")
	}
	// What it did receive is a prefix, in order, with no gap — the gap is what
	// the reset is for.
	var last int64
	for {
		select {
		case n := <-ch:
			if n.Seq != last+1 {
				t.Fatalf("delivered %d after %d: the queue dropped from the middle", n.Seq, last)
			}
			last = n.Seq
			continue
		default:
		}
		break
	}
	if last == 0 {
		t.Fatal("nothing was delivered at all")
	}
	// Nothing more is delivered to it once it is over.
	bus.Publish(Notification{Seq: 9999})
	select {
	case n := <-ch:
		t.Fatalf("an overrun subscriber kept receiving: %+v", n)
	default:
	}
}

// Publishing must never block on a subscriber, because it happens on the
// request goroutine that just committed a write.
func TestPublishNeverBlocks(t *testing.T) {
	bus := NewBus()
	_, _, cancel := bus.Subscribe()
	defer cancel()
	done := make(chan struct{})
	go func() {
		for i := 0; i < queue*4; i++ {
			bus.Publish(Notification{Seq: int64(i + 1)})
		}
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Publish blocked on a subscriber that was not reading")
	}
}
