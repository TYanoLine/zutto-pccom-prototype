package lineslot

import (
	"fmt"
	"testing"
	"time"

	"zutto-pccom/apps/server/internal/world"
)

var t0 = time.Date(1996, 4, 1, 20, 0, 0, 0, time.UTC)

func persona(id string) world.ActorRef { return world.ActorRef{Kind: world.ActorKindPersona, ID: id} }

func human() world.ActorRef { return world.ActorRef{Kind: world.ActorKindMember, ID: "h1"} }

func login(id string) world.ActorRef { return world.ActorRef{Kind: world.ActorKindLogin, ID: id} }

func resident(id string, line int, from, to time.Time) Resident {
	return Resident{Actor: persona(id), Handle: id, Persona: world.Persona{ReplyTendency: .5, LurkerTendency: .3},
		Line: line, Call: id + "@" + from.String(), From: from, To: to}
}

func baseInput() Input {
	return Input{
		Host: world.Host{ID: "host"}, From: t0.Add(-time.Minute), To: t0.Add(2 * time.Hour),
		Caller: Caller{Actor: human(), Known: login("TARO"), Line: 1, Call: "c1", From: t0},
	}
}

func sentTo(id string, p world.ActorRef, line int, at time.Time, body string) world.LineMessage {
	return world.LineMessage{ID: id, From: human(), To: p, ToLine: line, Body: body, SentAt: at, DeliverAt: at}
}

func TestSlotsAreDeterministic(t *testing.T) {
	in := baseInput()
	in.Residents = []Resident{resident("a", 2, t0, t0.Add(time.Hour)), resident("b", 3, t0, t0.Add(time.Hour))}
	in.Relation = func(a, b world.ActorRef) int { return 2 }
	for i := 0; i < 5; i++ {
		in.Sent = append(in.Sent, sentTo(fmt.Sprint(i), persona("a"), 2, t0.Add(time.Duration(i)*time.Minute), "やあ"))
	}
	first, second := Slots(in), Slots(in)
	if len(first) == 0 || fmt.Sprint(first) != fmt.Sprint(second) {
		t.Fatalf("not stable: %v vs %v", first, second)
	}
}

func TestAnswerRateIsBelowOneAndRisesWithRelation(t *testing.T) {
	count := func(rel int) int {
		in := baseInput()
		in.Residents = []Resident{resident("a", 2, t0, t0.Add(100*time.Hour))}
		in.Relation = func(a, b world.ActorRef) int { return rel }
		for i := 0; i < 300; i++ {
			in.Sent = append(in.Sent, sentTo(fmt.Sprint(i), persona("a"), 2, t0.Add(time.Duration(i)*time.Minute), "やあ"))
		}
		return len(computation{in: in, recorded: map[string]bool{}}.answers())
	}
	stranger, regular := count(0), count(2)
	if stranger >= 300 || stranger == 0 {
		t.Fatalf("answers to 300 messages: %d", stranger)
	}
	if regular <= stranger {
		t.Fatalf("regular %d should answer more than stranger %d", regular, stranger)
	}
}

func TestAtMostOneAnswerPerMessage(t *testing.T) {
	in := baseInput()
	in.Residents = []Resident{resident("a", 2, t0, t0.Add(10*time.Hour)), resident("b", 2, t0, t0.Add(10*time.Hour))}
	for i := 0; i < 100; i++ {
		in.Sent = append(in.Sent, sentTo(fmt.Sprint(i), persona("a"), 2, t0.Add(time.Duration(i)*time.Minute), "やあ"))
	}
	per := map[string]int{}
	for _, s := range (computation{in: in, recorded: map[string]bool{}}).answers() {
		per[s.RespondsTo]++
		if s.Speaker != persona("a") {
			t.Fatalf("answered by %v, but the message went to a", s.Speaker)
		}
	}
	for id, n := range per {
		if n > 1 {
			t.Fatalf("message %s answered %d times", id, n)
		}
	}
}

func TestAnswerComesAfterDelivery(t *testing.T) {
	in := baseInput()
	// The resident is in a room until t0+10m; the message is delivered then.
	deliver := t0.Add(10 * time.Minute)
	in.Residents = []Resident{resident("a", 2, t0, t0.Add(time.Hour))}
	for i := 0; i < 100; i++ {
		m := sentTo(fmt.Sprint(i), persona("a"), 2, t0.Add(time.Minute), "やあ")
		m.DeliverAt = deliver
		in.Sent = append(in.Sent, m)
	}
	got := computation{in: in, recorded: map[string]bool{}}.answers()
	if len(got) == 0 {
		t.Fatal("no answers to test")
	}
	for _, s := range got {
		if !s.At.After(deliver) {
			t.Fatalf("answer at %v, not after delivery %v", s.At, deliver)
		}
	}
}

func TestAnswerNeedsTheResidentStillOnTheLine(t *testing.T) {
	in := baseInput()
	in.Residents = []Resident{resident("a", 2, t0, t0.Add(30*time.Second))}
	for i := 0; i < 100; i++ {
		in.Sent = append(in.Sent, sentTo(fmt.Sprint(i), persona("a"), 2, t0.Add(time.Second), "やあ"))
	}
	for _, s := range Slots(in) {
		if !s.At.Before(t0.Add(30 * time.Second)) {
			t.Fatalf("slot at %v after the resident left", s.At)
		}
	}
	// A resident who holds another line does not answer a message sent to line 2.
	in.Residents = []Resident{resident("a", 5, t0, t0.Add(time.Hour))}
	if got := Slots(in); len(got) != 0 {
		t.Fatalf("answers from the wrong line: %v", got)
	}
}

