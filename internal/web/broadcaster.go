// Package web contains the HTTP handler and SSE broadcaster.
package web

import (
	"sync"

	"github.com/maxlesscode/ping-itor/internal/checker"
)

// Broadcaster distributes check results to all active SSE subscribers.
// Slow subscribers are silently skipped (non-blocking send).
type Broadcaster struct {
	mu   sync.Mutex
	subs map[uint64]chan checker.CheckResult
	next uint64
}

// NewBroadcaster creates a ready-to-use Broadcaster.
func NewBroadcaster() *Broadcaster {
	return &Broadcaster{
		subs: make(map[uint64]chan checker.CheckResult),
	}
}

// Subscribe returns a receive-only channel and an idempotent unsubscribe
// function. The caller must invoke unsub when done to avoid leaking goroutines
// and channels. Complies with CC-1: the sender (Broadcaster) closes the channel.
func (b *Broadcaster) Subscribe() (ch <-chan checker.CheckResult, unsub func()) {
	b.mu.Lock()
	id := b.next
	b.next++
	c := make(chan checker.CheckResult, 16)
	b.subs[id] = c
	b.mu.Unlock()

	var once sync.Once
	unsub = func() {
		once.Do(func() {
			b.mu.Lock()
			delete(b.subs, id)
			b.mu.Unlock()
			close(c) // sender closes — CC-1
		})
	}

	return c, unsub
}

// Publish sends a result to every current subscriber. Subscribers that are not
// ready to receive are skipped to prevent blocking.
func (b *Broadcaster) Publish(r checker.CheckResult) {
	b.mu.Lock()
	defer b.mu.Unlock()

	for _, ch := range b.subs {
		select {
		case ch <- r:
		default:
			// Subscriber too slow; drop the event.
		}
	}
}
