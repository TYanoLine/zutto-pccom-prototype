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

func TestTitleReviewRejectsSubjectRewriteAsCandidateLevelFailure(t *testing.T) {
	req := BBSTitleReviewRequest{
		Titles: []string{"ポケットモンスター赤緑　攻略情報交換", "新機種NINTENDO64を触ってみました"},
		Events: []BBSWorldWindowEvent{{EventID: "a"}, {EventID: "b"}},
	}
	draft := BBSTitleReview{Decisions: []BBSTitleDecision{
		{Candidate: 1, EventID: "a", Subject: req.Titles[1], Reason: "別候補へ補正", Summary: "NINTENDO64を触った"},
		{Candidate: 2, EventID: "b", Subject: req.Titles[1], Reason: "原文維持", Summary: "NINTENDO64を触った"},
	}}
	rejectRewrittenTitleDecisions(req, &draft)
	if draft.Decisions[0].EventID != "" || draft.Decisions[0].Subject != "" || draft.Decisions[0].Summary != "" || !strings.HasPrefix(draft.Decisions[0].Reason, "検査結果不備：") {
		t.Fatalf("rewritten candidate was not rejected: %+v", draft.Decisions[0])
	}
	if draft.Decisions[1].EventID != "b" || draft.Decisions[1].Subject != req.Titles[1] {
		t.Fatalf("unchanged candidate was modified: %+v", draft.Decisions[1])
	}
	if err := ValidateBBSTitleReview(req, draft); err != nil {
		t.Fatalf("sanitized review should validate: %v / %+v", err, draft)
	}
}

func TestTitleReviewDuplicateSlotRejectsOnlyLaterCandidateDeterministically(t *testing.T) {
	req := BBSTitleReviewRequest{Titles: []string{"第一候補", "第二候補", "第三候補"}, Events: []BBSWorldWindowEvent{{EventID: "a"}, {EventID: "b"}}}
	// Deliberately return decisions out of order. Candidate number, not JSON
	// response order, decides which proposal keeps the duplicated slot.
	draft := BBSTitleReview{Decisions: []BBSTitleDecision{
		{Candidate: 2, EventID: "a", Subject: "第二候補", Reason: "整合", Summary: "用件2"},
		{Candidate: 1, EventID: "a", Subject: "第一候補", Reason: "整合", Summary: "用件1"},
		{Candidate: 3, EventID: "b", Subject: "第三候補", Reason: "整合", Summary: "用件3"},
	}}
	rejectDuplicateTitleAssignments(&draft)
	byCandidate := map[int]BBSTitleDecision{}
	for _, d := range draft.Decisions {
		byCandidate[d.Candidate] = d
	}
	if byCandidate[1].EventID != "a" {
		t.Fatalf("lowest candidate did not keep slot: %+v", draft)
	}
	if byCandidate[2].EventID != "" || byCandidate[2].Subject != "" || byCandidate[2].Summary != "" || !strings.HasPrefix(byCandidate[2].Reason, "検査結果不備：") {
		t.Fatalf("later duplicate was not rejected cleanly: %+v", draft)
	}
	if byCandidate[3].EventID != "b" {
		t.Fatalf("unrelated assignment changed: %+v", draft)
	}
	if err := ValidateBBSTitleReview(req, draft); err != nil {
		t.Fatalf("sanitized review should validate: %v / %+v", err, draft)
	}
}
