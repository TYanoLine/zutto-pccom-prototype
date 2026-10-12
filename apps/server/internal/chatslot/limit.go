package chatslot

import (
	"sort"
	"time"

	"zutto-pccom/apps/server/internal/world"
)

// limit applies the room's limits in time order: a slot too close to an
// earlier line moves back by at most maxShift, and a slot that still does not
// fit is dropped. Recorded resident lines count and are never moved.
func (c computation) limit(slots []Slot, utterances []world.ChatUtterance) []Slot {
	type line struct {
		at      time.Time
		speaker world.ActorRef
	}
	var lines []line
	for _, u := range utterances {
		if u.Slot != "" {
			lines = append(lines, line{u.At, u.Actor})
		}
	}
	sort.SliceStable(slots, func(i, j int) bool {
		if !slots[i].At.Equal(slots[j].At) {
			return slots[i].At.Before(slots[j].At)
		}
		return slots[i].Key < slots[j].Key
	})
	var out []Slot
	for _, s := range slots {
		at := s.At
		for moved := true; moved; {
			moved = false
			for _, l := range lines {
				gap := roomGap
				if l.speaker == s.Speaker {
					gap = speakerGap
				}
				if d := at.Sub(l.at); d > -gap && d < gap {
					at = l.at.Add(gap)
					moved = true
				}
			}
			if at.Sub(s.At) > maxShift {
				break
			}
		}
		if at.Sub(s.At) > maxShift || !at.After(c.in.From) || at.After(c.in.To) || !c.observed(at) || !c.present(s.Speaker, at) {
			continue
		}
		times := make([]time.Time, len(lines))
		for i, l := range lines {
			times[i] = l.at
		}
		sort.Slice(times, func(i, j int) bool { return times[i].Before(times[j]) })
		if !fits(times, at, time.Minute, perMinute) || !fits(times, at, time.Hour, perHour) {
			continue
		}
		s.At = at
		lines = append(lines, line{at, s.Speaker})
		out = append(out, s)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if !out[i].At.Equal(out[j].At) {
			return out[i].At.Before(out[j].At)
		}
		return out[i].Key < out[j].Key
	})
	return out
}

// fits reports whether adding t keeps every span of length span holding at
// most max lines. times is sorted.
func fits(times []time.Time, t time.Time, span time.Duration, max int) bool {
	count := func(from time.Time) int {
		lo := sort.Search(len(times), func(i int) bool { return !times[i].Before(from) })
		hi := sort.Search(len(times), func(i int) bool { return !times[i].Before(from.Add(span)) })
		return hi - lo
	}
	if count(t)+1 > max {
		return false
	}
	first := sort.Search(len(times), func(i int) bool { return times[i].After(t.Add(-span)) })
	for i := first; i < len(times) && !times[i].After(t); i++ {
		if count(times[i])+1 > max {
			return false
		}
	}
	return true
}
