package presence

import (
	"math"
	"sort"
	"time"

	"zutto-pccom/apps/server/internal/world"
	"zutto-pccom/apps/server/internal/worldcalendar"
)

// Lookback covers the longest possible call, so every call that can still be
// on the line at t is considered.
const Lookback = 4 * time.Hour

// Input is everything the computation reads. It never writes.
type Input struct {
	Host     world.Host
	Personas []world.Persona
	// Facts returns a resident's stored persona facts; nil means none.
	Facts func(personaID string) []world.PersonaFact
	// Observed are stored sessions overlapping [t-Lookback, t]. They are world
	// history: they keep their lines and residents in them make no other call.
	Observed []world.PresenceSession
	// Humans are live human calls on this host.
	Humans   []world.HumanCall
	Calendar *worldcalendar.Calendar
}

// Result lists the sessions holding a line at t, in line order, and the
// profiles of the residents among them.
type Result struct {
	Sessions []world.PresenceSession
	Profiles map[string]Profile
}

// Lines is the number of logical lines of the host (at least one).
func Lines(host world.Host) int {
	if host.Lines < 1 {
		return 1
	}
	return host.Lines
}

// FreeLine returns the lowest line not held by any session, or ok=false.
func FreeLine(host world.Host, sessions []world.PresenceSession) (int, bool) {
	used := map[int]bool{}
	for _, s := range sessions {
		used[s.Line] = true
	}
	for line := 1; line <= Lines(host); line++ {
		if !used[line] {
			return line, true
		}
	}
	return 0, false
}

// duration of a call that starts at start. Telehodai callers who start in the
// discount window stay long and hang up before 08:00.
func duration(host world.Host, personaID string, prof Profile, start time.Time) time.Duration {
	median, maxMinutes := 12.0, 90.0
	long := prof.Teleho && inTeleho(start)
	if long {
		median, maxMinutes = 50, 240
	}
	minutes := median * math.Exp(.6*normal("duration", host.ID, personaID, start.Unix()))
	minutes = math.Max(3, math.Min(maxMinutes, minutes))
	d := time.Duration(minutes * float64(time.Minute))
	if long {
		end := time.Date(start.Year(), start.Month(), start.Day(), 7, 59, 0, 0, start.Location())
		if start.Hour() >= 23 {
			end = end.AddDate(0, 0, 1)
		}
		if start.Add(d).After(end) {
			d = end.Sub(start)
		}
	}
	return d
}

func overlaps(a, b world.PresenceSession) bool {
	return a.Overlaps(b.Start, b.End) && b.Overlaps(a.Start, a.End)
}

// Compute returns the sessions holding a line of the host at t.
func Compute(in Input, t time.Time) Result {
	cal := in.Calendar
	if cal == nil {
		cal = worldcalendar.Default()
	}
	host := in.Host
	first := t.Add(-Lookback).Truncate(Window)
	last := t.Truncate(Window)

	// Fixed occupants: stored history and live human calls.
	var assigned []world.PresenceSession
	observedByPersona := map[string][]world.PresenceSession{}
	for _, s := range in.Observed {
		assigned = append(assigned, s)
		if s.Actor.Kind == world.ActorKindPersona {
			observedByPersona[s.Actor.ID] = append(observedByPersona[s.Actor.ID], s)
		}
	}
	for _, h := range in.Humans {
		assigned = append(assigned, h.Session())
	}

	personas := append([]world.Persona(nil), in.Personas...)
	sort.Slice(personas, func(i, j int) bool { return personas[i].ID < personas[j].ID })

	type calKey struct {
		lifestyle string
		window    int64
	}
	calCache := map[calKey]calendarContext{}
	profiles := map[string]Profile{}
	var candidates []world.PresenceSession
	for _, p := range personas {
		var facts []world.PersonaFact
		if in.Facts != nil {
			facts = in.Facts(p.ID)
		}
		prof := ProfileFor(p, facts, host)
		profiles[p.ID] = prof
		var busyUntil time.Time
		for w := first; !w.After(last); w = w.Add(Window) {
			key := calKey{prof.Lifestyle, w.Unix()}
			cc, ok := calCache[key]
			if !ok {
				cc = calendarAt(cal, w, prof.Lifestyle)
				calCache[key] = cc
			}
			if unit("start", host.ID, p.ID, w.Unix()) >= startProbability(p, prof, host, cc, w) {
				continue
			}
			start := w.Add(time.Duration(unit("offset", host.ID, p.ID, w.Unix()) * float64(Window)))
			if start.Before(busyUntil) {
				continue
			}
			s := world.PresenceSession{
				HostID: host.ID, Actor: world.ActorRef{Kind: world.ActorKindPersona, ID: p.ID}, Handle: p.Handle,
				Baud: prof.Baud(host), Start: start, End: start.Add(duration(host, p.ID, prof, start)),
			}
			clash := false
			for _, o := range observedByPersona[p.ID] {
				if overlaps(s, o) {
					clash = true
					break
				}
			}
			if clash {
				continue
			}
			busyUntil = s.End
			candidates = append(candidates, s)
		}
	}

	sort.SliceStable(candidates, func(i, j int) bool {
		if !candidates[i].Start.Equal(candidates[j].Start) {
			return candidates[i].Start.Before(candidates[j].Start)
		}
		return candidates[i].Actor.ID < candidates[j].Actor.ID
	})
	lines := Lines(host)
	for _, c := range candidates {
		for line := 1; line <= lines; line++ {
			free := true
			for _, a := range assigned {
				if a.Line == line && overlaps(a, c) {
					free = false
					break
				}
			}
			if free {
				c.Line = line
				assigned = append(assigned, c)
				break
			}
		}
		// No free line: the resident's dial was BUSY and leaves no trace.
	}

	var out []world.PresenceSession
	used := map[string]Profile{}
	for _, s := range assigned {
		if !s.ActiveAt(t) {
			continue
		}
		out = append(out, s)
		if prof, ok := profiles[s.Actor.ID]; ok && s.Actor.Kind == world.ActorKindPersona {
			used[s.Actor.ID] = prof
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Line < out[j].Line })
	return Result{Sessions: out, Profiles: used}
}
