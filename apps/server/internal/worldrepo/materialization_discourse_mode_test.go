package worldrepo

import (
	"context"
	"testing"

	"zutto-pccom/apps/server/internal/llm"
	"zutto-pccom/apps/server/internal/world"
)

func TestDemoSelectRootDiscourseModeCyclesAllModesAndCapsQuestions(t *testing.T) {
	host := world.Host{ID: "host-a"}
	board := world.Board{ID: "1"}

	for block := 0; block < 3; block++ {
		seen := map[string]bool{}
		questions := 0
		for i := block * len(developmentRootDiscourseModes); i < (block+1)*len(developmentRootDiscourseModes); i++ {
			mode := demoSelectRootDiscourseMode(host, board, i)
			seen[mode] = true
			if mode == "ask_peers" {
				questions++
			}
		}
		if got, want := len(seen), len(developmentRootDiscourseModes); got != want {
			t.Fatalf("block %d distinct modes=%d want %d: %#v", block, got, want, seen)
		}
		if questions != 1 {
			t.Fatalf("block %d ask_peers=%d want 1", block, questions)
		}
	}
}

type discourseCaptureRenderer struct {
	req llm.BBSWorldWindowProductionRequest
}

func (r *discourseCaptureRenderer) GenerateBoardPost(context.Context, llm.BoardPostRequest) (llm.BoardPostDraft, error) {
	return llm.BoardPostDraft{}, nil
}

func (r *discourseCaptureRenderer) GenerateBBSWorldWindowProduction(_ context.Context, req llm.BBSWorldWindowProductionRequest) (llm.BBSWorldWindowProductionDraft, error) {
	r.req = req
	briefs := make([]llm.BBSArticleBriefDraft, 0, len(req.Events))
	for _, event := range req.Events {
		briefs = append(briefs, llm.BBSArticleBriefDraft{
			EventID:    event.EventID,
			Subject:    "test",
			Episode:    "test episode",
			Topic:      "test topic",
			Motivation: "test motivation",
			Stance:     "test stance",
			Goal:       "test goal",
		})
	}
	return llm.BBSWorldWindowProductionDraft{Briefs: briefs}, nil
}

func TestPlanDevelopmentWorldWindowPropagatesDiscourseMode(t *testing.T) {
	renderer := &discourseCaptureRenderer{}
	materializer := LLMMaterializer{Renderer: renderer}
	host := world.Host{ID: "h", Name: "H", Region: "R", Software: "S"}
	shell := developmentWindowShell{
		eventID: "board-1:event-0001",
		board:   world.Board{ID: "1", Name: "free"},
		shell: developmentTimelineShell{
			index:         1,
			persona:       world.Persona{ID: "p", Handle: "NEKO"},
			action:        "thread_start",
			anchorKey:     "local",
			causeKind:     "recent_salience",
			causeSummary:  "a concrete occurrence",
			discourseMode: "share_observation",
		},
	}
	if _, err := materializer.PlanDevelopmentWorldWindow(context.Background(), host, "1996-08-26", []developmentWindowShell{shell}, nil, ""); err != nil {
		t.Fatal(err)
	}
	if got := renderer.req.Events[0].DiscourseMode; got != "share_observation" {
		t.Fatalf("discourse mode=%q want share_observation", got)
	}
}
