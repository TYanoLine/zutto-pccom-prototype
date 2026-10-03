package bbsengine

import (
	"context"
	"testing"
	"time"

	"zutto-pccom/apps/server/internal/world"
)

type postBatchTrackingStore struct {
	*world.MemoryStore
	begin int
	end   int
}

func (s *postBatchTrackingStore) BeginPostBatch(string) { s.begin++ }
func (s *postBatchTrackingStore) EndPostBatch(string)   { s.end++ }

func TestCatchUpWrapsCanonicalPostCommitInOnePersistenceBatch(t *testing.T) {
	base := world.NewMemoryStore()
	store := &postBatchTrackingStore{MemoryStore: base}
	host, err := store.HostByPhone("0920000196")
	if err != nil {
		t.Fatal(err)
	}
	planner := &fakeBatchPlanner{}
	now := time.Date(1996, 8, 26, 23, 30, 0, 0, time.Local)
	engine := New(store, planner, func() time.Time { return now })

	if err := engine.CatchUpInitialCount(context.Background(), host, world.Board{ID: "20/1", Name: "ＧＡＭＥ"}, 12); err != nil {
		t.Fatal(err)
	}
	if store.begin != 1 || store.end != 1 {
		t.Fatalf("post persistence batches begin/end=%d/%d, want 1/1", store.begin, store.end)
	}
	if got := len(filterBoard(store.ListPosts(host.ID), "20/1")); got != 12 {
		t.Fatalf("committed posts=%d, want 12", got)
	}
}
