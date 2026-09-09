package llm

import (
	"strings"
	"testing"
)

func TestTitleCandidatePromptStaysMinimal(t *testing.T) {
	p := titleCandidatePrompt("1995-10-01", "ゲーム雑談")
	for _, want := range []string{"1995-10-01", "ゲーム雑談", "20個", "固有名詞を含めても良い"} {
		if !strings.Contains(p, want) {
			t.Fatal(p)
		}
	}
	for _, bad := range []string{"persona", "discourse", "いますか", "出来事", "36"} {
		if strings.Contains(p, bad) {
			t.Fatal(p)
		}
	}
}
func TestTitleReviewRejectsInvalidAssignments(t *testing.T) {
	req := BBSTitleReviewRequest{Titles: []string{"感想など", "最近何を遊んでます？"}, Events: []BBSWorldWindowEvent{{EventID: "a"}, {EventID: "b"}}}
	valid := BBSTitleReview{Decisions: []BBSTitleDecision{{Candidate: 1, EventID: "a", Subject: "感想など", Reason: "整合", Summary: "感想を共有"}, {Candidate: 2, Reason: "適合枠なし"}}}
	if err := ValidateBBSTitleReview(req, valid); err != nil {
		t.Fatal(err)
	}
	cases := []BBSTitleDecision{
		{Candidate: 1, Reason: "重複"},
		{Candidate: 2, EventID: "unknown", Subject: "話題", Reason: "整合", Summary: "話題"},
		{Candidate: 2, EventID: "a", Subject: "話題", Reason: "整合", Summary: "話題"},
		{Candidate: 2, EventID: "b", Subject: "Re: 感想", Reason: "整合", Summary: "話題"},
		{Candidate: 2, EventID: "b", Subject: "感想など", Reason: "整合", Summary: "話題"},
		{Candidate: 2, Subject: "不採用なのに本文", Reason: "拒否"},
	}
	for _, bad := range cases {
		draft := BBSTitleReview{Decisions: []BBSTitleDecision{valid.Decisions[0], bad}}
		if ValidateBBSTitleReview(req, draft) == nil {
			t.Fatalf("accepted invalid decision: %+v", bad)
		}
	}
}

func TestTitleReviewMissingReasonRejectsOnlyThatCandidate(t *testing.T) {
	draft := BBSTitleReview{Decisions: []BBSTitleDecision{
		{Candidate: 1, EventID: "a", Subject: "補正された件名", Summary: "用件"},
		{Candidate: 2, EventID: "b", Subject: "原文", Summary: "用件", Reason: "既存設定と整合"},
	}}
	rejectUnexplainedTitleDecisions(&draft)
	if draft.Decisions[0].EventID != "" || draft.Decisions[0].Subject != "" || draft.Decisions[0].Reason == "" {
		t.Fatal(draft)
	}
	if draft.Decisions[1].EventID != "b" || draft.Decisions[1].Subject != "原文" {
		t.Fatal("unrelated candidate changed")
	}
}
