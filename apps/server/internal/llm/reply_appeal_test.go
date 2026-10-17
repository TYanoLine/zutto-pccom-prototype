package llm

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"zutto-pccom/apps/server/internal/world"
)

func TestReplyAppealJudgeSuccess(t *testing.T) {
	responseJSON := `{"evaluations":[{"id":101,"appeal":4,"reason":"質問スレッドでトラブル解決を求めているため"},{"id":102,"appeal":1,"reason":"自己完結した日記のため"}]}`
	apiResponse := `{
		"model":"gpt-test-structured",
		"output":[{"content":[{"type":"output_text","text":` + string(mustJSON(responseJSON)) + `}]}],
		"usage":{"input_tokens":100,"output_tokens":50,"total_tokens":150}
	}`

	var capturedFormat map[string]any
	provider := StructuredOpenAIProvider{OpenAIProvider: OpenAIProvider{
		Endpoint: "https://test.openai.azure.com",
		APIKey:   "test-key",
		Model:    "gpt-test",
		Client: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			var payload map[string]any
			if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
				t.Fatalf("decode request: %v", err)
			}
			text := payload["text"].(map[string]any)
			capturedFormat = text["format"].(map[string]any)
			return &http.Response{
				StatusCode: http.StatusOK,
				Status:     "200 OK",
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(apiResponse)),
			}, nil
		})},
	}}

	req := world.ReplyAppealRequest{
		Host:     "博多キャナルネット",
		Board:    "パソコン・技術",
		BaseRate: 1.5,
		Members:  250,
		Roots: []world.AppealRoot{
			{ID: 101, Subject: "SCSIカードが認識しない", Summary: "PC-9821でSCSIボードのディップスイッチ設定に悩む"},
			{ID: 102, Subject: "今日のお昼ご飯", Summary: "天神でラーメンを食べた話"},
		},
		PostedAt: map[int64]time.Time{
			101: time.Date(1996, 8, 26, 23, 15, 0, 0, time.Local),
			102: time.Date(1996, 8, 26, 12, 30, 0, 0, time.Local),
		},
		ActiveByRoot: map[int64]int{
			101: 25,
			102: 8,
		},
		RecentCounts: []int{2, 0, 3, 1},
	}

	result, err := provider.JudgeReplyAppeal(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Appeal[101] != 4 {
		t.Fatalf("expected post 101 appeal == 4, got %d", result.Appeal[101])
	}
	if result.Appeal[102] != 1 {
		t.Fatalf("expected post 102 appeal == 1, got %d", result.Appeal[102])
	}
	if result.Reason[101] != "質問スレッドでトラブル解決を求めているため" {
		t.Fatalf("unexpected reason: %s", result.Reason[101])
	}

	// Schema check
	if capturedFormat["type"] != "json_schema" {
		t.Fatalf("expected format type json_schema, got %v", capturedFormat["type"])
	}
	if capturedFormat["name"] != "bbs_reply_appeal" {
		t.Fatalf("expected schema name bbs_reply_appeal, got %v", capturedFormat["name"])
	}
}

func TestReplyAppealJudgeInvalidAndMissingOutputFallsBackToTwo(t *testing.T) {
	// Root 101 has out-of-range appeal 99, Root 102 is omitted from response
	responseJSON := `{"evaluations":[{"id":101,"appeal":99,"reason":"不正な値"}]}`
	apiResponse := `{
		"model":"gpt-test-structured",
		"output":[{"content":[{"type":"output_text","text":` + string(mustJSON(responseJSON)) + `}]}],
		"usage":{"input_tokens":100,"output_tokens":50,"total_tokens":150}
	}`

	provider := StructuredOpenAIProvider{OpenAIProvider: OpenAIProvider{
		Endpoint: "https://test.openai.azure.com",
		APIKey:   "test-key",
		Model:    "gpt-test",
		Client: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Status:     "200 OK",
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(apiResponse)),
			}, nil
		})},
	}}

	req := world.ReplyAppealRequest{
		Host:     "テスト局",
		Board:    "雑談",
		BaseRate: 1.0,
		Roots: []world.AppealRoot{
			{ID: 101, Subject: "テスト1"},
			{ID: 102, Subject: "テスト2"},
		},
	}

	result, err := provider.JudgeReplyAppeal(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Appeal[101] != 2 {
		t.Fatalf("expected out-of-range post 101 to fall back to 2, got %d", result.Appeal[101])
	}
	if result.Appeal[102] != 2 {
		t.Fatalf("expected omitted post 102 to fall back to 2, got %d", result.Appeal[102])
	}
}

func TestReplyAppealJudgeTimeoutAndError(t *testing.T) {
	provider := StructuredOpenAIProvider{OpenAIProvider: OpenAIProvider{
		Endpoint: "https://test.openai.azure.com",
		APIKey:   "test-key",
		Model:    "gpt-test",
		Client: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return nil, errors.New("timeout or network failure")
		})},
	}}

	req := world.ReplyAppealRequest{
		Host:  "テスト局",
		Board: "雑談",
		Roots: []world.AppealRoot{
			{ID: 201, Subject: "テスト"},
		},
	}

	result, err := provider.JudgeReplyAppeal(context.Background(), req)
	if err == nil {
		t.Fatal("expected error on transport failure")
	}
	// Fallback should still be populated
	if result.Appeal[201] != 2 {
		t.Fatalf("expected fallback appeal 2 on error, got %d", result.Appeal[201])
	}
}
