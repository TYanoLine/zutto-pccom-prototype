// Package lineslot decides, deterministically and without an LLM, when
// residents send a human a line message, and in answer to what (spec 018). A
// slot is such a decision; an LLM may only put a selected slot into words. It
// only computes; callers write and record the messages.
//
// Slots exist only while the human's call is live and the resident holds the
// line: messages nobody receives are not world facts until someone observes
// them. A human's message body is never read here, so what a human writes
// cannot change who answers or when. Every probability, delay and limit here
// is a FICTIONAL TUNING VALUE for the simulation, not a historical statistic.
package lineslot

import (
	"hash/fnv"
	"math"
	"sort"
	"strings"
	"time"

	"zutto-pccom/apps/server/internal/world"
)

// Tuning values (fictional). They are provisional and set high while the
// messages are being debugged; change the values here only.
const (
	answerBase, answerReply, answerPerRelation = .4, .35, .15
	answerDelayMin, answerDelayMax             = 8 * time.Second, 45 * time.Second
	hailBase, hailPerRelation, hailSoc         = .3, .2, .2
	hailDelayMin, hailDelayMax                 = 10 * time.Second, 90 * time.Second
	hailNewsBase, hailNewsPerRelation          = .25, .15
	hailNewsDelayMin, hailNewsDelayMax         = 20 * time.Second, 3 * time.Minute

	// MaxHailsPerCall is the most messages one resident volunteers during one
	// call.
	MaxHailsPerCall = 2
	// NewsWindow is how old an event may be when a resident brings it up.
	NewsWindow = 30 * time.Minute

	perHour        = 12
	residentGap    = 2 * time.Minute
	anyResidentGap = 10 * time.Second
)

// Kind is what a slot's message does.
type Kind string

const (
	// Answer replies to a message the human sent the resident.
	Answer Kind = "answer"
	// Hail is a message a resident who knows the human sends unprompted.
	Hail Kind = "hail"
)

// Caller is the human call the messages go to.
type Caller struct {
	Actor world.ActorRef
	// Known is the human as residents know them (world.KnownAs); zero for a
	// guest, whom residents cannot know.
	Known world.ActorRef
	Line  int
	Call  string
	// From and To bound the call; a zero To is a call still in progress.
	From, To time.Time
}

// Resident is a resident holding a line from From until To.
type Resident struct {
	Actor    world.ActorRef
	Handle   string
	Persona  world.Persona
	Line     int
	Call     string
	From, To time.Time
}

// WorldEvent is a world fact about the human that residents may bring up.
type WorldEvent struct {
	ID    string
	At    time.Time
	Kind  string
	Board string
	Title string
	By    world.ActorRef
}

// Input is everything the computation reads.
type Input struct {
	Host world.Host
	// From and To bound the slots returned: At in (From, To].
	From, To time.Time
	Caller   Caller
	// Residents are the residents' calls that overlap the caller's.
	Residents []Resident
	// Sent are the messages the human sent; only their addressee, line and
	// delivery time are read, never their body.
	Sent []world.LineMessage
	// Recorded are the residents' messages to the human already recorded.
	Recorded []world.LineMessage
	// Relation is 0 for strangers, 1 for people who have talked before and 2
	// for regulars. Nil means strangers.
	Relation func(a, b world.ActorRef) int
	// Events are recent board events about the human.
	Events []WorldEvent
}

// Slot is a resident's chance to send one message.
type Slot struct {
	Key     string
	Kind    Kind
	Speaker world.ActorRef
	Handle  string
	// Line is the speaker's line.
	Line int
	At   time.Time
	// Trigger is what the slot answers: a message ID, "connect" or an event.
	Trigger string
	// RespondsTo is the human's message an answer replies to.
	RespondsTo string
	Event      string
}

// KindOf returns the kind of a slot key.
func KindOf(key string) Kind {
	kind, _, _ := strings.Cut(key, "|")
	return Kind(kind)
}

// Slots returns the slots in (in.From, in.To] that are not recorded yet, in
// time order, after the limits.
func Slots(in Input) []Slot {
	if !in.To.After(in.From) || in.Caller.Call == "" {
		return nil
	}
	c := computation{in: in, recorded: map[string]bool{}}
	for _, m := range in.Recorded {
		if m.Slot != "" {
			c.recorded[m.Slot] = true
		}
	}
	var slots []Slot
	slots = append(slots, c.answers()...)
	slots = append(slots, c.hails()...)

	var kept []Slot
	for _, s := range slots {
		if c.recorded[s.Key] || !c.live(s.At) || !c.holds(s.Speaker, s.Line, s.At) {
			continue
		}
		kept = append(kept, s)
	}
	return c.limit(kept)
}

type computation struct {
	in       Input
	recorded map[string]bool
}

func (c computation) key(kind Kind, speaker world.ActorRef, trigger string) string {
	return strings.Join([]string{string(kind), c.in.Host.ID, c.in.Caller.Actor.String(), speaker.String(), trigger}, "|")
}

// live reports whether the human's call is in progress at t.
func (c computation) live(t time.Time) bool {
	cl := c.in.Caller
	return !t.Before(cl.From) && (cl.To.IsZero() || t.Before(cl.To))
}

