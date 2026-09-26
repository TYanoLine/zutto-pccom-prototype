package bbsengine

import (
	"context"
	"testing"
	"time"

	"zutto-pccom/apps/server/internal/world"
)

func TestCatchUpInitialBoardActivityRealizesExactRetainedCounts(t *testing.T) {
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
	if got := len(planner.requests[0].Slots); got != 40 {
		t.Fatalf("activity slots=%d, want 40", got)
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
	if roots != 17 || replies != 23 {
		t.Fatalf("realized roots/replies=%d/%d, want 17/23", roots, replies)
	}
	if planner.requests[0].Since != state.RetainedSince {
		t.Fatalf("history since=%s, want %s", planner.requests[0].Since, state.RetainedSince)
	}
}
