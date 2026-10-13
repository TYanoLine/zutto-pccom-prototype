// Package callplan decides, deterministically and without an LLM, what a
// resident does within a call that presence (spec 015) placed on a line:
// logging in, staying idle, going to the paging screen and sitting in a chat
// room (spec 016). It only computes; callers persist what was observed.
//
// The vocabulary is the host-independent world.CallState. Host programs map it
// to their own screens. Every probability, weight and duration in this package
// is a FICTIONAL TUNING VALUE for the simulation, not a historical statistic.
//
// Human callers never enter the computation: a resident's plan is the same
// whether or not a human is on the host or in a room.
package callplan

import (
	"hash/fnv"
	"math"
	"sort"
	"time"

	"zutto-pccom/apps/server/internal/presence"
	"zutto-pccom/apps/server/internal/world"
)

// LoginLength is how long a call shows the login state after it connects.
const LoginLength = 2 * time.Minute

// Input is everything the computation reads.
type Input struct {
	Host world.Host
	// Rooms are the station's chat rooms; none means residents never chat.
	Rooms []world.ChatRoom
	// Chat enables chat segments. When false no plan contains one (FR-004).
	Chat bool
	// Sessions are the calls to plan and every call that may share a room with
	// them. Human calls are ignored.
	Sessions []world.PresenceSession
	// Profiles are the residents' presence profiles by persona ID; a missing
	// profile is derived from the persona alone.
	Profiles map[string]presence.Profile
	// Personas are the residents by persona ID.
	Personas map[string]world.Persona
	// Observed are stored plans. Their calls are not planned again, and they
	// occupy rooms exactly as stored.
	Observed []world.CallActivity
}

// draft is a call's chat intention before rooms are filled.
type draft struct {
	session world.PresenceSession
	key     string
	loginTo time.Time
	// chat is set when the resident wants to enter a room.
	chat     bool
	room     world.ChatRoom
	pageFrom time.Time
	enter    time.Time
	leave    time.Time
}

// Plan returns the plan of every resident session in in.Sessions that has no
// observed plan, keyed by world.CallActivity.Key.
func Plan(in Input) map[string]world.CallActivity {
	observed := map[string]bool{}
	for _, a := range in.Observed {
		observed[a.Key()] = true
	}
	var residents []world.PresenceSession
	seen := map[string]bool{}
	for _, s := range in.Sessions {
		if s.Actor.Kind != world.ActorKindPersona || s.Live() {
			continue
		}
		key := activityKey(s)
		if seen[key] {
			continue
		}
		seen[key] = true
		residents = append(residents, s)
	}
	sort.Slice(residents, func(i, j int) bool {
		if !residents[i].Start.Equal(residents[j].Start) {
			return residents[i].Start.Before(residents[j].Start)
		}
		return residents[i].Actor.ID < residents[j].Actor.ID
	})

	rooms := usableRooms(in.Rooms)
	chat := in.Chat && len(rooms) > 0

	// Level one: every call's wish from the base intention alone. Others' wishes
	// are what a resident sees when deciding how crowded a room is, so the
	// computation never recurses.
	base := make([]draft, len(residents))
	for i, s := range residents {
		base[i] = baseDraft(in, rooms, chat, s)
	}

	// Level two: the calls without a stored plan decide with the crowd.
	var drafts []draft
	for i, d := range base {
		if observed[d.key] {
			continue
		}
		if d.chat {
			n := crowd(base, i, d.room.Number, d.enter, in.Observed)
			d = crowdDraft(in, d, n)
		}
		drafts = append(drafts, d)
	}

	// Fill rooms in order of entry so no room ever holds more than its
	// capacity: occupancy only rises at an entry.
	sort.SliceStable(drafts, func(i, j int) bool {
		if drafts[i].chat != drafts[j].chat {
			return drafts[i].chat
		}
		if !drafts[i].enter.Equal(drafts[j].enter) {
			return drafts[i].enter.Before(drafts[j].enter)
		}
		return drafts[i].key < drafts[j].key
	})
	type stay struct {
		room        int
		enter, exit time.Time
	}
	var occupied []stay
	for _, a := range in.Observed {
		for _, seg := range a.Segments {
			if seg.State == world.CallChat {
				occupied = append(occupied, stay{seg.Room, seg.From, seg.To})
			}
		}
	}
	out := make(map[string]world.CallActivity, len(drafts))
	for _, d := range drafts {
		if d.chat && d.room.Capacity > 0 {
			// Count at the entry and at every later entry during the stay: a
			// stored plan may enter after this one.
			points := []time.Time{d.enter}
			for _, o := range occupied {
				if o.room == d.room.Number && o.enter.After(d.enter) && o.enter.Before(d.leave) {
					points = append(points, o.enter)
				}
			}
			for _, t := range points {
				n := 0
				for _, o := range occupied {
					if o.room == d.room.Number && !t.Before(o.enter) && t.Before(o.exit) {
						n++
					}
				}
				if n >= d.room.Capacity {
					d.chat = false
					break
				}
			}
		}
		if d.chat {
			occupied = append(occupied, stay{d.room.Number, d.enter, d.leave})
		}
		out[d.key] = d.activity()
	}
	return out
}

