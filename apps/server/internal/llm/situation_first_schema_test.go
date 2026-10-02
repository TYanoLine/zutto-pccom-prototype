package llm

import "testing"

func TestWorldSituationSchemaIsTypedByDiscourseMode(t *testing.T) {
	events := []BBSWorldWindowEvent{
		{EventID: "ask", Action: "thread_start", DiscourseMode: "ask_peers"},
		{EventID: "opinion", Action: "thread_start", DiscourseMode: "state_opinion"},
		{EventID: "tip", Action: "thread_start", DiscourseMode: "share_tip"},
	}
	schema := bbsWorldSituationProposalSchema(events)
	rootProps := schema["properties"].(map[string]any)
	situations := rootProps["situations"].(map[string]any)
	eventSchemas := situations["properties"].(map[string]any)

	askProps := eventSchemas["ask"].(map[string]any)["properties"].(map[string]any)
	if _, ok := askProps["question"]; !ok {
		t.Fatal("ask_peers schema has no question field")
	}
	for _, id := range []string{"opinion", "tip"} {
		props := eventSchemas[id].(map[string]any)["properties"].(map[string]any)
		if _, ok := props["question"]; ok {
			t.Fatalf("%s schema unexpectedly exposes question", id)
		}
	}
	opinionProps := eventSchemas["opinion"].(map[string]any)["properties"].(map[string]any)
	for _, field := range []string{"stance", "basis"} {
		if _, ok := opinionProps[field]; !ok {
			t.Fatalf("state_opinion schema missing %s", field)
		}
	}
	tipProps := eventSchemas["tip"].(map[string]any)["properties"].(map[string]any)
	for _, field := range []string{"attempted_actions", "result", "practical_point"} {
		if _, ok := tipProps[field]; !ok {
			t.Fatalf("share_tip schema missing %s", field)
		}
	}
}

func TestCompactSituationEventsExcludeLegacyPolicyText(t *testing.T) {
	events := []BBSWorldWindowEvent{{
		EventID: "e1", BoardID: "4", BoardName: "ゲーム", BoardScope: "ゲームについての相談と感想", AuthorHandle: "AKI",
		CreatedAt: "1996-01-01T20:00:00+09:00", Action: "thread_start",
		AnchorKey: "games", CauseKind: "board_activity_window",
		CauseSummary: "THIS SHOULD NOT ENTER THE PROMPT",
		DiscourseMode: "share_observation", PersonaProfile: "games=0.8",
		ExistingFacts: []string{"SITUATION KIND SELECTED BY WORLD: games_score_retry"},
	}}
	got := compactSituationEvents(events)
	if len(got) != 1 {
		t.Fatalf("events=%d", len(got))
	}
	if got[0].EventID != "e1" || got[0].DiscourseMode != "share_observation" || got[0].BoardScope != "ゲームについての相談と感想" {
		t.Fatalf("unexpected compact event: %#v", got[0])
	}
}
