package llm

import "testing"

func TestValidateBBSTitleArticleDetailsAcceptsSparseConcreteFacts(t *testing.T) {
	req := BBSTitleArticleDetailRequest{Articles: []BBSTitleArticleDetailSeed{{EventID: "e1", Subject: "攻略本の誤植を発見しました", Summary: "攻略本の誤植を発見した"}}}
	for _, details := range [][]BBSArticleDetail{
		{},
		{{Kind: "locator", Fact: "手元の攻略本の62ページ、一覧表の3行目だった"}},
		{
			{Kind: "locator", Fact: "手元の攻略本の62ページ、一覧表の3行目だった"},
			{Kind: "comparison", Fact: "本に印刷された表記と実際の画面表示が食い違っていた"},
		},
	} {
		draft := BBSTitleArticleDetailDraft{Articles: []BBSTitleArticleDetailSet{{EventID: "e1", Details: details}}}
		if err := ValidateBBSTitleArticleDetails(req, draft); err != nil {
			t.Fatalf("sparse concrete detail draft should validate: %v", err)
		}
	}
}

func TestValidateBBSTitleArticleDetailsRejectsEditorialRestatement(t *testing.T) {
	req := BBSTitleArticleDetailRequest{Articles: []BBSTitleArticleDetailSeed{{EventID: "e1", Subject: "攻略本の誤植を発見しました", Summary: "攻略本の誤植を発見した"}}}
	draft := BBSTitleArticleDetailDraft{Articles: []BBSTitleArticleDetailSet{{EventID: "e1", Details: []BBSArticleDetail{
		{Kind: "observation", Fact: "攻略本の誤植を話題にする"},
	}}}}
	if err := ValidateBBSTitleArticleDetails(req, draft); err == nil {
		t.Fatal("editorial restatements must be rejected")
	}
}

func TestValidateBBSTitleArticleDetailsRejectsMoreThanTwoDetails(t *testing.T) {
	req := BBSTitleArticleDetailRequest{Articles: []BBSTitleArticleDetailSeed{{EventID: "e1", Subject: "接続中に回線が切れる原因は？", Summary: "接続中の切断原因を尋ねる"}}}
	draft := BBSTitleArticleDetailDraft{Articles: []BBSTitleArticleDetailSet{{EventID: "e1", Details: []BBSArticleDetail{
		{Kind: "timing", Fact: "23時台に二度続けて切れた"},
		{Kind: "timing", Fact: "一度目は接続して数分後だった"},
		{Kind: "observation", Fact: "二度とも切断直前まで文字化けは起きていなかった"},
	}}}}
	if err := ValidateBBSTitleArticleDetails(req, draft); err == nil {
		t.Fatal("more than two details must be rejected")
	}
}

func TestValidateBBSTitleArticleDetailsRejectsHeaderMetadata(t *testing.T) {
	req := BBSTitleArticleDetailRequest{BoardName: "音楽", Articles: []BBSTitleArticleDetailSeed{{EventID: "e1", Subject: "YMOを聴き直しています", Summary: "YMOを聴き直している", CreatedAt: "1996-06-07T21:36:00+09:00"}}}
	for _, bad := range []BBSArticleDetail{
		{Kind: "locator", Fact: "音楽板のMSG 1201として掲示されている"},
		{Kind: "timing", Fact: "1996年6月7日21時36分に投稿された"},
		{Kind: "locator", Fact: "新規スレッドの先頭投稿になっている"},
	} {
		draft := BBSTitleArticleDetailDraft{Articles: []BBSTitleArticleDetailSet{{EventID: "e1", Details: []BBSArticleDetail{bad}}}}
		if err := ValidateBBSTitleArticleDetails(req, draft); err == nil {
			t.Fatalf("header metadata detail must be rejected: %+v", bad)
		}
	}
}

func TestArticleDetailFactIsRenderingMetadataAllowsEventTiming(t *testing.T) {
	for _, good := range []string{"接続して五分ほど後に一度切れた", "昨夜二度同じ症状が出た", "手元の攻略本の62ページだった"} {
		if ArticleDetailFactIsRenderingMetadata(good) {
			t.Fatalf("event-local detail wrongly rejected: %q", good)
		}
	}
}
