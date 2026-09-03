package worldrepo

import (
	"reflect"
	"testing"
	"time"

	"zutto-pccom/apps/server/internal/world"
)

func TestPC98PersonaFactsMaterializeOnlyRequestedSlots(t *testing.T) {
	base := world.NewMemoryStore()
	repo := New(base, nil, nil, "1996-08-29")
	h, err := repo.HostByPhone("0450000196")
	if err != nil {
		t.Fatal(err)
	}
	personas, _ := repo.MaterializationPersonas(h)
	taka, ok := personaByHandle(personas, "TAKA")
	if !ok {
		t.Fatal("TAKA persona missing")
	}
	factStore := any(base).(world.PersonaFactStore)
	if got := factStore.ListPersonaFacts(taka.ID); len(got) != 0 {
		t.Fatalf("persona skeleton eagerly contained concrete facts: %+v", got)
	}

	at := worldTime("1996-08-29").Add(-48 * time.Hour)
	usageFacts := repo.materializeDemoPersonaFactsForSlots(taka, "pc98_environment", []string{"usage_pattern"}, at)
	if len(usageFacts) != 1 {
		t.Fatalf("usage facts=%d, want exactly one lazy slot", len(usageFacts))
	}
	if usageFacts[0].Key != "computer.pc98.usage" || usageFacts[0].Topic != "pc98_environment" {
		t.Fatalf("unexpected usage fact: %+v", usageFacts[0])
	}
	if got := factStore.ListPersonaFacts(taka.ID); len(got) != 1 {
		t.Fatalf("materializing one slot created %d facts", len(got))
	}

	modemFacts := repo.materializeDemoPersonaFactsForSlots(taka, "pc98_environment", []string{"modem"}, at.Add(time.Hour))
	if len(modemFacts) != 1 || modemFacts[0].Key != "computer.pc98.modem" {
		t.Fatalf("unexpected modem materialization: %+v", modemFacts)
	}
	if got := factStore.ListPersonaFacts(taka.ID); len(got) != 2 {
		t.Fatalf("second requested slot did not extend facts one step at a time: %+v", got)
	}

	reused := repo.materializeDemoPersonaFactsForSlots(taka, "pc98_environment", []string{"usage_pattern"}, at.Add(24*time.Hour))
	if !reflect.DeepEqual(usageFacts, reused) {
		t.Fatalf("materialized persona fact changed on reuse:\nfirst=%+v\nreused=%+v", usageFacts, reused)
	}
}

func TestNaturalReplyEnvelopeDoesNotCreateQuestionnaireChain(t *testing.T) {
	base := world.NewMemoryStore()
	repo := New(base, nil, nil, "1996-08-29")
	h, _ := repo.HostByPhone("0450000196")
	personas, _ := repo.MaterializationPersonas(h)
	taka, _ := personaByHandle(personas, "TAKA")
	neko, _ := personaByHandle(personas, "NEKO")
	mari, _ := personaByHandle(personas, "MARI")
	boards, _ := repo.MaterializationBoards(h)
	board := boards[1]
	seed := demoTopicSeed{key: "pc98_environment", motivation: "PC-98側の環境について他の利用者の構成も聞いてみたい"}
	at := worldTime("1996-08-29").Add(-72 * time.Hour)

	root := repo.demoNaturalRootEnvelope(h, board, taka, seed, "みなさんの98環境", at)
	root = base.AddPost(h.ID, root)
	if len(root.Intent.Claims) == 0 || len(root.Intent.InformationSlots) == 0 {
		t.Fatalf("root has no lazily materialized personal detail: %+v", root.Intent)
	}
	if root.Intent.FollowUpSlot != "" || root.Intent.FollowUpQuestion != "" {
		t.Fatalf("root unexpectedly starts a forced question chain: %+v", root.Intent)
	}

	nekoReply := repo.demoNaturalReplyEnvelopeWithThread(h, board, neko, root, []world.Post{root}, at.Add(3*time.Hour))
	if nekoReply.ParentID != root.ID || nekoReply.Intent.Action != "reply" {
		t.Fatalf("reply linkage invalid: %+v", nekoReply)
	}
	if nekoReply.Intent.ResponseAct == "" || len(nekoReply.Intent.Claims) == 0 {
		t.Fatalf("reply missing actor move/claims: %+v", nekoReply.Intent)
	}
	if nekoReply.Intent.FollowUpSlot != "" || nekoReply.Intent.FollowUpQuestion != "" || nekoReply.Intent.RespondsToQuestion != "" {
		t.Fatalf("reply still behaves like a questionnaire: %+v", nekoReply.Intent)
	}
	nekoReply = base.AddPost(h.ID, nekoReply)

	mariReply := repo.demoNaturalReplyEnvelopeWithThread(h, board, mari, root, []world.Post{root, nekoReply}, at.Add(6*time.Hour))
	if mariReply.Intent.RespondsToPostID != nekoReply.ID {
		t.Fatalf("second reply should naturally see the latest semantic post, got target=%d want=%d", mariReply.Intent.RespondsToPostID, nekoReply.ID)
	}
	if mariReply.Intent.FollowUpSlot != "" || mariReply.Intent.FollowUpQuestion != "" || mariReply.Intent.RespondsToQuestion != "" {
		t.Fatalf("second reply reintroduced forced question progression: %+v", mariReply.Intent)
	}
}