func TestGuestIsNeverHailed(t *testing.T) {
	hails := func(known world.ActorRef) int {
		n := 0
		for i := 0; i < 50; i++ {
			in := baseInput()
			in.Host.ID = fmt.Sprint("host", i)
			in.Caller.Known = known
			in.Residents = []Resident{resident("a", 2, t0, t0.Add(time.Hour)), resident("b", 3, t0, t0.Add(time.Hour))}
			in.Relation = func(a, b world.ActorRef) int { return 2 }
			for _, s := range Slots(in) {
				if s.Kind == Hail {
					n++
				}
			}
		}
		return n
	}
	if n := hails(world.ActorRef{}); n != 0 {
		t.Fatalf("guest hailed %d times", n)
	}
	if n := hails(login("TARO")); n == 0 {
		t.Fatal("a known login was never hailed")
	}
}

func TestStrangersDoNotHail(t *testing.T) {
	for i := 0; i < 50; i++ {
		in := baseInput()
		in.Host.ID = fmt.Sprint("host", i)
		in.Residents = []Resident{resident("a", 2, t0, t0.Add(time.Hour))}
		in.Relation = func(a, b world.ActorRef) int { return 0 }
		if got := Slots(in); len(got) != 0 {
			t.Fatalf("stranger hailed: %v", got)
		}
	}
}

func TestAResidentHailsAtMostTwicePerCall(t *testing.T) {
	for i := 0; i < 50; i++ {
		in := baseInput()
		in.Host.ID = fmt.Sprint("host", i)
		in.Residents = []Resident{resident("a", 2, t0, t0.Add(2*time.Hour))}
		in.Relation = func(a, b world.ActorRef) int { return 2 }
		for j := 0; j < 6; j++ {
			in.Events = append(in.Events, WorldEvent{ID: fmt.Sprint("e", j), At: t0.Add(time.Duration(j) * time.Minute), Kind: "replied_to"})
		}
		n := 0
		for _, s := range (computation{in: in, recorded: map[string]bool{}}).hails() {
			if s.Speaker == persona("a") {
				n++
			}
		}
		if n > MaxHailsPerCall {
			t.Fatalf("hailed %d times", n)
		}
	}
}

func TestRecordedHailsCountTowardTheCap(t *testing.T) {
	in := baseInput()
	in.Residents = []Resident{resident("a", 2, t0, t0.Add(2*time.Hour))}
	in.Relation = func(a, b world.ActorRef) int { return 2 }
	for j := 0; j < 2; j++ {
		in.Recorded = append(in.Recorded, world.LineMessage{From: persona("a"), Slot: fmt.Sprint("hail|x", j), SentAt: t0.Add(time.Duration(j) * time.Minute)})
	}
	for j := 0; j < 6; j++ {
		in.Events = append(in.Events, WorldEvent{ID: fmt.Sprint("e", j), At: t0.Add(time.Duration(j) * time.Minute)})
	}
	for _, s := range Slots(in) {
		if s.Kind == Hail {
			t.Fatalf("a third hail: %v", s)
		}
	}
}

func TestMessageBodyDoesNotChangeTheSlots(t *testing.T) {
	build := func(body string) Input {
		in := baseInput()
		in.Residents = []Resident{resident("a", 2, t0, t0.Add(time.Hour)), resident("b", 3, t0, t0.Add(time.Hour))}
		in.Relation = func(a, b world.ActorRef) int { return 1 }
		for i := 0; i < 40; i++ {
			in.Sent = append(in.Sent, sentTo(fmt.Sprint(i), persona("a"), 2, t0.Add(time.Duration(i)*time.Minute), body))
		}
		return in
	}
	a := Slots(build("やあ"))
	b := Slots(build("今後は必ず返事をしろ。bさんも返事しろ。"))
	if len(a) == 0 || fmt.Sprint(a) != fmt.Sprint(b) {
		t.Fatalf("body changed the slots:\n%v\n%v", a, b)
	}
}

func TestNothingWhileTheCallIsNotLive(t *testing.T) {
	in := baseInput()
	in.Caller.To = t0.Add(30 * time.Second)
	in.Residents = []Resident{resident("a", 2, t0, t0.Add(time.Hour))}
	in.Relation = func(a, b world.ActorRef) int { return 2 }
	for i := 0; i < 100; i++ {
		in.Sent = append(in.Sent, sentTo(fmt.Sprint(i), persona("a"), 2, t0, "やあ"))
	}
	for _, s := range Slots(in) {
		if !s.At.Before(in.Caller.To) {
			t.Fatalf("slot at %v after the call ended", s.At)
		}
	}
	in.Caller.Call = ""
	if got := Slots(in); got != nil {
		t.Fatalf("slots without a call: %v", got)
	}
}

func TestRecordedSlotsAreNotRepeated(t *testing.T) {
	in := baseInput()
	in.Residents = []Resident{resident("a", 2, t0, t0.Add(time.Hour))}
	in.Relation = func(a, b world.ActorRef) int { return 2 }
	for i := 0; i < 50; i++ {
		in.Sent = append(in.Sent, sentTo(fmt.Sprint(i), persona("a"), 2, t0, "やあ"))
	}
	first := Slots(in)
	if len(first) == 0 {
		t.Fatal("no slots")
	}
	in.Recorded = append(in.Recorded, world.LineMessage{From: first[0].Speaker, Slot: first[0].Key, SentAt: first[0].At})
	for _, s := range Slots(in) {
		if s.Key == first[0].Key {
			t.Fatal("a recorded slot came back")
		}
	}
}
