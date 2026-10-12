package world

import (
	"sort"
	"time"
)

// Actor kinds used by presence sessions. ActorKindMember is a human caller;
// its ID is the call ID until member accounts exist.
const (
	ActorKindPersona = "persona"
	ActorKindMember  = "member"
)

// PresenceSession is one call that holds a logical line of a host from Start
// until End. A live human call has a zero End. Sessions that a caller observed
// (WHO, or a dial whose BUSY/CONNECT they decided) are world history and never
// change once saved. See spec 015.
type PresenceSession struct {
	HostID string    `json:"host_id"`
	Actor  ActorRef  `json:"actor"`
	Handle string    `json:"handle,omitempty"`
	Line   int       `json:"line"`
	Baud   int       `json:"baud"`
	Start  time.Time `json:"start"`
	End    time.Time `json:"end,omitempty"`
}

// Live reports whether the session has no planned end (a human call in progress).
func (s PresenceSession) Live() bool { return s.End.IsZero() }

// ActiveAt reports whether the session holds its line at t.
func (s PresenceSession) ActiveAt(t time.Time) bool {
	return !t.Before(s.Start) && (s.Live() || t.Before(s.End))
}

// Overlaps reports whether the session holds its line at any time in [from, to).
// A zero to means an open-ended interval.
func (s PresenceSession) Overlaps(from, to time.Time) bool {
	if !to.IsZero() && !s.Start.Before(to) {
		return false
	}
	return s.Live() || from.Before(s.End)
}

// HumanCall is a human caller's connection to a host, in world time.
type HumanCall struct {
	CallID string    `json:"call_id"`
	HostID string    `json:"host_id"`
	Handle string    `json:"handle,omitempty"`
	Line   int       `json:"line"`
	Baud   int       `json:"baud"`
	Start  time.Time `json:"start"`
}

// Session returns the call as a live presence session.
func (c HumanCall) Session() PresenceSession {
	return PresenceSession{HostID: c.HostID, Actor: ActorRef{Kind: ActorKindMember, ID: c.CallID}, Handle: c.Handle, Line: c.Line, Baud: c.Baud, Start: c.Start}
}

var _ PresenceStore = (*MemoryStore)(nil)

func sessionKey(s PresenceSession) string {
	return s.Actor.String() + "@" + s.Start.UTC().Format(time.RFC3339Nano)
}

func (s *MemoryStore) ListObservedSessions(hostID string, from, to time.Time) []PresenceSession {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []PresenceSession
	for _, p := range s.presence[hostID] {
		if p.Overlaps(from, to) {
			out = append(out, p)
		}
	}
	return out
}

func (s *MemoryStore) SaveObservedSessions(hostID string, sessions []PresenceSession) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.saveObservedLocked(hostID, sessions)
}

func (s *MemoryStore) saveObservedLocked(hostID string, sessions []PresenceSession) {
	known := map[string]bool{}
	for _, p := range s.presence[hostID] {
		known[sessionKey(p)] = true
	}
	for _, p := range sessions {
		if p.Live() || known[sessionKey(p)] {
			continue
		}
		known[sessionKey(p)] = true
		s.presence[hostID] = append(s.presence[hostID], p)
	}
	sort.SliceStable(s.presence[hostID], func(i, j int) bool {
		return s.presence[hostID][i].Start.Before(s.presence[hostID][j].Start)
	})
}

func (s *MemoryStore) SaveHumanCall(call HumanCall) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.humanCalls[call.CallID] = call
}

func (s *MemoryStore) HumanCall(callID string) (HumanCall, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	c, ok := s.humanCalls[callID]
	return c, ok
}

func (s *MemoryStore) EndHumanCall(callID string, at time.Time) (HumanCall, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	c, ok := s.humanCalls[callID]
	if !ok {
		return HumanCall{}, false
	}
	delete(s.humanCalls, callID)
	ended := c.Session()
	if !at.After(ended.Start) {
		at = ended.Start.Add(time.Second)
	}
	ended.End = at
	s.saveObservedLocked(c.HostID, []PresenceSession{ended})
	return c, true
}

func (s *MemoryStore) ListHumanCalls(hostID string) []HumanCall {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []HumanCall
	for _, c := range s.humanCalls {
		if c.HostID == hostID {
			out = append(out, c)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Line < out[j].Line })
	return out
}
