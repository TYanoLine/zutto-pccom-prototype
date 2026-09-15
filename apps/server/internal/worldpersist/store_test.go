package worldpersist

import (
	"context"
	"testing"

	"zutto-pccom/apps/server/internal/world"
)

type fakeBackend struct {
	data  []byte
	saves int
}

func (b *fakeBackend) Load(context.Context, string) ([]byte, bool, error) {
	if len(b.data) == 0 {
		return nil, false, nil
	}
	return append([]byte(nil), b.data...), true, nil
}
func (b *fakeBackend) Save(_ context.Context, _, _ string, _ int, data []byte) error {
	b.data = append([]byte(nil), data...)
	b.saves++
	return nil
}
func (b *fakeBackend) Close() {}

func TestStoreRestoresAcrossFreshMemoryStore(t *testing.T) {
	ctx := context.Background()
	backend := &fakeBackend{}
	base := world.NewMemoryStore()
	store, err := newStore(ctx, base, "0450000196", backend)
	if err != nil {
		t.Fatal(err)
	}
	host, _ := store.HostByPhone("0450000196")
	host.Name = "PERSISTED"
	store.SaveHost(host)
	store.SaveBoards(host.ID, []world.Board{{ID: "3", Name: "地域の話題"}})
	persona := world.Persona{ID: host.ID + "-p", Handle: "P", Interests: map[string]float64{"local": 1}}
	store.SavePersona(persona)
	store.AddMembership(host.ID, persona.ID)
	post := store.AddPost(host.ID, world.Post{BoardID: "3", Author: "P", AuthorPersonaID: persona.ID, Subject: "途中"})
	post.Body = "本文まで保存"
	if _, ok := store.UpdatePost(host.ID, post); !ok {
		t.Fatal("update failed")
	}
	if backend.saves == 0 {
		t.Fatal("expected persisted snapshots")
	}

	fresh := world.NewMemoryStore()
	restored, err := newStore(ctx, fresh, "0450000196", backend)
	if err != nil {
		t.Fatal(err)
	}
	status := restored.DevelopmentPersistenceStatus()
	if !status.Enabled || !status.Loaded {
		t.Fatalf("status=%+v", status)
	}
	gotHost, _ := restored.HostByPhone("0450000196")
	if gotHost.Name != "PERSISTED" {
		t.Fatalf("host=%+v", gotHost)
	}
	posts := restored.ListPosts(host.ID)
	if len(posts) != 1 || posts[0].Body != "本文まで保存" {
		t.Fatalf("posts=%+v", posts)
	}
	if personas := restored.ListHostPersonas(host.ID); len(personas) != 1 || personas[0].Handle != "P" {
		t.Fatalf("personas=%+v", personas)
	}

	before := backend.saves
	if count := restored.ClearHostPosts(host.ID); count != 1 {
		t.Fatalf("cleared=%d", count)
	}
	if backend.saves <= before {
		t.Fatal("reset mutation was not persisted")
	}

	freshAgain := world.NewMemoryStore()
	restoredAgain, err := newStore(ctx, freshAgain, "0450000196", backend)
	if err != nil {
		t.Fatal(err)
	}
	if posts := restoredAgain.ListPosts(host.ID); len(posts) != 0 {
		t.Fatalf("reset posts restored unexpectedly: %+v", posts)
	}
}

func TestStoreSkipsPersistenceForOtherHostPersonaFacts(t *testing.T) {
	ctx := context.Background()
	backend := &fakeBackend{}
	base := world.NewMemoryStore()
	store, err := newStore(ctx, base, "0450000196", backend)
	if err != nil {
		t.Fatal(err)
	}

	otherHost, err := store.HostByPhone("0920000196")
	if err != nil {
		t.Fatal(err)
	}
	otherPersona := world.Persona{ID: otherHost.ID + "-p", Handle: "OUTSIDE"}
	store.SavePersona(otherPersona)
	store.AddMembership(otherHost.ID, otherPersona.ID)
	store.SavePersonaFact(world.PersonaFact{PersonaID: otherPersona.ID, Key: "outside.note", Value: "other host"})
	if backend.saves != 0 {
		t.Fatalf("unexpected persist for other-host fact save: %d", backend.saves)
	}
	if cleared := store.ClearPersonaFacts([]string{otherPersona.ID}); cleared != 1 {
		t.Fatalf("cleared=%d", cleared)
	}
	if backend.saves != 0 {
		t.Fatalf("unexpected persist for other-host fact clear: %d", backend.saves)
	}
}
