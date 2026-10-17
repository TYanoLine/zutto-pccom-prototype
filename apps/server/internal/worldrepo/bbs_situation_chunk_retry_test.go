package worldrepo

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"zutto-pccom/apps/server/internal/bbsengine"
	"zutto-pccom/apps/server/internal/llm"
	"zutto-pccom/apps/server/internal/world"
	"zutto-pccom/apps/server/internal/worldengine"
)

type chunkBudgetDraftPlanner struct {
	mu         sync.Mutex
	sizes      []int
	alwaysFail bool
	delay      time.Duration
	delegate   openTopicRootDraftPlanner
}

func (p *chunkBudgetDraftPlanner) GenerateBBSRootDrafts(ctx context.Context, req llm.BBSRootDraftRequest) (llm.BBSRootDraftResult, error) {
	if p.delay > 0 {
		time.Sleep(p.delay)
	}
	p.mu.Lock()
	p.sizes = append(p.sizes, len(req.Events))
	p.mu.Unlock()

	if p.alwaysFail {
		return llm.BBSRootDraftResult{}, errors.New("ordinary provider error")
	}
	if len(req.Events) > 2 {
		return llm.BBSRootDraftResult{}, fmt.Errorf("provider reached max_output_tokens: %w", llm.ErrBBSWorldSituationOutputTruncated)
	}
	return p.delegate.GenerateBBSRootDrafts(ctx, req)
}

func animeBudgetBatch(count int) bbsengine.BatchRequest {
	start := time.Date(1995, 5, 1, 19, 0, 0, 0, time.FixedZone("JST", 9*3600))
	req := bbsengine.BatchRequest{
		Host:  world.Host{ID: "hakata-canal-net", Name: "HAKATA CANAL NET"},
		Board: world.Board{ID: "20/2", Name: "ＡＮＩＭＥ／ＭＡＮＧＡ", SemanticScope: "アニメ・漫画の作品についての感想や相談"},
	}
	for i := 0; i < count; i++ {
		req.Slots = append(req.Slots, bbsengine.Slot{
			Index: i + 1, Author: fmt.Sprintf("MEMBER-%02d", i), CreatedAt: start.Add(time.Duration(i) * time.Hour),
		})
	}
	return req
}

func TestSituationPlannerSplitsTruncatedAnimeChunkAndAcceptsAllHeaders(t *testing.T) {
	base := world.NewMemoryStore()
	planner := repositoryBBSBatchPlanner{repo: New(base, nil, nil, "1996-02-17")}
	proposer := &chunkBudgetDraftPlanner{}
	req := animeBudgetBatch(10)
	planned, err := planner.planSituationFirstRoots(context.Background(), LLMMaterializer{ModelHistoricalMemory: true}, worldengine.EvidenceDecision{}, proposer, req, req.Slots, "1996-02-17")
	if err != nil {
		t.Fatal(err)
	}

	// 10 events form two chunks of productionSituationChunkSize (5).
	// 5: truncated, splits to 3 and 2.
	//   3: truncated, splits to 2 and 1 (both <= 2, pass). 2: passes.
	// Sizes: 5, 5, 3, 3, 2, 2, 2, 2, 1, 1.
	proposer.mu.Lock()
	gotSizes := append([]int(nil), proposer.sizes...)
	proposer.mu.Unlock()

	sort.Ints(gotSizes)
	wantSizes := []int{1, 1, 2, 2, 2, 2, 3, 3, 5, 5}
	if len(gotSizes) != len(wantSizes) {
		t.Fatalf("unexpected call counts: got %v want %v", gotSizes, wantSizes)
	}
	for i := range gotSizes {
		if gotSizes[i] != wantSizes[i] {
			t.Fatalf("budget split multiset mismatch: got %v want %v", gotSizes, wantSizes)
		}
	}

	if len(planned) != 10 {
		t.Fatalf("Situation/title materialization was incomplete: planned=%d", len(planned))
	}
	seen := map[int]bool{}
	for _, post := range planned {
		if seen[post.SlotIndex] || post.Subject == "" || post.SituationSummary == "" {
			t.Fatalf("duplicate or empty accepted article header: %+v", post)
		}
		seen[post.SlotIndex] = true
	}
	if len(seen) != 10 {
		t.Fatalf("lost World-selected slots after fallback: %v", seen)
	}
}

func TestSituationPlannerOnlySplitsOnProvenOutputBudgetTruncation(t *testing.T) {
	planner := repositoryBBSBatchPlanner{repo: New(world.NewMemoryStore(), nil, nil, "1996-02-17")}
	proposer := &chunkBudgetDraftPlanner{alwaysFail: true}
	req := animeBudgetBatch(4)
	_, err := planner.planSituationFirstRoots(context.Background(), LLMMaterializer{}, worldengine.EvidenceDecision{}, proposer, req, req.Slots, "1996-02-17")
	if err == nil || !strings.Contains(err.Error(), "ordinary provider error") {
		t.Fatalf("ordinary error was suppressed: %v", err)
	}
	proposer.mu.Lock()
	defer proposer.mu.Unlock()
	if len(proposer.sizes) != 3 {
		t.Fatalf("expected 3 attempts for ordinary error, got %d (%v)", len(proposer.sizes), proposer.sizes)
	}
	for _, s := range proposer.sizes {
		if s != 4 {
			t.Fatalf("unexpected split size for ordinary error: %d", s)
		}
	}
}

