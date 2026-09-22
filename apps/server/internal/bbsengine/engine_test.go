package bbsengine

import (
	"context"
	"fmt"
	"testing"
	"time"

	"zutto-pccom/apps/server/internal/world"
)

type fakeBatchPlanner struct {
	calls    int
	requests []BatchRequest
}

func (p *fakeBatchPlanner) PlanBBSBatch(_ context.Context, req BatchRequest) ([]PlannedPost, error) {
	p.calls++
	p.requests = append(p.requests, req)
	out := make([]PlannedPost, 0, len(req.Slots))
	for _, slot := range req.Slots {
		subject := fmt.Sprintf("batch title %02d", slot.Index)
		if slot.ReplyToPostID != 0 {
			subject = ""
		}
		out = append(out, PlannedPost{
			SlotIndex:        slot.Index,
			Subject:          subject,
			ConcreteMatter:   fmt.Sprintf("batch title %02d の具体的な用件", slot.Index),
			SubjectAnchor:    func() string { if slot.ReplyToPostID != 0 { return "" }; return fmt.Sprintf("title %02d", slot.Index) }(),
			Topic:            fmt.Sprintf("topic-%02d", slot.Index),
			Motivation:       "periodic board activity",
			Stance:           "neutral",
			Goal:             "share or respond",
			SituationSummary: fmt.Sprintf("batch title %02d に関する具体的な出来事", slot.Index),
			Claims:           []string{"具体的な内容を述べる"},
		})
	}
	return out, nil
}

func TestSharedEngineBatchesMultiplePostsAndRearmsByWorldTime(t *testing.T) {
	store := world.NewMemoryStore()
	host, err := store.HostByPhone("0451234567")
	if err != nil {
		t.Fatal(err)
	}
	board := world.Board{ID: "main", Name: "フリートーク"}
	planner := &fakeBatchPlanner{}
	now := time.Date(1996, 8, 26, 18, 0, 0, 0, time.Local)
	engine := New(store, planner, func() time.Time { return now })

	before := len(store.ListPosts(host.ID))
	if err := engine.CatchUp(context.Background(), host, board); err != nil {
		t.Fatal(err)
	}
	after := store.ListPosts(host.ID)
	if planner.calls != 1 {
		t.Fatalf("planner calls=%d, want 1", planner.calls)
	}
	if got := len(after) - before; got < 3 {
		t.Fatalf("generated posts=%d, want a multi-post batch", got)
	}
	if len(planner.requests[0].RecentPosts) == 0 {
		t.Fatal("planner did not receive recent board history")
	}
	generated := 0
	replyCount := 0
	for _, post := range after {
		if post.Intent.Action != ActionWorldCatchup {
			continue
		}
		if post.Body != "" {
			t.Fatalf("header catch-up eagerly rendered body for post %d", post.ID)
		}
		generated++
		if post.Body != "" {
			t.Fatalf("header catch-up eagerly materialized body for post %d", post.ID)
		}
		if post.ParentID != 0 {
			replyCount++
		}
	}
	if generated < 3 {
		t.Fatalf("generated canonical posts=%d, want >=3", generated)
	}
	if replyCount == 0 {
		t.Fatal("shared batch did not materialize any resident reply topology")
	}

	// Same world-time window is a no-op, independent of how often a user opens it.
	if err := engine.CatchUp(context.Background(), host, board); err != nil {
		t.Fatal(err)
	}
	if planner.calls != 1 {
		t.Fatalf("same-window catch-up called planner again: %d", planner.calls)
	}

	now = now.Add(7 * time.Hour)
	if !engine.NeedsCatchUp(host, board) {
		t.Fatal("board did not become stale after cadence")
	}
	if err := engine.CatchUp(context.Background(), host, board); err != nil {
		t.Fatal(err)
	}
	if planner.calls != 2 {
		t.Fatalf("next-window planner calls=%d, want 2", planner.calls)
	}
}

func TestSharedEngineResetRemovesOnlyGeneratedHistory(t *testing.T) {
	store := world.NewMemoryStore()
	host, err := store.HostByPhone("0451234567")
	if err != nil {
		t.Fatal(err)
	}
	board := world.Board{ID: "main", Name: "フリートーク"}
	planner := &fakeBatchPlanner{}
	now := time.Date(1996, 8, 26, 18, 0, 0, 0, time.Local)
	engine := New(store, planner, func() time.Time { return now })

	seedBefore := len(store.ListPosts(host.ID))
	if err := engine.CatchUp(context.Background(), host, board); err != nil {
		t.Fatal(err)
	}
	posts := store.ListPosts(host.ID)
	var generatedRoot world.Post
	for _, post := range posts {
		if post.Intent.Action == ActionWorldCatchup && post.ParentID == 0 {
			generatedRoot = post
			break
		}
	}
	if generatedRoot.ID == 0 {
		t.Fatal("generated root not found")
	}
	store.AddPost(host.ID, world.Post{
		BoardID:   board.ID,
		ParentID:  generatedRoot.ID,
		Author:    "USER",
		Subject:   "Re: " + generatedRoot.Subject,
		Body:      "user reply to generated root",
		CreatedAt: now.Add(time.Minute),
	})

	removed, kept, ok := engine.ResetGenerated(host.ID)
	if !ok {
		t.Fatal("reset unavailable")
	}
	if removed == 0 {
		t.Fatal("reset removed no generated history")
	}
	if kept != seedBefore {
		t.Fatalf("kept=%d, want original seed count %d", kept, seedBefore)
	}
	for _, post := range store.ListPosts(host.ID) {
		if post.Intent.Action == ActionWorldCatchup || post.ParentID == generatedRoot.ID {
			t.Fatalf("generated history survived reset: %+v", post)
		}
	}
}

