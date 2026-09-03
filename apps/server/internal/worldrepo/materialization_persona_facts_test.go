package worldrepo

import (
	"reflect"
	"testing"
	"time"

	"zutto-pccom/apps/server/internal/world"
)

func TestPC98PersonaFactsMaterializeOnlyWhenTopicNeedsThem(t *testing.T) {
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
	facts := repo.materializeDemoPersonaTopicFacts(taka, "pc98_environment", at)
	if len(facts) < 2 {
		t.Fatalf("pc98 topic facts=%d, want concrete lazy details", len(facts))
	}
	if facts[0].Value == "" || facts[0].Topic != "pc98_environment" {
		t.Fatalf("invalid materialized fact: %+v", facts[0])
	}

	reused := repo.materializeDemoPersonaTopicFacts(taka, "pc98_environment", at.Add(24*time.Hour))
	if !reflect.DeepEqual(facts, reused) {
		t.Fatalf("materialized persona facts changed on reuse:\nfirst=%+v\nreused=%+v", facts, reused)
	}
}

func TestReplyEnvelopeRespondsToConcreteParentClaim(t *testing.T) {
	base := world.NewMemoryStore()
	repo := New(base, nil, nil, "1996-08-29")
	h, _ := repo.HostByPhone("0450000196")
	personas, _ := repo.MaterializationPersonas(h)
	taka, _ := personaByHandle(personas, "TAKA")
	mari, _ := personaByHandle(personas, "MARI")
	boards, _ := repo.MaterializationBoards(h)
	board := boards[1]
	seed := demoTopicSeed{key: "pc98_environment", motivation: "PC-98側の環境について他の利用者の構成も聞いてみたい"}
	at := worldTime("1996-08-29").Add(-72 * time.Hour)

	root := repo.demoRootEnvelope(h, board, taka, seed, "みなさんの98環境", at)
	root = base.AddPost(h.ID, root)
	if len(root.Intent.Claims) < 2 {
		t.Fatalf("root claims too thin: %+v", root.Intent)
	}
	reply := repo.demoReplyEnvelope(h, board, mari, root, at.Add(3*time.Hour))
	if reply.ParentID != root.ID || reply.Intent.Action != "reply" {
		t.Fatalf("reply linkage invalid: %+v", reply)
	}
	if len(reply.Intent.RespondsToClaims) != 1 || reply.Intent.RespondsToClaims[0] != root.Intent.Claims[0] {
		t.Fatalf("reply does not identify parent semantic hook: %+v", reply.Intent)
	}
	if len(reply.Intent.Claims) < 2 {
		t.Fatalf("reply has no concrete own claims: %+v", reply.Intent)
	}
	if reflect.DeepEqual(reply.Intent.Claims, root.Intent.Claims) {
		t.Fatal("reply copied parent claims instead of using the reply persona's facts")
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

	facts := repo.materializeDemoPersonaTopicFacts(taka, "pc98_environment", at)
	if len(facts) == 0 {
		t.Fatal("expected persona facts before reset")
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

func TestPersonaHistoryAvoidsRecentDuplicateSemanticRoots(t *testing.T) {
	base := world.NewMemoryStore()
	repo := New(base, nil, nil, "1996-08-29")
	h, _ := repo.HostByPhone("0450000196")
	boards, _ := repo.MaterializationBoards(h)
	posts, _ := repo.MaterializationPersonaArticleHeaders(h, boards[1])

	lastRoot := map[string]time.Time{}
	for _, post := range posts {
		if post.ParentID != 0 || post.Intent.Topic == "board_housekeeping" {
			continue
		}
		if previous, ok := lastRoot[post.Intent.Topic]; ok && post.CreatedAt.Sub(previous) < 10*24*time.Hour {
			t.Fatalf("duplicate recent semantic root topic=%s previous=%s next=%s", post.Intent.Topic, previous, post.CreatedAt)
		}
		lastRoot[post.Intent.Topic] = post.CreatedAt
	}
}
