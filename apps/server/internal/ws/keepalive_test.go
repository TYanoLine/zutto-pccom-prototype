package ws

import (
	"context"
	"testing"
	"time"
)

func TestKeepaliveLoopSendsPingMessages(t *testing.T) {
	col := &collector{}
	done := make(chan struct{})
	defer close(done)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go keepaliveLoop(ctx, done, 10*time.Millisecond, col.send)

	waitFor(t, func() bool {
		return col.count() >= 2
	})

	col.mu.Lock()
	defer col.mu.Unlock()
	for _, msg := range col.msgs {
		if msg.Type != "ping" {
			t.Fatalf("expected ping message, got %q", msg.Type)
		}
	}
}

func TestKeepaliveLoopStopsOnDone(t *testing.T) {
	col := &collector{}
	done := make(chan struct{})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go keepaliveLoop(ctx, done, 10*time.Millisecond, col.send)

	waitFor(t, func() bool {
		return col.count() >= 1
	})

	close(done)
	time.Sleep(30 * time.Millisecond)

	countBefore := col.count()
	time.Sleep(30 * time.Millisecond)
	countAfter := col.count()

	if countAfter != countBefore {
		t.Fatalf("expected keepalive loop to stop after done closed, but count grew from %d to %d", countBefore, countAfter)
	}
}
