package chatslot

import (
	"fmt"
	"sort"
	"testing"
	"time"

	"zutto-pccom/apps/server/internal/world"
)

func TestSlotsStayWithinLimits(t *testing.T) {
	in := room()
	in.Residents = nil
	for i := 0; i < 8; i++ {
		r := resident(fmt.Sprintf("p%d", i), fmt.Sprintf("R%d", i), t0.Add(-time.Hour), t0.Add(2*time.Hour))
		r.Persona.LurkerTendency, r.Persona.ReplyTendency = 0, 1
		in.Residents = append(in.Residents, r)
	}
	for i := 0; i < 120; i++ {
		in.Utterances = append(in.Utterances, human(i, "R1 R2 R3 どう？", t0.Add(time.Duration(i)*30*time.Second)))
	}
	// A resident line already written counts and is not moved.
	written := world.ChatUtterance{ID: "h:999", Actor: in.Residents[0].Actor, Body: "やあ", At: t0.Add(5 * time.Minute), Slot: "talk|h|0|written"}
	in.Utterances = append(in.Utterances, written)
	slots := Slots(in)
	if len(slots) < perMinute {
		t.Fatalf("only %d slots; the limits were not exercised", len(slots))
	}
	type line struct {
		at      time.Time
		speaker world.ActorRef
	}
	lines := []line{{written.At, written.Actor}}
	for _, s := range slots {
		lines = append(lines, line{s.At, s.Speaker})
	}
	sort.Slice(lines, func(i, j int) bool { return lines[i].at.Before(lines[j].at) })
	last := map[world.ActorRef]time.Time{}
	for i, l := range lines {
		if i > 0 && l.at.Sub(lines[i-1].at) < roomGap {
			t.Fatalf("lines %v apart at %v", l.at.Sub(lines[i-1].at), l.at)
		}
		if prev, ok := last[l.speaker]; ok && l.at.Sub(prev) < speakerGap {
			t.Fatalf("%s spoke %v apart at %v", l.speaker.ID, l.at.Sub(prev), l.at)
		}
		last[l.speaker] = l.at
		perMin, perHr := 0, 0
		for _, o := range lines {
			if !o.at.Before(l.at) && o.at.Before(l.at.Add(time.Minute)) {
				perMin++
			}
			if !o.at.Before(l.at) && o.at.Before(l.at.Add(time.Hour)) {
				perHr++
			}
		}
		if perMin > perMinute || perHr > perHour {
			t.Fatalf("%d lines in a minute and %d in an hour from %v", perMin, perHr, l.at)
		}
	}
	for _, s := range slots {
		if s.Key == written.Slot {
			t.Fatalf("a written line came back as a slot: %+v", s)
		}
	}
}
