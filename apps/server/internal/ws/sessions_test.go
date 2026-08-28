package ws

import (
	"testing"
	"time"

	"zutto-pccom/apps/server/internal/world"
)

func TestSessionManagerResumeReplacesTransportGeneration(t *testing.T) {
	m := NewSessionManager(time.Minute)
	host := world.Host{ID: "host-1", Phone: "0451234567", Name: "TEST"}

	s, firstToken, err := m.Create(host, 14400, 2, nil)
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
	s, token, err := m.Create(world.Host{ID: "host-2"}, 9600, 1, nil)
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
	s, token, err := m.Create(world.Host{ID: "host-3"}, 28800, 4, nil)
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