func (d draft) activity() world.CallActivity {
	s := d.session
	a := world.CallActivity{HostID: s.HostID, Actor: s.Actor, Start: s.Start}
	add := func(from, to time.Time, state world.CallState, room int) {
		if to.After(from) {
			a.Segments = append(a.Segments, world.CallSegment{From: from, To: to, State: state, Room: room})
		}
	}
	add(s.Start, d.loginTo, world.CallLogin, 0)
	if !d.chat {
		add(d.loginTo, s.End, world.CallIdle, 0)
		return a
	}
	add(d.loginTo, d.pageFrom, world.CallIdle, 0)
	add(d.pageFrom, d.enter, world.CallPaging, 0)
	add(d.enter, d.leave, world.CallChat, d.room.Number)
	add(d.leave, s.End, world.CallIdle, 0)
	return a
}

// baseDraft decides the call's login, entry time and room, and whether the
// base intention alone takes the resident into the room. Its stay is the
// "someone is there" stay; crowdDraft settles the real one.
func baseDraft(in Input, rooms []world.ChatRoom, chat bool, s world.PresenceSession) draft {
	d := draft{session: s, key: activityKey(s)}
	d.loginTo = minTime(s.Start.Add(LoginLength), s.End)
	if !chat {
		return d
	}
	length := s.End.Sub(s.Start)
	d.enter = s.Start.Add(time.Duration((.15 + .45*unit(s, "enter")) * float64(length)))
	page := time.Duration((.5 + 1.5*unit(s, "page")) * float64(time.Minute))
	d.pageFrom = d.enter.Add(-page)
	if d.pageFrom.Before(d.loginTo) {
		d.pageFrom = d.loginTo
		d.enter = d.pageFrom.Add(page)
	}
	if !d.enter.Before(s.End) {
		return d
	}
	d.room = pickRoom(rooms, unit(s, "room"))
	d.chat = unit(s, "chat") < intent(in, s)
	d.leave = minTime(d.enter.Add(time.Duration((.3+.6*unit(s, "stay"))*float64(s.End.Sub(d.enter)))), s.End)
	return d
}

// crowdDraft decides with n residents already in the room at entry: an empty
// room is less inviting and quickly left, a lively one draws people in.
func crowdDraft(in Input, d draft, n int) draft {
	s := d.session
	p := intent(in, s)
	if n == 0 {
		p *= .4
	} else {
		p *= 1.5
	}
	// Whether a seat is free is settled in Plan from the plans that really
	// enter, not from these wishes.
	d.chat = unit(s, "chat") < math.Min(p, maxIntent)
	if n == 0 {
		d.leave = d.enter.Add(time.Duration((2 + 4*unit(s, "alone")) * float64(time.Minute)))
	}
	d.leave = minTime(d.leave, s.End)
	if !d.leave.After(d.enter) {
		d.chat = false
	}
	return d
}

