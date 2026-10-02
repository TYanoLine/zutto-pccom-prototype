package llm

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

// Assert that the computed ceiling reaches Azure's actual max_output_tokens
// request field, rather than only testing an unused budgeting helper.
func TestBBSWorldSituationSendsExpandedCompletionBudget(t *testing.T) {
	for _, tc := range []struct {
		events int
		want   int
	}{
		{1, 4000},
		{4, 8000},
		{6, 12000},
	} {
		t.Run(string(rune('0'+tc.events)), func(t *testing.T) {
			events := make([]BBSWorldWindowEvent, 0, tc.events)
			wire := bbsWorldSituationProposalWire{Situations: make(map[string]bbsWorldSituationWire)}
			for i := 0; i < tc.events; i++ {
				id := string(rune('a' + i))
				events = append(events, BBSWorldWindowEvent{
					EventID: id, Action: "thread_start",
					DiscourseMode: "share_observation", BoardName: "GAME",
				})
				wire.Situations[id] = bbsWorldSituationWire{
					ObjectClass: "game", Occurrence: "a concrete event",
					Observation: "what happened", NoveltyKey: id,
				}
			}
			generated, err := json.Marshal(wire)
			if err != nil { t.Fatal(err) }
			apiResponse, err := json.Marshal(map[string]any{
				"model": "test-deployment",
				"output": []any{map[string]any{"type": "message", "content": []any{
					map[string]any{"type": "output_text", "text": string(generated)},
				}}},
				"usage": map[string]any{
					"input_tokens": 40, "output_tokens": 90, "total_tokens": 130,
					"output_tokens_details": map[string]any{"reasoning_tokens": 0},
				},
			})
			if err != nil { t.Fatal(err) }
			gotBudget := 0
			provider := StructuredOpenAIProvider{OpenAIProvider: OpenAIProvider{
				Endpoint: "https://test.openai.azure.com", APIKey: "test-key", Model: "test-deployment",
				Client: &http.Client{Transport: generationContextRoundTripFunc(func(req *http.Request) (*http.Response, error) {
					var payload struct { MaxOutputTokens int `json:"max_output_tokens"` }
					if err := json.NewDecoder(req.Body).Decode(&payload); err != nil { t.Fatalf("decode payload: %v", err) }
					gotBudget = payload.MaxOutputTokens
					return &http.Response{
						StatusCode: http.StatusOK,
						Header: make(http.Header),
						Body: io.NopCloser(strings.NewReader(string(apiResponse))),
					}, nil
				})},
			}}
			draft, err := provider.GenerateBBSWorldSituationProposals(context.Background(), BBSWorldSituationProposalRequest{Events: events})
			if err != nil { t.Fatal(err) }
			if gotBudget != tc.want || len(draft.Situations) != tc.events {
				t.Fatalf("events=%d: sent max_output_tokens=%d want=%d, situations=%d", tc.events, gotBudget, tc.want, len(draft.Situations))
			}
		})
	}
}
