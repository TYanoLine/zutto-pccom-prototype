package worldrepo

import (
	"context"
	"testing"

	"zutto-pccom/apps/server/internal/historicalkb"
	"zutto-pccom/apps/server/internal/world"
	"zutto-pccom/apps/server/internal/worldengine"
)

type fakeEngine struct{ calls int }

func (f *fakeEngine) ResolveEvidence(_ context.Context, r worldengine.EvidenceRequest) (worldengine.EvidenceDecision, error) {
	f.calls++
	if !r.Persistence {
		panic("materialized post must be persistent")
	}
	return worldengine.EvidenceDecision{Level: historicalkb.EvidenceAtmospheric, ModelFirst: true, Knowledge: historicalkb.KnowledgeResult{CanUse: true}}, nil
}

func TestListPostsIsPureAndFailedObservationRemainsRetryable(t *testing.T) {
	base := world.NewMemoryStore()
	engine := &fakeEngine{}
	repo := New(base, engine, FallbackMaterializer{}, "1996-08-29")
	h, err := repo.HostByPhone("0450000001")
	if err != nil {
		t.Fatal(err)
	}
	if got := base.ListPosts(h.ID); len(got) != 0 {
		t.Fatalf("fixture should start empty: %d", len(got))
	}

	// Plain reads are metadata/state reads only. They must not observe the host.
	for i := 0; i < 2; i++ {
		if got := repo.ListPosts(h.ID); len(got) != 0 {
			t.Fatalf("plain read invented posts: %#v", got)
		}
	}
	if engine.calls != 0 {
		t.Fatalf("plain ListPosts triggered generation: evidence calls=%d", engine.calls)
	}

	board := world.Board{ID: "main", Name: "フリートーク"}
	for attempt := 1; attempt <= 2; attempt++ {
		if _, err := repo.WaitForBoardHeaders(context.Background(), h, board); err == nil {
			t.Fatalf("attempt %d unexpectedly succeeded with removed fallback materializer", attempt)
		}
		if got := base.ListPosts(h.ID); len(got) != 0 {
			t.Fatalf("failed observation invented posts: %#v", got)
		}
	}
	if engine.calls != 2 {
		t.Fatalf("failed observation should remain retryable; evidence resolved %d times, want 2", engine.calls)
	}
}
