package llm

import "testing"

func TestValidateBBSTitleArticleDetailsAcceptsConcreteDistinctFacts(t *testing.T) {
	req := BBSTitleArticleDetailRequest{Articles: []BBSTitleArticleDetailSeed{{EventID: "e1", Subject: "攻略本の誤植を発見しました", Summary: "攻略本の誤植を発見した"}}}
	draft := BBSTitleArticleDetailDraft{Articles: []BBSTitleArticleDetailSet{{EventID: "e1", Details: []BBSArticleDetail{
		{Kind: "locator", Fact: "手元の攻略本の62ページ、一覧表の3行目だった"},
		{Kind: "comparison", Fact: "本に印刷された表記と実際の画面表示が食い違っていた"},
		{Kind: "sequence", Fact: "同じ箇所を読み直してから、もう一度画面と見比べた"},
	}}}}
	if err := ValidateBBSTitleArticleDetails(req, draft); err != nil {
		t.Fatalf("concrete detail draft should validate: %v", err)
	}
}

func TestValidateBBSTitleArticleDetailsRejectsEditorialRestatement(t *testing.T) {
	req := BBSTitleArticleDetailRequest{Articles: []BBSTitleArticleDetailSeed{{EventID: "e1", Subject: "攻略本の誤植を発見しました", Summary: "攻略本の誤植を発見した"}}}
	draft := BBSTitleArticleDetailDraft{Articles: []BBSTitleArticleDetailSet{{EventID: "e1", Details: []BBSArticleDetail{
		{Kind: "observation", Fact: "攻略本の誤植を話題にする"},
		{Kind: "question_scope", Fact: "読者に確認を求める"},
	}}}}
	if err := ValidateBBSTitleArticleDetails(req, draft); err == nil {
		t.Fatal("editorial restatements must be rejected")
	}
}

func TestValidateBBSTitleArticleDetailsRequiresDistinctKinds(t *testing.T) {
	req := BBSTitleArticleDetailRequest{Articles: []BBSTitleArticleDetailSeed{{EventID: "e1", Subject: "接続中に回線が切れる原因は？", Summary: "接続中の切断原因を尋ねる"}}}
	draft := BBSTitleArticleDetailDraft{Articles: []BBSTitleArticleDetailSet{{EventID: "e1", Details: []BBSArticleDetail{
		{Kind: "timing", Fact: "23時台に二度続けて切れた"},
		{Kind: "timing", Fact: "一度目は接続して数分後だった"},
	}}}}
	if err := ValidateBBSTitleArticleDetails(req, draft); err == nil {
		t.Fatal("details must cover at least two distinct kinds")
	}
}
