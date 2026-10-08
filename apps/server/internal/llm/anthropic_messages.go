package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

const (
	// APIAnthropic selects the Anthropic Messages API exposed by Azure AI
	// Foundry for Claude deployments. The default is the OpenAI Responses API.
	APIAnthropic     = "anthropic"
	anthropicVersion = "2023-06-01"
)

func (p OpenAIProvider) useAnthropic() bool {
	return strings.EqualFold(strings.TrimSpace(p.API), APIAnthropic)
}

// anthropicMessagesURL derives the resource-level Messages endpoint from the
// configured Azure endpoint, which may be a Foundry project URL
// (.../api/projects/<name>) or an OpenAI-style base (.../openai/v1).
func anthropicMessagesURL(endpoint string) (string, error) {
	base := strings.TrimRight(strings.TrimSpace(endpoint), "/")
	if base == "" {
		return "", errors.New("AZURE_OPENAI_ENDPOINT is not set")
	}
	for _, marker := range []string{"/api/projects", "/openai", "/anthropic", "/models"} {
		if i := strings.Index(base, marker); i >= 0 {
			base = base[:i]
		}
	}
	return base + "/anthropic/v1/messages", nil
}

// anthropicMessages sends one prompt to the Messages API. When schema is
// non-nil the reply is constrained with output_config.format; bounds the API
// does not accept are stripped, so callers must keep validating the result.
func (p OpenAIProvider) anthropicMessages(ctx context.Context, prompt string, maxOutputTokens int, schemaName string, schema map[string]any) (responseTextResult, error) {
	if p.APIKey == "" {
		return responseTextResult{}, errors.New("AZURE_OPENAI_API_KEY is not set")
	}
	client := p.Client
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	if maxOutputTokens <= 0 {
		maxOutputTokens = 1200
	}
	payload := map[string]any{
		"model":      p.Model,
		"max_tokens": maxOutputTokens,
		"messages":   []map[string]any{{"role": "user", "content": prompt}},
	}
	if schema != nil {
		stripped, _ := stripUnsupportedSchemaConstraints(schema).(map[string]any)
		payload["output_config"] = map[string]any{"format": map[string]any{"type": "json_schema", "schema": stripped}}
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return responseTextResult{}, err
	}
	endpoint, err := anthropicMessagesURL(p.Endpoint)
	if err != nil {
		return responseTextResult{}, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return responseTextResult{}, err
	}
	httpReq.Header.Set("x-api-key", p.APIKey)
	httpReq.Header.Set("anthropic-version", anthropicVersion)
	httpReq.Header.Set("Content-Type", "application/json")
	resp, err := doStructuredOpenAIRequest(ctx, client, httpReq)
	if err != nil {
		return responseTextResult{}, err
	}
	defer resp.Body.Close()
	var decoded struct {
		Model   string `json:"model"`
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		StopReason string `json:"stop_reason"`
		Usage      struct {
			InputTokens          int `json:"input_tokens"`
			CacheReadInputTokens int `json:"cache_read_input_tokens"`
			OutputTokens         int `json:"output_tokens"`
			OutputTokenDetails   struct {
				ThinkingTokens int `json:"thinking_tokens"`
			} `json:"output_tokens_details"`
		} `json:"usage"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		return responseTextResult{}, err
	}
	usage := TokenUsage{
		InputTokens:       decoded.Usage.InputTokens,
		CachedInputTokens: decoded.Usage.CacheReadInputTokens,
		OutputTokens:      decoded.Usage.OutputTokens,
		ReasoningTokens:   decoded.Usage.OutputTokenDetails.ThinkingTokens,
		TotalTokens:       decoded.Usage.InputTokens + decoded.Usage.OutputTokens,
		Model:             decoded.Model,
	}
	if usage.Model == "" {
		usage.Model = p.Model
	}
	if decoded.StopReason == "max_tokens" {
		return responseTextResult{}, fmt.Errorf("Anthropic messages response hit max_tokens=%d (%s)", maxOutputTokens, schemaName)
	}
	var text strings.Builder
	for _, c := range decoded.Content {
		if c.Type == "text" {
			text.WriteString(c.Text)
		}
	}
	if strings.TrimSpace(text.String()) == "" {
		return responseTextResult{}, errors.New("no text in Anthropic messages response")
	}
	return responseTextResult{Text: text.String(), Usage: usage}, nil
}