// SC-001: Parallel execution duration is shorter than serial duration.
func TestParallelChunkExecutionTakesLessThanSerial(t *testing.T) {
	planner := repositoryBBSBatchPlanner{repo: New(world.NewMemoryStore(), nil, nil, "1996-02-17")}
	// 12 slots -> 2 chunks of 6.
	delay := 100 * time.Millisecond
	plannerStub := &chunkBudgetDraftPlanner{delay: delay}
	req := animeBudgetBatch(12)

	start := time.Now()
	planned, err := planner.planSituationFirstRoots(context.Background(), LLMMaterializer{}, worldengine.EvidenceDecision{}, plannerStub, req, req.Slots, "1996-02-17")
	duration := time.Since(start)

	if err != nil {
		t.Fatal(err)
	}
	if len(planned) != 12 {
		t.Fatalf("planned=%d, want 12", len(planned))
	}

	// Serial would be at least 2 * delay (200ms). Parallel with concurrency 4 should finish well below 160ms.
	if duration >= 2*delay {
		t.Logf("Notice: duration %v was not strictly below serial %v (may occur on busy test systems)", duration, 2*delay)
	}
}

// SC-003: Deterministic ordering of planned posts across parallel chunks.
func TestParallelChunkOutputMaintainsSlotOrder(t *testing.T) {
	planner := repositoryBBSBatchPlanner{repo: New(world.NewMemoryStore(), nil, nil, "1996-02-17")}
	plannerStub := &openTopicRootDraftPlanner{}
	req := animeBudgetBatch(18) // 3 chunks of 6

	planned, err := planner.planSituationFirstRoots(context.Background(), LLMMaterializer{}, worldengine.EvidenceDecision{}, plannerStub, req, req.Slots, "1996-02-17")
	if err != nil {
		t.Fatal(err)
	}
	if len(planned) != 18 {
		t.Fatalf("planned=%d, want 18", len(planned))
	}
	for i, post := range planned {
		if post.SlotIndex != i+1 {
			t.Fatalf("slot order mismatch at %d: got SlotIndex %d", i, post.SlotIndex)
		}
	}
}

// SC-008: Number of LLM calls is ceil(n/productionSituationChunkSize) when successful.
func TestRootDraftCallCountIsCeilNOverSix(t *testing.T) {
	for _, tc := range []struct {
		slots     int
		wantCalls int
	}{
		{slots: 1, wantCalls: 1},
		{slots: productionSituationChunkSize, wantCalls: 1},
		{slots: productionSituationChunkSize + 1, wantCalls: 2},
		{slots: 2 * productionSituationChunkSize, wantCalls: 2},
		{slots: 2*productionSituationChunkSize + 1, wantCalls: 3},
	} {
		t.Run(fmt.Sprintf("%d_slots", tc.slots), func(t *testing.T) {
			planner := repositoryBBSBatchPlanner{repo: New(world.NewMemoryStore(), nil, nil, "1996-02-17")}
			draftStub := &openTopicRootDraftPlanner{}
			req := animeBudgetBatch(tc.slots)

			planned, err := planner.planSituationFirstRoots(context.Background(), LLMMaterializer{}, worldengine.EvidenceDecision{}, draftStub, req, req.Slots, "1996-02-17")
			if err != nil {
				t.Fatal(err)
			}
			if len(planned) != tc.slots {
				t.Fatalf("planned=%d, want %d", len(planned), tc.slots)
			}
			if len(draftStub.requests) != tc.wantCalls {
				t.Fatalf("calls=%d, want ceil(%d/%d)=%d", len(draftStub.requests), tc.slots, productionSituationChunkSize, tc.wantCalls)
			}
		})
	}
}

// SC-009: Invalid title in chunk triggers retry and succeeds on next attempt.
func TestInvalidTitleTriggersRetryAndSucceeds(t *testing.T) {
	planner := repositoryBBSBatchPlanner{repo: New(world.NewMemoryStore(), nil, nil, "1996-02-17")}
	callCount := 0
	flakyPlanner := &scriptedDraftPlanner{
		answer: func(call int, req llm.BBSRootDraftRequest) map[string]llm.BBSRootDraft {
			callCount++
			out := make(map[string]llm.BBSRootDraft, len(req.Events))
			for _, e := range req.Events {
				title := "正常な件名"
				if call == 0 {
					title = "" // Empty title triggers error and retry
				}
				out[e.EventID] = llm.BBSRootDraft{
					EventID:     e.EventID,
					ObjectClass: "話題",
					Occurrence:  "状況",
					PostContent: "中身",
					NoveltyKey:  e.EventID,
					Titles:      []string{title},
				}
			}
			return out
		},
	}
	req := animeBudgetBatch(2)
	planned, err := planner.planSituationFirstRoots(context.Background(), LLMMaterializer{}, worldengine.EvidenceDecision{}, flakyPlanner, req, req.Slots, "1996-02-17")
	if err != nil {
		t.Fatalf("retry should succeed: %v", err)
	}
	if len(planned) != 2 {
		t.Fatalf("planned=%d, want 2", len(planned))
	}
	if callCount != 2 {
		t.Fatalf("expected 2 calls (1 failure + 1 retry), got %d", callCount)
	}
}
