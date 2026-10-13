package ws

import (
	"testing"
	"time"

	"zutto-pccom/apps/server/internal/world"
)

func TestSessionManagerResumeReplacesTransportGeneration(t *testing.T) {
	m := NewSessionManager(time.Minute)
	host := world.Host{ID: "host-1", Phone: "0451234567", Name: "TEST"}

	s, firstToken, err := m.Create(host, 14400, 2, nil, "")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	m.Detach(s.ID, firstToken)

	resumed, secondToken, result := m.Resume(s.ID)
	if result != "ok" {
		t.Fatalf("Resume result = %q, want ok", result)
	}
	if resumed != s {
		t.Fatal("Resume returned a different logical session")
	}
	if secondToken == firstToken {
		t.Fatal("resume did not replace the transport attachment generation")
	}
	if !m.IsCurrent(s.ID, secondToken) {
		t.Fatal("resumed transport is not current")
	}

	// A delayed close from the old WebSocket must not detach the replacement.
	m.Detach(s.ID, firstToken)
	if !m.IsCurrent(s.ID, secondToken) {
		t.Fatal("stale transport detached the replacement transport")
	}
}

func TestSessionManagerRejectsExpiredResume(t *testing.T) {
	m := NewSessionManager(time.Minute)
	s, token, err := m.Create(world.Host{ID: "host-2"}, 9600, 1, nil, "")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	m.Detach(s.ID, token)

	m.mu.Lock()
	s.ReconnectDeadline = time.Now().Add(-time.Second)
	m.mu.Unlock()

	if got, _, result := m.Resume(s.ID); got != nil || result != "expired" {
		t.Fatalf("Resume expired = (%v, %q), want (nil, expired)", got, result)
	}
	if m.IsCurrent(s.ID, token) {
		t.Fatal("expired session remained available")
	}
}

func TestSessionManagerEndRemovesSession(t *testing.T) {
	m := NewSessionManager(time.Minute)
	s, token, err := m.Create(world.Host{ID: "host-3"}, 28800, 4, nil, "")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if !m.IsCurrent(s.ID, token) {
		t.Fatal("new session is not attached")
	}

	m.End(s.ID)
	if got, _, result := m.Resume(s.ID); got != nil || result != "not_found" {
		t.Fatalf("Resume ended session = (%v, %q), want (nil, not_found)", got, result)
	}
}

// recordingLines is a LineOccupancy with a fixed number of free lines.
type recordingLines struct {
	free  int
	held  map[string]int
	ended []string
}

func (l *recordingLines) DialLine(world.Host, time.Time) (int, bool) { return 1, l.free > len(l.held) }
func (l *recordingLines) BeginHumanCall(_ world.Host, callID, _ string, _ int) (int, bool) {
	if len(l.held) >= l.free {
		return 0, false
	}
	if l.held == nil {
		l.held = map[string]int{}
	}
	l.held[callID] = len(l.held) + 1
	return l.held[callID], true
}
func (l *recordingLines) SetHumanCallHandle(string, string) {}
func (l *recordingLines) EndHumanCall(callID string) {
	delete(l.held, callID)
	l.ended = append(l.ended, callID)
}

func TestSessionManagerHoldsAndReleasesLines(t *testing.T) {
	lines := &recordingLines{free: 1}
	m := NewSessionManager(time.Minute)
	m.SetLineOccupancy(lines)
	host := world.Host{ID: "host-3"}

	s, token, err := m.Create(host, 9600, 7, nil, "")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if s.Line != 1 {
		t.Fatalf("line = %d, want the line the world assigned", s.Line)
	}
	if _, _, err := m.Create(host, 9600, 1, nil, ""); err != ErrLineBusy {
		t.Fatalf("second Create err = %v, want ErrLineBusy", err)
	}
	m.End(s.ID)
	if len(lines.ended) != 1 || lines.ended[0] != s.ID {
		t.Fatalf("hang-up did not release the line: %v", lines.ended)
	}

	s2, token2, err := m.Create(host, 9600, 1, nil, "")
	if err != nil {
		t.Fatalf("Create after hang-up: %v", err)
	}
	_ = token
	m.Detach(s2.ID, token2)
	m.mu.Lock()
	s2.ReconnectDeadline = time.Now().Add(-time.Second)
	m.mu.Unlock()
	m.expire(s2.ID, token2)
	if len(lines.ended) != 2 || lines.ended[1] != s2.ID {
		t.Fatalf("expiry did not release the line: %v", lines.ended)
	}
}
