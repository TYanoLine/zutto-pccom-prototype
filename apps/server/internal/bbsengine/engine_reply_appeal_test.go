package bbsengine

import (
	"context"
	"errors"
	"math/rand"
	"strings"
	"testing"
	"time"

	"zutto-pccom/apps/server/internal/world"
)

type mockReplyAppealJudge struct {
	calls    int
	requests []world.ReplyAppealRequest
	fn       func(context.Context, world.ReplyAppealRequest) (world.ReplyAppealResult, error)
}

func (m *mockReplyAppealJudge) JudgeReplyAppeal(ctx context.Context, req world.ReplyAppealRequest) (world.ReplyAppealResult, error) {
	m.calls++
	m.requests = append(m.requests, req)
	if m.fn != nil {
		return m.fn(ctx, req)
	}
	res := world.ReplyAppealResult{
		Appeal: make(map[int64]int, len(req.Roots)),
		Reason: make(map[int64]string, len(req.Roots)),
	}
	for _, r := range req.Roots {
		res.Appeal[r.ID] = 3
		res.Reason[r.ID] = "mock appeal"
	}
	return res, nil
}

// SC-006 & T011(a): Judge call count is proportional to batches (1 call per batch), not roots.
func TestReplyAppealJudgeCallCountProportionalToBatchCount(t *testing.T) {
	store := world.NewMemoryStore()
	host := genericTestHost(store)
	board := world.Board{ID: "main", Name: "フリートーク", ReplyRate: 1.5}
	planner := &fakeBatchPlanner{}
	now := time.Date(1996, 8, 26, 18, 0, 0, 0, time.Local)

	judge := &mockReplyAppealJudge{}
	engine := New(store, planner, func() time.Time { return now })
	engine.ReplyAppealJudge = judge
	engine.RNG = rand.New(rand.NewSource(42))

	if err := engine.CatchUp(context.Background(), host, board); err != nil {
		t.Fatalf("CatchUp failed: %v", err)
	}

	if judge.calls != 1 {
		t.Fatalf("judge calls = %d, want 1 (1 batch)", judge.calls)
	}

	req := judge.requests[0]
	if len(req.Roots) < 3 {
		t.Fatalf("batch had %d roots, expected >= 3", len(req.Roots))
	}
	if req.Host != host.Name || req.Board != board.Name {
		t.Fatalf("request host=%q board=%q, want %q %q", req.Host, req.Board, host.Name, board.Name)
	}

	// Advance time past cadence to trigger a second batch.
	now = now.Add(7 * time.Hour)
	if err := engine.CatchUp(context.Background(), host, board); err != nil {
		t.Fatalf("second CatchUp failed: %v", err)
	}

	if judge.calls != 2 {
		t.Fatalf("judge calls after 2 batches = %d, want 2", judge.calls)
	}
}

// SC-004 & T011(b): Judge fallback when Judge is nil or returns error.
func TestReplyAppealJudgeFallbackOnError(t *testing.T) {
	store := world.NewMemoryStore()
	host := genericTestHost(store)
	board := world.Board{ID: "main", Name: "フリートーク", ReplyRate: 1.5}
	planner := &fakeBatchPlanner{}
	now := time.Date(1996, 8, 26, 18, 0, 0, 0, time.Local)

	failingJudge := &mockReplyAppealJudge{
		fn: func(_ context.Context, _ world.ReplyAppealRequest) (world.ReplyAppealResult, error) {
			return world.ReplyAppealResult{}, errors.New("simulated LLM provider outage")
		},
	}

	engine := New(store, planner, func() time.Time { return now })
	engine.ReplyAppealJudge = failingJudge
	engine.RNG = rand.New(rand.NewSource(12345))

	if err := engine.CatchUp(context.Background(), host, board); err != nil {
		t.Fatalf("CatchUp should succeed despite judge failure: %v", err)
	}

	plans := store.ListReactionPlans(host.ID, board.ID)
	if len(plans) == 0 {
		t.Fatal("expected ReactionPlans to be saved on fallback")
	}

	for _, p := range plans {
		if !p.Judged {
			t.Errorf("plan for root %d Judged=false, want true", p.TargetPostID)
		}
		if p.Appeal != 2 {
			t.Errorf("plan for root %d Appeal=%d, want 2 (baseline fallback)", p.TargetPostID, p.Appeal)
		}
		if !strings.HasPrefix(p.Reason, "fallback:") {
			t.Errorf("plan for root %d Reason=%q, want fallback prefix", p.TargetPostID, p.Reason)
		}
	}
}

