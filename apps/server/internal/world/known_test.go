package world

import "testing"

func TestKnownAs(t *testing.T) {
	resident := ActorRef{Kind: ActorKindPersona, ID: "p1"}
	human := ActorRef{Kind: ActorKindMember, ID: "u1"}
	if got := KnownAs(resident, ""); got != resident {
		t.Fatalf("resident = %+v", got)
	}
	if got := KnownAs(human, "TARO"); got != (ActorRef{Kind: ActorKindLogin, ID: "TARO"}) {
		t.Fatalf("human = %+v", got)
	}
	for _, login := range []string{"GUEST", ""} {
		if got := KnownAs(human, login); !got.IsZero() {
			t.Fatalf("login %q is known as %+v", login, got)
		}
	}
	// The same human under two login IDs is two people to residents.
	if KnownAs(human, "TARO") == KnownAs(human, "JIRO") {
		t.Fatal("two login IDs known as one person")
	}
}

func TestPostActorKeepsOldRecords(t *testing.T) {
	old := Post{Author: "TARO", AuthorMemberID: "TARO"}
	if got := PostActor(old); got != (ActorRef{Kind: ActorKindMember, ID: "TARO"}) {
		t.Fatalf("old post actor = %+v", got)
	}
	if got := PostKnownAs(old); got != (ActorRef{Kind: ActorKindLogin, ID: "TARO"}) {
		t.Fatalf("old post known as %+v", got)
	}
	now := Post{Author: "TARO", AuthorMemberID: "TARO", AuthorHumanID: "u1"}
	if got := PostActor(now); got != (ActorRef{Kind: ActorKindMember, ID: "u1"}) {
		t.Fatalf("post actor = %+v", got)
	}
	if got := PostKnownAs(now); got != (ActorRef{Kind: ActorKindLogin, ID: "TARO"}) {
		t.Fatalf("post known as %+v", got)
	}
	guest := Post{Author: "GUEST", AuthorMemberID: "GUEST", AuthorHumanID: "u2"}
	if got := PostKnownAs(guest); !got.IsZero() {
		t.Fatalf("guest known as %+v", got)
	}
	resident := Post{Author: "NORI", AuthorPersonaID: "p1"}
	if got := PostActor(resident); got.Kind != ActorKindPersona || PostKnownAs(resident) != got {
		t.Fatalf("resident post = %+v", got)
	}
	if !PostActor(Post{}).IsZero() || !PostKnownAs(Post{}).IsZero() {
		t.Fatal("an empty post has an author")
	}
}

func TestEventKnownActors(t *testing.T) {
	old := InteractionEvent{Actor: ActorRef{Kind: ActorKindMember, ID: "TARO"}, AddressedTo: []ActorRef{{Kind: ActorKindMember, ID: "GUEST"}}}
	if got := old.KnownActor(); got != (ActorRef{Kind: ActorKindLogin, ID: "TARO"}) {
		t.Fatalf("old event actor known as %+v", got)
	}
	if got := old.KnownAddressed(0); !got.IsZero() {
		t.Fatalf("old guest addressee known as %+v", got)
	}
	e := InteractionEvent{
		Actor: ActorRef{Kind: ActorKindMember, ID: "u1"}, ActorLogin: "TARO",
		AddressedTo:     []ActorRef{{Kind: ActorKindPersona, ID: "p1"}, {Kind: ActorKindMember, ID: "u2"}},
		AddressedLogins: []string{"", "JIRO"},
	}
	if e.KnownActor().ID != "TARO" || e.KnownAddressed(0).ID != "p1" || e.KnownAddressed(1).ID != "JIRO" || !e.KnownAddressed(2).IsZero() {
		t.Fatalf("known actors = %+v %+v %+v", e.KnownActor(), e.KnownAddressed(0), e.KnownAddressed(1))
	}
}
