package llm

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestValidateBBSTitleArticleDetailsAcceptsSparseConcreteFacts(t *testing.T) {
	req := BBSTitleArticleDetailRequest{Articles: []BBSTitleArticleDetailSeed{{EventID: "e1", Subject: "攻略本の誤植を発見しました", Summary: "攻略本の誤植を発見した"}}}
	for _, details := range [][]BBSArticleDetail{
		{},
		{{Kind: "locator", Fact: "手元の攻略本の62ページ、一覧表の3行目だった"}},
		{{Kind: "referent", Fact: "今回話している作品は発売済みのゲーム『テスト作品』である"}},
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

func TestValidateBBSTitleArticleDetailsRejectsDuplicateFacts(t *testing.T) {
	req := BBSTitleArticleDetailRequest{Articles: []BBSTitleArticleDetailSeed{{EventID: "post-1", Subject: "接続のこと", Summary: "接続を確認した"}}}
	draft := BBSTitleArticleDetailDraft{Articles: []BBSTitleArticleDetailSet{{EventID: "post-1", Details: []BBSArticleDetail{
		{Kind: "observation", Fact: "画面の右側に線が残った"},
		{Kind: "comparison", Fact: " 画面の右側に線が残った "},
	}}}}
	if err := ValidateBBSTitleArticleDetails(req, draft); err == nil {
		t.Fatal("duplicate detail facts should be rejected")
	}
}

func TestArticleDetailFactIsRenderingMetadataAllowsEventTiming(t *testing.T) {
	for _, good := range []string{"接続して五分ほど後に一度切れた", "昨夜二度同じ症状が出た", "手元の攻略本の62ページだった"} {
		if ArticleDetailFactIsRenderingMetadata(good) {
			t.Fatalf("event-local detail wrongly rejected: %q", good)
		}
	}
}


func TestMaterializeBBSTitleArticleDetailsEnablesOptionalWebSearchAndReportsUse(t *testing.T) {
	var captured map[string]any
	provider := StructuredOpenAIProvider{OpenAIProvider: OpenAIProvider{
		APIKey: "test-key",
		Model:  "gpt-test",
		Client: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if err := json.NewDecoder(req.Body).Decode(&captured); err != nil {
				t.Fatalf("decode request: %v", err)
			}
			response := `{
				"model":"gpt-test",
				"output":[
					{"type":"web_search_call","action":{"type":"search","sources":[{"type":"url","url":"https://example.com/source"}]}},
					{"type":"message","content":[{"type":"output_text","text":"{\"articles\":[{\"event_id\":\"e1\",\"details\":[{\"kind\":\"referent\",\"fact\":\"今回話している作品は『テスト作品』である\"}]}]}"}]}
				],
				"usage":{"input_tokens":20,"input_tokens_details":{"cached_tokens":0},"output_tokens":12,"output_tokens_details":{"reasoning_tokens":2},"total_tokens":32}
			}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Status:     "200 OK",
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(response)),
			}, nil
		})},
	}}

	draft, err := provider.MaterializeBBSTitleArticleDetails(t.Context(), BBSTitleArticleDetailRequest{
		BoardName: "GAME",
		WorldDate: "1996-07-19",
		Articles: []BBSTitleArticleDetailSeed{{
			EventID:       "e1",
			Subject:       "エンディングを見た人へ",
			Summary:       "最後の場面について感想を聞きたい",
			AuthorHandle:  "X68.V",
			CreatedAt:     "1996-07-19T01:43:53+09:00",
			DiscourseMode: "thread_start",
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	tools, ok := captured["tools"].([]any)
	if !ok || len(tools) != 1 {
		t.Fatalf("web search tools missing: %#v", captured["tools"])
	}
	tool, _ := tools[0].(map[string]any)
	if tool["type"] != "web_search" {
		t.Fatalf("tool type=%v, want web_search", tool["type"])
	}
	if captured["tool_choice"] != "auto" {
		t.Fatalf("tool_choice=%v, want auto", captured["tool_choice"])
	}
	prompt, _ := captured["input"].(string)
	for _, want := range []string{"Web検索ツール", "投稿日時点", "記事意図を変えず"} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("article detail prompt missing %q", want)
		}
	}
	if draft.WebSearchCalls != 1 {
		t.Fatalf("web search calls=%d, want 1", draft.WebSearchCalls)
	}
	if len(draft.WebSearchSources) != 1 || draft.WebSearchSources[0] != "https://example.com/source" {
		t.Fatalf("web search sources=%v", draft.WebSearchSources)
	}
	if len(draft.Articles) != 1 || len(draft.Articles[0].Details) != 1 || draft.Articles[0].Details[0].Kind != "referent" {
		t.Fatalf("unexpected grounded detail draft: %+v", draft.Articles)
	}
}
