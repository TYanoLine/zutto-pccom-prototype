package lineslot

import (
	"sort"
	"time"

	"zutto-pccom/apps/server/internal/world"
)

// limit applies the limits in time order and drops a slot that does not fit:
// too many messages to the human within an hour, or too close to an earlier
// message, from the same resident or from any resident. Recorded messages
// count and are never moved.
func (c computation) limit(slots []Slot) []Slot {
	type sent struct {
		at      time.Time
		speaker world.ActorRef
	}
	var prior []sent
	for _, m := range c.in.Recorded {
		if m.Slot != "" {
			prior = append(prior, sent{m.SentAt, m.From})
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
		if !s.At.After(c.in.From) || s.At.After(c.in.To) {
			continue
		}
		ok := true
		inHour := 0
		for _, p := range prior {
			d := s.At.Sub(p.at)
			if d < 0 {
				d = -d
			}
			if d < anyResidentGap || (p.speaker == s.Speaker && d < residentGap) {
				ok = false
			}
			if d < time.Hour {
				inHour++
			}
		}
		if !ok || inHour+1 > perHour {
			continue
		}
		prior = append(prior, sent{s.At, s.Speaker})
		out = append(out, s)
	}
	return out
}
