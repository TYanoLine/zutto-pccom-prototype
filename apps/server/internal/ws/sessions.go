package ws

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"

	"zutto-pccom/apps/server/internal/bbs"
	"zutto-pccom/apps/server/internal/world"
)

const DefaultReconnectGrace = 60 * time.Second

type CallSession struct {
	ID                string
	Host              world.Host
	Baud              int
	Line              int
	Runtime           *bbs.Runtime
	ConnectedAt       time.Time
	ReconnectDeadline time.Time

	attached   bool
	attachment uint64
	expiry     *time.Timer
}

type SessionManager struct {
	mu             sync.Mutex
	sessions       map[string]*CallSession
	reconnectGrace time.Duration
}

func NewSessionManager(reconnectGrace time.Duration) *SessionManager {
	if reconnectGrace <= 0 {
		reconnectGrace = DefaultReconnectGrace
	}
	return &SessionManager{
		sessions:       make(map[string]*CallSession),
		reconnectGrace: reconnectGrace,
	}
}

func (m *SessionManager) Create(host world.Host, baud, line int, runtime *bbs.Runtime) (*CallSession, uint64, error) {
	id, err := newSessionID()
	if err != nil {
		return nil, 0, err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	s := &CallSession{
		ID:           id,
		Host:         host,
		Baud:         baud,
		Line:         line,
		Runtime:      runtime,
		ConnectedAt:  time.Now(),
		attached:     true,
		attachment:   1,
	}
	m.sessions[id] = s
	return s, s.attachment, nil
}

// Resume attaches a new transport to an existing logical call. The attachment
// generation is incremented so a late close event from the previous WebSocket
// cannot accidentally detach the replacement transport.
func (m *SessionManager) Resume(id string) (*CallSession, uint64, string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	s, ok := m.sessions[id]
	if !ok {
		return nil, 0, "not_found"
	}
	if !s.ReconnectDeadline.IsZero() && !time.Now().Before(s.ReconnectDeadline) {
		m.deleteLocked(s)
		return nil, 0, "expired"
	}

	if s.expiry != nil {
		s.expiry.Stop()
		s.expiry = nil
	}
	s.ReconnectDeadline = time.Time{}
	s.attached = true
	s.attachment++
	return s, s.attachment, "ok"
}

// Detach marks the logical line as temporarily transport-less. It does not end
// the call immediately; Safari/backgrounding, Wi-Fi changes, and mobile radio
// transitions may all drop a WebSocket without the user hanging up.
func (m *SessionManager) Detach(id string, attachment uint64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	s, ok := m.sessions[id]
	if !ok || !s.attached || s.attachment != attachment {
		return
	}

	s.attached = false
	s.ReconnectDeadline = time.Now().Add(m.reconnectGrace)
	if s.expiry != nil {
		s.expiry.Stop()
	}
	s.expiry = time.AfterFunc(m.reconnectGrace, func() {
		m.expire(id, attachment)
	})
}

func (m *SessionManager) IsCurrent(id string, attachment uint64) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.sessions[id]
	return ok && s.attached && s.attachment == attachment
}

func (m *SessionManager) End(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if s, ok := m.sessions[id]; ok {
		m.deleteLocked(s)
	}
}

func (m *SessionManager) expire(id string, attachment uint64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	s, ok := m.sessions[id]
	if !ok || s.attached || s.attachment != attachment {
		return
	}
	if s.ReconnectDeadline.IsZero() || time.Now().Before(s.ReconnectDeadline) {
		return
	}
	m.deleteLocked(s)
}

func (m *SessionManager) deleteLocked(s *CallSession) {
	if s.expiry != nil {
		s.expiry.Stop()
		s.expiry = nil
	}
	delete(m.sessions, s.ID)
}

func newSessionID() (string, error) {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(raw[:]), nil
}
