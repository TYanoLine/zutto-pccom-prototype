package world

import "time"

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
