package worldrepo

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"zutto-pccom/apps/server/internal/llm"
	"zutto-pccom/apps/server/internal/world"
)

type batchingPlannerRenderer struct {
	requests []llm.BBSTimelineIntentRequest
}

func (b *batchingPlannerRenderer) GenerateBoardPost(_ context.Context, _ llm.BoardPostRequest) (llm.BoardPostDraft, error) {
	return llm.BoardPostDraft{}, nil
}

func (b *batchingPlannerRenderer) GenerateBBSTimelineIntent(_ context.Context, req llm.BBSTimelineIntentRequest) (llm.BBSTimelineIntentDraft, error) {
	b.requests = append(b.requests, req)
	events := make([]llm.BBSIntentDraft, 0, len(req.Events))
	for _, event := range req.Events {
		events = append(events, llm.BBSIntentDraft{
			Index:      event.Index,
			Subject:    fmt.Sprintf("自由な件名%d", event.Index),
			Topic:      fmt.Sprintf("自由な話題%d", event.Index),
			Motivation: "その時の関心から書いた",
			Stance:     "特に決め打ちしない",
			Goal:       "自分のことを少し書く",
			Facts: []llm.BBSIntentFactDraft{{
				Key:   fmt.Sprintf("personal.detail.%02d", event.Index),
				Value: fmt.Sprintf("投稿%dで初めて必要になった事実", event.Index),
			}},
		})
	}
	return llm.BBSTimelineIntentDraft{
		Events: events,
		Usage:  llm.TokenUsage{Model: "planner-test", InputTokens: 10, OutputTokens: 20, TotalTokens: 30},
	}, nil
}

func TestDevelopmentTimelinePlanningUsesBoundedChronologicalBatches(t *testing.T) {
	renderer := &batchingPlannerRenderer{}
	materializer := LLMMaterializer{Renderer: renderer}
	persona := world.Persona{ID: "p1", Handle: "NEKO", WritingStyle: "short"}
	base := time.Date(1996, 8, 1, 22, 0, 0, 0, time.FixedZone("JST", 9*60*60))
	shells := make([]developmentTimelineShell, 0, 14)
	for i := 1; i <= 14; i++ {
		shells = append(shells, developmentTimelineShell{
			index:        i,
			persona:      persona,
			createdAt:    base.Add(time.Duration(i) * time.Hour),
			action:       "thread_start",
			anchorKey:    "chat",
			causeKind:    "recent_salience",
			causeSummary: "test world-selected cause",
		})
	}

	plan, err := materializer.PlanDevelopmentTimeline(
		context.Background(),
		world.Host{Name: "TEST NET", Region: "東京", Software: "DEV"},
		world.Board{ID: "2", Name: "パソコン通信・モデム"},
		"1996-08-29",
		shells,
		map[string][]world.PersonaFact{"p1": {{PersonaID: "p1", Key: "existing.fact", Value: "既存事実"}}},
		"MSG 0999 earlier board state",
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.events) != len(shells) {
		t.Fatalf("planned=%d want=%d", len(plan.events), len(shells))
	}
	if len(renderer.requests) != 3 {
		t.Fatalf("planning calls=%d want=3", len(renderer.requests))
	}
	for i, req := range renderer.requests {
		if len(req.Events) == 0 || len(req.Events) > developmentPlanningBatchSize {
			t.Fatalf("batch %d size=%d", i+1, len(req.Events))
		}
		for _, event := range req.Events {
			if event.AnchorKey != "chat" || event.CauseKind != "recent_salience" || event.CauseSummary == "" {
				t.Fatalf("batch %d lost causal shell: %+v", i+1, event)
			}
		}
	}
	if !strings.Contains(renderer.requests[1].RecentBBSState, "PLANNED EARLIER EVENTS IN THIS SAME TIMELINE") ||
		!strings.Contains(renderer.requests[1].RecentBBSState, "自由な件名1") {
		t.Fatalf("second batch did not receive earlier planned semantics: %q", renderer.requests[1].RecentBBSState)
	}
	if !strings.Contains(renderer.requests[1].RecentBBSState, "MSG 0999 earlier board state") {
		t.Fatalf("earlier canonical BBS state was dropped: %q", renderer.requests[1].RecentBBSState)
	}
	if len(renderer.requests[1].Events) == 0 || !containsString(renderer.requests[1].Events[0].ExistingFacts, "BACKGROUND ONLY: personal.detail.01=投稿1で初めて必要になった事実") {
		t.Fatalf("later batch did not receive transient earlier persona facts as background: %#v", renderer.requests[1].Events)
	}
	if !containsString(renderer.requests[1].Events[0].ExistingFacts, "BACKGROUND ONLY: existing.fact=既存事実") {
		t.Fatalf("canonical persona fact was not marked background-only: %#v", renderer.requests[1].Events[0].ExistingFacts)
	}
	if plan.usage.Model != "planner-test" || plan.usage.TotalTokens != 90 {
		t.Fatalf("aggregated usage=%+v", plan.usage)
	}
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
