package lineslot

import (
	"fmt"
	"testing"
	"time"

	"zutto-pccom/apps/server/internal/world"
)

func TestLimitsHoldInABusyStation(t *testing.T) {
	in := baseInput()
	in.To = t0.Add(time.Hour)
	in.Relation = func(a, b world.ActorRef) int { return 2 }
	for i := 0; i < 20; i++ {
		id := fmt.Sprint("p", i)
		in.Residents = append(in.Residents, resident(id, 2+i, t0, t0.Add(2*time.Hour)))
		for j := 0; j < 5; j++ {
			in.Sent = append(in.Sent, sentTo(fmt.Sprint(id, "-", j), persona(id), 2+i, t0.Add(time.Duration(j)*time.Minute), "やあ"))
		}
	}
	got := Slots(in)
	if len(got) == 0 || len(got) > perHour {
		t.Fatalf("%d slots in the hour, want 1..%d", len(got), perHour)
	}
	for i := 1; i < len(got); i++ {
		if got[i].At.Sub(got[i-1].At) < anyResidentGap {
			t.Fatalf("slots %v and %v are too close", got[i-1], got[i])
		}
	}
	last := map[world.ActorRef]time.Time{}
	for _, s := range got {
		if prev, ok := last[s.Speaker]; ok && s.At.Sub(prev) < residentGap {
			t.Fatalf("%v wrote twice within %v", s.Speaker, residentGap)
		}
		last[s.Speaker] = s.At
	}
}

func TestRecordedMessagesCountTowardTheHourlyLimit(t *testing.T) {
	in := baseInput()
	in.To = t0.Add(time.Hour)
	in.Residents = []Resident{resident("a", 2, t0, t0.Add(2*time.Hour))}
	in.Relation = func(a, b world.ActorRef) int { return 2 }
	for i := 0; i < perHour; i++ {
		in.Recorded = append(in.Recorded, world.LineMessage{From: persona("z"), Slot: fmt.Sprint("answer|r", i), SentAt: t0.Add(time.Duration(i) * 4 * time.Minute)})
	}
	for j := 0; j < 30; j++ {
		in.Sent = append(in.Sent, sentTo(fmt.Sprint(j), persona("a"), 2, t0.Add(time.Duration(j)*time.Minute), "やあ"))
	}
	if got := Slots(in); len(got) != 0 {
		t.Fatalf("slots over the hourly limit: %v", got)
	}
}
