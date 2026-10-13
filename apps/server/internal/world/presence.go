package world

import (
	"sort"
	"time"
)

// Actor kinds. See spec 019 for how humans are named.
const (
	ActorKindPersona = "persona"
	// ActorKindMember is a human user. Its ID is the human ID, the same across
	// calls, login IDs and worlds. Records made before spec 019 hold a call ID
	// or a handle here.
	ActorKindMember = "member"
	// ActorKindLogin is a person as residents know them: by the login ID shown
	// on the record's host. Residents never see a human ID.
	ActorKindLogin = "login"
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

	// CallID is a human call's own connection; empty for a resident. A human
	// may hold several calls at once under one actor.
	CallID string `json:"call_id,omitempty"`
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

	// HumanID is the user who made the call (spec 019).
	HumanID string `json:"human_id,omitempty"`
}

// CallInfo is what a host program learns about its own caller's call.
type CallInfo struct {
	CallID  string // this connection
	HumanID string // the user, across calls (spec 019)
	Line    int
	Baud    int
}

// Session returns the call as a live presence session. A call without a human
// ID (made before spec 019) is named by its call ID.
func (c HumanCall) Session() PresenceSession {
	id := c.HumanID
	if id == "" {
		id = c.CallID
	}
	return PresenceSession{HostID: c.HostID, Actor: ActorRef{Kind: ActorKindMember, ID: id}, Handle: c.Handle, Line: c.Line, Baud: c.Baud, Start: c.Start, CallID: c.CallID}
}

// CallState is what a caller is doing within a call. The vocabulary is shared
// by every host program; each program maps it to its own labels (Erika-K:
// paging -> "CALL", chat -> "CHAT"). See spec 016.
type CallState string

const (
	CallLogin  CallState = "login"
	CallIdle   CallState = "idle"
	CallPaging CallState = "paging" // at the page / line-message screen
	CallChat   CallState = "chat"
)

// CallSegment is one stretch [From, To) of a call in one state. Room is set
// for chat.
type CallSegment struct {
	From  time.Time `json:"from"`
	To    time.Time `json:"to"`
	State CallState `json:"state"`
	Room  int       `json:"room,omitempty"`
}

// CallActivity is the plan of one resident call, observed as a whole the first
// time any part of it is shown, and never rewritten afterwards.
type CallActivity struct {
	HostID   string        `json:"host_id"`
	Actor    ActorRef      `json:"actor"`
	Start    time.Time     `json:"start"` // == PresenceSession.Start
	Segments []CallSegment `json:"segments"`
}

// Key identifies the call the plan belongs to, like a presence session key.
func (a CallActivity) Key() string {
	return a.Actor.String() + "@" + a.Start.UTC().Format(time.RFC3339Nano)
}

// End is the end of the last segment.
func (a CallActivity) End() time.Time {
	if len(a.Segments) == 0 {
		return a.Start
	}
	return a.Segments[len(a.Segments)-1].To
}

// SegmentAt returns the segment that covers t.
func (a CallActivity) SegmentAt(t time.Time) (CallSegment, bool) {
	for _, seg := range a.Segments {
		if !t.Before(seg.From) && t.Before(seg.To) {
			return seg, true
		}
	}
	return CallSegment{}, false
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

func (s *MemoryStore) ListCallActivities(hostID string, from, to time.Time) []CallActivity {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []CallActivity
	for _, a := range s.callPlans[hostID] {
		if a.Start.Before(from) || (!to.IsZero() && !a.Start.Before(to)) {
			continue
		}
		out = append(out, cloneCallActivity(a))
	}
	return out
}

func (s *MemoryStore) SaveCallActivities(hostID string, acts []CallActivity) {
	s.mu.Lock()
	defer s.mu.Unlock()
	known := map[string]bool{}
	for _, a := range s.callPlans[hostID] {
		known[a.Key()] = true
	}
	for _, a := range acts {
		if known[a.Key()] {
			continue
		}
		known[a.Key()] = true
		s.callPlans[hostID] = append(s.callPlans[hostID], cloneCallActivity(a))
	}
	sort.SliceStable(s.callPlans[hostID], func(i, j int) bool {
		return s.callPlans[hostID][i].Start.Before(s.callPlans[hostID][j].Start)
	})
}

func cloneCallActivities(acts []CallActivity) []CallActivity {
	if len(acts) == 0 {
		return nil
	}
	out := make([]CallActivity, len(acts))
	for i, a := range acts {
		out[i] = cloneCallActivity(a)
	}
	return out
}

func cloneCallActivity(a CallActivity) CallActivity {
	a.Segments = append([]CallSegment(nil), a.Segments...)
	return a
}
