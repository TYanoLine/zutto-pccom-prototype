package llm

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestAnthropicMessagesURL(t *testing.T) {
	cases := map[string]string{
		"https://r.services.ai.azure.com/api/projects/p":    "https://r.services.ai.azure.com/anthropic/v1/messages",
		"https://r.services.ai.azure.com/api/projects/p/":   "https://r.services.ai.azure.com/anthropic/v1/messages",
		"https://r.openai.azure.com/openai/v1":              "https://r.openai.azure.com/anthropic/v1/messages",
		"https://r.services.ai.azure.com":                   "https://r.services.ai.azure.com/anthropic/v1/messages",
		"https://r.services.ai.azure.com/anthropic/v1":      "https://r.services.ai.azure.com/anthropic/v1/messages",
	}
	for in, want := range cases {
		got, err := anthropicMessagesURL(in)
		if err != nil || got != want {
			t.Fatalf("anthropicMessagesURL(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	if _, err := anthropicMessagesURL(" "); err == nil {
		t.Fatal("empty endpoint accepted")
	}
}

func anthropicTestProvider(t *testing.T, status int, reply string, seen *http.Request, seenBody *map[string]any) StructuredOpenAIProvider {
	t.Helper()
	return StructuredOpenAIProvider{OpenAIProvider: OpenAIProvider{
		Endpoint: "https://r.services.ai.azure.com/api/projects/p",
		APIKey:   "secret",
		Model:    "claude-test",
		API:      "Anthropic",
		Client: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			*seen = *req
			raw, _ := io.ReadAll(req.Body)
			_ = json.Unmarshal(raw, seenBody)
			return &http.Response{StatusCode: status, Status: http.StatusText(status), Header: http.Header{}, Body: io.NopCloser(strings.NewReader(reply))}, nil
		})},
	}}
}

func TestAnthropicStructuredRequestAndUsage(t *testing.T) {
	var req http.Request
	var body map[string]any
	reply := `{"model":"claude-haiku-5-5","content":[{"type":"text","text":"{\"a\":\"b\"}"}],"stop_reason":"end_turn",
		"usage":{"input_tokens":100,"cache_read_input_tokens":20,"output_tokens":30,"output_tokens_details":{"thinking_tokens":5}}}`
	p := anthropicTestProvider(t, 200, reply, &req, &body)
	schema := map[string]any{"type": "object", "properties": map[string]any{"a": map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "maxItems": 2}}}
	res, err := p.responseTextWithJSONSchema(context.Background(), "プロンプト", "low", 321, "demo", schema)
	if err != nil {
		t.Fatal(err)
	}
	if req.URL.String() != "https://r.services.ai.azure.com/anthropic/v1/messages" {
		t.Fatalf("url = %s", req.URL)
	}
	if req.Header.Get("x-api-key") != "secret" || req.Header.Get("anthropic-version") != "2023-06-01" || req.Header.Get("api-key") != "" {
		t.Fatalf("headers = %v", req.Header)
	}
	if body["model"] != "claude-test" || body["max_tokens"] != float64(642) {
		t.Fatalf("payload = %v", body)
	}
	if _, has := body["input"]; has {
		t.Fatal("responses-style input sent")
	}
	if _, has := body["text"]; has {
		t.Fatal("responses-style text block sent")
	}
	format := body["output_config"].(map[string]any)["format"].(map[string]any)
	if format["type"] != "json_schema" || strings.Contains(mustJSON(format), "maxItems") {
		t.Fatalf("output_config.format = %v", format)
	}
	if res.Text != `{"a":"b"}` {
		t.Fatalf("text = %q", res.Text)
	}
	u := res.Usage
	if u.InputTokens != 100 || u.CachedInputTokens != 20 || u.OutputTokens != 30 || u.ReasoningTokens != 5 || u.TotalTokens != 130 || u.Model != "claude-haiku-5-5" {
		t.Fatalf("usage = %+v", u)
	}
}

func TestAnthropicPlainTextOmitsOutputConfig(t *testing.T) {
	var req http.Request
	var body map[string]any
	p := anthropicTestProvider(t, 200, `{"content":[{"type":"text","text":"こんにちは"}],"stop_reason":"end_turn","usage":{}}`, &req, &body)
	res, err := p.OpenAIProvider.responseTextWithLimit(context.Background(), "p", "low", 0)
	if err != nil || res.Text != "こんにちは" {
		t.Fatalf("res=%+v err=%v", res, err)
	}
	if _, has := body["output_config"]; has || body["max_tokens"] != float64(2400) {
		t.Fatalf("payload = %v", body)
	}
}

