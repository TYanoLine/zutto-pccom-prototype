// Package chatslot decides, deterministically and without an LLM, when
// residents in a chat room speak, to whom and in answer to what (spec 017).
// A slot is such a decision; an LLM may only put a selected slot into words.
// It only computes; callers write and record the lines.
//
// Slots exist only while a caller is in the room: lines nobody hears are not
// world facts until someone observes them. Every probability, delay and limit
// here is a FICTIONAL TUNING VALUE for the simulation, not a historical
// statistic.
package chatslot

import (
	"hash/fnv"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"zutto-pccom/apps/server/internal/world"
)

// Tuning values (fictional). They are provisional and set high while the
// chat is being debugged (PR #359); change the values here only.
const (
	greetBase, greetSoc                = .5, .4
	greetDelayMin, greetDelayMax       = 3 * time.Second, 12 * time.Second
	welcomeNew, welcomeOpenness        = .3, .5
	welcomeKnown, welcomePerRelation   = .5, .2
	welcomeDelayMin, welcomeDelayMax   = 4 * time.Second, 20 * time.Second
	replyNamed, replyToSelf            = .85, .6
	replyBase, replyPerRelation        = .5, .3
	replyDelayMin, replyDelayMax       = 6 * time.Second, 30 * time.Second
	talkGapMin, talkGapMax             = 15 * time.Second, 2 * time.Minute
	talkMinSoc                         = .3
	farewellSoc                        = .7
	farewellLeadMin, farewellLeadMax   = 5 * time.Second, 20 * time.Second
	sendoffBase, sendoffPerRelation    = .3, .25
	sendoffSoc                         = .2
	sendoffDelayMin, sendoffDelayMax   = 2 * time.Second, 10 * time.Second
	sendoffSecondMin, sendoffSecondMax = 3 * time.Second, 8 * time.Second
	maxSendoffs                        = 2
	newsBase, newsPerRelation          = .25, .15
	newsDelayMin, newsDelayMax         = 20 * time.Second, 3 * time.Minute
	newsPerSubject                     = 2

	// MaxDepth is the longest chain of replies to replies.
	MaxDepth = 6
	// NewsWindow is how old an event may be when a resident brings it up.
	NewsWindow = 30 * time.Minute

	roomGap    = 3 * time.Second
	maxShift   = 8 * time.Second
	speakerGap = 15 * time.Second
	perMinute  = 12
	perHour    = 360
)

// leavingPhrases mark a line that says its speaker is about to leave. The
// list is fictional tuning, shared by every host program.
var leavingPhrases = []string{
	"落ちます", "おちます", "落ちるね", "おちるね", "寝ます", "ねます", "おやすみ", "失礼します", "しつれいします",
	"お先に", "おさきに", "抜けます", "ぬけます", "ではでは", "ばいばい", "さよなら", "また来ます", "またきます",
}

// Kind is what a slot's line does.
type Kind string

const (
	Greet    Kind = "greet"
	Welcome  Kind = "welcome"
	Reply    Kind = "reply"
	Talk     Kind = "talk"
	Farewell Kind = "farewell"
	Sendoff  Kind = "sendoff"
	News     Kind = "news"
)

// Stay is a caller in the room from From until To; a zero To is still there.
type Stay struct {
	Actor    world.ActorRef
	Handle   string
	From, To time.Time
}

// ResidentStay is a resident's chat segment in the room (spec 016 plan).
// Call identifies the call the segment belongs to.
type ResidentStay struct {
	Actor    world.ActorRef
	Handle   string
	Persona  world.Persona
	Call     string
	From, To time.Time
}

// WorldEvent is a world fact about someone that residents may bring up.
type WorldEvent struct {
	ID      string
	Subject world.ActorRef
	At      time.Time
	// Kind is "posted" (Subject wrote) or "replied_to" (By answered Subject).
	Kind  string
	Board string
	Title string
	By    world.ActorRef
}

// Input is everything the computation reads.
type Input struct {
	Host world.Host
	Room int
	// From and To bound the slots returned: At in (From, To].
	From, To   time.Time
	Viewers    []Stay
	Residents  []ResidentStay
	Utterances []world.ChatUtterance
	// Relation is 0 for strangers, 1 for people who have talked before and 2
	// for regulars. Nil means strangers.
	Relation func(a, b world.ActorRef) int
	Events   []WorldEvent
}

// Slot is a resident's chance to say one line.
type Slot struct {
	Key     string
	Kind    Kind
	Room    int
	Speaker world.ActorRef
	Handle  string
	At      time.Time
	// Trigger is what the slot answers: an utterance's slot key or ID, a
	// join, a call or an event.
	Trigger    string
	RespondsTo string
	Addressee  world.ActorRef
	Depth      int
	Event      string
}

