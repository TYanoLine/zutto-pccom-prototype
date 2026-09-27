package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type GeminiProvider struct {
	APIKey   string
	Model    string
	Client   *http.Client
	Endpoint string
}

type geminiInteractionResponse struct {
	Status string `json:"status"`
	Steps  []struct {
		Type    string `json:"type"`
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	} `json:"steps"`
	Usage struct {
		TotalInputTokens   int `json:"total_input_tokens"`
		TotalOutputTokens  int `json:"total_output_tokens"`
		TotalThoughtTokens int `json:"total_thought_tokens"`
		TotalTokens        int `json:"total_tokens"`
	} `json:"usage"`
}

func (p GeminiProvider) GenerateBoardPost(ctx context.Context, req BoardPostRequest) (BoardPostDraft, error) {
	if strings.TrimSpace(p.APIKey) == "" {
		return BoardPostDraft{}, errors.New("GEMINI_API_KEY is not set")
	}
	model := strings.TrimSpace(p.Model)
	if model == "" {
		model = "gemini-3.8-flash"
	}
	endpoint := strings.TrimSpace(p.Endpoint)
	if endpoint == "" {
		endpoint = "https://generativelanguage.googleapis.com/v1beta/interactions"
	}
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"author":  map[string]any{"type": "string"},
			"subject": map[string]any{"type": "string"},
			"body":    map[string]any{"type": "string"},
		},
		"required": []string{"author", "subject", "body"},
	}
	payload := map[string]any{
		"model":             model,
		"input":             BuildBoardPostPrompt(req),
		"generation_config": map[string]any{"thinking_level": "low"},
		"response_format": map[string]any{
			"type": "text", "mime_type": "application/json", "schema": schema,
		},
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return BoardPostDraft{}, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(raw))
	if err != nil {
		return BoardPostDraft{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-goog-api-key", p.APIKey)
	client := p.Client
	if client == nil {
		client = &http.Client{Timeout: 90 * time.Second}
	}
	resp, err := client.Do(httpReq)
	if err != nil {
		return BoardPostDraft{}, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return BoardPostDraft{}, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return BoardPostDraft{}, fmt.Errorf("gemini interactions HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var interaction geminiInteractionResponse
	if err := json.Unmarshal(body, &interaction); err != nil {
		return BoardPostDraft{}, fmt.Errorf("decode gemini interaction: %w", err)
	}
	var texts []string
	for _, step := range interaction.Steps {
		if step.Type != "model_output" {
			continue
		}
		for _, content := range step.Content {
			if content.Type == "text" && strings.TrimSpace(content.Text) != "" {
				texts = append(texts, content.Text)
			}
		}
	}
	if len(texts) == 0 {
		return BoardPostDraft{}, fmt.Errorf("gemini interaction returned no model text (status=%s)", interaction.Status)
	}
	var draft BoardPostDraft
	if err := json.Unmarshal([]byte(strings.Join(texts, "\n")), &draft); err != nil {
		return BoardPostDraft{}, fmt.Errorf("decode gemini board post JSON: %w", err)
	}
	if req.QuoteText != "" {
		draft.Body, err = ensureExactQuote(draft.Body, req.QuoteText)
		if err != nil {
			return BoardPostDraft{}, err
		}
	}
	if err := validateBoardPostWorkerDraft(req, draft); err != nil {
		return BoardPostDraft{}, err
	}
	draft.Author = strings.ToUpper(strings.TrimSpace(draft.Author))
	draft.Subject = strings.TrimSpace(draft.Subject)
	draft.Body = normalizeCRLF(draft.Body)
	total := interaction.Usage.TotalTokens
	if total == 0 {
		total = interaction.Usage.TotalInputTokens + interaction.Usage.TotalOutputTokens + interaction.Usage.TotalThoughtTokens
	}
	draft.Usage = TokenUsage{
		InputTokens:     interaction.Usage.TotalInputTokens,
		OutputTokens:    interaction.Usage.TotalOutputTokens,
		ReasoningTokens: interaction.Usage.TotalThoughtTokens,
		TotalTokens:     total,
		Model:           model,
	}
	return draft, nil
}