func TestMaterializationConversationResetKeepsPersonaSkeletons(t *testing.T) {
	base := world.NewMemoryStore()
	repo := New(base, nil, nil, "1996-08-29")
	h, _ := repo.HostByPhone("0450000196")
	personas, _ := repo.MaterializationPersonas(h)
	taka, _ := personaByHandle(personas, "TAKA")
	boards, _ := repo.MaterializationBoards(h)
	at := worldTime("1996-08-29").Add(-24 * time.Hour)

	facts := repo.materializeDemoPersonaFactsForSlots(taka, "pc98_environment", []string{"usage_pattern", "modem"}, at)
	if len(facts) != 2 {
		t.Fatalf("expected two lazy persona facts before reset, got %d", len(facts))
	}
	base.AddPost(h.ID, world.Post{BoardID: boards[1].ID, Author: taka.Handle, AuthorPersonaID: taka.ID, Subject: "test", CreatedAt: at})

	postsCleared, factsCleared, ok := repo.ResetMaterializationConversation(h)
	if !ok {
		t.Fatal("development reset capability unavailable")
	}
	if postsCleared != 1 || factsCleared != len(facts) {
		t.Fatalf("reset counts posts=%d facts=%d, want 1/%d", postsCleared, factsCleared, len(facts))
	}
	if got := base.ListPosts(h.ID); len(got) != 0 {
		t.Fatalf("posts remain after reset: %+v", got)
	}
	if got := base.ListPersonaFacts(taka.ID); len(got) != 0 {
		t.Fatalf("persona facts remain after reset: %+v", got)
	}
	if got := base.ListHostPersonas(h.ID); len(got) != len(personas) {
		t.Fatalf("core persona skeletons were lost: got=%d want=%d", len(got), len(personas))
	}
	if got := base.ListBoards(h.ID); len(got) != len(boards) {
		t.Fatalf("board catalog was lost: got=%d want=%d", len(got), len(boards))
	}
}

func TestPersonaHistoryKeepsOnePC98SemanticRootInObservationWindow(t *testing.T) {
	base := world.NewMemoryStore()
	repo := New(base, nil, nil, "1996-08-29")
	h, _ := repo.HostByPhone("0450000196")
	boards, _ := repo.MaterializationBoards(h)
	posts, _ := repo.MaterializationPersonaArticleHeaders(h, boards[1])

	pc98Roots := 0
	for _, post := range posts {
		if post.ParentID == 0 && post.Intent.Topic == "pc98_environment" {
			pc98Roots++
		}
	}
	if pc98Roots > 1 {
		t.Fatalf("pc98_environment opened %d separate roots inside one 14-day observation window", pc98Roots)
	}
}
