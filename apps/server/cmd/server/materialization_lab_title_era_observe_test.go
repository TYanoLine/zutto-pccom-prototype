package main

import (
	"context"
	"errors"
	"strings"
	"testing"

	"zutto-pccom/apps/server/internal/llm"
	"zutto-pccom/apps/server/internal/worldrepo"
)

type observeOnlyTestRenderer struct{}

func (observeOnlyTestRenderer) GenerateBoardPost(context.Context, llm.BoardPostRequest) (llm.BoardPostDraft, error) {
	return llm.BoardPostDraft{}, nil
}

func (observeOnlyTestRenderer) GenerateBBSTitleCandidates(context.Context, string, string) (llm.BBSTitleCandidates, error) {
	return llm.BBSTitleCandidates{}, nil
}

func (observeOnlyTestRenderer) ReviewBBSTitleCandidates(context.Context, llm.BBSTitleReviewRequest) (llm.BBSTitleReview, error) {
	return llm.BBSTitleReview{}, nil
}

func (observeOnlyTestRenderer) ValidateBBSTitleEra(context.Context, llm.BBSTitleEraRequest) (llm.BBSTitleEraReview, error) {
	return llm.BBSTitleEraReview{Decisions: []llm.BBSTitleEraDecision{
		{Candidate: 1, Status: llm.BBSTitleEraNG, Reason: "future title"},
		{Candidate: 2, Status: llm.BBSTitleEraResearch, Reason: "needs source"},
	}}, nil
}

func (observeOnlyTestRenderer) MaterializeBBSTitleArticleDetails(context.Context, llm.BBSTitleArticleDetailRequest) (llm.BBSTitleArticleDetailDraft, error) {
	return llm.BBSTitleArticleDetailDraft{}, nil
}

type observeOnlyFailingEraRenderer struct{ observeOnlyTestRenderer }

func (observeOnlyFailingEraRenderer) ValidateBBSTitleEra(context.Context, llm.BBSTitleEraRequest) (llm.BBSTitleEraReview, error) {
	return llm.BBSTitleEraReview{Usage: llm.TokenUsage{TotalTokens: 17}}, errors.New("invalid/duplicate era candidate 1")
}

func TestTitleEraObserveOnlyRendererKeepsOriginalDecisionInReason(t *testing.T) {
	renderer, err := newTitleEraObserveOnlyRenderer(observeOnlyTestRenderer{})
	if err != nil {
		t.Fatal(err)
	}
	review, err := renderer.ValidateBBSTitleEra(context.Background(), llm.BBSTitleEraRequest{Titles: []string{"A", "B"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(review.Decisions) != 2 {
		t.Fatalf("decisions=%d, want 2", len(review.Decisions))
	}
	for _, decision := range review.Decisions {
		if decision.Status != llm.BBSTitleEraOK {
			t.Fatalf("candidate %d status=%q, want %q", decision.Candidate, decision.Status, llm.BBSTitleEraOK)
		}
		if !strings.Contains(decision.Reason, "LAB observe-only: original=") {
			t.Fatalf("candidate %d reason does not preserve original status: %q", decision.Candidate, decision.Reason)
		}
	}
}

func TestTitleEraObserveOnlyRendererConvertsValidatorErrorToDiagnosticOKs(t *testing.T) {
	renderer, err := newTitleEraObserveOnlyRenderer(observeOnlyFailingEraRenderer{})
	if err != nil {
		t.Fatal(err)
	}
	review, err := renderer.ValidateBBSTitleEra(context.Background(), llm.BBSTitleEraRequest{Titles: []string{"A", "B", "C"}})
	if err != nil {
		t.Fatal(err)
	}
	if review.Usage.TotalTokens != 17 {
		t.Fatalf("usage lost: %+v", review.Usage)
	}
	if len(review.Decisions) != 3 {
		t.Fatalf("decisions=%d, want 3", len(review.Decisions))
	}
	for i, decision := range review.Decisions {
		if decision.Candidate != i+1 || decision.Status != llm.BBSTitleEraOK {
			t.Fatalf("decision[%d]=%+v", i, decision)
		}
		if !strings.Contains(decision.Reason, "validator_error=invalid/duplicate era candidate 1") {
			t.Fatalf("validator error not preserved: %q", decision.Reason)
		}
	}
}

func TestWithTitleEraObserveOnlyPreservesPlannerCapabilities(t *testing.T) {
	materializer := worldrepo.LLMMaterializer{Renderer: observeOnlyTestRenderer{}}
	wrapped, err := withTitleEraObserveOnly(materializer)
	if err != nil {
		t.Fatal(err)
	}
	m, ok := wrapped.(worldrepo.LLMMaterializer)
	if !ok {
		t.Fatalf("wrapped type=%T, want worldrepo.LLMMaterializer", wrapped)
	}
	if _, ok := m.Renderer.(llm.BBSTitleCandidatePlanner); !ok {
		t.Fatal("title candidate planner capability lost")
	}
	if _, ok := m.Renderer.(llm.BBSTitleEraValidator); !ok {
		t.Fatal("title era validator capability lost")
	}
	if _, ok := m.Renderer.(llm.BBSTitleArticleDetailPlanner); !ok {
		t.Fatal("title article detail planner capability lost")
	}
}

func TestNormalizeFreshEraGate(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want string
		ok   bool
	}{
		{"", "strict", true},
		{"strict", "strict", true},
		{"OBSERVE-ONLY", "observe-only", true},
		{"off", "", false},
	} {
		got, ok := normalizeFreshEraGate(tc.in)
		if got != tc.want || ok != tc.ok {
			t.Fatalf("normalizeFreshEraGate(%q)=(%q,%v), want (%q,%v)", tc.in, got, ok, tc.want, tc.ok)
		}
	}
}
