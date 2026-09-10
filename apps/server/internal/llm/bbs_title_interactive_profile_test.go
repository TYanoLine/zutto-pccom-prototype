package llm

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

func interactiveProfileProvider(t *testing.T, outputText string, captured *map[string]any) StructuredOpenAIProvider {
	t.Helper()
	outer, err := json.Marshal(map[string]any{
		"model":  "gpt-5.6-luna-test",
		"output": []any{map[string]any{"content": []any{map[string]any{"type": "output_text", "text": outputText}}}},
		"usage":  map[string]any{"input_tokens": 10, "output_tokens": 10, "total_tokens": 20},
	})
	if err != nil {
		t.Fatal(err)
	}
	return StructuredOpenAIProvider{OpenAIProvider: OpenAIProvider{
		APIKey: "test-key",
		Model:  "gpt-5.6-luna",
		Client: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			var payload map[string]any
			if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
				t.Fatalf("decode request: %v", err)
			}
			*captured = payload
			return &http.Response{
				StatusCode: http.StatusOK,
				Status:     "200 OK",
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(string(outer))),
			}, nil
		})},
	}}
}

func capturedReasoningEffort(payload map[string]any) string {
	reasoning, _ := payload["reasoning"].(map[string]any)
	effort, _ := reasoning["effort"].(string)
	return effort
}

func TestInteractiveTitleMethodsPinLatencyReasoningProfile(t *testing.T) {
	t.Run("candidate generation uses none", func(t *testing.T) {
		titles := make([]string, 20)
		for i := range titles {
			titles[i] = "候補タイトル"
		}
		output, _ := json.Marshal(map[string]any{"titles": titles})
		captured := map[string]any{}
		provider := interactiveProfileProvider(t, string(output), &captured)
		if _, err := provider.GenerateInteractiveBBSTitleCandidates(context.Background(), "1996-08-29", "ゲーム"); err != nil {
			t.Fatal(err)
		}
		if got := capturedReasoningEffort(captured); got != "none" {
			t.Fatalf("reasoning effort=%q, want none", got)
		}
	})

	t.Run("era routing uses none", func(t *testing.T) {
		output, _ := json.Marshal(map[string]any{"decisions": []any{map[string]any{"candidate": 1, "status": BBSTitleEraOK, "reason": "時点依存なし"}}})
		captured := map[string]any{}
		provider := interactiveProfileProvider(t, string(output), &captured)
		if _, err := provider.ValidateInteractiveBBSTitleEra(context.Background(), BBSTitleEraRequest{WorldDate: "1996-08-29", BoardName: "ゲーム", Titles: []string{"最近なに遊んでます？"}}); err != nil {
			t.Fatal(err)
		}
		if got := capturedReasoningEffort(captured); got != "none" {
			t.Fatalf("reasoning effort=%q, want none", got)
		}
	})

	t.Run("slot review uses low", func(t *testing.T) {
		output, _ := json.Marshal(map[string]any{"decisions": []any{map[string]any{
			"candidate": 1, "event_id": "", "subject": "", "reason": "今回は枠に合わない", "summary": "", "details": []string{},
		}}})
		captured := map[string]any{}
		provider := interactiveProfileProvider(t, string(output), &captured)
		if _, err := provider.ReviewInteractiveBBSTitleCandidates(context.Background(), BBSTitleReviewRequest{BoardName: "ゲーム", Titles: []string{"最近なに遊んでます？"}}); err != nil {
			t.Fatal(err)
		}
		if got := capturedReasoningEffort(captured); got != "low" {
			t.Fatalf("reasoning effort=%q, want low", got)
		}
	})
}

func TestNormalTitleGenerationDoesNotForceInteractiveReasoningProfile(t *testing.T) {
	titles := make([]string, 20)
	for i := range titles {
		titles[i] = "候補タイトル"
	}
	output, _ := json.Marshal(map[string]any{"titles": titles})
	captured := map[string]any{}
	provider := interactiveProfileProvider(t, string(output), &captured)
	if _, err := provider.GenerateBBSTitleCandidates(context.Background(), "1996-08-29", "ゲーム"); err != nil {
		t.Fatal(err)
	}
	if _, exists := captured["reasoning"]; exists {
		t.Fatalf("normal/Lab title generation unexpectedly pinned interactive reasoning: %+v", captured["reasoning"])
	}
}
