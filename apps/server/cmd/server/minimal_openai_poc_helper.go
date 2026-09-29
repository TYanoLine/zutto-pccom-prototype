package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type minimalOpenAIResult struct {
	Model     string
	Text      string
	Usage     map[string]any
	LatencyMS int64
}

func callMinimalOpenAIJSON(ctx context.Context, apiKey, model, prompt, schemaName string, schema map[string]any, maxOutputTokens int) (minimalOpenAIResult, error) {
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
		return minimalOpenAIResult{}, fmt.Errorf("encode OpenAI request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.openai.com/v1/responses", bytes.NewReader(body))
	if err != nil {
		return minimalOpenAIResult{}, fmt.Errorf("build OpenAI request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	started := time.Now()
	resp, err := http.DefaultClient.Do(req)
	latency := time.Since(started).Milliseconds()
	if err != nil {
		return minimalOpenAIResult{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return minimalOpenAIResult{}, fmt.Errorf("openai returned %s", resp.Status)
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
		return minimalOpenAIResult{}, fmt.Errorf("decode OpenAI response: %w", err)
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
		return minimalOpenAIResult{}, fmt.Errorf("OpenAI response contained no output_text")
	}
	return minimalOpenAIResult{
		Model: decoded.Model,
		Text: strings.TrimSpace(text.String()),
		Usage: decoded.Usage,
		LatencyMS: latency,
	}, nil
}
