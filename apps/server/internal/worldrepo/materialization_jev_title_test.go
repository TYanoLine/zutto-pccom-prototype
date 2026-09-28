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

func TestJevTitlePlannerSpecificityBonusBreaksPlausibleTieOnly(t *testing.T) {
	planner := developmentJevTitlePlanner{
		titles: []string{"ゲームの話", "バーチャファイター２"},
		advice: worldengine.TitleCandidateAdviceDecision{
			Fit: map[string]float64{
				worldengine.TitleCandidatePairKey(1, "e1"): 0.60,
				worldengine.TitleCandidatePairKey(2, "e1"): 0.55,
			},
		},
		specificityBonus: map[int]float64{2: 0.12},
	}
	req := llm.BBSTitleReviewRequest{
		BoardName: "ＧＡＭＥ",
		Titles: []string{"ゲームの話", "バーチャファイター２"},
		Events: []llm.BBSWorldWindowEvent{{EventID: "e1"}},
	}
	got, err := planner.ReviewBBSTitleCandidates(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range got.Decisions {
		if d.EventID == "e1" {
			if d.Subject != "バーチャファイター２" {
				t.Fatalf("specific supported title did not win near-tie: %+v", got.Decisions)
			}
			return
		}
	}
	t.Fatal("event was not assigned")
}

func TestJevTitlePlannerSpecificityBonusCannotRescueBelowFitFloor(t *testing.T) {
	planner := developmentJevTitlePlanner{
		titles: []string{"バーチャファイター２", "ゲームの話"},
		advice: worldengine.TitleCandidateAdviceDecision{
			Fit: map[string]float64{
				worldengine.TitleCandidatePairKey(1, "e1"): 0.20,
				worldengine.TitleCandidatePairKey(2, "e1"): 0.50,
			},
		},
		specificityBonus: map[int]float64{1: 0.40},
	}
	req := llm.BBSTitleReviewRequest{
		BoardName: "ＧＡＭＥ",
		Titles: []string{"バーチャファイター２", "ゲームの話"},
		Events: []llm.BBSWorldWindowEvent{{EventID: "e1"}},
	}
	got, err := planner.ReviewBBSTitleCandidates(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range got.Decisions {
		if d.EventID == "e1" && d.Subject != "ゲームの話" {
			t.Fatalf("below-floor specific title was incorrectly rescued: %+v", got.Decisions)
		}
	}
}


func TestJevTitlePlannerRejectsGenericRootBelowSpecificityFloor(t *testing.T) {
	planner := developmentJevTitlePlanner{
		titles: []string{"お気に入りの見開き", "エヴァ第九話の作画について"},
		advice: worldengine.TitleCandidateAdviceDecision{
			Fit: map[string]float64{
				worldengine.TitleCandidatePairKey(1, "e1"): 0.92,
				worldengine.TitleCandidatePairKey(2, "e1"): 0.70,
			},
			Specificity: map[int]float64{1: 0.18, 2: 0.91},
		},
	}
	req := llm.BBSTitleReviewRequest{
		BoardName: "ＡＮＩＭＥ／ＭＡＮＧＡ",
		Titles: []string{"お気に入りの見開き", "エヴァ第九話の作画について"},
		Events: []llm.BBSWorldWindowEvent{{EventID: "e1"}},
	}
	got, err := planner.ReviewBBSTitleCandidates(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range got.Decisions {
		if d.EventID == "e1" {
			if d.Subject != "エヴァ第九話の作画について" {
				t.Fatalf("generic root crossed semantic specificity floor: %+v", got.Decisions)
			}
			return
		}
	}
	t.Fatal("specific root was not assigned")
}

func TestJevTitlePlannerRankingRecoveryStillKeepsSpecificityFloor(t *testing.T) {
	planner := developmentJevTitlePlanner{
		titles: []string{"次号の展開を予想"},
		advice: worldengine.TitleCandidateAdviceDecision{
			Fit: map[string]float64{
				worldengine.TitleCandidatePairKey(1, "e1"): 0.99,
			},
			Specificity: map[int]float64{1: 0.10},
		},
		rankingOnly: true,
	}
	req := llm.BBSTitleReviewRequest{
		BoardName: "ＡＮＩＭＥ／ＭＡＮＧＡ",
		Titles: []string{"次号の展開を予想"},
		Events: []llm.BBSWorldWindowEvent{{EventID: "e1"}},
	}
	got, err := planner.ReviewBBSTitleCandidates(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Decisions) != 1 || got.Decisions[0].EventID != "" {
		t.Fatalf("ranking-only recovery must not rescue generic roots: %+v", got.Decisions)
	}
}