// KindOf returns the kind of a slot key.
func KindOf(key string) Kind {
	kind, _, _ := strings.Cut(key, "|")
	return Kind(kind)
}

// Leaving reports whether a line says its speaker is about to leave.
func Leaving(body string) bool {
	for _, p := range leavingPhrases {
		if strings.Contains(body, p) {
			return true
		}
	}
	return false
}

// Slots returns the slots in (in.From, in.To] that are not recorded yet, in
// time order, after the limits.
func Slots(in Input) []Slot {
	if len(in.Viewers) == 0 || !in.To.After(in.From) {
		return nil
	}
	c := computation{in: in, recorded: map[string]bool{}, depth: map[string]int{}, byID: map[string]world.ChatUtterance{}}
	utterances := append([]world.ChatUtterance(nil), in.Utterances...)
	sort.SliceStable(utterances, func(i, j int) bool { return utterances[i].At.Before(utterances[j].At) })
	for _, u := range utterances {
		c.byID[u.ID] = u
		if u.Slot != "" {
			c.recorded[u.Slot] = true
		}
		if KindOf(u.Slot) == Reply {
			if prev, ok := c.byID[u.RespondsTo]; ok {
				c.depth[u.ID] = c.depth[prev.ID] + 1
			}
		}
	}
	var slots []Slot
	slots = append(slots, c.joins()...)
	slots = append(slots, c.answers(utterances)...)
	slots = append(slots, c.talks()...)
	slots = append(slots, c.farewells()...)
	slots = append(slots, c.news()...)

	var kept []Slot
	for _, s := range slots {
		if c.recorded[s.Key] || !c.observed(s.At) || !c.present(s.Speaker, s.At) {
			continue
		}
		kept = append(kept, s)
	}
	return c.limit(kept, utterances)
}

type computation struct {
	in       Input
	recorded map[string]bool
	depth    map[string]int
	byID     map[string]world.ChatUtterance
}

func (c computation) key(kind Kind, speaker world.ActorRef, trigger string) string {
	return strings.Join([]string{string(kind), c.in.Host.ID, itoa(c.in.Room), speaker.String(), trigger}, "|")
}

// observed reports whether a caller is in the room at t.
func (c computation) observed(t time.Time) bool {
	for _, v := range c.in.Viewers {
		if !t.Before(v.From) && (v.To.IsZero() || t.Before(v.To)) {
			return true
		}
	}
	return false
}

func (c computation) present(actor world.ActorRef, t time.Time) bool {
	for _, r := range c.in.Residents {
		if r.Actor == actor && !t.Before(r.From) && t.Before(r.To) {
			return true
		}
	}
	for _, v := range c.in.Viewers {
		if v.Actor == actor && !t.Before(v.From) && (v.To.IsZero() || t.Before(v.To)) {
			return true
		}
	}
	return false
}

// residentsAt returns the residents in the room at t, except one actor.
func (c computation) residentsAt(t time.Time, except world.ActorRef) []ResidentStay {
	var out []ResidentStay
	for _, r := range c.in.Residents {
		if r.Actor != except && !t.Before(r.From) && t.Before(r.To) {
			out = append(out, r)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Actor.String() < out[j].Actor.String() })
	return out
}

// othersAt reports whether anyone besides actor is in the room at t.
func (c computation) othersAt(t time.Time, actor world.ActorRef) bool {
	if len(c.residentsAt(t, actor)) > 0 {
		return true
	}
	for _, v := range c.in.Viewers {
		if v.Actor != actor && !t.Before(v.From) && (v.To.IsZero() || t.Before(v.To)) {
			return true
		}
	}
	return false
}

func (c computation) relation(a, b world.ActorRef) int {
	if c.in.Relation == nil {
		return 0
	}
	return c.in.Relation(a, b)
}

// pick returns the candidates whose draw falls under their chance, best draw
// first: the one with the smallest draw relative to its chance.
func pick(cands []ResidentStay, draw func(ResidentStay) float64, chance func(ResidentStay) float64) []ResidentStay {
	type scored struct {
		r     ResidentStay
		score float64
	}
	var hits []scored
	for _, r := range cands {
		p := math.Min(chance(r), 1)
		if u := draw(r); p > 0 && u < p {
			hits = append(hits, scored{r, u / p})
		}
	}
	sort.SliceStable(hits, func(i, j int) bool { return hits[i].score < hits[j].score })
	out := make([]ResidentStay, len(hits))
	for i, h := range hits {
		out[i] = h.r
	}
	return out
}

