package worldrepo

import (
	"testing"
	"time"

	"zutto-pccom/apps/server/internal/llm"
	"zutto-pccom/apps/server/internal/world"
)

func TestGenericPersonaFactsMaterializeFromOpenSemanticKeys(t *testing.T) {
	base := world.NewMemoryStore()
	repo := New(base, nil, nil, "1996-08-29")
	h, _ := repo.HostByPhone("0450000196")
	personas, _ := repo.MaterializationPersonas(h)
	taka, _ := personaByHandle(personas, "TAKA")
	if got := base.ListPersonaFacts(taka.ID); len(got) != 0 { t.Fatalf("persona skeleton eagerly contained facts: %+v", got) }

	at := worldTime("1996-08-29").Add(-48 * time.Hour)
	claims := repo.commitPlannedFacts(taka, "通信に使う自宅PCの話", []llm.BBSIntentFactDraft{
		{Key: "computer.communication_usage", Value: "自宅のPCを通信とゲームの両方に使っている"},
		{Key: "household.machine_access", Value: "自分が使いたい時間にPCが空いているとは限らない"},
	}, at)
	if len(claims) != 2 { t.Fatalf("claims=%d, want 2", len(claims)) }
	facts := base.ListPersonaFacts(taka.ID)
	if len(facts) != 2 { t.Fatalf("facts=%d, want 2", len(facts)) }
	if facts[0].SourceKind != "validated_semantic_proposal" { t.Fatalf("unexpected source kind: %+v", facts[0]) }
}

func TestExistingGenericPersonaFactWinsOverContradictoryProposal(t *testing.T) {
	base := world.NewMemoryStore()
	repo := New(base, nil, nil, "1996-08-29")
	h, _ := repo.HostByPhone("0450000196")
	personas, _ := repo.MaterializationPersonas(h)
	neko, _ := personaByHandle(personas, "NEKO")
	at := worldTime("1996-08-29").Add(-24 * time.Hour)

	first := repo.commitPlannedFacts(neko, "自宅PC", []llm.BBSIntentFactDraft{{Key: "computer.communication_usage", Value: "通信とゲームに兼用している"}}, at)
	second := repo.commitPlannedFacts(neko, "別の会話", []llm.BBSIntentFactDraft{{Key: "computer.communication_usage", Value: "通信専用機として使っている"}}, at.Add(time.Hour))
	if len(first) != 1 || len(second) != 1 || second[0] != first[0] { t.Fatalf("existing fact did not win: first=%v second=%v", first, second) }
	facts := base.ListPersonaFacts(neko.ID)
	if len(facts) != 1 || facts[0].Value != first[0] { t.Fatalf("canonical fact was overwritten: %+v", facts) }
}

func TestMaterializationConversationResetKeepsPersonaSkeletons(t *testing.T) {
	base := world.NewMemoryStore()
	repo := New(base, nil, nil, "1996-08-29")
	h, _ := repo.HostByPhone("0450000196")
	personas, _ := repo.MaterializationPersonas(h)
	taka, _ := personaByHandle(personas, "TAKA")
	boards, _ := repo.MaterializationBoards(h)
	at := worldTime("1996-08-29").Add(-24 * time.Hour)

	claims := repo.commitPlannedFacts(taka, "test topic", []llm.BBSIntentFactDraft{{Key: "personal.test_fact", Value: "テスト時にだけ必要になった個人的事実"}}, at)
	if len(claims) != 1 { t.Fatal("expected one lazy fact before reset") }
	base.AddPost(h.ID, world.Post{BoardID: boards[1].ID, Author: taka.Handle, AuthorPersonaID: taka.ID, Subject: "test", CreatedAt: at})
	postsCleared, factsCleared, ok := repo.ResetMaterializationConversation(h)
	if !ok || postsCleared != 1 || factsCleared != 1 { t.Fatalf("reset result posts=%d facts=%d ok=%v", postsCleared, factsCleared, ok) }
	if got := base.ListPosts(h.ID); len(got) != 0 { t.Fatalf("posts remain after reset: %+v", got) }
	if got := base.ListPersonaFacts(taka.ID); len(got) != 0 { t.Fatalf("facts remain after reset: %+v", got) }
	if got := base.ListHostPersonas(h.ID); len(got) != len(personas) { t.Fatalf("persona skeletons lost: got=%d want=%d", len(got), len(personas)) }
	if got := base.ListBoards(h.ID); len(got) != len(boards) { t.Fatalf("board catalog lost: got=%d want=%d", len(got), len(boards)) }
}