func TestSharedEngineWorksForDifferentHostPrograms(t *testing.T) {
	store := world.NewMemoryStore()
	planner := &fakeBatchPlanner{}
	now := time.Date(1996, 8, 26, 18, 0, 0, 0, time.Local)
	engine := New(store, planner, func() time.Time { return now })

	cases := []struct {
		phone string
		board world.Board
	}{
		{"0451234567", world.Board{ID: "main", Name: "フリートーク"}},
		{"0470001080", world.Board{ID: "8", Name: "LOCAL TALK"}},
	}
	for _, tc := range cases {
		host, err := store.HostByPhone(tc.phone)
		if err != nil {
			t.Fatal(err)
		}
		before := len(store.ListPosts(host.ID))
		if err := engine.CatchUp(context.Background(), host, tc.board); err != nil {
			t.Fatalf("%s: %v", tc.phone, err)
		}
		if len(store.ListPosts(host.ID)) <= before {
			t.Fatalf("%s: no generated posts", tc.phone)
		}
	}
}

func TestCatchUpInitialIgnoresCadenceOnlyUntilFirstGeneratedBatch(t *testing.T) {
	store := world.NewMemoryStore()
	host, err := store.HostByPhone("0920000196")
	if err != nil {
		t.Fatal(err)
	}
	board := world.Board{ID: "60/1", Name: "ＰＣ－９８／ＭＯＤＥＭ"}
	planner := &fakeBatchPlanner{}
	now := time.Date(1996, 8, 26, 0, 50, 0, 0, time.Local)
	engine := New(store, planner, func() time.Time { return now })
	store.AddPost(host.ID, world.Post{
		BoardID:   board.ID,
		Author:    "NORI",
		Subject:   "直近の既存記事",
		Body:      "baseline",
		CreatedAt: now.Add(-30 * time.Minute),
	})

	if err := engine.CatchUp(context.Background(), host, board); err != nil {
		t.Fatal(err)
	}
	if planner.calls != 0 {
		t.Fatalf("normal cadence unexpectedly generated: calls=%d", planner.calls)
	}

	if err := engine.CatchUpInitial(context.Background(), host, board); err != nil {
		t.Fatal(err)
	}
	if planner.calls != 1 {
		t.Fatalf("immediate debug catch-up calls=%d, want 1", planner.calls)
	}

	// Reopening the same board in the same connection must reuse that batch.
	if err := engine.CatchUpInitial(context.Background(), host, board); err != nil {
		t.Fatal(err)
	}
	if planner.calls != 1 {
		t.Fatalf("same board regenerated inside one debug connection: calls=%d", planner.calls)
	}
}

func TestActiveActorTargetScalesWithMembershipWithoutUsingWholePopulation(t *testing.T) {
	if got := activeActorTarget(326, 326); got < 45 || got > 65 {
		t.Fatalf("HAKATA active actor window=%d, want realistic bounded subset around 18%%", got)
	}
	if got := activeActorTarget(326, 326); got >= 326 {
		t.Fatalf("active actor window must not equal whole membership: %d", got)
	}
	if got := activeActorTarget(52, 52); got != 12 {
		t.Fatalf("small-host active floor=%d, want 12", got)
	}
}

func TestHakataActorRosterDrawsFromLargeMembershipButStaysBounded(t *testing.T) {
	store := world.NewMemoryStore()
	host, err := store.HostByPhone("0920000196")
	if err != nil {
		t.Fatal(err)
	}
	engine := New(store, &fakeBatchPlanner{}, func() time.Time {
		return time.Date(1996, 8, 26, 23, 30, 0, 0, time.Local)
	})
	board := world.Board{ID: "20/1", Name: "ＧＡＭＥ"}
	roster := engine.actorRoster(host, board, nil, engine.currentTime())
	if len(roster) < 45 || len(roster) > 72 {
		t.Fatalf("active roster=%d, expected bounded subset of 326 members", len(roster))
	}
	all := store.ListHostPersonas(host.ID)
	if len(all) != host.Members {
		t.Fatalf("membership=%d, host.Members=%d", len(all), host.Members)
	}
	if len(roster) >= len(all) {
		t.Fatalf("active roster should be smaller than membership: active=%d membership=%d", len(roster), len(all))
	}
}