// joins gives each observed entry a greeting from the resident who entered
// and a welcome from at most one resident already there.
func (c computation) joins() []Slot {
	type join struct {
		actor  world.ActorRef
		handle string
		at     time.Time
		res    *ResidentStay
	}
	var joins []join
	for i, r := range c.in.Residents {
		joins = append(joins, join{r.Actor, r.Handle, r.From, &c.in.Residents[i]})
	}
	for _, v := range c.in.Viewers {
		joins = append(joins, join{v.Actor, v.Handle, v.From, nil})
	}
	var out []Slot
	for _, j := range joins {
		if !c.observed(j.at) {
			continue
		}
		trigger := j.actor.String() + "@" + itoa64(j.at.Unix())
		if j.res != nil && c.othersAt(j.at, j.actor) {
			key := c.key(Greet, j.actor, trigger)
			if unit(key, "chance") < greetBase+greetSoc*sociability(j.res.Persona) {
				out = append(out, Slot{Key: key, Kind: Greet, Room: c.in.Room, Speaker: j.actor, Handle: j.handle,
					At: j.at.Add(between(key, greetDelayMin, greetDelayMax)), Trigger: trigger})
			}
		}
		before := j.at.Add(-time.Nanosecond)
		cands := c.residentsAt(before, j.actor)
		chosen := pick(cands,
			func(r ResidentStay) float64 { return unit(c.key(Welcome, r.Actor, trigger), "chance") },
			func(r ResidentStay) float64 {
				if rel := c.relation(r.Actor, j.actor); rel > 0 {
					return welcomeKnown + welcomePerRelation*float64(rel)
				}
				return welcomeNew + welcomeOpenness*clamp01(r.Persona.NewcomerOpenness)
			})
		if len(chosen) > 0 {
			r := chosen[0]
			key := c.key(Welcome, r.Actor, trigger)
			out = append(out, Slot{Key: key, Kind: Welcome, Room: c.in.Room, Speaker: r.Actor, Handle: r.Handle,
				At: j.at.Add(between(key, welcomeDelayMin, welcomeDelayMax)), Trigger: trigger, Addressee: j.actor})
		}
	}
	return out
}

// answers gives each observed line a reply from at most one resident, or,
// when the line says its speaker is leaving, send-offs from at most two.
func (c computation) answers(utterances []world.ChatUtterance) []Slot {
	var out []Slot
	for _, u := range utterances {
		if !c.observed(u.At) {
			continue
		}
		kind := KindOf(u.Slot)
		if kind == Sendoff {
			continue
		}
		trigger := u.Slot
		if trigger == "" {
			trigger = u.ID
		}
		cands := c.residentsAt(u.At, u.Actor)
		if kind == Farewell || Leaving(u.Body) {
			chosen := pick(cands,
				func(r ResidentStay) float64 { return unit(c.key(Sendoff, r.Actor, trigger), "chance") },
				func(r ResidentStay) float64 {
					return sendoffBase + sendoffPerRelation*float64(c.relation(r.Actor, u.Actor)) + sendoffSoc*sociability(r.Persona)
				})
			at := u.At
			for i, r := range chosen {
				if i == maxSendoffs {
					break
				}
				key := c.key(Sendoff, r.Actor, trigger)
				if i == 0 {
					at = at.Add(between(key, sendoffDelayMin, sendoffDelayMax))
				} else {
					at = at.Add(between(key, sendoffSecondMin, sendoffSecondMax))
				}
				out = append(out, Slot{Key: key, Kind: Sendoff, Room: c.in.Room, Speaker: r.Actor, Handle: r.Handle,
					At: at, Trigger: trigger, RespondsTo: u.ID, Addressee: u.Actor})
			}
			continue
		}
		depth := c.depth[u.ID] + 1
		if depth > MaxDepth {
			continue
		}
		answered := world.ActorRef{}
		if prev, ok := c.byID[u.RespondsTo]; ok {
			answered = prev.Actor
		}
		lower := strings.ToLower(u.Body)
		chosen := pick(cands,
			func(r ResidentStay) float64 { return unit(c.key(Reply, r.Actor, trigger), "chance") },
			func(r ResidentStay) float64 {
				switch {
				case r.Handle != "" && strings.Contains(lower, strings.ToLower(r.Handle)):
					return replyNamed
				case answered == r.Actor:
					return replyToSelf
				}
				return replyBase * clamp01(r.Persona.ReplyTendency) * (1 + replyPerRelation*float64(c.relation(r.Actor, u.Actor)))
			})
		if len(chosen) > 0 {
			r := chosen[0]
			key := c.key(Reply, r.Actor, trigger)
			out = append(out, Slot{Key: key, Kind: Reply, Room: c.in.Room, Speaker: r.Actor, Handle: r.Handle,
				At: u.At.Add(between(key, replyDelayMin, replyDelayMax)), Trigger: trigger, RespondsTo: u.ID,
				Addressee: u.Actor, Depth: depth})
		}
	}
	return out
}