// SC-005 & T011(c): Persisted ReactionPlan counts survive restart and are not rejudged.
func TestReplyAppealPersistedCountSurvivesRestart(t *testing.T) {
	store := world.NewMemoryStore()
	host := genericTestHost(store)
	board := world.Board{ID: "main", Name: "フリートーク", ReplyRate: 1.5}
	planner := &fakeBatchPlanner{}
	now := time.Date(1996, 8, 26, 18, 0, 0, 0, time.Local)

	judge1 := &mockReplyAppealJudge{
		fn: func(_ context.Context, req world.ReplyAppealRequest) (world.ReplyAppealResult, error) {
			res := world.ReplyAppealResult{
				Appeal: make(map[int64]int, len(req.Roots)),
				Reason: make(map[int64]string, len(req.Roots)),
			}
			for _, r := range req.Roots {
				res.Appeal[r.ID] = 4
				res.Reason[r.ID] = "first run high appeal"
			}
			return res, nil
		},
	}

	engine1 := New(store, planner, func() time.Time { return now })
	engine1.ReplyAppealJudge = judge1
	engine1.RNG = rand.New(rand.NewSource(99))

	if err := engine1.CatchUp(context.Background(), host, board); err != nil {
		t.Fatal(err)
	}

	savedPlans := store.ListReactionPlans(host.ID, board.ID)
	if len(savedPlans) == 0 {
		t.Fatal("no plans saved in first run")
	}

	countsBefore := make(map[int64]int, len(savedPlans))
	for _, p := range savedPlans {
		countsBefore[p.TargetPostID] = p.Count
	}

	// Create engine2 simulating restart with judge2 that returns appeal 0.
	judge2 := &mockReplyAppealJudge{
		fn: func(_ context.Context, req world.ReplyAppealRequest) (world.ReplyAppealResult, error) {
			res := world.ReplyAppealResult{
				Appeal: make(map[int64]int, len(req.Roots)),
				Reason: make(map[int64]string, len(req.Roots)),
			}
			for _, r := range req.Roots {
				res.Appeal[r.ID] = 0
				res.Reason[r.ID] = "restart should not rejudge old plans"
			}
			return res, nil
		},
	}

	engine2 := New(store, planner, func() time.Time { return now })
	engine2.ReplyAppealJudge = judge2
	engine2.RNG = rand.New(rand.NewSource(999))

	// Re-run reaction planning for existing roots
	personas := engine2.personasForReactionPlanning(host, board, nil, now)
	posts := filterBoard(store.ListPosts(host.ID), board.ID)
	for _, post := range posts {
		if world.IsSemanticRoot(post) {
			engine2.planReactionsForRoot(host, board, post, int(post.ID), personas, nil)
		}
	}

	// Verify that none of the already judged plans were re-evaluated or mutated
	plansAfter := store.ListReactionPlans(host.ID, board.ID)
	for _, p := range plansAfter {
		if beforeCount, ok := countsBefore[p.TargetPostID]; ok {
			if p.Count != beforeCount {
				t.Fatalf("root %d count changed from %d to %d after restart", p.TargetPostID, beforeCount, p.Count)
			}
			if p.Appeal != 4 {
				t.Fatalf("root %d appeal changed from 4 to %d after restart", p.TargetPostID, p.Appeal)
			}
		}
	}
}

// SC-001: Changing RNG changes the reply count sequence; same RNG gives same sequence.
func TestReplyCountRNGDeterminismAndVariation(t *testing.T) {
	rate := 1.5
	appeal := 2
	active := 30
	standard := 20
	maxReplies := 30

	seqA1 := make([]int, 50)
	rngA1 := rand.New(rand.NewSource(12345))
	for i := range seqA1 {
		seqA1[i] = world.ReplyCountForWithStandard(rngA1, rate, appeal, active, standard, maxReplies)
	}

	seqA2 := make([]int, 50)
	rngA2 := rand.New(rand.NewSource(12345))
	for i := range seqA2 {
		seqA2[i] = world.ReplyCountForWithStandard(rngA2, rate, appeal, active, standard, maxReplies)
	}

	for i := range seqA1 {
		if seqA1[i] != seqA2[i] {
			t.Fatalf("same RNG produced different count at index %d: %d vs %d", i, seqA1[i], seqA2[i])
		}
	}

	seqB := make([]int, 50)
	rngB := rand.New(rand.NewSource(67890))
	for i := range seqB {
		seqB[i] = world.ReplyCountForWithStandard(rngB, rate, appeal, active, standard, maxReplies)
	}

	sameCount := 0
	for i := range seqA1 {
		if seqA1[i] == seqB[i] {
			sameCount++
		}
	}
	if sameCount == len(seqA1) {
		t.Fatal("different RNG produced identical sequence of 50 reply counts")
	}
}

// SC-003: Active population limit (active - 1) is never exceeded.
func TestReplyCountActiveLimitBounded(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	rate := 4.0
	appeal := 4 // Highest appeal

	for active := 1; active <= 5; active++ {
		for trial := 0; trial < 1000; trial++ {
			c := world.ReplyCountForWithStandard(rng, rate, appeal, active, 10, 30)
			maxAllowed := active - 1
			if maxAllowed < 0 {
				maxAllowed = 0
			}
			if c > maxAllowed {
				t.Fatalf("active=%d produced count=%d, want <= %d", active, c, maxAllowed)
			}
		}
	}
}
