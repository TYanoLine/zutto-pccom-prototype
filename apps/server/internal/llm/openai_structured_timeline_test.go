package llm

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestStructuredTimelinePlannerRequestsStrictJSONSchema(t *testing.T) {
	response := `{
		"model":"gpt-test-structured",
		"output":[{"content":[{"type":"output_text","text":"{\"events\":[{\"index\":1,\"subject\":\"98の話\",\"topic\":\"自宅のPC環境\",\"motivation\":\"近況を共有したい\",\"stance\":\"気軽に書く\",\"goal\":\"自分の環境について話す\",\"facts\":[]}]}"}]}],
		"usage":{"input_tokens":100,"input_tokens_details":{"cached_tokens":0},"output_tokens":50,"output_tokens_details":{"reasoning_tokens":0},"total_tokens":150}
	}`

	var capturedPrompt string
	provider := StructuredOpenAIProvider{OpenAIProvider: OpenAIProvider{
		APIKey: "test-key",
		Model:  "gpt-test",
		Client: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			var payload map[string]any
			if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
				t.Fatalf("decode request: %v", err)
			}
			input, ok := payload["input"].(string)
			if !ok {
				t.Fatalf("input prompt missing: %#v", payload["input"])
			}
			capturedPrompt = input
			text, ok := payload["text"].(map[string]any)
			if !ok {
				t.Fatalf("text config missing: %#v", payload["text"])
			}
			format, ok := text["format"].(map[string]any)
			if !ok {
				t.Fatalf("structured format missing: %#v", text["format"])
			}
			if got := format["type"]; got != "json_schema" {
				t.Fatalf("format type=%v, want json_schema", got)
			}
			if got := format["strict"]; got != true {
				t.Fatalf("strict=%v, want true", got)
			}
			schema, ok := format["schema"].(map[string]any)
			if !ok || schema["type"] != "object" {
				t.Fatalf("schema missing or invalid: %#v", format["schema"])
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Status:     "200 OK",
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(response)),
			}, nil
		})},
	}}

	draft, err := provider.GenerateBBSTimelineIntent(context.Background(), BBSTimelineIntentRequest{
		HostName:       "TEST BBS",
		BoardID:        "2",
		BoardName:      "パソコン通信・モデム",
		WorldDate:      "1996-08-29",
		RecentBBSState: "MSG 1001 TAKA: 最近どうですか",
		Events: []BBSIntentEvent{{
			Index:          1,
			AuthorHandle:   "TAKA",
			CreatedAt:      "1996-08-20T23:00:00+09:00",
			Action:         "thread_start",
			PersonaProfile: "test persona",
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(draft.Events) != 1 || draft.Events[0].Index != 1 {
		t.Fatalf("unexpected draft: %+v", draft)
	}
	if draft.Usage.Model != "gpt-test-structured" || draft.Usage.TotalTokens != 150 {
		t.Fatalf("unexpected usage: %+v", draft.Usage)
	}

	for _, want := range []string{
		"SUBJECT-LINE CALIBRATION FROM PRESERVED PERIOD CORPORA",
		"The subject does NOT need to summarize the body",
		"Do not default to polite survey/request forms",
		"Compare the root subjects in this batch with recent supplied subjects",
		"Do NOT rotate through categories, enforce quotas",
	} {
		if !strings.Contains(capturedPrompt, want) {
			t.Fatalf("planner prompt missing historical subject calibration %q:\n%s", want, capturedPrompt)
		}
	}
}