// talks draws each resident's own lines over the stay.
func (c computation) talks() []Slot {
	var out []Slot
	for _, r := range c.in.Residents {
		soc := math.Max(sociability(r.Persona), talkMinSoc)
		t := r.From
		for n := 0; ; n++ {
			trigger := r.Call + "#" + itoa(n)
			key := c.key(Talk, r.Actor, trigger)
			gap := between(key, talkGapMin, talkGapMax)
			t = t.Add(time.Duration(float64(gap) / soc))
			if !t.Before(r.To) || t.After(c.in.To) {
				break
			}
			if !t.After(c.in.From) || !c.observed(t) || !c.othersAt(t, r.Actor) {
				continue
			}
			out = append(out, Slot{Key: key, Kind: Talk, Room: c.in.Room, Speaker: r.Actor, Handle: r.Handle, At: t, Trigger: trigger})
		}
	}
	return out
}

// farewells give a resident about to leave a chance to say so.
func (c computation) farewells() []Slot {
	var out []Slot
	for _, r := range c.in.Residents {
		key := c.key(Farewell, r.Actor, r.Call)
		at := r.To.Add(-between(key, farewellLeadMin, farewellLeadMax))
		if !at.After(r.From) || !c.othersAt(at, r.Actor) {
			continue
		}
		if unit(key, "chance") < farewellSoc*sociability(r.Persona) {
			out = append(out, Slot{Key: key, Kind: Farewell, Room: c.in.Room, Speaker: r.Actor, Handle: r.Handle, At: at, Trigger: r.Call})
		}
	}
	return out
}

// news lets at most one acquaintance of someone in the room bring up a recent
// event about them.
func (c computation) news() []Slot {
	bySubject := map[world.ActorRef][]WorldEvent{}
	for _, e := range c.in.Events {
		if e.At.After(c.in.To) {
			continue
		}
		bySubject[e.Subject] = append(bySubject[e.Subject], e)
	}
	var out []Slot
	for subject, events := range bySubject {
		sort.Slice(events, func(i, j int) bool {
			if !events[i].At.Equal(events[j].At) {
				return events[i].At.After(events[j].At)
			}
			return events[i].ID < events[j].ID
		})
		if len(events) > newsPerSubject {
			events = events[:newsPerSubject]
		}
		for _, e := range events {
			var cands []ResidentStay
			seen := map[world.ActorRef]bool{}
			for _, r := range c.in.Residents {
				if r.Actor == subject || r.Actor == e.By || seen[r.Actor] || c.relation(r.Actor, subject) < 1 {
					continue
				}
				seen[r.Actor] = true
				cands = append(cands, r)
			}
			sort.Slice(cands, func(i, j int) bool { return cands[i].Actor.String() < cands[j].Actor.String() })
			chosen := pick(cands,
				func(r ResidentStay) float64 { return unit(c.key(News, r.Actor, e.ID), "chance") },
				func(r ResidentStay) float64 { return newsBase + newsPerRelation*float64(c.relation(r.Actor, subject)) })
			if len(chosen) == 0 {
				continue
			}
			r := chosen[0]
			together, ok := c.together(r.Actor, subject, e.At)
			if !ok {
				continue
			}
			key := c.key(News, r.Actor, e.ID)
			at := together.Add(between(key, newsDelayMin, newsDelayMax))
			if at.Sub(e.At) > NewsWindow {
				continue
			}
			out = append(out, Slot{Key: key, Kind: News, Room: c.in.Room, Speaker: r.Actor, Handle: r.Handle,
				At: at, Trigger: e.ID, Addressee: subject, Event: e.ID})
		}
	}
	return out
}

// together returns the first time at or after t when a and b are both in the
// room and a caller is watching.
func (c computation) together(a, b world.ActorRef, t time.Time) (time.Time, bool) {
	var points []time.Time
	add := func(x time.Time) {
		if !x.Before(t) {
			points = append(points, x)
		}
	}
	points = append(points, t)
	for _, r := range c.in.Residents {
		add(r.From)
	}
	for _, v := range c.in.Viewers {
		add(v.From)
	}
	sort.Slice(points, func(i, j int) bool { return points[i].Before(points[j]) })
	for _, p := range points {
		if p.After(c.in.To) {
			break
		}
		if c.present(a, p) && c.present(b, p) && c.observed(p) {
			return p, true
		}
	}
	return time.Time{}, false
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
	for _, part := range []string{"chatslot", purpose, key} {
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

func itoa(n int) string { return strconv.Itoa(n) }

func itoa64(n int64) string { return strconv.FormatInt(n, 10) }
