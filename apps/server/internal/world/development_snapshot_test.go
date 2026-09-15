package world

import (
	"testing"
	"time"
)

func TestDevelopmentSnapshotRoundTripPreservesMaterializedState(t *testing.T) {
	store := NewMemoryStore()
	host, err := store.HostByPhone("0450000196")
	if err != nil {
		t.Fatal(err)
	}
	host.Name = "PERSIST TEST"
	store.SaveHost(host)
	store.SaveBoards(host.ID, []Board{{ID: "1", Name: "雑談"}})
	persona := Persona{ID: host.ID + "-p1", Handle: "P1", EverydayContext: []string{"日常"}, Interests: map[string]float64{"local": .8}}
	store.SavePersona(persona)
	store.AddMembership(host.ID, persona.ID)
	store.SavePersonaFact(PersonaFact{PersonaID: persona.ID, Key: "commute.route", Value: "駅まで徒歩", MaterializedAt: time.Date(1996, 8, 20, 1, 2, 3, 0, time.UTC)})
	post := store.AddPost(host.ID, Post{BoardID: "1", Author: "P1", AuthorPersonaID: persona.ID, Subject: "test", Intent: PostIntent{Action: "root", AnchorKey: "local", Claims: []string{"x"}}, CreatedAt: time.Date(1996, 8, 21, 1, 2, 3, 0, time.UTC)})
	post.Body = "saved body"
	if _, ok := store.UpdatePost(host.ID, post); !ok {
		t.Fatal("update post failed")
	}

	snapshot, ok := store.DevelopmentSnapshot(host.Phone)
	if !ok {
		t.Fatal("snapshot not found")
	}
	if snapshot.NextPostID < post.ID {
		t.Fatalf("next post id=%d, post id=%d", snapshot.NextPostID, post.ID)
	}

	restored := NewMemoryStore()
	if err := restored.RestoreDevelopmentSnapshot(snapshot); err != nil {
		t.Fatal(err)
	}
	gotHost, _ := restored.HostByPhone(host.Phone)
	if gotHost.Name != host.Name {
		t.Fatalf("host name=%q, want %q", gotHost.Name, host.Name)
	}
	if got := restored.ListBoards(host.ID); len(got) != 1 || got[0].Name != "雑談" {
		t.Fatalf("boards=%+v", got)
	}
	if got := restored.ListHostPersonas(host.ID); len(got) != 1 || got[0].Handle != "P1" {
		t.Fatalf("personas=%+v", got)
	}
	if got := restored.ListPersonaFacts(persona.ID); len(got) != 1 || got[0].Value != "駅まで徒歩" {
		t.Fatalf("facts=%+v", got)
	}
	if got := restored.ListPosts(host.ID); len(got) != 1 || got[0].Body != "saved body" || got[0].Intent.AnchorKey != "local" {
		t.Fatalf("posts=%+v", got)
	}

	next := restored.AddPost(host.ID, Post{BoardID: "1", Author: "P1", Subject: "next"})
	if next.ID <= post.ID {
		t.Fatalf("restored next id=%d, want > %d", next.ID, post.ID)
	}
}
