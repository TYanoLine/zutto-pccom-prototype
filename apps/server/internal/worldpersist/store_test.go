package worldpersist

import (
	"context"
	"testing"

	"zutto-pccom/apps/server/internal/world"
)

type fakeBackend struct {
	data  map[string][]byte
	saves int
}

func (b *fakeBackend) Load(_ context.Context, hostID string) ([]byte, bool, error) {
	data := b.data[hostID]
	if len(data) == 0 {
		return nil, false, nil
	}
	return append([]byte(nil), data...), true, nil
}
func (b *fakeBackend) Save(_ context.Context, hostID, _ string, _ int, data []byte) error {
	if b.data == nil {
		b.data = map[string][]byte{}
	}
	b.data[hostID] = append([]byte(nil), data...)
	b.saves++
	return nil
}
func (b *fakeBackend) Close() {}

func TestStoreRestoresAcrossFreshMemoryStore(t *testing.T) {
	ctx := context.Background()
	backend := &fakeBackend{}
	base := world.NewMemoryStore()
	store, err := newStore(ctx, base, []HostTarget{{Phone: "0450000001"}}, backend)
	if err != nil {
		t.Fatal(err)
	}
	host, _ := store.HostByPhone("0450000001")
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
	restored, err := newStore(ctx, fresh, []HostTarget{{Phone: "0450000001"}}, backend)
	if err != nil {
		t.Fatal(err)
	}
	status := restored.DevelopmentPersistenceStatus()
	if !status.Enabled || !status.Loaded {
		t.Fatalf("status=%+v", status)
	}
	gotHost, _ := restored.HostByPhone("0450000001")
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
	restoredAgain, err := newStore(ctx, freshAgain, []HostTarget{{Phone: "0450000001"}}, backend)
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
	store, err := newStore(ctx, base, []HostTarget{{Phone: "0450000001"}}, backend)
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

func TestStorePersistsErikaWorldButKeepsCodeDefinedHostConfig(t *testing.T) {
	ctx := context.Background()
	backend := &fakeBackend{}
	targets := []HostTarget{
		{Phone: "0450000001"},
		{Phone: "0920000196", KeepSeedHostConfig: true},
	}

	base := world.NewMemoryStore()
	store, err := newStore(ctx, base, targets, backend)
	if err != nil {
		t.Fatal(err)
	}
	host, err := store.HostByPhone("0920000196")
	if err != nil {
		t.Fatal(err)
	}
	seedName := host.Name
	seedLines := host.Lines

	// Even if a runtime mutation writes host metadata into the snapshot, this
	// experiment host must come back with the code-defined fixture settings.
	host.Name = "MUTATED SNAPSHOT NAME"
	host.Lines = 99
	store.SaveHost(host)

	persona := world.Persona{ID: host.ID + "-mari-persist", Handle: "MARI", Interests: map[string]float64{"local": 0.9}}
	store.SavePersona(persona)
	store.AddMembership(host.ID, persona.ID)
	store.SavePersonaFact(world.PersonaFact{PersonaID: persona.ID, Key: "local.favorite_place", Value: "天神"})
	post := store.AddPost(host.ID, world.Post{
		BoardID:         "4",
		Author:          "MARI",
		AuthorPersonaID: persona.ID,
		Subject:         "永続化テスト",
		Body:            "再起動しても残る本文",
	})

	if len(backend.data[host.ID]) == 0 {
		t.Fatal("expected Erika host snapshot")
	}

	fresh := world.NewMemoryStore()
	restored, err := newStore(ctx, fresh, targets, backend)
	if err != nil {
		t.Fatal(err)
	}
	gotHost, err := restored.HostByPhone("0920000196")
	if err != nil {
		t.Fatal(err)
	}
	if gotHost.Name != seedName || gotHost.Lines != seedLines {
		t.Fatalf("fixed host config was restored from snapshot: got=%+v want name=%q lines=%d", gotHost, seedName, seedLines)
	}

	posts := restored.ListPosts(host.ID)
	foundPost := false
	for _, got := range posts {
		if got.ID == post.ID && got.Subject == "永続化テスト" && got.Body == "再起動しても残る本文" {
			foundPost = true
			break
		}
	}
	if !foundPost {
		t.Fatalf("persisted Erika post not restored: %+v", posts)
	}
	personas := restored.ListHostPersonas(host.ID)
	foundPersona := false
	for _, got := range personas {
		if got.ID == persona.ID {
			foundPersona = true
			break
		}
	}
	if !foundPersona {
		t.Fatalf("persisted Erika persona not restored among resident cast: %+v", personas)
	}
	facts := restored.ListPersonaFacts(persona.ID)
	if len(facts) != 1 || facts[0].Value != "天神" {
		t.Fatalf("persisted Erika persona facts not restored: %+v", facts)
	}
}


func TestPostBatchPersistsOneSnapshotAfterManyPosts(t *testing.T) {
	ctx := context.Background()
	backend := &fakeBackend{}
	base := world.NewMemoryStore()
	store, err := newStore(ctx, base, []HostTarget{{Phone: "0920000196", KeepSeedHostConfig: true}}, backend)
	if err != nil {
		t.Fatal(err)
	}
	host, err := store.HostByPhone("0920000196")
	if err != nil {
		t.Fatal(err)
	}

	before := backend.saves
	store.BeginPostBatch(host.ID)
	for i := 0; i < 25; i++ {
		store.AddPost(host.ID, world.Post{
			BoardID: "20/1",
			Author:  "NPC",
			Subject: "batch",
		})
	}
	if backend.saves != before {
		t.Fatalf("batch persisted early: saves=%d before=%d", backend.saves, before)
	}
	store.EndPostBatch(host.ID)
	if backend.saves != before+1 {
		t.Fatalf("batch snapshots=%d, want exactly %d", backend.saves, before+1)
	}

	fresh := world.NewMemoryStore()
	restored, err := newStore(ctx, fresh, []HostTarget{{Phone: "0920000196", KeepSeedHostConfig: true}}, backend)
	if err != nil {
		t.Fatal(err)
	}
	posts := restored.ListPosts(host.ID)
	if len(posts) != 25 {
		t.Fatalf("restored batch posts=%d, want 25", len(posts))
	}
}

func TestNestedPostBatchesPersistOnlyWhenOutermostBatchEnds(t *testing.T) {
	ctx := context.Background()
	backend := &fakeBackend{}
	base := world.NewMemoryStore()
	store, err := newStore(ctx, base, []HostTarget{{Phone: "0920000196", KeepSeedHostConfig: true}}, backend)
	if err != nil {
		t.Fatal(err)
	}
	host, _ := store.HostByPhone("0920000196")

	store.BeginPostBatch(host.ID)
	store.AddPost(host.ID, world.Post{BoardID: "4", Author: "A", Subject: "one"})
	store.BeginPostBatch(host.ID)
	store.AddPost(host.ID, world.Post{BoardID: "20/1", Author: "B", Subject: "two"})
	store.EndPostBatch(host.ID)
	if backend.saves != 0 {
		t.Fatalf("nested batch persisted before outer end: %d", backend.saves)
	}
	store.EndPostBatch(host.ID)
	if backend.saves != 1 {
		t.Fatalf("nested batch snapshots=%d, want 1", backend.saves)
	}
}
