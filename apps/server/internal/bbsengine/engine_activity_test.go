package bbsengine

import (
	"context"
	"math/rand"
	"testing"
	"time"

	"zutto-pccom/apps/server/internal/world"
)

func initialActivityFixture(t *testing.T, state world.BoardActivityState) (*world.MemoryStore, world.Host, world.Board, *fakeBatchPlanner, time.Time) {
	t.Helper()
	store := world.NewMemoryStore()
	host, err := store.HostByPhone("0920000196")
	if err != nil {
		t.Fatal(err)
	}
	board := world.Board{ID: "20/1", Name: "ＧＡＭＥ"}
	planner := &fakeBatchPlanner{}
	now := time.Date(1996, 8, 26, 23, 30, 0, 0, testJST)
	engine := New(store, planner, func() time.Time { return now })
	// Seed 42 provides deterministic replies across the 10 initial roots
	// (some answered, some unanswered) to prevent test flakiness across runs.
	engine.RNG = rand.New(rand.NewSource(42))
	state.BoardID = board.ID
	if err := engine.CatchUpInitialBoardActivity(context.Background(), host, board, state); err != nil {
		t.Fatal(err)
	}
	return store, host, board, planner, now
}

func TestCatchUpInitialBoardActivityGivesEachThreadItsOwnReplies(t *testing.T) {
	state := world.BoardActivityState{
		TotalRoots: 40, RetainedRoots: 17, RetainedReplies: 23, ReplyRate: 1.4,
		RetainedSince: time.Date(1996, 7, 12, 23, 30, 0, 0, testJST),
	}
	store, host, board, planner, _ := initialActivityFixture(t, state)
	if planner.calls != 1 {
		t.Fatalf("planner calls=%d, want 1", planner.calls)
	}

	slots := planner.requests[0].Slots
	if got := len(slots); got != InteractiveInitialRootLimit {
		t.Fatalf("slots=%d, want %d roots", got, InteractiveInitialRootLimit)
	}
	for i, slot := range slots {
		if slot.Index != i+1 {
			t.Fatalf("slot %d has index %d", i, slot.Index)
		}
		if slot.ReplyToPostID != 0 || slot.ReplyToSlotIndex != 0 {
			t.Fatalf("root slot %d has reply target: %+v", i, slot)
		}
		if i > 0 && slot.CreatedAt.Before(slots[i-1].CreatedAt) {
			t.Fatalf("slots not chronological at %d", i)
		}
	}

	posts := filterBoard(store.ListPosts(host.ID), board.ID)
	var rootPosts []world.Post
	var replyPosts []world.Post
	for _, post := range posts {
		if world.IsSemanticRoot(post) {
			rootPosts = append(rootPosts, post)
		} else {
			replyPosts = append(replyPosts, post)
		}
	}
	if len(rootPosts) != InteractiveInitialRootLimit {
		t.Fatalf("realized roots=%d, want %d", len(rootPosts), InteractiveInitialRootLimit)
	}

	planStore, ok := any(store).(world.ReactionPlanStore)
	if !ok {
		t.Fatal("store does not implement ReactionPlanStore")
	}

	repliesByRoot := map[int64]int{}
	for _, reply := range replyPosts {
		if reply.ParentID == 0 {
			t.Fatalf("reply post %d has no parent ID", reply.ID)
		}
		repliesByRoot[reply.ParentID]++
	}

	totalPlannedReplies := 0
	nonZeroThreads := 0
	for _, root := range rootPosts {
		plan, exists := planStore.ReactionPlan(host.ID, root.ID)
		if !exists {
			t.Fatalf("missing reaction plan for root %d", root.ID)
		}
		if !plan.Judged {
			t.Errorf("plan for root %d is not judged", root.ID)
		}
		if plan.Appeal != 2 {
			t.Errorf("plan for root %d has appeal %d, want 2 (fallback)", root.ID, plan.Appeal)
		}
		if plan.Reason != "fallback:no_appeal" {
			t.Errorf("plan for root %d has reason %q, want 'fallback:no_appeal'", root.ID, plan.Reason)
		}
		if gotReplies := repliesByRoot[root.ID]; gotReplies != plan.Count {
			t.Errorf("root %d has %d materialized replies, want plan.Count=%d", root.ID, gotReplies, plan.Count)
		}
		totalPlannedReplies += plan.Count
		if plan.Count > 0 {
			nonZeroThreads++
		}
	}

	if len(replyPosts) != totalPlannedReplies {
		t.Fatalf("materialized replies=%d, want %d", len(replyPosts), totalPlannedReplies)
	}
	if totalPlannedReplies == 0 {
		t.Fatalf("expected some replies to be planned and materialized, got 0")
	}
	// Verify that threads have varying individual reply counts (some answered, some unanswered).
	if nonZeroThreads == 0 || nonZeroThreads == len(rootPosts) {
		t.Fatalf("expected some answered and some unanswered threads, got nonZeroThreads=%d of %d", nonZeroThreads, len(rootPosts))
	}

	if planner.requests[0].Since != state.RetainedSince {
		t.Fatalf("history since=%s, want %s", planner.requests[0].Since, state.RetainedSince)
	}
}

func TestCatchUpInitialBoardActivityWithoutReplyRateMakesOnlyRoots(t *testing.T) {
	state := world.BoardActivityState{
		TotalRoots: 3, RetainedRoots: 3, RetainedReplies: 10,
		RetainedSince: time.Date(1996, 8, 6, 23, 30, 0, 0, testJST),
	}
	_, _, _, planner, _ := initialActivityFixture(t, state)
	if got := len(planner.requests[0].Slots); got != 3 {
		t.Fatalf("initial slots=%d, want 3 roots", got)
	}
	for _, slot := range planner.requests[0].Slots {
		if slot.ReplyToPostID != 0 || slot.ReplyToSlotIndex != 0 {
			t.Fatalf("reply materialized without a reply rate: %+v", slot)
		}
	}
	// This is a projection limit, not a rewrite of the coarse canonical counts.
	if state.RetainedRoots != 3 || state.RetainedReplies != 10 {
		t.Fatalf("canonical activity state was changed: %+v", state)
	}
}

func TestCatchUpInitialBoardActivityBoundsRootSlots(t *testing.T) {
	state := world.BoardActivityState{
		TotalRoots: 200, RetainedRoots: 60, ReplyRate: 4,
		RetainedSince: time.Date(1996, 1, 1, 0, 0, 0, 0, testJST),
	}
	_, _, _, planner, _ := initialActivityFixture(t, state)
	if got := len(planner.requests[0].Slots); got != InteractiveInitialRootLimit {
		t.Fatalf("slots=%d, want bounded by %d roots", got, InteractiveInitialRootLimit)
	}
}
