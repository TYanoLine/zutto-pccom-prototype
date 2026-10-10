package bbsengine

import (
	"context"
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
	now := time.Date(1996, 8, 26, 23, 30, 0, 0, time.Local)
	engine := New(store, planner, func() time.Time { return now })
	state.BoardID = board.ID
	if err := engine.CatchUpInitialBoardActivity(context.Background(), host, board, state); err != nil {
		t.Fatal(err)
	}
	return store, host, board, planner, now
}

func TestCatchUpInitialBoardActivityGivesEachThreadItsOwnReplies(t *testing.T) {
	state := world.BoardActivityState{
		TotalRoots: 40, RetainedRoots: 17, RetainedReplies: 23, ReplyRate: 1.4,
		RetainedSince: time.Date(1996, 7, 12, 23, 30, 0, 0, time.Local),
	}
	store, host, board, planner, _ := initialActivityFixture(t, state)
	if planner.calls != 1 {
		t.Fatalf("planner calls=%d, want 1", planner.calls)
	}

	want := make([]int, InteractiveInitialRootLimit)
	wantReplies := 0
	for i := range want {
		want[i] = world.ThreadReplyCount(host.ID, board.ID, state.TotalRoots-InteractiveInitialRootLimit+1+i, state.ReplyRate)
		wantReplies += want[i]
	}
	if wantReplies > InteractiveInitialReplyLimit {
		t.Skipf("fixture draws %d replies, above the %d bound", wantReplies, InteractiveInitialReplyLimit)
	}

	slots := planner.requests[0].Slots
	if got := len(slots); got != InteractiveInitialRootLimit+wantReplies {
		t.Fatalf("slots=%d, want %d roots + %d replies", got, InteractiveInitialRootLimit, wantReplies)
	}
	byIndex := map[int]Slot{}
	perRoot := map[int]int{}
	for i, slot := range slots {
		if slot.Index != i+1 {
			t.Fatalf("slot %d has index %d", i, slot.Index)
		}
		byIndex[slot.Index] = slot
		if i > 0 && slot.CreatedAt.Before(slots[i-1].CreatedAt) {
			t.Fatalf("slots not chronological at %d", i)
		}
		if slot.ReplyToSlotIndex != 0 {
			parent, ok := byIndex[slot.ReplyToSlotIndex]
			if !ok || parent.ReplyToSlotIndex != 0 || !slot.CreatedAt.After(parent.CreatedAt) {
				t.Fatalf("reply slot %+v does not follow a root", slot)
			}
			perRoot[slot.ReplyToSlotIndex]++
		}
	}
	// Replies per root must be exactly the thread counts, in some order.
	counts := map[int]int{}
	for _, n := range want {
		counts[n]++
	}
	for idx, slot := range byIndex {
		if slot.ReplyToSlotIndex == 0 {
			counts[perRoot[idx]]--
		}
	}
	for n, c := range counts {
		if c != 0 {
			t.Fatalf("thread reply-count multiset differs at %d (%d)", n, c)
		}
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
	if roots != InteractiveInitialRootLimit || replies != wantReplies {
		t.Fatalf("realized roots/replies=%d/%d, want %d/%d", roots, replies, InteractiveInitialRootLimit, wantReplies)
	}
	if planner.requests[0].Since != state.RetainedSince {
		t.Fatalf("history since=%s, want %s", planner.requests[0].Since, state.RetainedSince)
	}
}

func TestCatchUpInitialBoardActivityWithoutReplyRateMakesOnlyRoots(t *testing.T) {
	state := world.BoardActivityState{
		TotalRoots: 3, RetainedRoots: 3, RetainedReplies: 10,
		RetainedSince: time.Date(1996, 8, 6, 23, 30, 0, 0, time.Local),
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

func TestCatchUpInitialBoardActivityBoundsTotalReplies(t *testing.T) {
	state := world.BoardActivityState{
		TotalRoots: 200, RetainedRoots: 60, ReplyRate: 4,
		RetainedSince: time.Date(1996, 1, 1, 0, 0, 0, 0, time.Local),
	}
	_, _, _, planner, _ := initialActivityFixture(t, state)
	if got := len(planner.requests[0].Slots); got > InteractiveInitialRootLimit+InteractiveInitialReplyLimit {
		t.Fatalf("slots=%d exceed the %d+%d bound", got, InteractiveInitialRootLimit, InteractiveInitialReplyLimit)
	}
}
