package main

import (
	"strings"
	"testing"

	"zutto-pccom/apps/server/internal/world"
	"zutto-pccom/apps/server/internal/worldrepo"
)

func TestMinimalBatchPromptsStaySituationFirstAndSmall(t *testing.T) {
	host := world.Host{Name: "HAKATA CANAL NET", Region: "福岡県福岡市"}
	board := world.Board{ID: "20/1", Name: "GAME"}
	slots := []worldrepo.DevelopmentMinimalRootSlot{{
		EventID: "board-20/1:event-0001", BoardID: "20/1", BoardName: "GAME",
		AuthorHandle: "AKI", CreatedAt: "1996-03-01T21:00:00+09:00",
		Action: "thread_start", AnchorKey: "games", CauseKind: "ordinary_interest",
		CauseSummary: "ゲームについて小さな出来事を話す", DiscourseMode: "share_experience",
		PersonaProfile: "handle=AKI games=0.86 hardware=0.78",
		SituationKind: "games_progress_setback",
		SituationSummary: "The actor recently had one small concrete experience.",
		SituationFacts: []string{"focus=a recent progress setback", "occurrence=A retry went more smoothly."},
	}}
	situationPrompt := minimalSituationPrompt(host, board, slots)
	postPrompt := minimalPostPrompt(host, board, slots, map[string]minimalBatchSituation{
		slots[0].EventID: {
			Object: "難しい区間の再挑戦", Occurrence: "終盤で失敗したあと、同じ区間を再挑戦して前回より先まで進んだ。",
			ActorObservation: "前回より落ち着いて操作できた。", Impact: "少し進行できた。", Uncertainty: "",
		},
	})
	for _, prompt := range []string{situationPrompt, postPrompt} {
		for _, want := range []string{"AKI", "GAME", "1996"} {
			if !strings.Contains(prompt, want) {
				t.Fatalf("prompt missing %q: %s", want, prompt)
			}
		}
		for _, avoid := range []string{
			"referent_requirement",
			"historical_claims",
			"article_detail_contract",
			"Jev",
			"specificity threshold",
		} {
			if strings.Contains(prompt, avoid) {
				t.Fatalf("minimal prompt unexpectedly contains %q", avoid)
			}
		}
	}
	if !strings.Contains(postPrompt, "終盤で失敗したあと") {
		t.Fatalf("post prompt does not carry detailed Situation")
	}
	for _, prompt := range []string{situationPrompt, postPrompt} {
		if strings.Contains(prompt, "ゲームについて小さな出来事を話す") {
			t.Fatalf("verbose cause summary leaked into compact prompt: %s", prompt)
		}
		if strings.Contains(prompt, "The actor recently had one small concrete experience.") {
			t.Fatalf("redundant situation summary leaked into compact prompt: %s", prompt)
		}
	}
}

func TestMinimalBatchSchemasRequireEveryEvent(t *testing.T) {
	slots := []worldrepo.DevelopmentMinimalRootSlot{
		{EventID: "e1"},
		{EventID: "e2"},
	}
	for _, schema := range []map[string]any{minimalBatchSituationSchema(slots), minimalBatchPostSchema(slots)} {
		if schema["type"] != "object" {
			t.Fatalf("unexpected root schema: %#v", schema)
		}
	}
}