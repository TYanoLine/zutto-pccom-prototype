package llm

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestValidateBoardPostDraft(t *testing.T) {
	if err := validateBoardPostDraft(BoardPostDraft{Author: "NORI96", Subject: "通信ソフトの設定", Body: "最近設定をいじっています(^^;"}); err != nil {
		t.Fatalf("valid draft rejected: %v", err)
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
		APIKey: "test-key",
		Model:  "gpt-test",
		Client: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
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
