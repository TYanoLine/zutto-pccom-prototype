package worldrepo

import (
	"sync"
	"time"

	"zutto-pccom/apps/server/internal/presence"
	"zutto-pccom/apps/server/internal/world"
)

// Resident presence and line occupancy (spec 015). The computation is pure and
// deterministic; this file decides what becomes observed world history and
// serializes observation per host so two callers never save different results.

var (
	_ world.PresenceObserver = (*Repository)(nil)
	_ world.LineOccupancy    = (*Repository)(nil)
)

var presenceLocks sync.Map // host ID -> *sync.Mutex

func presenceLock(hostID string) *sync.Mutex {
	m, _ := presenceLocks.LoadOrStore(hostID, &sync.Mutex{})
	return m.(*sync.Mutex)
}

func (r *Repository) presenceStore() (world.PresenceStore, bool) {
	ps, ok := r.Base.(world.PresenceStore)
	return ps, ok
}

// presenceAt computes the sessions on the host's lines at t. The caller holds
// the host's presence lock.
func (r *Repository) presenceAt(ps world.PresenceStore, host world.Host, t time.Time) presence.Result {
	in := presence.Input{
		Host:     host,
		Observed: ps.ListObservedSessions(host.ID, t.Add(-presence.Lookback), time.Time{}),
		Humans:   ps.ListHumanCalls(host.ID),
	}
	if store, ok := r.Base.(world.PersonaStore); ok {
		in.Personas = store.ListHostPersonas(host.ID)
	}
	if facts, ok := r.Base.(world.PersonaFactStore); ok {
		in.Facts = facts.ListPersonaFacts
	}
	return presence.Compute(in, t)
}

// observePresence stores the residents' sessions and their connection profiles
// as world history, with the plans of those calls when given (spec 016). Live
// human calls are stored separately when they end.
func (r *Repository) observePresence(ps world.PresenceStore, host world.Host, res presence.Result, at time.Time, plans ...world.CallActivity) {
	if batch, ok := r.Base.(world.PostBatchStore); ok {
		batch.BeginPostBatch(host.ID)
		defer batch.EndPostBatch(host.ID)
	}
	var residents []world.PresenceSession
	for _, s := range res.Sessions {
		if s.Actor.Kind == world.ActorKindPersona {
			residents = append(residents, s)
		}
	}
	ps.SaveObservedSessions(host.ID, residents)
	if len(plans) > 0 {
		ps.SaveCallActivities(host.ID, plans)
	}

	factStore, ok := r.Base.(world.PersonaFactStore)
	if !ok {
		return
	}
	for personaID, prof := range res.Profiles {
		stored := map[string]bool{}
		for _, f := range factStore.ListPersonaFacts(personaID) {
			stored[f.Key] = true
		}
		for _, f := range prof.Facts(personaID, host.ID, at) {
			if !stored[f.Key] {
				factStore.SavePersonaFact(f)
			}
		}
	}
}

// ObservePresenceNow returns who holds the host's lines now and records it.
func (r *Repository) ObservePresenceNow(host world.Host) []world.PresenceSession {
	ps, ok := r.presenceStore()
	if !ok {
		return nil
	}
	lock := presenceLock(host.ID)
	lock.Lock()
	defer lock.Unlock()
	now := r.currentWorldTime()
	res := r.presenceAt(ps, host, now)
	r.observePresence(ps, host, res, now)
	return res.Sessions
}

// DialLine decides a dial at world time at from real line occupancy.
func (r *Repository) DialLine(host world.Host, at time.Time) (int, bool) {
	ps, ok := r.presenceStore()
	if !ok {
		return 1, true
	}
	lock := presenceLock(host.ID)
	lock.Lock()
	defer lock.Unlock()
	res := r.presenceAt(ps, host, at)
	r.observePresence(ps, host, res, at)
	return presence.FreeLine(host, res.Sessions)
}

// BeginHumanCall registers a human call on the lowest free line now.
func (r *Repository) BeginHumanCall(host world.Host, callID string, baud int) (int, bool) {
	ps, ok := r.presenceStore()
	if !ok {
		return 1, true
	}
	lock := presenceLock(host.ID)
	lock.Lock()
	defer lock.Unlock()
	now := r.currentWorldTime()
	res := r.presenceAt(ps, host, now)
	line, free := presence.FreeLine(host, res.Sessions)
	if !free {
		return 0, false
	}
	r.observePresence(ps, host, res, now)
	ps.SaveHumanCall(world.HumanCall{CallID: callID, HostID: host.ID, Line: line, Baud: baud, Start: now})
	return line, true
}

func (r *Repository) SetHumanCallHandle(callID, handle string) {
	ps, ok := r.presenceStore()
	if !ok {
		return
	}
	if c, found := ps.HumanCall(callID); found {
		c.Handle = handle
		ps.SaveHumanCall(c)
	}
}

func (r *Repository) EndHumanCall(callID string) {
	ps, ok := r.presenceStore()
	if !ok {
		return
	}
	c, found := ps.HumanCall(callID)
	if !found {
		return
	}
	lock := presenceLock(c.HostID)
	lock.Lock()
	defer lock.Unlock()
	ps.EndHumanCall(callID, r.currentWorldTime())
}
