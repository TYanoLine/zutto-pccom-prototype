package llm

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

type debugTraceTestRoundTripper func(*http.Request) (*http.Response, error)

func (f debugTraceTestRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

func TestSituationTitleTraceCapturesActualPromptAndRawResponse(t *testing.T) {
	var sentPrompt, recordedPrompt, rawResponse, stage string
	calls := 0
	resultJSON := `{"titles":{"slot-1":"具体的な件名"}}`
	client := &http.Client{Transport: debugTraceTestRoundTripper(func(req *http.Request) (*http.Response, error) {
		calls++
		var request struct { Input string `json:"input"` }
		if err := json.NewDecoder(req.Body).Decode(&request); err != nil { t.Fatal(err) }
		sentPrompt = request.Input
		payload, _ := json.Marshal(map[string]any{
			"model": "gpt-test",
			"output": []any{map[string]any{"type":"message","content":[]any{map[string]any{"type":"output_text","text":resultJSON}}}},
		})
		return &http.Response{StatusCode:200, Header:http.Header{}, Body:io.NopCloser(strings.NewReader(string(payload)))}, nil
	})}
	ctx := WithDebugTrace(context.Background(), func(s, p string) func(string, error) {
		stage, recordedPrompt = s, p
		return func(result string, failure error) {
			if failure != nil { t.Errorf("unexpected provider error: %v", failure) }
			rawResponse = result
		}
	})
	provider := StructuredOpenAIProvider{OpenAIProvider:OpenAIProvider{
		Endpoint:"https://example.openai.azure.com", APIKey:"test-key", Model:"gpt-test", Client:client,
	}}
	result, err := provider.GenerateBBSSituationTitles(ctx, BBSSituationTitleRequest{
		BoardName:"GAME", WorldDate:"1996-08-26", Articles:[]BBSSituationTitleSeed{{EventID:"slot-1"}},
	})
	if err != nil { t.Fatal(err) }
	if calls != 1 || stage != "件名" || sentPrompt == "" || recordedPrompt != sentPrompt ||
		rawResponse != resultJSON || len(result.Titles) != 1 || result.Titles[0].Subject != "具体的な件名" {
		t.Fatalf("trace mismatch: calls=%d stage=%q prompt_equal=%t result=%q titles=%+v", calls, stage, recordedPrompt==sentPrompt, rawResponse, result.Titles)
	}
}

func TestDebugTraceIsInertWithoutContext(t *testing.T) {
	beginDebugTrace(context.Background(), "Situation", "private prompt")("result", nil)
}
