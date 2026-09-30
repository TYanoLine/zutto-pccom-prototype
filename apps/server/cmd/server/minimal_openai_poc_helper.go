package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"zutto-pccom/apps/server/internal/azureopenai"
)

type minimalAzureOpenAIResult struct {
	Model     string
	Text      string
	Usage     map[string]any
	LatencyMS int64
}

func callMinimalAzureOpenAIJSON(ctx context.Context, endpoint, apiKey, model, prompt, schemaName string, schema map[string]any, maxOutputTokens int) (minimalAzureOpenAIResult, error) {
	payload := map[string]any{
		"model": model,
		"input": prompt,
		"reasoning": map[string]any{"effort": "low"},
		"text": map[string]any{
			"verbosity": "low",
			"format": map[string]any{
				"type": "json_schema",
				"name": schemaName,
				"strict": true,
				"schema": schema,
			},
		},
		"max_output_tokens": maxOutputTokens,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return minimalAzureOpenAIResult{}, fmt.Errorf("encode Azure OpenAI request: %w", err)
	}
	url, err := azureopenai.URL(endpoint, "responses")
	if err != nil { return minimalAzureOpenAIResult{}, err }
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil { return minimalAzureOpenAIResult{}, fmt.Errorf("build Azure OpenAI request: %w", err) }
	if err := azureopenai.ApplyAPIKey(req, apiKey); err != nil { return minimalAzureOpenAIResult{}, err }

	started := time.Now()
	resp, err := http.DefaultClient.Do(req)
	latency := time.Since(started).Milliseconds()
	if err != nil {
		return minimalAzureOpenAIResult{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return minimalAzureOpenAIResult{}, fmt.Errorf("Azure OpenAI returned %s", resp.Status)
	}

	var decoded struct {
		Model string `json:"model"`
		Output []struct {
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
		} `json:"output"`
		Usage map[string]any `json:"usage"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		return minimalAzureOpenAIResult{}, fmt.Errorf("decode Azure OpenAI response: %w", err)
	}
	var text strings.Builder
	for _, out := range decoded.Output {
		for _, c := range out.Content {
			if c.Type == "output_text" && strings.TrimSpace(c.Text) != "" {
				text.WriteString(c.Text)
			}
		}
	}
	if strings.TrimSpace(text.String()) == "" {
		return minimalAzureOpenAIResult{}, fmt.Errorf("Azure OpenAI response contained no output_text")
	}
	return minimalAzureOpenAIResult{
		Model: decoded.Model,
		Text: strings.TrimSpace(text.String()),
		Usage: decoded.Usage,
		LatencyMS: latency,
	}, nil
}
