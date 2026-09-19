package worldrepo

import (
	"context"
	"testing"

	"zutto-pccom/apps/server/internal/llm"
	"zutto-pccom/apps/server/internal/worldengine"
)

func TestJevTitlePlannerUsesRelativeRankingAbovePlausibilityFloor(t *testing.T) {
	planner := developmentJevTitlePlanner{
		titles: []string{"候補A", "候補B", "候補C"},
		advice: worldengine.TitleCandidateAdviceDecision{
			Fit: map[string]float64{
				worldengine.TitleCandidatePairKey(1, "e1"): 0.46,
				worldengine.TitleCandidatePairKey(1, "e2"): 0.12,
				worldengine.TitleCandidatePairKey(2, "e1"): 0.41,
				worldengine.TitleCandidatePairKey(2, "e2"): 0.44,
				worldengine.TitleCandidatePairKey(3, "e1"): 0.20,
				worldengine.TitleCandidatePairKey(3, "e2"): 0.30,
			},
		},
	}
	req := llm.BBSTitleReviewRequest{
		BoardName: "テスト",
		Titles: []string{"候補A", "候補B", "候補C"},
		Events: []llm.BBSWorldWindowEvent{{EventID: "e1"}, {EventID: "e2"}},
	}
	got, err := planner.ReviewBBSTitleCandidates(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	assigned := map[string]string{}
	for _, d := range got.Decisions {
		if d.EventID != "" {
			assigned[d.EventID] = d.Subject
		}
	}
	if assigned["e1"] != "候補A" || assigned["e2"] != "候補B" {
		t.Fatalf("expected relative ranking to fill both plausible slots, got %+v", assigned)
	}
	for _, d := range got.Decisions {
		if d.Candidate == 3 && d.EventID != "" {
			t.Fatalf("score below plausibility floor should remain unassigned: %+v", d)
		}
	}
}
