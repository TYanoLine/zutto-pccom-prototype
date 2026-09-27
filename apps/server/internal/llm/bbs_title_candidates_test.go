package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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
func TestContextualTitlePromptUsesBoardScopeAndWarnsBroadBoardsAgainstDomainCollapse(t *testing.T) {
	prompt := contextualTitleCandidatePrompt(BBSContextualTitleCandidateRequest{
		WorldDate:  "1996-08-26",
		BoardName:  "Ｑ＆Ａ（質問ボード）",
		BoardScope: "一般質問板。地域生活、仕事・学校、買い物、交通、食事、趣味などが混在する。",
	})
	for _, want := range []string{
		"BoardNameとBoardScope",
		"板名の語感から勝手に意味を補わず",
		"PC・ゲーム・通信のような一分野へ偏らせない",
		"地域生活、仕事・学校、買い物、交通、食事",
		"一般質問板",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("prompt missing %q:\n%s", want, prompt)
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


func TestContextualTitleCandidatesAllowUncommittedNamesAndUseLowReasoning(t *testing.T) {
	titles := []string{
		"バーチャファイター２の対戦", "セガサターンのソフト選び", "パンツァードラグーンの感想", "PlayStationのゲーム",
		"候補05", "候補06", "候補07", "候補08", "候補09", "候補10",
		"候補11", "候補12", "候補13", "候補14", "候補15", "候補16",
		"候補17", "候補18", "候補19", "候補20",
	}
	payloadText, err := json.Marshal(map[string]any{
		"titles": titles,
		"historical_claims": []map[string]any{
			{"candidate": 1, "subject": "バーチャファイター２", "kind": "product_availability", "need": "1996-08-26までに日本で存在・利用可能だったか"},
			{"candidate": 2, "subject": "セガサターン", "kind": "product_availability", "need": "1996-08-26までに日本で存在・利用可能だったか"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	var captured map[string]any
	provider := StructuredOpenAIProvider{OpenAIProvider: OpenAIProvider{
		APIKey: "test-key",
		Model:  "gpt-test",
		Client: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if err := json.NewDecoder(req.Body).Decode(&captured); err != nil {
				t.Fatalf("decode request: %v", err)
			}
			response, _ := json.Marshal(map[string]any{
				"model": "gpt-test",
				"output": []any{map[string]any{"content": []any{map[string]any{"type": "output_text", "text": string(payloadText)}}}},
				"usage": map[string]any{
					"input_tokens": 10,
					"input_tokens_details": map[string]any{"cached_tokens": 0},
					"output_tokens": 20,
					"output_tokens_details": map[string]any{"reasoning_tokens": 2},
					"total_tokens": 30,
				},
			})
			return &http.Response{
				StatusCode: http.StatusOK,
				Status:     "200 OK",
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(string(response))),
			}, nil
		})},
	}}

	got, err := provider.GenerateContextualBBSTitleCandidates(context.Background(), BBSContextualTitleCandidateRequest{
		WorldDate:       "1996-08-26",
		BoardName:       "ＧＡＭＥ",
		HistoricalFacts: []string{"セガサターンは家庭用ゲーム機として存在する。"},
		EraRules:        "世界時刻より未来の事実は採用しない。",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Titles) != 20 {
		t.Fatalf("titles=%d, want 20", len(got.Titles))
	}
	if len(got.HistoricalClaims) != 2 {
		t.Fatalf("historical claims=%d, want 2", len(got.HistoricalClaims))
	}
	if got.HistoricalClaims[0].Subject != "バーチャファイター２" || got.HistoricalClaims[0].Candidate != 1 {
		t.Fatalf("historical claim did not preserve reusable subject: %+v", got.HistoricalClaims[0])
	}
	reasoning, ok := captured["reasoning"].(map[string]any)
	if !ok || reasoning["effort"] != "low" {
		t.Fatalf("reasoning=%#v, want effort=low", captured["reasoning"])
	}
	prompt, _ := captured["input"].(string)
	for _, want := range []string{
		"まだ未確定の候補",
		"supplied historical facts にない実在固有名詞も",
		"後段の史料検証で確認できなければcanonicalには採用されない",
		"発売日・価格・仕様・売上・対応状況など追加の歴史事実を断定しない",
		"subjectはタイトル全文ではなく再利用可能な正式名称",
		"historical_claimsは史実そのものではなく",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("candidate prompt missing %q:\n%s", want, prompt)
		}
	}
}


func TestLargeTitlePoolStructurallySeparatesClaimFreeAndClaimBearingCandidates(t *testing.T) {
	claimFree := make([]map[string]any, 0, 80)
	for i := 1; i <= 80; i++ {
		claimFree = append(claimFree, map[string]any{
			"title": fmt.Sprintf("日常候補%03d", i),
			"historical_claims": []any{},
		})
	}
	claimBearing := make([]map[string]any, 0, 20)
	for i := 1; i <= 20; i++ {
		claimBearing = append(claimBearing, map[string]any{
			"title": fmt.Sprintf("実在候補%03d", i),
			"historical_claims": []map[string]any{{
				"subject": fmt.Sprintf("実在対象%03d", i),
				"kind": "general",
				"need": "基準日までの存在確認",
			}},
		})
	}
	payloadText, err := json.Marshal(map[string]any{
		"claim_free_candidates": claimFree,
		"claim_bearing_candidates": claimBearing,
	})
	if err != nil {
		t.Fatal(err)
	}
	var captured map[string]any
	provider := StructuredOpenAIProvider{OpenAIProvider: OpenAIProvider{
		APIKey: "test-key",
		Model: "gpt-test",
		Client: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if err := json.NewDecoder(req.Body).Decode(&captured); err != nil {
				t.Fatalf("decode request: %v", err)
			}
			response, _ := json.Marshal(map[string]any{
				"model": "gpt-test",
				"output": []any{map[string]any{"content": []any{map[string]any{"type": "output_text", "text": string(payloadText)}}}},
				"usage": map[string]any{"input_tokens": 10, "output_tokens": 20, "total_tokens": 30},
			})
			return &http.Response{StatusCode: http.StatusOK, Status: "200 OK", Header: make(http.Header), Body: io.NopCloser(strings.NewReader(string(response)))}, nil
		})},
	}}

	got, err := provider.GenerateContextualBBSTitleCandidates(context.Background(), BBSContextualTitleCandidateRequest{
		WorldDate: "1996-08-26",
		BoardName: "街角情報スポット",
		BoardScope: "福岡の地域情報",
		RemainingNeeded: 48,
		CandidateCount: 100,
		VerifiedReferentTarget: 5,
		ClaimBearingCandidateTarget: 20,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Titles) != 100 || len(got.HistoricalClaims) != 20 {
		t.Fatalf("titles=%d claims=%d, want 100/20", len(got.Titles), len(got.HistoricalClaims))
	}
	for i := 0; i < 80; i++ {
		if strings.HasPrefix(got.Titles[i], "実在候補") {
			t.Fatalf("claim-bearing candidate leaked into claim-free partition at %d", i)
		}
	}
	for _, claim := range got.HistoricalClaims {
		if claim.Candidate <= 80 {
			t.Fatalf("claim mapped to claim-free candidate: %+v", claim)
		}
	}
	prompt, _ := captured["input"].(string)
	if !strings.Contains(prompt, "claim_free_candidatesは80件") || !strings.Contains(prompt, "claim_bearing_candidatesは20件") {
		t.Fatalf("partition contract missing from prompt:\n%s", prompt)
	}
}
