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

func TestRootDraftsTraceCapturesActualPromptAndRawResponse(t *testing.T) {
	var sentPrompt, recordedPrompt, rawResponse, stage string
	calls := 0
	resultJSON := `{"roots":{"slot-1":{"object_class":"game","occurrence":"playing game","post_content":"seeking tips","novelty_key":"game_tips","must_not":[],"titles":["具体的な件名"]}}}`
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
	result, err := provider.GenerateBBSRootDrafts(ctx, BBSRootDraftRequest{
		BoardName:"GAME", WorldDate:"1996-08-26", Events:[]BBSWorldWindowEvent{{EventID:"slot-1", Action:"thread_start"}},
	})
	if err != nil { t.Fatal(err) }
	if calls != 1 || stage != "Situation+件名" || sentPrompt == "" || recordedPrompt != sentPrompt ||
		rawResponse != resultJSON || len(result.Roots) != 1 || result.Roots["slot-1"].Titles[0] != "具体的な件名" {
		t.Fatalf("trace mismatch: calls=%d stage=%q prompt_equal=%t result=%q roots=%+v", calls, stage, recordedPrompt==sentPrompt, rawResponse, result.Roots)
	}
}

func TestDebugTraceIsInertWithoutContext(t *testing.T) {
	beginDebugTrace(context.Background(), "Situation+件名", "private prompt")("result", nil)
}
