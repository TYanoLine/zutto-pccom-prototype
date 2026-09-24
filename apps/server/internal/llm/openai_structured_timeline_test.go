package llm

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestStructuredTimelinePlannerRequestsStrictJSONSchema(t *testing.T) {
	response := `{
		"model":"gpt-test-structured",
		"output":[{"content":[{"type":"output_text","text":"{\"events\":[{\"index\":1,\"subject\":\"途中で切れた\",\"topic\":\"通信中の突然の切断\",\"motivation\":\"通信中に切断が起きたため\",\"stance\":\"状況を短く記す\",\"goal\":\"切断について共有する\",\"facts\":[]}]}"}]}],
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
		EraRules:       DiegeticWorldFrame,
		RecentBBSState: "MSG 1001 TAKA: 設定変えました",
		Events: []BBSIntentEvent{{
			Index:          1,
			AuthorHandle:   "TAKA",
			CreatedAt:      "1996-08-20T23:00:00+09:00",
			Action:         "thread_start",
			AnchorKey:      "communications",
			CauseKind:      "recent_salience",
			CauseSummary:   `The world layer selected broad interest domain "communications" as routing context; ordinary participation itself is not the event.`,
			PersonaProfile: "everyday_baseline=[自宅のパソコンと通信環境は日常の道具]; interests=[communications=0.44]",
			ExistingFacts:  []string{"BACKGROUND ONLY: offline_meeting.preference=前向き"},
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
		"CRITICAL CAUSAL BOUNDARY",
		"anchor_key is INTERNAL ROUTING METADATA",
		"DIEGETIC PRESENT / ERA NORMALITY",
		"Ordinary baseline conditions stay implicit",
		"everyday_baseline is deliberately ordinary and normally UNMENTIONED",
		"A recent_salience event does NOT authorize you to invent a purchase",
		"Do not use retrospective shortcuts such as",
		"〜からでも入れた",
		"Do not treat the list as a menu of possible subjects",
		`"anchor_key":"communications"`,
		`"cause_kind":"recent_salience"`,
		"SUBJECT-LINE CALIBRATION FROM PRESERVED PERIOD CORPORA",
		"The subject does NOT need to summarize the body",
		"Do not default to polite survey/request forms",
		"Compare the root subjects in this batch with recent supplied subjects",
		"Do NOT rotate through categories, enforce quotas",
	} {
		if !strings.Contains(capturedPrompt, want) {
			t.Fatalf("planner prompt missing diegetic/causal/subject calibration %q:\n%s", want, capturedPrompt)
		}
	}
}

func TestStructuredOpenAIReasoningEffortIsOptionalAndExplicit(t *testing.T) {
	var gotReasoning any
	provider := StructuredOpenAIProvider{OpenAIProvider: OpenAIProvider{
		APIKey: "test-key",
		Model:  "gpt-test",
		Client: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			var payload map[string]any
			if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
				t.Fatalf("decode request: %v", err)
			}
			gotReasoning = payload["reasoning"]
			return &http.Response{
				StatusCode: http.StatusOK,
				Status:     "200 OK",
				Header:     make(http.Header),
				Body: io.NopCloser(strings.NewReader(`{
					"model":"gpt-test",
					"output":[{"content":[{"type":"output_text","text":"{}"}]}],
					"usage":{"input_tokens":1,"input_tokens_details":{"cached_tokens":0},"output_tokens":1,"output_tokens_details":{"reasoning_tokens":0},"total_tokens":2}
				}`)),
			}, nil
		})},
	}}

	_, err := provider.responseTextWithJSONSchemaReasoning(context.Background(), "test", "low", "low", 100, "test_schema", map[string]any{
		"type": "object", "properties": map[string]any{}, "additionalProperties": false,
	})
	if err != nil {
		t.Fatal(err)
	}
	reasoning, ok := gotReasoning.(map[string]any)
	if !ok || reasoning["effort"] != "low" {
		t.Fatalf("reasoning=%#v, want effort=low", gotReasoning)
	}
}

func TestStructuredOpenAIRetriesTransient429(t *testing.T) {
	attempts := 0
	provider := StructuredOpenAIProvider{OpenAIProvider: OpenAIProvider{
		APIKey: "test-key",
		Model:  "gpt-test",
		Client: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			attempts++
			if attempts == 1 {
				return &http.Response{
					StatusCode: http.StatusTooManyRequests,
					Status:     "429 Too Many Requests",
					Header:     http.Header{"Retry-After": []string{"0"}},
					Body:       io.NopCloser(strings.NewReader(`{"error":{"type":"rate_limit_exceeded"}}`)),
				}, nil
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Status:     "200 OK",
				Header:     make(http.Header),
				Body: io.NopCloser(strings.NewReader(`{
					"model":"gpt-test",
					"output":[{"content":[{"type":"output_text","text":"{}"}]}],
					"usage":{"input_tokens":1,"input_tokens_details":{"cached_tokens":0},"output_tokens":1,"output_tokens_details":{"reasoning_tokens":0},"total_tokens":2}
				}`)),
			}, nil
		})},
	}}
	result, err := provider.responseTextWithJSONSchema(context.Background(), "test", "low", 100, "test_schema", map[string]any{
		"type": "object",
		"properties": map[string]any{},
		"additionalProperties": false,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Text != "{}" {
		t.Fatalf("unexpected result: %+v", result)
	}
	if attempts != 2 {
		t.Fatalf("attempts=%d, want 2", attempts)
	}
}

func resetStructuredOpenAIGateForTest() {
	sharedStructuredOpenAIGate.mu.Lock()
	sharedStructuredOpenAIGate.notBefore = time.Time{}
	sharedStructuredOpenAIGate.mu.Unlock()
}

func TestStructuredOpenAIQuota429DoesNotRetry(t *testing.T) {
	resetStructuredOpenAIGateForTest()
	t.Cleanup(resetStructuredOpenAIGateForTest)

	attempts := 0
	provider := StructuredOpenAIProvider{OpenAIProvider: OpenAIProvider{
		APIKey: "test-key",
		Model:  "gpt-test",
		Client: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			attempts++
			header := make(http.Header)
			header.Set("x-request-id", "req_quota_test")
			return &http.Response{
				StatusCode: http.StatusTooManyRequests,
				Status:     "429 Too Many Requests",
				Header:     header,
				Body: io.NopCloser(strings.NewReader(`{"error":{"message":"quota exhausted","type":"insufficient_quota","code":"credit_balance_exhausted"}}`)),
			}, nil
		})},
	}}

	_, err := provider.responseTextWithJSONSchema(context.Background(), "test", "low", 100, "test_schema", map[string]any{
		"type": "object", "properties": map[string]any{}, "additionalProperties": false,
	})
	if err == nil {
		t.Fatal("quota 429 unexpectedly succeeded")
	}
	if attempts != 1 {
		t.Fatalf("quota 429 attempts=%d, want 1", attempts)
	}
	for _, want := range []string{"type=insufficient_quota", "code=credit_balance_exhausted", "request_id=req_quota_test"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("quota diagnostic missing %q: %v", want, err)
		}
	}
}

func TestStructuredOpenAIRate429HonorsResetHeaderAndRetries(t *testing.T) {
	resetStructuredOpenAIGateForTest()
	t.Cleanup(resetStructuredOpenAIGateForTest)

	attempts := 0
	start := time.Now()
	provider := StructuredOpenAIProvider{OpenAIProvider: OpenAIProvider{
		APIKey: "test-key",
		Model:  "gpt-test",
		Client: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			attempts++
			if attempts == 1 {
				header := make(http.Header)
				header.Set("x-ratelimit-reset-requests", "600ms")
				header.Set("x-ratelimit-remaining-requests", "0")
				return &http.Response{
					StatusCode: http.StatusTooManyRequests,
					Status:     "429 Too Many Requests",
					Header:     header,
					Body: io.NopCloser(strings.NewReader(`{"error":{"message":"rate limited","type":"rate_limit_exceeded","code":"rate_limit_exceeded"}}`)),
				}, nil
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Status:     "200 OK",
				Header:     make(http.Header),
				Body: io.NopCloser(strings.NewReader(`{
					"model":"gpt-test",
					"output":[{"content":[{"type":"output_text","text":"{}"}]}],
					"usage":{"input_tokens":1,"input_tokens_details":{"cached_tokens":0},"output_tokens":1,"output_tokens_details":{"reasoning_tokens":0},"total_tokens":2}
				}`)),
			}, nil
		})},
	}}

	result, err := provider.responseTextWithJSONSchema(context.Background(), "test", "low", 100, "test_schema", map[string]any{
		"type": "object", "properties": map[string]any{}, "additionalProperties": false,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Text != "{}" {
		t.Fatalf("unexpected result: %+v", result)
	}
	if attempts != 2 {
		t.Fatalf("attempts=%d, want 2", attempts)
	}
	if elapsed := time.Since(start); elapsed < 550*time.Millisecond {
		t.Fatalf("retry ignored rate reset header: elapsed=%s", elapsed)
	}
}
