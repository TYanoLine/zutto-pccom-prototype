package bbsengine

import (
	"context"
	"testing"
	"time"

	"zutto-pccom/apps/server/internal/world"
)

func TestCatchUpInitialBoardActivityGeneratesAtMostTenRootHeaders(t *testing.T) {
	store := world.NewMemoryStore()
	host, err := store.HostByPhone("0920000196")
	if err != nil {
		t.Fatal(err)
	}
	board := world.Board{ID: "20/1", Name: "ＧＡＭＥ"}
	planner := &fakeBatchPlanner{}
	now := time.Date(1996, 8, 26, 23, 30, 0, 0, time.Local)
	engine := New(store, planner, func() time.Time { return now })
	state := world.BoardActivityState{
		BoardID:         board.ID,
		RetainedRoots:   17,
		RetainedReplies: 23,
		RetainedSince:   now.Add(-45 * 24 * time.Hour),
	}

	if err := engine.CatchUpInitialBoardActivity(context.Background(), host, board, state); err != nil {
		t.Fatal(err)
	}
	if planner.calls != 1 {
		t.Fatalf("planner calls=%d, want 1", planner.calls)
	}
	if got := len(planner.requests[0].Slots); got != InteractiveInitialRootLimit {
		t.Fatalf("activity slots=%d, want %d", got, InteractiveInitialRootLimit)
	}

	posts := filterBoard(store.ListPosts(host.ID), board.ID)
	roots, replies := 0, 0
	for _, post := range posts {
		if world.IsSemanticRoot(post) {
			roots++
		} else {
			replies++
		}
	}
	if roots != InteractiveInitialRootLimit || replies != 0 {
		t.Fatalf("realized roots/replies=%d/%d, want %d/0", roots, replies, InteractiveInitialRootLimit)
	}
	if planner.requests[0].Since != state.RetainedSince {
		t.Fatalf("history since=%s, want %s", planner.requests[0].Since, state.RetainedSince)
	}
}

func TestCatchUpInitialBoardActivityPreservesSmallerRootCount(t *testing.T) {
	store := world.NewMemoryStore()
	host, err := store.HostByPhone("0920000196")
	if err != nil {
		t.Fatal(err)
	}
	board := world.Board{ID: "20/1", Name: "ＧＡＭＥ"}
	planner := &fakeBatchPlanner{}
	now := time.Date(1996, 8, 26, 23, 30, 0, 0, time.Local)
	engine := New(store, planner, func() time.Time { return now })
	state := world.BoardActivityState{
		BoardID: board.ID, RetainedRoots: 3, RetainedReplies: 10,
		RetainedSince: now.Add(-20 * 24 * time.Hour),
	}
	if err := engine.CatchUpInitialBoardActivity(context.Background(), host, board, state); err != nil {
		t.Fatal(err)
	}
	if got := len(planner.requests); got != 1 {
		t.Fatalf("planner requests=%d, want 1", got)
	}
	if got := len(planner.requests[0].Slots); got != 3 {
		t.Fatalf("initial slots=%d, want 3", got)
	}
	for _, slot := range planner.requests[0].Slots {
		if slot.ReplyToPostID != 0 || slot.ReplyToSlotIndex != 0 {
			t.Fatalf("speculative reply materialized in lightweight index: %+v", slot)
		}
	}
	// This is a projection limit, not a rewrite of the coarse canonical counts.
	if state.RetainedRoots != 3 || state.RetainedReplies != 10 {
		t.Fatalf("canonical activity state was changed: %+v", state)
	}
}
