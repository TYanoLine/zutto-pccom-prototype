package worldrepo

import (
	"testing"
	"time"

	"zutto-pccom/apps/server/internal/world"
)

func TestDenseMaterializationSamplesPersonaDrivenHistory(t *testing.T) {
	base := world.NewMemoryStore()
	repo := New(base, nil, nil, "1996-08-29")
	h, err := repo.HostByPhone("0450000196")
	if err != nil {
		t.Fatal(err)
	}
	boards, _ := repo.MaterializationBoards(h)
	if len(boards) == 0 {
		t.Fatal("no boards")
	}

	posts, created := repo.MaterializationDenseArticleHeaders(h, boards[0])
	if !created {
		t.Fatal("persona-driven envelopes should be created on first board observation")
	}
	if len(posts) < 15 || len(posts) > 36 {
		t.Fatalf("envelope count=%d, want a useful but bounded sampled history", len(posts))
	}

	counts := map[string]int{}
	topics := map[string]bool{}
	rootSubjects := map[string]int{}
	replies := 0
	for i, p := range posts {
		if p.Body != "" {
			t.Fatalf("post %d body was eagerly materialized", p.ID)
		}
		if p.AuthorPersonaID == "" {
			t.Fatalf("post %d has no persistent persona", p.ID)
		}
		if p.Intent.Action == "" || p.Intent.Topic == "" {
			t.Fatalf("post %d missing envelope intent: %+v", p.ID, p.Intent)
		}
		if p.ParentID != 0 {
			replies++
		} else {
			rootSubjects[p.Subject]++
		}
		if i > 0 && p.CreatedAt.Before(posts[i-1].CreatedAt) {
			t.Fatalf("posts are not chronological: %s before %s", p.CreatedAt, posts[i-1].CreatedAt)
		}
		if p.CreatedAt.After(worldTime("1996-08-29")) {
			t.Fatalf("post %d was generated in the future: %s", p.ID, p.CreatedAt)
		}
		counts[p.Author]++
		topics[p.Intent.Topic] = true
	}
	if replies < 3 {
		t.Fatalf("reply envelopes=%d, want at least 3", replies)
	}
	if len(topics) < 6 {
		t.Fatalf("topic diversity=%d, want at least 6 distinct semantic topics", len(topics))
	}
	if counts["NEKO"] <= counts["TAKA"] {
		t.Fatalf("active/lurker bias not visible: NEKO=%d TAKA=%d", counts["NEKO"], counts["TAKA"])
	}
	duplicateRoots := 0
	for _, count := range rootSubjects {
		if count > 1 {
			duplicateRoots += count - 1
		}
	}
	if duplicateRoots > 2 {
		t.Fatalf("too many exact duplicate root subjects: %d in %#v", duplicateRoots, rootSubjects)
	}
	if posts[len(posts)-1].CreatedAt.Sub(posts[0].CreatedAt) < 8*24*time.Hour {
		t.Fatalf("history span too short: %s", posts[len(posts)-1].CreatedAt.Sub(posts[0].CreatedAt))
	}

	reused, created := repo.MaterializationDenseArticleHeaders(h, boards[0])
	if created {
		t.Fatal("persona-driven envelopes should be reused after first materialization")
	}
	if len(reused) != len(posts) || reused[0].ID != posts[0].ID || reused[len(reused)-1].ID != posts[len(posts)-1].ID {
		t.Fatal("stored history was not reused")
	}
}

func TestDenseMaterializationIsDeterministicBeforeCommit(t *testing.T) {
	materialize := func() []world.Post {
		base := world.NewMemoryStore()
		repo := New(base, nil, nil, "1996-08-29")
		h, err := repo.HostByPhone("0450000196")
		if err != nil {
			t.Fatal(err)
		}
		boards, _ := repo.MaterializationBoards(h)
		posts, _ := repo.MaterializationDenseArticleHeaders(h, boards[0])
		return posts
	}

	first := materialize()
	second := materialize()
	if len(first) != len(second) {
		t.Fatalf("deterministic history length changed: %d vs %d", len(first), len(second))
	}
	for i := range first {
		a, b := first[i], second[i]
		if a.Author != b.Author || a.Subject != b.Subject || a.ParentID != b.ParentID || a.Intent.Action != b.Intent.Action || a.Intent.Topic != b.Intent.Topic || !a.CreatedAt.Equal(b.CreatedAt) {
			t.Fatalf("history differs at %d:\nA=%+v\nB=%+v", i, a, b)
		}
	}
}

func TestTopicAffinityUsesPersonaInterests(t *testing.T) {
	base := world.NewMemoryStore()
	repo := New(base, nil, nil, "1996-08-29")
	h, _ := repo.HostByPhone("0450000196")
	personas, _ := repo.MaterializationPersonas(h)
	yuki, _ := personaByHandle(personas, "YUKI")
	nori, _ := personaByHandle(personas, "NORI")
	boards, _ := repo.MaterializationBoards(h)

	freeTopics := demoTopicsForBoard(boards[0])
	gameTopic, ok := demoTopicByKey(freeTopics, "games")
	if !ok {
		t.Fatal("games topic missing")
	}
	if demoTopicInterestScore(yuki, gameTopic) <= demoTopicInterestScore(nori, gameTopic) {
		t.Fatal("YUKI should be more attracted to game chatter than NORI")
	}

	techTopics := demoTopicsForBoard(boards[1])
	modemTopic, ok := demoTopicByKey(techTopics, "modem_settings")
	if !ok {
		t.Fatal("modem topic missing")
	}
	if demoTopicInterestScore(nori, modemTopic) <= demoTopicInterestScore(yuki, modemTopic) {
		t.Fatal("NORI should be more attracted to modem topics than YUKI")
	}
}
