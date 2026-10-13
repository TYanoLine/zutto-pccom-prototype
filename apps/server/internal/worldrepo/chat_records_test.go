package worldrepo

import (
	"testing"
	"time"

	"zutto-pccom/apps/server/internal/world"
)

func TestRecordChatUtteranceAddressesTheRoom(t *testing.T) {
	repo, store, host, now := chatTestRepo(t, true)
	// Find a moment when someone is in room 0.
	var at time.Time
	var inRoom []world.ActorRef
	for step := 0; step < 48 && inRoom == nil; step++ {
		ts := now.Add(time.Duration(step-24) * 5 * time.Minute)
		repo.SetWorldNow(func() time.Time { return ts })
		for _, a := range repo.CallActivitiesNow(host) {
			if seg, ok := a.SegmentAt(ts); ok && seg.State == world.CallChat && seg.Room == 0 {
				at = ts
				inRoom = append(inRoom, a.Actor)
			}
		}
	}
	if inRoom == nil {
		t.Fatal("nobody in room 0 in four busy hours")
	}
	me := world.ActorRef{Kind: world.ActorKindMember, ID: "call-1"}
	saved, err := repo.RecordChatUtterance(host, world.ChatUtterance{Room: 0, Actor: me, Handle: "GUEST", Body: "こんばんは(^^)", At: at})
	if err != nil {
		t.Fatal(err)
	}
	if saved.ID == "" || saved.HostID != host.ID {
		t.Fatalf("saved = %+v", saved)
	}
	if got := store.ListChatUtterances(host.ID, 0, at, time.Time{}); len(got) != 1 || got[0].Body != "こんばんは(^^)" {
		t.Fatalf("stored utterances = %+v", got)
	}
	event, ok := store.GetInteractionEvent(world.ChannelChatUtterance + ":" + saved.ID)
	if !ok {
		t.Fatal("no ledger event")
	}
	if event.Channel != world.ChannelChatUtterance || event.AudienceScope != world.ChatRoomScope(host.ID, 0) ||
		event.ContentRef != "chat:"+saved.ID || event.Actor != me || event.SourceKind != "human_authored" {
		t.Fatalf("event = %+v", event)
	}
	addressed := map[world.ActorRef]bool{}
	for _, a := range event.AddressedTo {
		addressed[a] = true
	}
	for _, a := range inRoom {
		if !addressed[a] {
			t.Fatalf("%v was in the room but not addressed: %+v", a, event.AddressedTo)
		}
	}
	if len(event.AddressedTo) != len(inRoom) {
		t.Fatalf("addressed %v, in room %v", event.AddressedTo, inRoom)
	}
}

func TestSendLineMessageRecordsAPrivateLedgerEntry(t *testing.T) {
	repo, store, host, now := chatTestRepo(t, true)
	from := world.ActorRef{Kind: world.ActorKindMember, ID: "call-1"}
	to := world.ActorRef{Kind: world.ActorKindPersona, ID: "someone"}
	saved, err := repo.SendLineMessage(host, world.LineMessage{From: from, To: to, ToLine: 2, Body: "おひさしぶりです", SentAt: now, DeliverAt: now.Add(10 * time.Minute)})
	if err != nil {
		t.Fatal(err)
	}
	if got := store.ListLineMessages(host.ID); len(got) != 1 || got[0] != saved || !got[0].DeliverAt.Equal(now.Add(10*time.Minute)) {
		t.Fatalf("stored messages = %+v", got)
	}
	event, ok := store.GetInteractionEvent(world.ChannelLineMessage + ":" + saved.ID)
	if !ok {
		t.Fatal("no ledger event")
	}
	if event.AudienceScope != world.PrivateScope(host.ID) || len(event.AddressedTo) != 1 || event.AddressedTo[0] != to ||
		event.ContentRef != "line_message:"+saved.ID || !event.OccurredAt.Equal(now) {
		t.Fatalf("event = %+v", event)
	}
}
