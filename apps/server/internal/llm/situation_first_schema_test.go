package llm

import (
	"encoding/json"
	"reflect"
	"sort"
	"strings"
	"testing"
)

// Every root gets the same fields: the world layer selects no post type, so
// nothing in the schema may vary with one.
func TestWorldSituationSchemaIsTheSameForEveryRoot(t *testing.T) {
	events := []BBSWorldWindowEvent{
		{EventID: "a", Action: "thread_start"},
		{EventID: "b", Action: "thread_start", BoardName: "自己紹介", BoardScope: "新規会員の自己紹介。"},
		{EventID: "c", Action: "thread_start", BoardName: "質問", BoardScope: "疑問を尋ねる板。"},
	}
	schema := bbsWorldSituationProposalSchema(events)
	rootProps := schema["properties"].(map[string]any)
	situations := rootProps["situations"].(map[string]any)
	eventSchemas := situations["properties"].(map[string]any)

	want := []string{"must_not", "novelty_key", "object_class", "occurrence", "post_content"}
	for _, id := range []string{"a", "b", "c"} {
		event := eventSchemas[id].(map[string]any)
		props := event["properties"].(map[string]any)
		got := make([]string, 0, len(props))
		for key := range props {
			got = append(got, key)
		}
		sort.Strings(got)
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("%s fields=%v want %v", id, got, want)
		}
		required := append([]string(nil), event["required"].([]string)...)
		sort.Strings(required)
		if !reflect.DeepEqual(required, want) {
			t.Fatalf("%s required=%v want %v", id, required, want)
		}
	}
}

func TestCompactSituationEventsExcludeLegacyPolicyAndRoutingText(t *testing.T) {
	events := []BBSWorldWindowEvent{{
		EventID: "e1", BoardID: "4", BoardName: "ゲーム", BoardScope: "ゲームについての相談と感想", AuthorHandle: "AKI",
		CreatedAt: "1996-01-01T20:00:00+09:00", Action: "thread_start",
		AnchorKey: "4", CauseKind: "board_activity_window",
		CauseSummary: "THIS SHOULD NOT ENTER THE PROMPT",
		DiscourseMode: "share_observation", PersonaProfile: "games=0.8",
		ExistingFacts: []string{"SITUATION KIND SELECTED BY WORLD: games_score_retry"},
	}}
	got := compactSituationEvents(events)
	if len(got) != 1 {
		t.Fatalf("events=%d", len(got))
	}
	if got[0].EventID != "e1" || got[0].BoardName != "ゲーム" || got[0].BoardScope != "ゲームについての相談と感想" {
		t.Fatalf("unexpected compact event: %#v", got[0])
	}
	encoded, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{"discourse_mode", "post_purpose", "anchor_key", "THIS SHOULD NOT ENTER THE PROMPT"} {
		if strings.Contains(string(encoded), bad) {
			t.Fatalf("compact event leaked %q: %s", bad, encoded)
		}
	}
}
