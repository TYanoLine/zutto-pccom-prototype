package worldrepo

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"zutto-pccom/apps/server/internal/callplan"
	"zutto-pccom/apps/server/internal/presence"
	"zutto-pccom/apps/server/internal/world"
)

// What residents do within their calls (spec 016). The plans are computed by
// callplan; this file gathers the calls that matter, saves the plans a caller
// observed, and serializes it per host with the presence lock.

var _ world.ChatHost = (*Repository)(nil)

// SetResidentChat lets residents enter chat rooms. Off by default; the server
// turns it on only when it can also generate what residents say.
func (r *Repository) SetResidentChat(enabled bool) { r.residentChat.Store(enabled) }

func (r *Repository) chatRooms(host world.Host) []world.ChatRoom {
	if !r.residentChat.Load() {
		return nil
	}
	detail, ok := r.HostDetail(host.ID)
	if !ok {
		return nil
	}
	return world.ChatRoomsOf(detail)
}

// callContext is what a plan computation at time t reads.
type callContext struct {
	sessions []world.PresenceSession
	profiles map[string]presence.Profile
	observed []world.CallActivity
}

// gatherCalls collects the resident calls that may be in a room at any time a
// call on the line at t could be, by computing presence every presence.Window
// over [t-Lookback, t] plus the given extra times. The caller holds the
// host's presence lock.
func (r *Repository) gatherCalls(ps world.PresenceStore, host world.Host, t time.Time, extra ...time.Time) callContext {
	ctx := callContext{profiles: map[string]presence.Profile{}}
	seen := map[string]bool{}
	add := func(res presence.Result) {
		for _, s := range res.Sessions {
			if s.Actor.Kind != world.ActorKindPersona {
				continue
			}
			key := world.CallActivity{Actor: s.Actor, Start: s.Start}.Key()
			if seen[key] {
				continue
			}
			seen[key] = true
			ctx.sessions = append(ctx.sessions, s)
		}
		for id, prof := range res.Profiles {
			ctx.profiles[id] = prof
		}
	}
	for _, at := range append(extra, t) {
		add(r.presenceAt(ps, host, at))
	}
	windows := r.presenceWindows.forHost(host.ID, presenceFingerprint(r, ps, host), t)
	for w := t.Add(-presence.Lookback).Truncate(presence.Window); w.Before(t); w = w.Add(presence.Window) {
		res, ok := windows.results[w.Unix()]
		if !ok {
			res = r.presenceAt(ps, host, w)
			windows.results[w.Unix()] = res
		}
		add(res)
	}
	ctx.observed = ps.ListCallActivities(host.ID, t.Add(-2*presence.Lookback), time.Time{})
	return ctx
}

// plans returns the plan of every call in ctx: the stored one when it exists,
// otherwise a computed one.
func (r *Repository) plans(host world.Host, rooms []world.ChatRoom, ctx callContext) map[string]world.CallActivity {
	in := callplan.Input{
		Host: host, Rooms: rooms, Chat: len(rooms) > 0,
		Sessions: ctx.sessions, Profiles: ctx.profiles, Observed: ctx.observed,
		Personas: map[string]world.Persona{},
	}
	if store, ok := r.Base.(world.PersonaStore); ok {
		for _, p := range store.ListHostPersonas(host.ID) {
			in.Personas[p.ID] = p
		}
	}
	out := callplan.Plan(in)
	for _, a := range ctx.observed {
		out[a.Key()] = a
	}
	return out
}

// CallActivitiesNow returns the plans of the residents on the host's lines
// now, in line order, saving the calls and their plans as observed.
func (r *Repository) CallActivitiesNow(host world.Host) []world.CallActivity {
	ps, ok := r.presenceStore()
	if !ok {
		return nil
	}
	lock := presenceLock(host.ID)
	lock.Lock()
	defer lock.Unlock()
	now := r.currentWorldTime()
	res := r.presenceAt(ps, host, now)

	rooms := r.chatRooms(host)
	var ctx callContext
	if len(rooms) == 0 {
		// Without chat a plan depends on its own call only.
		ctx = callContext{sessions: res.Sessions, profiles: res.Profiles,
			observed: ps.ListCallActivities(host.ID, now.Add(-2*presence.Lookback), time.Time{})}
	} else {
		ctx = r.gatherCalls(ps, host, now)
	}
	plans := r.plans(host, rooms, ctx)

	var out []world.CallActivity
	for _, s := range res.Sessions {
		if s.Actor.Kind != world.ActorKindPersona {
			continue
		}
		if a, ok := plans[world.CallActivity{Actor: s.Actor, Start: s.Start}.Key()]; ok {
			out = append(out, a)
		}
	}
	r.observePresence(ps, host, res, now, out...)
	return out
}

