package bbsengine

import (
	"context"
	"fmt"
	"math/rand"
	"testing"
	"time"

	"zutto-pccom/apps/server/internal/world"
)

type customAppealBatchPlanner struct {
	calls        int
	appeal       int
	appealReason string
	hasAppeal    bool
}

func (p *customAppealBatchPlanner) PlanBBSBatch(_ context.Context, req BatchRequest) ([]PlannedPost, error) {
	p.calls++
	out := make([]PlannedPost, 0, len(req.Slots))
	for _, slot := range req.Slots {
		subject := fmt.Sprintf("batch title %02d", slot.Index)
		if slot.ReplyToPostID != 0 || slot.ReplyToSlotIndex != 0 {
			subject = ""
		}
		out = append(out, PlannedPost{
			SlotIndex:         slot.Index,
			Subject:           subject,
			Topic:             fmt.Sprintf("topic-%02d", slot.Index),
			Motivation:        "periodic board activity",
			Stance:            "neutral",
			Goal:              "share or respond",
			SituationSummary:  fmt.Sprintf("batch title %02d に関する具体的な出来事", slot.Index),
			Claims:            []string{"具体的な内容を述べる"},
			ReplyAppeal:       p.appeal,
			ReplyAppealReason: p.appealReason,
			HasReplyAppeal:    p.hasAppeal,
		})
	}
	return out, nil
}

// SC-002: Verify PlannedPost appeal and reason are saved into ReactionPlan.
func TestReplyAppealCarriedFromPlannerIntoReactionPlan(t *testing.T) {
	store := world.NewMemoryStore()
	host := genericTestHost(store)
	board := world.Board{ID: "main", Name: "フリートーク", ReplyRate: 1.5}
	planner := &customAppealBatchPlanner{appeal: 3, appealReason: "planned appeal", hasAppeal: true}
	now := time.Date(1996, 8, 26, 18, 0, 0, 0, time.Local)

	engine := New(store, planner, func() time.Time { return now })
	engine.RNG = rand.New(rand.NewSource(42))

	if err := engine.CatchUp(context.Background(), host, board); err != nil {
		t.Fatalf("CatchUp failed: %v", err)
	}

	plans := store.ListReactionPlans(host.ID, board.ID)
	if len(plans) == 0 {
		t.Fatal("expected ReactionPlans to be saved")
	}

	for _, p := range plans {
		if !p.Judged {
			t.Errorf("plan for root %d Judged=false, want true", p.TargetPostID)
		}
		if p.TargetPostID >= 1000 {
			if p.Appeal != 3 {
				t.Errorf("plan for batch root %d Appeal=%d, want 3", p.TargetPostID, p.Appeal)
			}
			if p.Reason != "planned appeal" {
				t.Errorf("plan for batch root %d Reason=%q, want 'planned appeal'", p.TargetPostID, p.Reason)
			}
		} else {
			if p.Appeal != 2 {
				t.Errorf("plan for seed root %d Appeal=%d, want 2 (fallback)", p.TargetPostID, p.Appeal)
			}
			if p.Reason != "fallback:no_appeal" {
				t.Errorf("plan for seed root %d Reason=%q, want 'fallback:no_appeal'", p.TargetPostID, p.Reason)
			}
		}
	}
}

// FR-006: Fallback when appeal is not carried (HasReplyAppeal = false).
func TestReplyAppealFallbackWhenNoAppeal(t *testing.T) {
	store := world.NewMemoryStore()
	host := genericTestHost(store)
	board := world.Board{ID: "main", Name: "フリートーク", ReplyRate: 1.5}
	planner := &customAppealBatchPlanner{hasAppeal: false}
	now := time.Date(1996, 8, 26, 18, 0, 0, 0, time.Local)

	engine := New(store, planner, func() time.Time { return now })
	engine.RNG = rand.New(rand.NewSource(12345))

	if err := engine.CatchUp(context.Background(), host, board); err != nil {
		t.Fatalf("CatchUp should succeed on fallback: %v", err)
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
		if p.Reason != "fallback:no_appeal" {
			t.Errorf("plan for root %d Reason=%q, want 'fallback:no_appeal'", p.TargetPostID, p.Reason)
		}
	}
}

