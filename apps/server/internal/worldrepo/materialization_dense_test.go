package worldrepo

import (
	"testing"
	"time"

	"zutto-pccom/apps/server/internal/world"
)

func TestDenseMaterializationCreatesUnevenPersonaHistory(t *testing.T) {
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
		t.Fatal("dense envelopes should be created on first board observation")
	}
	if len(posts) != 20 {
		t.Fatalf("dense envelope count=%d, want 20", len(posts))
	}

	counts := map[string]int{}
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
		}
		if i > 0 && p.CreatedAt.Before(posts[i-1].CreatedAt) {
			t.Fatalf("posts are not chronological: %s before %s", p.CreatedAt, posts[i-1].CreatedAt)
		}
		if p.CreatedAt.After(worldTime("1996-08-29")) {
			t.Fatalf("post %d was generated in the future: %s", p.ID, p.CreatedAt)
		}
		counts[p.Author]++
	}
	if replies < 3 {
		t.Fatalf("reply envelopes=%d, want at least 3", replies)
	}
	if counts["NEKO"] <= counts["TAKA"] {
		t.Fatalf("active/lurker bias not visible: NEKO=%d TAKA=%d", counts["NEKO"], counts["TAKA"])
	}
	if posts[len(posts)-1].CreatedAt.Sub(posts[0].CreatedAt) < 7*24*time.Hour {
		t.Fatalf("history span too short: %s", posts[len(posts)-1].CreatedAt.Sub(posts[0].CreatedAt))
	}

	reused, created := repo.MaterializationDenseArticleHeaders(h, boards[0])
	if created {
		t.Fatal("dense envelopes should be reused after first materialization")
	}
	if len(reused) != len(posts) || reused[0].ID != posts[0].ID || reused[len(reused)-1].ID != posts[len(posts)-1].ID {
		t.Fatal("stored dense history was not reused")
	}
}
