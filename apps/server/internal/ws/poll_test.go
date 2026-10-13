package ws

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"zutto-pccom/apps/server/internal/world"
)

// pollingRuntime emits one numbered line per Poll and counts lines in
// HandleLine; a data race between the two would show under -race.
type pollingRuntime struct {
	polls int
	lines int
}

func (r *pollingRuntime) Welcome() string { return "" }

func (r *pollingRuntime) HandleLine(string) (string, bool) {
	r.lines++
	return fmt.Sprintf("line %d", r.lines), false
}

func (r *pollingRuntime) Poll(time.Time) string {
	r.polls++
	return fmt.Sprintf("async %d", r.polls)
}

type collector struct {
	mu   sync.Mutex
	msgs []serverMessage
}

func (c *collector) send(msg serverMessage) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.msgs = append(c.msgs, msg)
	return nil
}

func (c *collector) count() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.msgs)
}

func waitFor(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatal("condition not reached")
		}
		time.Sleep(time.Millisecond)
	}
}

func TestPollSendsAsynchronousOutputAsTerminalText(t *testing.T) {
	m := NewSessionManager(time.Minute)
	call, token, err := m.Create(world.Host{ID: "poll"}, 9600, 1, &pollingRuntime{}, "")
	if err != nil {
		t.Fatal(err)
	}
	var out collector
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go pollCall(ctx, m, call, token, time.Millisecond, time.Now, out.send)

	waitFor(t, func() bool { return out.count() >= 2 })
	out.mu.Lock()
	first := out.msgs[0]
	out.mu.Unlock()
	if first.Type != "terminal" || first.Text != "async 1" {
		t.Fatalf("first message = %+v, want terminal \"async 1\"", first)
	}
}

func TestPollStopsWhenTheCallEndsOrMovesToAnotherConnection(t *testing.T) {
	m := NewSessionManager(time.Minute)
	call, token, err := m.Create(world.Host{ID: "poll"}, 9600, 1, &pollingRuntime{}, "")
	if err != nil {
		t.Fatal(err)
	}
	var out collector
	done := make(chan struct{})
	go func() {
		pollCall(context.Background(), m, call, token, time.Millisecond, time.Now, out.send)
		close(done)
	}()
	waitFor(t, func() bool { return out.count() >= 1 })
	m.Detach(call.ID, token)
	if _, _, result := m.Resume(call.ID); result != "ok" {
		t.Fatalf("resume = %q", result)
	}
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("poller kept running for a superseded connection")
	}
}

func TestPollDoesNotRunForProgramsWithoutAsyncOutput(t *testing.T) {
	m := NewSessionManager(time.Minute)
	call, token, err := m.Create(world.Host{ID: "plain"}, 9600, 1, plainRuntime{}, "")
	if err != nil {
		t.Fatal(err)
	}
	var out collector
	pollCall(context.Background(), m, call, token, time.Millisecond, time.Now, out.send)
	if out.count() != 0 {
		t.Fatalf("sent %d messages for a program without AsyncOutput", out.count())
	}
}

type plainRuntime struct{}

func (plainRuntime) Welcome() string                  { return "" }
func (plainRuntime) HandleLine(string) (string, bool) { return "", false }

// Input lines and polls may arrive at the same time; the call lock keeps the
// runtime single-threaded (run with -race).
func TestPollAndInputDoNotRunConcurrently(t *testing.T) {
	m := NewSessionManager(time.Minute)
	rt := &pollingRuntime{}
	call, token, err := m.Create(world.Host{ID: "poll"}, 9600, 1, rt, "")
	if err != nil {
		t.Fatal(err)
	}
	var out collector
	ctx, cancel := context.WithCancel(context.Background())
	go pollCall(ctx, m, call, token, time.Microsecond, time.Now, out.send)
	for i := 0; i < 200; i++ {
		call.HandleLine("x")
	}
	cancel()
	call.mu.Lock()
	lines := rt.lines
	call.mu.Unlock()
	if lines != 200 {
		t.Fatalf("lines = %d, want 200", lines)
	}
}
