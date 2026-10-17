package worldrepo

import (
	"context"
	"fmt"
	"math"
	"sync"
	"testing"

	"zutto-pccom/apps/server/internal/llm"
	"zutto-pccom/apps/server/internal/world"
	"zutto-pccom/apps/server/internal/worldengine"
)

type appealTrackingDraftPlanner struct {
	mu           sync.Mutex
	calls        int
	appealFn     func(eventID string) (int, string, bool)
	receivedReqs []llm.BBSRootDraftRequest
}

func (p *appealTrackingDraftPlanner) GenerateBBSRootDrafts(_ context.Context, req llm.BBSRootDraftRequest) (llm.BBSRootDraftResult, error) {
	p.mu.Lock()
	p.calls++
	p.receivedReqs = append(p.receivedReqs, req)
	p.mu.Unlock()

	out := llm.BBSRootDraftResult{
		Roots: make(map[string]llm.BBSRootDraft, len(req.Events)),
	}
	for _, event := range req.Events {
		appeal := 2
		reason := "default"
		hasAppeal := false
		if p.appealFn != nil {
			appeal, reason, hasAppeal = p.appealFn(event.EventID)
		}
		out.Roots[event.EventID] = llm.BBSRootDraft{
			EventID:           event.EventID,
			ObjectClass:       event.BoardName,
			Occurrence:        event.BoardName + "に投稿する状況",
			PostContent:       event.BoardName + "の内容",
			NoveltyKey:        fmt.Sprintf("%s-appeal-%s", event.BoardID, event.EventID),
			Titles:            []string{"題名 " + event.EventID},
			ReplyAppeal:       appeal,
			ReplyAppealReason: reason,
			HasReplyAppeal:    hasAppeal,
		}
	}
	return out, nil
}

// SC-001: For n roots, LLM call count is exactly ceil(n / productionSituationChunkSize) with 0 judge calls.
func TestSituationReplyAppealPlannerCallCount(t *testing.T) {
	base := world.NewMemoryStore()
	repo := New(base, nil, nil, "1996-02-17")
	planner := repositoryBBSBatchPlanner{repo: repo}

	n := 12
	draftPlanner := &appealTrackingDraftPlanner{}
	req := animeBudgetBatch(n)

	planned, err := planner.planSituationFirstRoots(
		context.Background(),
		LLMMaterializer{ModelHistoricalMemory: true},
		worldengine.EvidenceDecision{},
		draftPlanner,
		req,
		req.Slots,
		"1996-02-17",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedCalls := int(math.Ceil(float64(n) / float64(productionSituationChunkSize)))
	if draftPlanner.calls != expectedCalls {
		t.Fatalf("expected %d draft planner calls, got %d", expectedCalls, draftPlanner.calls)
	}
	if len(planned) != n {
		t.Fatalf("expected %d planned posts, got %d", n, len(planned))
	}
}

// SC-002: reply_appeal and reason returned by fake draft planner are saved into PlannedPost.
func TestSituationReplyAppealSavedIntoPlannedPost(t *testing.T) {
	base := world.NewMemoryStore()
	repo := New(base, nil, nil, "1996-02-17")
	planner := repositoryBBSBatchPlanner{repo: repo}

	draftPlanner := &appealTrackingDraftPlanner{
		appealFn: func(eventID string) (int, string, bool) {
			return 3, "盛り上がりやすい話題", true
		},
	}
	req := animeBudgetBatch(5)

	planned, err := planner.planSituationFirstRoots(
		context.Background(),
		LLMMaterializer{ModelHistoricalMemory: true},
		worldengine.EvidenceDecision{},
		draftPlanner,
		req,
		req.Slots,
		"1996-02-17",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, p := range planned {
		if !p.HasReplyAppeal {
			t.Fatalf("expected HasReplyAppeal=true for slot %d", p.SlotIndex)
		}
		if p.ReplyAppeal != 3 {
			t.Fatalf("expected ReplyAppeal=3 for slot %d, got %d", p.SlotIndex, p.ReplyAppeal)
		}
		if p.ReplyAppealReason != "盛り上がりやすい話題" {
			t.Fatalf("expected reason '盛り上がりやすい話題' for slot %d, got %q", p.SlotIndex, p.ReplyAppealReason)
		}
	}
}

// SC-003: Out-of-bounds or missing appeal falls back to 2 and does not trigger chunk retry.
func TestSituationReplyAppealOutOfRangeDoesNotTriggerRetry(t *testing.T) {
	base := world.NewMemoryStore()
	repo := New(base, nil, nil, "1996-02-17")
	planner := repositoryBBSBatchPlanner{repo: repo}

	draftPlanner := &appealTrackingDraftPlanner{
		appealFn: func(eventID string) (int, string, bool) {
			// Simulates invalid/missing appeal from planner (HasReplyAppeal=false, appeal=2)
			return 2, "fallback:invalid_appeal", false
		},
	}
	req := animeBudgetBatch(10)

	planned, err := planner.planSituationFirstRoots(
		context.Background(),
		LLMMaterializer{ModelHistoricalMemory: true},
		worldengine.EvidenceDecision{},
		draftPlanner,
		req,
		req.Slots,
		"1996-02-17",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// 10 events with chunk size 5 => exactly 2 calls, no retries triggered
	if draftPlanner.calls != 2 {
		t.Fatalf("expected exactly 2 calls without retries, got %d", draftPlanner.calls)
	}
	if len(planned) != 10 {
		t.Fatalf("expected 10 planned posts, got %d", len(planned))
	}
	for _, p := range planned {
		if p.HasReplyAppeal {
			t.Fatalf("expected HasReplyAppeal=false for fallback post %d", p.SlotIndex)
		}
		if p.ReplyAppeal != 2 {
			t.Fatalf("expected ReplyAppeal=2 for fallback post %d, got %d", p.SlotIndex, p.ReplyAppeal)
		}
	}
}