func TestAnthropicFailureModes(t *testing.T) {
	var req http.Request
	var body map[string]any
	p := anthropicTestProvider(t, 200, `{"content":[{"type":"text","text":"{\"a\":"}],"stop_reason":"max_tokens","usage":{}}`, &req, &body)
	if _, err := p.responseTextWithJSONSchema(context.Background(), "p", "low", 10, "demo", map[string]any{"type": "object"}); err == nil || !strings.Contains(err.Error(), "max_tokens") {
		t.Fatalf("truncation not reported: %v", err)
	}
	p = anthropicTestProvider(t, 200, `{"content":[],"stop_reason":"end_turn","usage":{}}`, &req, &body)
	if _, err := p.responseTextWithJSONSchema(context.Background(), "p", "low", 10, "demo", map[string]any{"type": "object"}); err == nil {
		t.Fatal("empty content accepted")
	}
	p = anthropicTestProvider(t, 400, `{"type":"error","error":{"type":"invalid_request_error","message":"bad schema"}}`, &req, &body)
	if _, err := p.responseTextWithJSONSchema(context.Background(), "p", "low", 10, "demo", map[string]any{"type": "object"}); err == nil || !strings.Contains(err.Error(), "bad schema") {
		t.Fatalf("API error not surfaced: %v", err)
	}
	p = anthropicTestProvider(t, 200, `{}`, &req, &body)
	if _, err := p.responseTextWithJSONSchemaWebSearch(context.Background(), "p", "low", "medium", 10, "demo", map[string]any{"type": "object"}); err == nil || !strings.Contains(err.Error(), "web search") {
		t.Fatalf("web search not rejected: %v", err)
	}
}

func TestResponsesAPIRemainsDefault(t *testing.T) {
	var req http.Request
	var body map[string]any
	p := anthropicTestProvider(t, 200, `{"output":[{"content":[{"type":"output_text","text":"{}"}]}]}`, &req, &body)
	p.API = ""
	p.Endpoint = "https://r.openai.azure.com"
	if _, err := p.responseTextWithJSONSchema(context.Background(), "p", "low", 10, "demo", map[string]any{"type": "object"}); err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(req.URL.Path, "/openai/v1/responses") || req.Header.Get("api-key") != "secret" || body["input"] != "p" {
		t.Fatalf("default path changed: %s %v", req.URL, req.Header)
	}
}

func mustJSON(v any) string { b, _ := json.Marshal(v); return string(b) }

var testFence = strings.Repeat("`", 3)

func TestAnthropicTokenCeilingAndFenceStripping(t *testing.T) {
	if got := anthropicTokenCeiling(1200); got != 2400 {
		t.Fatalf("ceiling(1200) = %d", got)
	}
	if got := anthropicTokenCeiling(9000); got != 16000 {
		t.Fatalf("ceiling(9000) = %d, want capped 16000", got)
	}
	cases := []struct{ in, want string }{
		{testFence + "json\n{\"a\":1}\n" + testFence, `{"a":1}`},
		{testFence + "\n{\"a\":1}\n" + testFence + "\n", `{"a":1}`},
		{"  " + testFence + "json\n{\"a\":\"x\"}" + testFence, `{"a":"x"}`},
		{`{"a":1}`, `{"a":1}`},
	}
	for _, c := range cases {
		if got := stripCodeFence(c.in); got != c.want {
			t.Fatalf("stripCodeFence(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestAnthropicPlainTextStripsFenceButSchemaReplyIsUntouched(t *testing.T) {
	var req http.Request
	var body map[string]any
	text, _ := json.Marshal(testFence + "json\n{\"body\":\"x\"}\n" + testFence)
	reply := `{"content":[{"type":"text","text":` + string(text) + `}],"stop_reason":"end_turn","usage":{}}`
	p := anthropicTestProvider(t, 200, reply, &req, &body)
	res, err := p.OpenAIProvider.responseTextWithLimit(context.Background(), "p", "low", 100)
	if err != nil || res.Text != `{"body":"x"}` {
		t.Fatalf("plain: %q %v", res.Text, err)
	}
	res, err = p.responseTextWithJSONSchema(context.Background(), "p", "low", 100, "s", map[string]any{"type": "object"})
	if err != nil || !strings.HasPrefix(res.Text, testFence) {
		t.Fatalf("schema reply was altered: %q %v", res.Text, err)
	}
}
