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
		if slot.ReplyToPostID != 0 || slot.ReplyToSlotIndex != 0 {
			subject = ""
		}
		out = append(out, PlannedPost{
			SlotIndex:        slot.Index,
			Subject:          subject,
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

// The engine tests need a normally busy generic-runtime host and a TurboBBS-style
// host, each with a little existing history. Both are defined here so no
// production station is needed. Activity matters: the engine's batch size grows
// with popularity (3 + round(popularity*3)) and every fourth event is a reply, so
// a nearly idle host would never produce reply topology.
func genericTestHost(store *world.MemoryStore) world.Host {
	host := world.Host{ID: "generic-test", Phone: "0450000010", Name: "GENERIC TEST BBS", Region: "神奈川県", Software: "generic", SoftwareID: "generic", Lines: 4, Popularity: .70, MaxBaud: 14400, Members: 187, FoundedOn: "1994-06-12", ANSI: true, GuestAllowed: true, TelehoFriendly: true}
	store.SaveHost(host)
	seedFreeTalk(store, host.ID)
	return host
}

// seedFreeTalk gives a host a small "main" board history before the test's world
// time, like the stations these tests were originally written against.
func seedFreeTalk(store *world.MemoryStore, hostID string) {
	store.ReplaceHostPosts(hostID, []world.Post{
		{ID: 1, BoardID: "main", Author: "SYSOP", Subject: "seed 1", Body: "seed history", CreatedAt: time.Date(1996, 8, 25, 21, 14, 0, 0, time.Local)},
		{ID: 2, BoardID: "main", Author: "NEKO", Subject: "seed 2", Body: "seed history", CreatedAt: time.Date(1996, 8, 26, 0, 42, 0, 0, time.Local)},
		{ID: 3, BoardID: "main", Author: "TAKA", Subject: "seed 3", Body: "seed history", CreatedAt: time.Date(1996, 8, 26, 1, 7, 0, 0, time.Local)},
	})
}

// turboTestHost registers a TurboBBS-style host (sections "1".."10") with one
// existing message per section 1/4/2/8 before the test's world time.
func turboTestHost(store *world.MemoryStore) world.Host {
	host := world.Host{ID: "turbobbs-test", Phone: "0470000001", Name: "TURBOBBS TEST", Software: "TurboBBS 1.08 compatible", SoftwareID: "turbobbs", Lines: 1, Popularity: .18, MaxBaud: 2400, Members: 52, FoundedOn: "1989-08-20", TelehoFriendly: true}
	store.SaveHost(host)
	store.ReplaceHostPosts(host.ID, []world.Post{
		{ID: 601, BoardID: "1", Author: "SYSOP", Subject: "seed 1", Body: "seed history", CreatedAt: time.Date(1996, 8, 24, 22, 18, 0, 0, time.Local)},
		{ID: 602, BoardID: "4", Author: "TARO YAMADA", Subject: "seed 2", Body: "seed history", CreatedAt: time.Date(1996, 8, 25, 0, 14, 0, 0, time.Local)},
		{ID: 603, BoardID: "2", Author: "MIKA", Subject: "seed 3", Body: "seed history", CreatedAt: time.Date(1996, 8, 25, 21, 47, 0, 0, time.Local)},
		{ID: 604, BoardID: "8", Author: "KEN", Subject: "seed 4", Body: "seed history", CreatedAt: time.Date(1996, 8, 26, 1, 8, 0, 0, time.Local)},
	})
	return host
}

func TestSharedEngineBatchesMultiplePostsAndRearmsByWorldTime(t *testing.T) {
	store := world.NewMemoryStore()
	host := genericTestHost(store)
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
	host := genericTestHost(store)
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

	generic := genericTestHost(store)
	cases := []struct {
		name  string
		host  world.Host
		board world.Board
	}{
		{"generic", generic, world.Board{ID: "main", Name: "フリートーク"}},
		{"turbobbs", turboTestHost(store), world.Board{ID: "8", Name: "LOCAL TALK"}},
	}
	for _, tc := range cases {
		before := len(store.ListPosts(tc.host.ID))
		if err := engine.CatchUp(context.Background(), tc.host, tc.board); err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		if len(store.ListPosts(tc.host.ID)) <= before {
			t.Fatalf("%s: no generated posts", tc.name)
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

func TestCatchUpInitialCountCanMaterializeLargeDebugBatchWithoutChangingNormalBatchSize(t *testing.T) {
	store := world.NewMemoryStore()
	host, err := store.HostByPhone("0920000196")
	if err != nil {
		t.Fatal(err)
	}
	board := world.Board{ID: "20/1", Name: "ＧＡＭＥ"}
	planner := &fakeBatchPlanner{}
	now := time.Date(1996, 8, 26, 23, 30, 0, 0, time.Local)
	engine := New(store, planner, func() time.Time { return now })

	if got := batchSize(host, time.Time{}, now, engine.cadence()); got != 5 {
		t.Fatalf("normal HAKATA batch=%d, want 5", got)
	}
	if err := engine.CatchUpInitialCount(context.Background(), host, board, 40); err != nil {
		t.Fatal(err)
	}
	if planner.calls != 1 {
		t.Fatalf("planner calls=%d, want 1", planner.calls)
	}
	if got := len(planner.requests[0].Slots); got != 40 {
		t.Fatalf("debug initial slots=%d, want 40", got)
	}
	posts := filterBoard(store.ListPosts(host.ID), board.ID)
	if got := len(posts); got != 40 {
		t.Fatalf("debug initial posts=%d, want 40", got)
	}

	// The override is first-materialization-only. Reopening the board must not
	// append another 40-post batch.
	if err := engine.CatchUpInitialCount(context.Background(), host, board, 40); err != nil {
		t.Fatal(err)
	}
	if planner.calls != 1 {
		t.Fatalf("debug board regenerated on reopen: calls=%d", planner.calls)
	}
	if got := len(filterBoard(store.ListPosts(host.ID), board.ID)); got != 40 {
		t.Fatalf("debug board changed on reopen: posts=%d", got)
	}
}

func TestPlanSlotsUsesStableNonUniformTimestamps(t *testing.T) {
	store := world.NewMemoryStore()
	host, err := store.HostByPhone("0920000196")
	if err != nil {
		t.Fatal(err)
	}
	board := world.Board{ID: "20/1", Name: "ＧＡＭＥ"}
	now := time.Date(1996, 8, 26, 23, 30, 0, 0, time.Local)
	start := now.Add(-28 * 24 * time.Hour)
	engine := New(store, &fakeBatchPlanner{}, func() time.Time { return now })

	a := engine.planSlots(host, board, nil, time.Time{}, now, 20, start, -1)
	b := engine.planSlots(host, board, nil, time.Time{}, now, 20, start, -1)
	if len(a) != 20 || len(b) != 20 {
		t.Fatalf("slot counts=%d/%d, want 20", len(a), len(b))
	}
	var firstGap time.Duration
	allEqual := true
	for i := range a {
		if !a[i].CreatedAt.Equal(b[i].CreatedAt) {
			t.Fatalf("slot %d timestamp is not deterministic: %s vs %s", i+1, a[i].CreatedAt, b[i].CreatedAt)
		}
		if !a[i].CreatedAt.After(start) || !a[i].CreatedAt.Before(now) {
			t.Fatalf("slot %d timestamp %s outside history window", i+1, a[i].CreatedAt)
		}
		if i == 0 {
			continue
		}
		if !a[i].CreatedAt.After(a[i-1].CreatedAt) {
			t.Fatalf("slot timestamps not increasing at %d: %s <= %s", i+1, a[i].CreatedAt, a[i-1].CreatedAt)
		}
		gap := a[i].CreatedAt.Sub(a[i-1].CreatedAt)
		if i == 1 {
			firstGap = gap
		} else if gap != firstGap {
			allEqual = false
		}
	}
	if allEqual {
		t.Fatal("simulated history timestamps remained mechanically evenly spaced")
	}
}

func TestCatchUpInitialRootHistoryCountSpreadsFortyRootsAndAddsReplies(t *testing.T) {
	store := world.NewMemoryStore()
	host, err := store.HostByPhone("0920000196")
	if err != nil {
		t.Fatal(err)
	}
	board := world.Board{ID: "20/1", Name: "ＧＡＭＥ"}
	planner := &fakeBatchPlanner{}
	now := time.Date(1996, 8, 26, 23, 30, 0, 0, time.Local)
	engine := New(store, planner, func() time.Time { return now })
	lookback := 28 * 24 * time.Hour

	if err := engine.CatchUpInitialRootHistoryCount(context.Background(), host, board, 40, lookback); err != nil {
		t.Fatal(err)
	}
	if planner.calls != 1 {
		t.Fatalf("planner calls=%d, want 1", planner.calls)
	}
	req := planner.requests[0]
	if got := len(req.Slots); got != 53 {
		t.Fatalf("history slots=%d, want 53 for 40 roots + 13 replies", got)
	}
	if req.Since != now.Add(-lookback) {
		t.Fatalf("history since=%s, want %s", req.Since, now.Add(-lookback))
	}

	posts := filterBoard(store.ListPosts(host.ID), board.ID)
	roots, replies := 0, 0
	byID := map[int64]world.Post{}
	for _, post := range posts {
		byID[post.ID] = post
		if post.ParentID == 0 {
			roots++
		} else {
			replies++
		}
	}
	if roots != 40 || replies != 13 {
		t.Fatalf("roots=%d replies=%d, want 40/13", roots, replies)
	}
	if len(posts) != 53 {
		t.Fatalf("posts=%d, want 53", len(posts))
	}
	if posts[0].CreatedAt.Sub(now.Add(-lookback)) <= 0 {
		t.Fatalf("first post did not land inside historical lookback: %s", posts[0].CreatedAt)
	}
	if !posts[len(posts)-1].CreatedAt.Before(now) {
		t.Fatalf("last post=%s, want before world now %s", posts[len(posts)-1].CreatedAt, now)
	}
	for _, post := range posts {
		if post.ParentID == 0 {
			continue
		}
		parent, ok := byID[post.ParentID]
		if !ok || parent.ParentID != 0 {
			t.Fatalf("reply %d has invalid same-window parent %d", post.ID, post.ParentID)
		}
		if !parent.CreatedAt.Before(post.CreatedAt) {
			t.Fatalf("reply %d precedes parent %d", post.ID, parent.ID)
		}
		if post.Subject != "" {
			t.Fatalf("low-level engine invented reply subject=%q; host projector owns surface convention", post.Subject)
		}
		if post.Intent.RespondsToPostID != parent.ID || post.Intent.SourcePostID != parent.ID {
			t.Fatalf("reply %d lost semantic response link: %+v", post.ID, post.Intent)
		}
	}
}

func TestReplyProjectorCanDecoupleSemanticResponseFromNativeTopology(t *testing.T) {
	store := world.NewMemoryStore()
	host := turboTestHost(store)
	board := world.Board{ID: "8", Name: "LOCAL TALK"}
	planner := &fakeBatchPlanner{}
	now := time.Date(1996, 8, 26, 18, 0, 0, 0, time.Local)
	engine := New(store, planner, func() time.Time { return now })
	engine.ReplyProjector = ReplyProjectorFunc(func(_ world.Host, source world.Post, proposed string) (ReplyRepresentation, error) {
		// Simulate a flat-message host: it can semantically answer source while
		// exposing no parent/child article topology.
		return ReplyRepresentation{ParentID: 0, Subject: "flat:" + proposed}, nil
	})

	if err := engine.CatchUpInitialCount(context.Background(), host, board, 8); err != nil {
		t.Fatal(err)
	}
	posts := filterBoard(store.ListPosts(host.ID), board.ID)
	var response world.Post
	for _, post := range posts {
		if post.Intent.RespondsToPostID != 0 {
			response = post
			break
		}
	}
	if response.ID == 0 {
		t.Fatal("no semantic response generated")
	}
	if response.ParentID != 0 {
		t.Fatalf("flat host response unexpectedly got native parent %d", response.ParentID)
	}
	if response.Intent.DiscourseMode != "reply" || response.Intent.SourcePostID == 0 {
		t.Fatalf("semantic response metadata lost: %+v", response.Intent)
	}
	if response.Subject != "flat:" {
		t.Fatalf("projected subject=%q, want projector result", response.Subject)
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


func TestWorldSelectedStaffBoardRootAuthors(t *testing.T) {
    store := world.NewMemoryStore()
    host, err := store.HostByPhone("0920000196")
    if err != nil { t.Fatal(err) }
    engine := New(store, &fakeBatchPlanner{}, nil)
    now := time.Date(1996, 8, 26, 23, 30, 0, 0, time.Local)
    board := world.Board{ID: "1", Name: "事務局からのお知らせ", RootAuthorPolicy: "sysop_only"}
    slots := engine.planSlots(host, board, nil, time.Time{}, now, 10, now.Add(-30*24*time.Hour), 0)
    if len(slots) != 10 { t.Fatalf("staff slots=%d want 10", len(slots)) }
    for _, slot := range slots {
        if slot.Author != "SYSOP" || slot.ReplyToPostID != 0 || slot.ReplyToSlotIndex != 0 {
            t.Fatalf("World scheduled a non-staff notice root: %+v", slot)
        }
    }
    // A board with the same ID but without the station-specific policy is
    // not globally restricted. Host-program semantics are not a global ID map.
    regular := world.Board{ID: "1", Name: "general board"}
    others := engine.planSlots(host, regular, nil, time.Time{}, now, 10, now.Add(-30*24*time.Hour), 0)
    for _, slot := range others {
        if slot.Author != "SYSOP" { return }
    }
    t.Fatal("unrestricted board unexpectedly scheduled only SYSOP")
}