// ChatEvents returns room's join and leave events in (from, to], in time
// order, saving the calls and plans they come from as observed. A call that
// ends while its resident is in the room leaves it then.
func (r *Repository) ChatEvents(host world.Host, room int, from, to time.Time) []world.ChatEvent {
	ps, ok := r.presenceStore()
	if !ok || !to.After(from) {
		return nil
	}
	rooms := r.chatRooms(host)
	known := false
	for _, cr := range rooms {
		known = known || cr.Number == room
	}
	if !known {
		return nil
	}
	lock := presenceLock(host.ID)
	lock.Lock()
	defer lock.Unlock()

	// A call with an event in (from, to] is on the line at from or to, or
	// starts in between; sampling from and to covers windows of seconds, and
	// the grid covers longer ones.
	ctx := r.gatherCalls(ps, host, to, from)
	plans := r.plans(host, rooms, ctx)
	sessions := map[string]world.PresenceSession{}
	for _, s := range ctx.sessions {
		sessions[world.CallActivity{Actor: s.Actor, Start: s.Start}.Key()] = s
	}

	in := func(t time.Time) bool { return t.After(from) && !t.After(to) }
	var events []world.ChatEvent
	var seen []world.CallActivity
	var seenSessions []world.PresenceSession
	for key, a := range plans {
		s, onLine := sessions[key]
		found := false
		for _, seg := range a.Segments {
			if seg.State != world.CallChat || seg.Room != room {
				continue
			}
			if in(seg.From) {
				events = append(events, world.ChatEvent{At: seg.From, Kind: world.ChatJoin, Room: room, Actor: a.Actor, Handle: s.Handle})
				found = true
			}
			if in(seg.To) {
				events = append(events, world.ChatEvent{At: seg.To, Kind: world.ChatLeave, Room: room, Actor: a.Actor, Handle: s.Handle})
				found = true
			}
		}
		if found {
			seen = append(seen, a)
			if onLine {
				seenSessions = append(seenSessions, s)
			}
		}
	}
	// Stored plans whose calls were not gathered still need a handle.
	for i, e := range events {
		if e.Handle == "" {
			events[i].Handle = r.handleOf(host, e.Actor)
		}
	}
	// Lines said in the room (spec 017), and the residents a caller watched
	// there while they were said.
	says, watched := r.chatLines(host, room, from, to, plans, sessions)
	events = append(events, says...)
	for _, a := range watched {
		if !found(seen, a) {
			seen = append(seen, a)
			if s, ok := sessions[a.Key()]; ok {
				seenSessions = append(seenSessions, s)
			}
		}
	}
	// At the same time: leaves, then lines, then joins.
	rank := map[world.ChatEventKind]int{world.ChatLeave: 0, world.ChatSay: 1, world.ChatJoin: 2}
	sort.SliceStable(events, func(i, j int) bool {
		a, b := events[i], events[j]
		if !a.At.Equal(b.At) {
			return a.At.Before(b.At)
		}
		if a.Kind != b.Kind {
			return rank[a.Kind] < rank[b.Kind]
		}
		if a.Kind == world.ChatSay {
			return false
		}
		return a.Actor.ID < b.Actor.ID
	})
	if len(seen) > 0 {
		observed := presence.Result{Sessions: seenSessions, Profiles: map[string]presence.Profile{}}
		for _, s := range seenSessions {
			if prof, ok := ctx.profiles[s.Actor.ID]; ok {
				observed.Profiles[s.Actor.ID] = prof
			}
		}
		r.observePresence(ps, host, observed, to, seen...)
	}
	return events
}

func found(plans []world.CallActivity, a world.CallActivity) bool {
	for _, p := range plans {
		if p.Key() == a.Key() {
			return true
		}
	}
	return false
}

func (r *Repository) handleOf(host world.Host, actor world.ActorRef) string {
	store, ok := r.Base.(world.PersonaStore)
	if !ok {
		return ""
	}
	for _, p := range store.ListHostPersonas(host.ID) {
		if p.ID == actor.ID {
			return p.Handle
		}
	}
	return ""
}

// presenceWindowCache keeps presence computed on the window grid, per host.
// A chat plan needs four hours of it; recomputing it on every poll of a caller
// in a room costs about 100 ms (spec 016 plan §9). A grid time's result only
// changes when the host's stored sessions, live human calls or residents
// change, which the fingerprint captures; any change drops the host's entries.
type presenceWindowCache struct {
	mu    sync.Mutex
	hosts map[string]*presenceWindows
}

type presenceWindows struct {
	fingerprint string
	results     map[int64]presence.Result
}

// forHost returns the host's cached windows for the fingerprint, dropping
// windows too old to matter at t. The caller holds the host's presence lock,
// which also guards the returned entry.
func (c *presenceWindowCache) forHost(hostID, fingerprint string, t time.Time) *presenceWindows {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.hosts == nil {
		c.hosts = map[string]*presenceWindows{}
	}
	w := c.hosts[hostID]
	if w == nil || w.fingerprint != fingerprint {
		w = &presenceWindows{fingerprint: fingerprint, results: map[int64]presence.Result{}}
		c.hosts[hostID] = w
	}
	oldest := t.Add(-2 * presence.Lookback).Unix()
	for at := range w.results {
		if at < oldest {
			delete(w.results, at)
		}
	}
	return w
}

// presenceFingerprint summarizes what presence on a host is computed from.
// Stored sessions and residents are only ever added, so counts tell a change.
func presenceFingerprint(r *Repository, ps world.PresenceStore, host world.Host) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%d|", len(ps.ListObservedSessions(host.ID, time.Time{}, time.Time{})))
	if store, ok := r.Base.(world.PersonaStore); ok {
		fmt.Fprintf(&b, "%d|", len(store.ListHostPersonas(host.ID)))
	}
	for _, c := range ps.ListHumanCalls(host.ID) {
		fmt.Fprintf(&b, "%s@%d,", c.CallID, c.Line)
	}
	return b.String()
}