// SC-005 & T011(c): Persisted ReactionPlan counts survive restart and are not rejudged.
func TestReplyAppealPersistedCountSurvivesRestart(t *testing.T) {
	store := world.NewMemoryStore()
	host := genericTestHost(store)
	board := world.Board{ID: "main", Name: "フリートーク", ReplyRate: 1.5}
	planner := &customAppealBatchPlanner{appeal: 4, appealReason: "first run high appeal", hasAppeal: true}
	now := time.Date(1996, 8, 26, 18, 0, 0, 0, time.Local)

	engine1 := New(store, planner, func() time.Time { return now })
	engine1.RNG = rand.New(rand.NewSource(99))

	if err := engine1.CatchUp(context.Background(), host, board); err != nil {
		t.Fatal(err)
	}

	savedPlans := store.ListReactionPlans(host.ID, board.ID)
	if len(savedPlans) == 0 {
		t.Fatal("no plans saved in first run")
	}

	countsBefore := make(map[int64]int, len(savedPlans))
	appealBefore := make(map[int64]int, len(savedPlans))
	for _, p := range savedPlans {
		countsBefore[p.TargetPostID] = p.Count
		appealBefore[p.TargetPostID] = p.Appeal
	}

	// Create engine2 simulating restart
	engine2 := New(store, planner, func() time.Time { return now })
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
			if p.Appeal != appealBefore[p.TargetPostID] {
				t.Fatalf("root %d appeal changed from %d to %d after restart", p.TargetPostID, appealBefore[p.TargetPostID], p.Appeal)
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

// SC-002 & Review #2: Verify reply expectation differs by time of day for the same station.
// Night (23:30) active population and expected replies should exceed dawn (04:30).
func TestReplyCountTimeFactorDiffersByTimeOfDay(t *testing.T) {
	store := world.NewMemoryStore()
	host := genericTestHost(store)
	host.Members = 300
	board := world.Board{ID: "main", Name: "フリートーク", ReplyRate: 2.0}

	personas := make([]world.Persona, 30)
	for i := range personas {
		pattern := "active"
		if i%3 == 0 {
			pattern = "night_owl"
		} else if i%3 == 1 {
			pattern = "standard"
		}
		personas[i] = world.Persona{
			ID:              host.ID + ":p" + string(rune('a'+i)),
			Handle:          "user" + string(rune('a'+i)),
			ActivityPattern: pattern,
		}
	}

	jst, err := time.LoadLocation("Asia/Tokyo")
	if err != nil {
		jst = time.FixedZone("JST", 9*60*60)
	}

	nightTime := time.Date(1996, 8, 26, 23, 30, 0, 0, jst)
	dawnTime := time.Date(1996, 8, 26, 4, 30, 0, 0, jst)

	activeNight, standardNight := world.ActivePopulationAt(host, board, personas, nil, nightTime)
	activeDawn, standardDawn := world.ActivePopulationAt(host, board, personas, nil, dawnTime)

	if standardNight != standardDawn {
		t.Fatalf("standard 24h baseline should be equal regardless of time of day: night=%d dawn=%d", standardNight, standardDawn)
	}
	if activeNight <= activeDawn {
		t.Fatalf("expected night active (%d) > dawn active (%d)", activeNight, activeDawn)
	}

	// Compute average reply count over 1000 trials with fixed seeds for night vs dawn.
	trials := 1000
	rate := 2.0
	appeal := 2
	maxReplies := 30

	sumNight := 0
	rngNight := rand.New(rand.NewSource(1001))
	for i := 0; i < trials; i++ {
		sumNight += world.ReplyCountForWithStandard(rngNight, rate, appeal, activeNight, standardNight, maxReplies)
	}

	sumDawn := 0
	rngDawn := rand.New(rand.NewSource(1001))
	for i := 0; i < trials; i++ {
		sumDawn += world.ReplyCountForWithStandard(rngDawn, rate, appeal, activeDawn, standardDawn, maxReplies)
	}

	avgNight := float64(sumNight) / float64(trials)
	avgDawn := float64(sumDawn) / float64(trials)

	if avgNight <= avgDawn {
		t.Fatalf("expected average replies at night (%.2f) > dawn (%.2f)", avgNight, avgDawn)
	}
}

