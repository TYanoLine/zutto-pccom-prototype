package worldrepo

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"zutto-pccom/apps/server/internal/historicalkb"
	"zutto-pccom/apps/server/internal/llm"
	"zutto-pccom/apps/server/internal/world"
	"zutto-pccom/apps/server/internal/worldengine"
)

type reusableClaimEvidence struct {
	mu       sync.Mutex
	requests []worldengine.EvidenceRequest
	delay    time.Duration
}

func (e *reusableClaimEvidence) ResolveEvidence(ctx context.Context, req worldengine.EvidenceRequest) (worldengine.EvidenceDecision, error) {
	if e.delay > 0 {
		select {
		case <-time.After(e.delay):
		case <-ctx.Done():
			return worldengine.EvidenceDecision{}, ctx.Err()
		}
	}
	e.mu.Lock()
	e.requests = append(e.requests, req)
	e.mu.Unlock()
	return worldengine.EvidenceDecision{
		Level: historicalkb.EvidenceVerified,
		Knowledge: historicalkb.KnowledgeResult{
			CanUse: true,
			Facts: []historicalkb.HistoricalFact{{
				Kind:       req.Kind,
				Subject:    req.Subject,
				Claim:      req.Subject + " は基準日までに日本で存在していた。",
				Confidence: .95,
				Status:     historicalkb.FactVerified,
			}},
		},
	}, nil
}

func TestDevelopmentResearchTitleEraUsesReusableHistoricalClaimSubject(t *testing.T) {
	engine := &reusableClaimEvidence{}
	repo := &Repository{Engine: engine}
	claim := llm.BBSTitleHistoricalClaim{
		Candidate: 1,
		Subject:   "PC-9821Xa",
		Kind:      "product_availability",
		Need:      "1996-08-26までに日本で存在・利用可能だったか",
	}

	for _, title := range []string{"PC-9821Xa使ってる人います？", "PC-9821Xaのメモリについて"} {
		outcome := repo.developmentResearchTitleEra(
			context.Background(),
			world.Host{ID: "host"},
			world.Board{ID: "70/1", Name: "ＰＣ－９８"},
			"1996-08-26",
			title,
			[]llm.BBSTitleHistoricalClaim{claim},
		)
		if outcome.status != "verified" {
			t.Fatalf("title=%q outcome=%+v, want verified", title, outcome)
		}
	}

	engine.mu.Lock()
	defer engine.mu.Unlock()
	if len(engine.requests) != 2 {
		t.Fatalf("requests=%d, want 2", len(engine.requests))
	}
	for _, req := range engine.requests {
		if req.Subject != "PC-9821Xa" {
			t.Fatalf("subject=%q, want reusable entity key", req.Subject)
		}
		if strings.HasPrefix(req.Subject, "bbs-title-era:") {
			t.Fatalf("title-wide cache key leaked into reusable claim: %q", req.Subject)
		}
		if req.Kind != historicalkb.KnowledgeProductAvailability {
			t.Fatalf("kind=%q, want product_availability", req.Kind)
		}
	}
}

func TestHistoricalClaimsForTitleUsesOriginalCandidateIdentity(t *testing.T) {
	pool := llm.BBSTitleCandidates{
		Titles: []string{"PC-9821Xa使ってる人います？", "一般的な質問"},
		HistoricalClaims: []llm.BBSTitleHistoricalClaim{
			{Candidate: 1, Subject: "PC-9821Xa", Kind: "product_availability", Need: "基準日までの存在確認"},
		},
	}
	got := historicalClaimsForTitle(pool, pool.Titles[0])
	if len(got) != 1 || got[0].Subject != "PC-9821Xa" {
		t.Fatalf("claims=%+v", got)
	}
	if got := historicalClaimsForTitle(pool, pool.Titles[1]); len(got) != 0 {
		t.Fatalf("generic title unexpectedly inherited claims: %+v", got)
	}
}

func TestDetachedTitleResearchFinishesIndependentlyOfForegroundWait(t *testing.T) {
	engine := &reusableClaimEvidence{delay: 25 * time.Millisecond}
	repo := &Repository{Engine: engine}
	jobs := []developmentTitleEraResearchJob{{
		candidate: 1,
		title:     "PC-9821Xa使ってる人います？",
		claims: []llm.BBSTitleHistoricalClaim{{
			Candidate: 1,
			Subject:   "PC-9821Xa",
			Kind:      "product_availability",
			Need:      "基準日までの存在確認",
		}},
	}}

	ch := repo.developmentResearchTitleEraBatchDetached(
		world.Host{ID: "host"},
		world.Board{ID: "70/1", Name: "ＰＣ－９８"},
		"1996-08-26",
		jobs,
	)

	select {
	case outcomes := <-ch:
		if outcomes[1].status != "verified" {
			t.Fatalf("detached outcome=%+v", outcomes[1])
		}
	case <-time.After(time.Second):
		t.Fatal("detached historical research did not finish")
	}
}
