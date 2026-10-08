package llm

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestValidateBoardPostDraft(t *testing.T) {
	if err := validateBoardPostDraft(BoardPostDraft{Author: "NORI96", Subject: "通信ソフトの設定", Body: "最近設定をいじっています(^^;"}); err != nil {
		t.Fatalf("valid draft rejected: %v", err)
	}
	if err := validateBoardPostDraft(BoardPostDraft{Author: "MAKO.J", Subject: "通信ソフトの設定", Body: "最近設定をいじっています(^^;"}); err != nil {
		t.Fatalf("ASCII-symbol handle rejected: %v", err)
	}
	if err := validateBoardPostDraft(BoardPostDraft{Author: "KAZU-O", Subject: "通信ソフトの設定", Body: "最近設定をいじっています(^^;"}); err != nil {
		t.Fatalf("dash handle rejected: %v", err)
	}
	if err := validateBoardPostDraft(BoardPostDraft{Author: "日本語", Subject: "test", Body: "body"}); err == nil {
		t.Fatal("non-ASCII handle accepted")
	}
	if err := validateBoardPostDraft(BoardPostDraft{Author: "NORI", Subject: "スマホの話", Body: "body"}); err == nil {
		t.Fatal("future term accepted")
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

func TestGenerateBoardPostCapturesResponsesUsage(t *testing.T) {
	response := `{
		"model":"gpt-test-1996",
		"output":[{"content":[{"type":"output_text","text":"{\"author\":\"NORI\",\"subject\":\"モデムの話\",\"body\":\"設定を少し見直してます。\"}"}]}],
		"usage":{
			"input_tokens":812,
			"input_tokens_details":{"cached_tokens":256},
			"output_tokens":94,
			"output_tokens_details":{"reasoning_tokens":18},
			"total_tokens":906
		}
	}`
	provider := OpenAIProvider{
		Endpoint: "https://test.openai.azure.com",
		APIKey: "test-key",
		Model:  "gpt-test",
		Client: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if req.URL.String() != "https://test.openai.azure.com/openai/v1/responses" {
				t.Fatalf("url=%q", req.URL.String())
			}
			if req.Header.Get("api-key") != "test-key" || req.Header.Get("Authorization") != "" {
				t.Fatalf("unexpected Azure auth headers: api-key=%q authorization=%q", req.Header.Get("api-key"), req.Header.Get("Authorization"))
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Status:     "200 OK",
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(response)),
			}, nil
		})},
	}
	draft, err := provider.GenerateBoardPost(context.Background(), BoardPostRequest{BoardTopic: "モデムの話", WorldDate: "1996-08-29"})
	if err != nil {
		t.Fatal(err)
	}
	if draft.Usage.InputTokens != 812 || draft.Usage.CachedInputTokens != 256 || draft.Usage.OutputTokens != 94 || draft.Usage.ReasoningTokens != 18 || draft.Usage.TotalTokens != 906 {
		t.Fatalf("unexpected token usage: %+v", draft.Usage)
	}
	if draft.Usage.Model != "gpt-test-1996" {
		t.Fatalf("model=%q, want response model", draft.Usage.Model)
	}
}

func TestGenerateBoardPostIncludesDiegeticPresentAndBaselineRules(t *testing.T) {
	response := `{
		"model":"gpt-test-1996",
		"output":[{"content":[{"type":"output_text","text":"{\"author\":\"TAKA\",\"subject\":\"途中で切れた\",\"body\":\"さっき途中で切れました。もう一度つないだら今度は大丈夫みたいです。\"}"}]}],
		"usage":{"input_tokens":10,"output_tokens":10,"total_tokens":20}
	}`
	var capturedPrompt string
	provider := OpenAIProvider{
		Endpoint: "https://test.openai.azure.com",
		APIKey: "test-key",
		Model:  "gpt-test",
		Client: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			var payload map[string]any
			if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
				t.Fatalf("decode request: %v", err)
			}
			capturedPrompt, _ = payload["input"].(string)
			return &http.Response{
				StatusCode: http.StatusOK,
				Status:     "200 OK",
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(response)),
			}, nil
		})},
	}

	_, err := provider.GenerateBoardPost(context.Background(), BoardPostRequest{
		BoardTopic:       "パソコン通信・モデム",
		WorldDate:        "1996-08-29",
		EraRules:         "世界時刻より未来の知識を使わない。",
		AuthorHandle:     "TAKA",
		PersonaProfile:   "everyday_baseline=[自宅のパソコンと通信環境は普段使いの道具]; interests=[communications=0.44]",
		PostIntent:       "internal_routing_domain=communications; topic=通信中の切断; motivation=さっき通信中に切れたため; goal=起きたことを短く共有する",
		CanonicalSubject: "途中で切れた",
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"世界日付: 1996-08-29",
		"確定済み件名: 途中で切れた",
		"everyday_baseline=[自宅のパソコンと通信環境は普段使いの道具]",
		"canonical Situation / thread facts",
		"この記事を書いている本人の自然な文章",
		"時代背景: 世界時刻より未来の知識を使わない。",
	} {
		if !strings.Contains(capturedPrompt, want) {
			t.Fatalf("board-post prompt missing material/context %q:\n%s", want, capturedPrompt)
		}
	}
}

func TestVerbosityOverrideReplacesCallSiteValue(t *testing.T) {
	cases := map[string]string{"": "low", "medium": "medium", " medium ": "medium"}
	for override, want := range cases {
		var got string
		provider := StructuredOpenAIProvider{OpenAIProvider: OpenAIProvider{
			Endpoint:  "https://test.openai.azure.com",
			APIKey:    "test-key",
			Model:     "gpt-test",
			Verbosity: override,
			Client: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				raw, _ := io.ReadAll(req.Body)
				var payload struct {
					Text struct {
						Verbosity string `json:"verbosity"`
					} `json:"text"`
				}
				if err := json.Unmarshal(raw, &payload); err != nil {
					t.Fatalf("payload: %v", err)
				}
				got = payload.Text.Verbosity
				return &http.Response{StatusCode: 200, Status: "200 OK", Header: http.Header{}, Body: io.NopCloser(strings.NewReader(`{"output":[{"content":[{"type":"output_text","text":"{}"}]}]}`))}, nil
			})},
		}}
		_, _ = provider.responseTextWithJSONSchema(context.Background(), "p", "low", 100, "s", map[string]any{"type": "object"})
		if got != want {
			t.Fatalf("override %q: verbosity = %q, want %q", override, got, want)
		}
		_, _ = provider.OpenAIProvider.responseTextWithLimit(context.Background(), "p", "low", 100)
		if got != want {
			t.Fatalf("override %q (plain): verbosity = %q, want %q", override, got, want)
		}
	}
}
