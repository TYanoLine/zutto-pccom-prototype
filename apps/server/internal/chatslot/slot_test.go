package chatslot

import (
	"fmt"
	"reflect"
	"testing"
	"time"

	"zutto-pccom/apps/server/internal/world"
)

var t0 = time.Date(1996, 10, 15, 23, 30, 0, 0, time.FixedZone("JST", 9*3600))

var me = world.ActorRef{Kind: world.ActorKindMember, ID: "call-1"}

func resident(id, handle string, from, to time.Time) ResidentStay {
	return ResidentStay{
		Actor: world.ActorRef{Kind: world.ActorKindPersona, ID: id}, Handle: handle, Call: id + "@call",
		Persona: world.Persona{ID: id, Handle: handle, ReplyTendency: .6, LurkerTendency: .3, NewcomerOpenness: .6},
		From:    from, To: to,
	}
}

// room is three residents in the room for two hours and a caller who enters
// at t0 and stays an hour.
func room() Input {
	return Input{
		Host: world.Host{ID: "h"}, Room: 0, From: t0.Add(-time.Minute), To: t0.Add(time.Hour),
		Viewers: []Stay{{Actor: me, Handle: "GUEST", From: t0}},
		Residents: []ResidentStay{
			resident("p1", "NORI", t0.Add(-time.Hour), t0.Add(time.Hour)),
			resident("p2", "KAZU", t0.Add(-30*time.Minute), t0.Add(40*time.Minute)),
			resident("p3", "ﾐﾅﾐ", t0.Add(10*time.Minute), t0.Add(2*time.Hour)),
		},
	}
}

func human(n int, body string, at time.Time) world.ChatUtterance {
	return world.ChatUtterance{ID: fmt.Sprintf("h:%d", n), HostID: "h", Actor: me, Handle: "GUEST", Body: body, At: at}
}

func TestSlotsAreDeterministic(t *testing.T) {
	in := room()
	in.Utterances = []world.ChatUtterance{human(1, "こんばんは", t0.Add(time.Minute))}
	a, b := Slots(in), Slots(in)
	if len(a) == 0 {
		t.Fatal("no slots in an hour with three residents")
	}
	if !reflect.DeepEqual(a, b) {
		t.Fatalf("the same input gave different slots:\n%v\n%v", a, b)
	}
}

func TestSlotsOnlyWhileWatched(t *testing.T) {
	in := room()
	in.From, in.To = t0.Add(-time.Hour), t0.Add(2*time.Hour)
	in.Viewers = []Stay{{Actor: me, From: t0, To: t0.Add(20 * time.Minute)}}
	slots := Slots(in)
	if len(slots) == 0 {
		t.Fatal("no slots while watched")
	}
	for _, s := range slots {
		if s.At.Before(t0) || !s.At.Before(t0.Add(20*time.Minute)) {
			t.Fatalf("slot outside the caller's stay: %+v", s)
		}
	}
	in.Viewers = nil
	if slots := Slots(in); len(slots) != 0 {
		t.Fatalf("slots with nobody watching: %+v", slots)
	}
	// Joins before the caller came are not greeted.
	in.Viewers = []Stay{{Actor: me, From: t0.Add(15 * time.Minute)}}
	for _, s := range Slots(in) {
		if s.Kind == Greet && KindOf(s.Key) == Greet && s.Speaker.ID == "p3" {
			t.Fatalf("greeting for an entry nobody saw: %+v", s)
		}
	}
}

func TestOneAnswerPerTrigger(t *testing.T) {
	in := room()
	for i := 0; i < 50; i++ {
		in.Utterances = append(in.Utterances, human(i, "うーん", t0.Add(time.Duration(i)*time.Minute)))
	}
	count := map[string]int{}
	for _, s := range Slots(in) {
		if s.Kind == Welcome || s.Kind == Reply || s.Kind == News {
			count[string(s.Kind)+s.Trigger]++
		}
	}
	for k, n := range count {
		if n > 1 {
			t.Fatalf("%d answers to %s", n, k)
		}
	}
}

// replyRates counts, over many single-line rooms, how often a line gets a
// reply and how often NORI gives it.
func replyRates(body string) (any, nori, total int) {
	for i := 0; i < 400; i++ {
		in := room()
		in.Host.ID = fmt.Sprintf("h%d", i)
		in.Utterances = []world.ChatUtterance{human(i, body, t0.Add(time.Minute))}
		total++
		for _, s := range Slots(in) {
			if s.Kind == Reply && s.RespondsTo == in.Utterances[0].ID {
				any++
				if s.Speaker.ID == "p1" {
					nori++
				}
			}
		}
	}
	return any, nori, total
}