// holds reports whether the resident holds the line at t.
func (c computation) holds(actor world.ActorRef, line int, t time.Time) bool {
	for _, r := range c.in.Residents {
		if r.Actor == actor && r.Line == line && !t.Before(r.From) && t.Before(r.To) {
			return true
		}
	}
	return false
}

func (c computation) relation(a, b world.ActorRef) int {
	if c.in.Relation == nil || a.IsZero() || b.IsZero() {
		return 0
	}
	return c.in.Relation(a, b)
}

// answers gives each message the human sent a reply from the resident it was
// sent to, sometimes. The reply is after the message reached the resident.
func (c computation) answers() []Slot {
	var out []Slot
	for _, m := range c.in.Sent {
		if m.From != c.in.Caller.Actor || m.To.Kind != world.ActorKindPersona || m.Slot != "" {
			continue
		}
		for _, r := range c.in.Residents {
			if r.Actor != m.To || r.Line != m.ToLine || m.DeliverAt.Before(r.From) || !m.DeliverAt.Before(r.To) {
				continue
			}
			key := c.key(Answer, r.Actor, m.ID)
			chance := answerBase + answerReply*clamp01(r.Persona.ReplyTendency) +
				answerPerRelation*float64(c.relation(r.Actor, c.in.Caller.Known))
			if unit(key, "chance") >= math.Min(chance, 1) {
				continue
			}
			out = append(out, Slot{Key: key, Kind: Answer, Speaker: r.Actor, Handle: r.Handle, Line: r.Line,
				At: m.DeliverAt.Add(between(key, answerDelayMin, answerDelayMax)), Trigger: m.ID, RespondsTo: m.ID})
		}
	}
	return out
}

// hails lets residents who know the human greet them on their own: on the
// call, and about recent board events. A guest is nobody residents know, so
// nobody hails them.
func (c computation) hails() []Slot {
	known := c.in.Caller.Known
	if known.IsZero() {
		return nil
	}
	taken := map[world.ActorRef]int{}
	for _, m := range c.in.Recorded {
		if KindOf(m.Slot) == Hail {
			taken[m.From]++
		}
	}
	var cands []Resident
	seen := map[world.ActorRef]bool{}
	for _, r := range c.in.Residents {
		if !seen[r.Actor] {
			seen[r.Actor] = true
			cands = append(cands, r)
		}
	}
	sort.Slice(cands, func(i, j int) bool { return cands[i].Actor.String() < cands[j].Actor.String() })
	var out []Slot
	for _, r := range cands {
		rel := c.relation(r.Actor, known)
		if rel < 1 {
			continue
		}
		var mine []Slot
		start := later(c.in.Caller.From, r.From)
		key := c.key(Hail, r.Actor, "connect")
		if unit(key, "chance") < math.Min(hailBase+hailPerRelation*float64(rel)+hailSoc*sociability(r.Persona), 1) {
			mine = append(mine, Slot{Key: key, Kind: Hail, Speaker: r.Actor, Handle: r.Handle, Line: r.Line,
				At: start.Add(between(key, hailDelayMin, hailDelayMax)), Trigger: "connect"})
		}
		for _, e := range c.in.Events {
			if e.By == r.Actor || e.At.After(c.in.To) {
				continue
			}
			ekey := c.key(Hail, r.Actor, e.ID)
			if unit(ekey, "chance") >= math.Min(hailNewsBase+hailNewsPerRelation*float64(rel), 1) {
				continue
			}
			at := later(later(e.At, start), c.in.From).Add(between(ekey, hailNewsDelayMin, hailNewsDelayMax))
			if at.Sub(e.At) > NewsWindow {
				continue
			}
			mine = append(mine, Slot{Key: ekey, Kind: Hail, Speaker: r.Actor, Handle: r.Handle, Line: r.Line,
				At: at, Trigger: e.ID, Event: e.ID})
		}
		sort.SliceStable(mine, func(i, j int) bool {
			if !mine[i].At.Equal(mine[j].At) {
				return mine[i].At.Before(mine[j].At)
			}
			return mine[i].Key < mine[j].Key
		})
		for _, s := range mine {
			if taken[r.Actor] >= MaxHailsPerCall {
				break
			}
			taken[r.Actor]++
			out = append(out, s)
		}
	}
	return out
}

func later(a, b time.Time) time.Time {
	if b.After(a) {
		return b
	}
	return a
}

// sociability is how readily the resident talks rather than reads, the same
// measure callplan uses.
func sociability(p world.Persona) float64 {
	return ((1 - clamp01(p.LurkerTendency)) + clamp01(p.ReplyTendency)) / 2
}

// between returns a stable duration in [lo, hi) for a key.
func between(key string, lo, hi time.Duration) time.Duration {
	return lo + time.Duration(unit(key, "delay")*float64(hi-lo))
}

// unit returns a stable value in [0, 1) for a key and a purpose.
func unit(key, purpose string) float64 {
	h := fnv.New64a()
	for _, part := range []string{"lineslot", purpose, key} {
		_, _ = h.Write([]byte(part))
		_, _ = h.Write([]byte{0})
	}
	return float64(h.Sum64()>>11) / float64(1<<53)
}

func clamp01(v float64) float64 {
	if math.IsNaN(v) || v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}