// crowd counts the residents in room at t by the base wishes of the other
// calls and the stored plans.
func crowd(base []draft, self int, room int, t time.Time, observed []world.CallActivity) int {
	stored := map[string]bool{}
	n := 0
	for _, a := range observed {
		stored[a.Key()] = true
		if a.Actor == base[self].session.Actor {
			continue
		}
		if seg, ok := a.SegmentAt(t); ok && seg.State == world.CallChat && seg.Room == room {
			n++
		}
	}
	for i, o := range base {
		if i == self || stored[o.key] || o.session.Actor == base[self].session.Actor || !o.chat || o.room.Number != room {
			continue
		}
		if !t.Before(o.enter) && t.Before(o.leave) {
			n++
		}
	}
	return n
}

// maxIntent caps the chance of entering a room.
const maxIntent = .95

// intent is the base chance that the resident goes to a chat room during the
// call: the lifestyle's daytime base, tripled in the Telehodai hours, times
// the resident's sociability.
func intent(in Input, s world.PresenceSession) float64 {
	persona := in.Personas[s.Actor.ID]
	prof, ok := in.Profiles[s.Actor.ID]
	if !ok {
		prof = presence.ProfileFor(persona, nil, in.Host)
	}
	p := lifestyleBase(prof.Lifestyle)
	if teleho(s.Start) {
		p *= 3
	}
	p *= sociability(persona)
	return math.Min(p, maxIntent)
}

// lifestyleBase is the daytime chance by lifestyle.
func lifestyleBase(lifestyle string) float64 {
	switch lifestyle {
	case presence.SecondaryStudent:
		return .3
	case presence.UniversityStudent:
		return .35
	case presence.Homemaker:
		return .2
	case presence.SelfEmployed, presence.ShiftWorker:
		return .15
	}
	return .2
}

// sociability is how readily the resident talks rather than reads: the mean
// of 1 - LurkerTendency and ReplyTendency.
func sociability(p world.Persona) float64 {
	return ((1 - clamp01(p.LurkerTendency)) + clamp01(p.ReplyTendency)) / 2
}

// teleho reports the Telehodai discount window, 23:00-07:59.
func teleho(t time.Time) bool { return t.Hour() >= 23 || t.Hour() < 8 }

// usableRooms drops rooms nobody would pick.
func usableRooms(rooms []world.ChatRoom) []world.ChatRoom {
	var out []world.ChatRoom
	for _, r := range rooms {
		if r.Weight > 0 {
			out = append(out, r)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Number < out[j].Number })
	return out
}

func pickRoom(rooms []world.ChatRoom, u float64) world.ChatRoom {
	total := 0.0
	for _, r := range rooms {
		total += r.Weight
	}
	acc := 0.0
	for _, r := range rooms {
		acc += r.Weight / total
		if u < acc {
			return r
		}
	}
	return rooms[len(rooms)-1]
}

func activityKey(s world.PresenceSession) string {
	return world.CallActivity{Actor: s.Actor, Start: s.Start}.Key()
}

// unit returns a stable value in [0, 1) for the call and a purpose.
func unit(s world.PresenceSession, purpose string) float64 {
	h := fnv.New64a()
	for _, part := range []string{"callplan", purpose, s.HostID, s.Actor.ID} {
		_, _ = h.Write([]byte(part))
		_, _ = h.Write([]byte{0})
	}
	v := s.Start.Unix()
	for i := 0; i < 8; i++ {
		_, _ = h.Write([]byte{byte(v >> (8 * i))})
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

func minTime(a, b time.Time) time.Time {
	if b.Before(a) {
		return b
	}
	return a
}