func TestNamedResidentRepliesMoreOften(t *testing.T) {
	_, named, total := replyRates("NORIさん、それって何ですか？")
	anyPlain, plain, _ := replyRates("それって何ですか？")
	if named <= plain {
		t.Fatalf("NORI replied to %d/%d lines naming them and %d/%d plain lines", named, total, plain, total)
	}
	if anyPlain == total {
		t.Fatal("every plain line got a reply; silence must be possible")
	}
}

func TestUnrecordedSlotsGetNoAnswer(t *testing.T) {
	in := room()
	slots := Slots(in)
	for _, s := range slots {
		for _, o := range slots {
			if o.Kind == Reply && o.Trigger == s.Key {
				t.Fatalf("reply %+v to a line that was never written", o)
			}
		}
	}
}

func TestReplyDepthIsLimited(t *testing.T) {
	in := room()
	in.From = t0
	prev := human(0, "どう？", t0.Add(time.Minute))
	in.Utterances = []world.ChatUtterance{prev}
	speakers := []world.ActorRef{in.Residents[0].Actor, in.Residents[1].Actor}
	for d := 1; d <= MaxDepth; d++ {
		u := world.ChatUtterance{ID: fmt.Sprintf("h:%d", 100+d), Actor: speakers[d%2], Body: "うん",
			At: prev.At.Add(30 * time.Second), Slot: fmt.Sprintf("reply|h|0|%s|x%d", speakers[d%2], d), RespondsTo: prev.ID}
		in.Utterances = append(in.Utterances, u)
		prev = u
	}
	for _, s := range Slots(in) {
		if s.Kind == Reply && s.Depth > MaxDepth {
			t.Fatalf("reply deeper than %d: %+v", MaxDepth, s)
		}
		if s.Kind == Reply && s.RespondsTo == prev.ID {
			t.Fatalf("reply to a line at depth %d: %+v", MaxDepth, s)
		}
	}
}

func TestSendoffsInsteadOfReplies(t *testing.T) {
	known, stranger := 0, 0
	sawSendoff := false
	for i := 0; i < 300; i++ {
		in := room()
		in.Host.ID = fmt.Sprintf("h%d", i)
		in.Utterances = []world.ChatUtterance{human(i, "そろそろ落ちます〜", t0.Add(time.Minute))}
		if i%2 == 0 {
			in.Relation = func(a, b world.ActorRef) int { return 2 }
		}
		n := 0
		for _, s := range Slots(in) {
			if s.RespondsTo != in.Utterances[0].ID {
				continue
			}
			if s.Kind == Reply {
				t.Fatalf("reply to a leaving line: %+v", s)
			}
			if s.Kind == Sendoff {
				n++
				sawSendoff = true
			}
		}
		if n > maxSendoffs {
			t.Fatalf("%d send-offs", n)
		}
		if i%2 == 0 {
			known += n
		} else {
			stranger += n
		}
	}
	if !sawSendoff || known <= stranger {
		t.Fatalf("send-offs: %d from regulars, %d from strangers", known, stranger)
	}
	in := room()
	in.Utterances = []world.ChatUtterance{human(1, "なるほど", t0.Add(time.Minute))}
	for _, s := range Slots(in) {
		if s.Kind == Sendoff {
			t.Fatalf("send-off for a line that does not leave: %+v", s)
		}
	}
}

func TestNewsOnlyFromAcquaintances(t *testing.T) {
	friendOf := func(id string) func(a, b world.ActorRef) int {
		return func(a, b world.ActorRef) int {
			if (a.ID == id && b == me) || (b.ID == id && a == me) {
				return 2
			}
			return 0
		}
	}
	got := 0
	for i := 0; i < 200; i++ {
		in := room()
		in.Host.ID = fmt.Sprintf("h%d", i)
		in.Relation = friendOf("p1")
		in.Events = []WorldEvent{{ID: fmt.Sprintf("post:%d", i), Subject: me, At: t0.Add(-10 * time.Minute), Kind: "posted", Board: "雑談", Title: "テスト"}}
		n := 0
		for _, s := range Slots(in) {
			if s.Kind != News {
				continue
			}
			n++
			if s.Speaker.ID != "p1" || s.Addressee != me || s.Event != in.Events[0].ID {
				t.Fatalf("news from a stranger or about someone else: %+v", s)
			}
			if s.At.Sub(in.Events[0].At) > NewsWindow {
				t.Fatalf("news older than %v: %+v", NewsWindow, s)
			}
		}
		if n > 1 {
			t.Fatalf("%d residents brought up one event", n)
		}
		got += n
	}
	if got == 0 {
		t.Fatal("no acquaintance ever brought up the event")
	}

	in := room()
	in.Relation = func(world.ActorRef, world.ActorRef) int { return 2 }
	in.Events = []WorldEvent{{ID: "old", Subject: me, At: t0.Add(-NewsWindow - time.Minute), Kind: "posted"}}
	for _, s := range Slots(in) {
		if s.Kind == News {
			t.Fatalf("news about an event older than %v: %+v", NewsWindow, s)
		}
	}
}
