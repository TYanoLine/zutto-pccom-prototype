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

func TestListPostsDoesNotInventCannedPostWhenRendererUnavailable(t *testing.T) {
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
	first := repo.ListPosts(h.ID)
	if len(first) != 0 {
		t.Fatalf("unavailable renderer invented posts: %#v", first)
	}
	second := repo.ListPosts(h.ID)
	if len(second) != 0 {
		t.Fatalf("retry invented posts: %#v", second)
	}
	if engine.calls != 2 {
		t.Fatalf("failed render should remain retryable; evidence resolved %d times, want 2", engine.calls)
	}
}
