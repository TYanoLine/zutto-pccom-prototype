package llm

import (
	"context"
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

	draft, err := provider.MaterializeBBSTitleArticleDetails(context.Background(), BBSTitleArticleDetailRequest{
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
	reasoning, _ := captured["reasoning"].(map[string]any)
	if reasoning["effort"] != "medium" {
		t.Fatalf("reasoning effort=%v, want medium", reasoning["effort"])
	}
	prompt, _ := captured["input"].(string)
	for _, want := range []string{"Web検索ツール", "投稿日時点", "記事意図を変えず", "具体的な命題が検索結果に直接支持", "似た名前の敵・別機種版・移植版"} {
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


func TestArticleDetailNeedsForcedWebSearchForGenericGameRoot(t *testing.T) {
	req := BBSTitleArticleDetailRequest{
		BoardName: "GAME",
		Articles: []BBSTitleArticleDetailSeed{{
			EventID: "e1", Subject: "ボスの攻撃が避けられない", Summary: "ボス攻撃を避けられず困っている", DiscourseMode: "thread_start",
		}},
	}
	draft := BBSTitleArticleDetailDraft{Articles: []BBSTitleArticleDetailSet{{
		EventID: "e1",
		Details: []BBSArticleDetail{{Kind: "observation", Fact: "画面端へ逃げても追い詰められた"}},
	}}}
	if !articleDetailNeedsForcedWebSearch(req, draft) {
		t.Fatal("GAME root without search or referent should force one Web-search retry")
	}
	draft.Articles[0].Details = append(draft.Articles[0].Details, BBSArticleDetail{Kind: "referent", Fact: "今回遊んでいる作品は『テスト作品』である"})
	if articleDetailNeedsForcedWebSearch(req, draft) {
		t.Fatal("existing canonical referent should avoid forced retry")
	}
}

func TestArticleDetailDoesNotForceGameReplyWithoutReferent(t *testing.T) {
	req := BBSTitleArticleDetailRequest{
		BoardName: "GAME",
		Articles: []BBSTitleArticleDetailSeed{{
			EventID: "e1", Subject: "", Summary: "先行記事へ自分の経験を返す", DiscourseMode: "reply",
		}},
	}
	draft := BBSTitleArticleDetailDraft{Articles: []BBSTitleArticleDetailSet{{EventID: "e1"}}}
	if articleDetailNeedsForcedWebSearch(req, draft) {
		t.Fatal("reply should inherit thread context instead of forcing a new referent search")
	}
}

func TestMaterializeBBSTitleArticleDetailsRetriesWithRequiredSearchForGenericGameRoot(t *testing.T) {
	var captured []map[string]any
	call := 0
	provider := StructuredOpenAIProvider{OpenAIProvider: OpenAIProvider{
		APIKey: "test-key",
		Model:  "gpt-test",
		Client: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			var payload map[string]any
			if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
				t.Fatalf("decode request: %v", err)
			}
			captured = append(captured, payload)
			call++
			var response string
			if call == 1 {
				response = `{
					"model":"gpt-test",
					"output":[{"type":"message","content":[{"type":"output_text","text":"{\"articles\":[{\"event_id\":\"e1\",\"details\":[{\"kind\":\"observation\",\"fact\":\"画面端へ逃げても追い詰められて当たった\"}]}]}"}]}],
					"usage":{"input_tokens":20,"input_tokens_details":{"cached_tokens":0},"output_tokens":10,"output_tokens_details":{"reasoning_tokens":2},"total_tokens":30}
				}`
			} else {
				response = `{
					"model":"gpt-test",
					"output":[
						{"type":"web_search_call","action":{"type":"search","sources":[{"type":"url","url":"https://example.com/game"}]}},
						{"type":"message","content":[{"type":"output_text","text":"{\"articles\":[{\"event_id\":\"e1\",\"details\":[{\"kind\":\"referent\",\"fact\":\"今回遊んでいる作品は『テスト作品』である\"},{\"kind\":\"observation\",\"fact\":\"同じ攻撃を三回避けようとしたが当たった\"}]}]}"}]}
					],
					"usage":{"input_tokens":30,"input_tokens_details":{"cached_tokens":5},"output_tokens":14,"output_tokens_details":{"reasoning_tokens":4},"total_tokens":44}
				}`
			}
			return &http.Response{StatusCode: http.StatusOK, Status: "200 OK", Header: make(http.Header), Body: io.NopCloser(strings.NewReader(response))}, nil
		})},
	}}

	draft, err := provider.MaterializeBBSTitleArticleDetails(context.Background(), BBSTitleArticleDetailRequest{
		BoardName: "GAME",
		WorldDate: "1996-08-10",
		Articles: []BBSTitleArticleDetailSeed{{
			EventID: "e1", Subject: "ボスの攻撃が避けられない", Summary: "ボスの攻撃を避けられず困っている", AuthorHandle: "X68.V", CreatedAt: "1996-08-10T00:32:00+09:00", DiscourseMode: "thread_start",
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if call != 2 {
		t.Fatalf("OpenAI calls=%d, want 2", call)
	}
	if got := captured[0]["tool_choice"]; got != "auto" {
		t.Fatalf("first tool_choice=%v, want auto", got)
	}
	if got := captured[1]["tool_choice"]; got != "required" {
		t.Fatalf("retry tool_choice=%v, want required", got)
	}
	forcedPrompt, _ := captured[1]["input"].(string)
	if !strings.Contains(forcedPrompt, "FORCED WEB SEARCH RETRY") || !strings.Contains(forcedPrompt, "referent detailとして必ず") {
		t.Fatalf("forced retry prompt missing referent requirement: %s", forcedPrompt)
	}
	if !draft.ForcedWebSearchRetry {
		t.Fatal("forced retry diagnostic flag was not set")
	}
	if draft.WebSearchCalls != 1 || len(draft.WebSearchSources) != 1 {
		t.Fatalf("search diagnostics calls=%d sources=%v", draft.WebSearchCalls, draft.WebSearchSources)
	}
	if draft.Usage.TotalTokens != 74 || draft.Usage.InputTokens != 50 || draft.Usage.CachedInputTokens != 5 {
		t.Fatalf("retry usage was not accumulated: %+v", draft.Usage)
	}
	if len(draft.Articles) != 1 || len(draft.Articles[0].Details) != 2 || draft.Articles[0].Details[0].Kind != "referent" {
		t.Fatalf("forced grounded draft=%+v", draft.Articles)
	}
}
