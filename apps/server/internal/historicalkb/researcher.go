package historicalkb

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

type ResearchBudget struct {
	MaxToolCalls    int
	MaxOutputTokens int
	Timeout         time.Duration
	SearchContext   string
}

type Researcher struct {
	APIKey string
	Model  string
	Client *http.Client
	Budget ResearchBudget
}

func (r Researcher) Research(ctx context.Context, topic, question, worldDate, priorContext string) (ResearchResult, error) {
	if r.APIKey == "" {
		return ResearchResult{}, errors.New("OPENAI_API_KEY is not set")
	}
	b := r.Budget
	if b.MaxToolCalls <= 0 {
		b.MaxToolCalls = 4
	}
	if b.MaxOutputTokens <= 0 {
		b.MaxOutputTokens = 1800
	}
	if b.Timeout <= 0 {
		b.Timeout = 60 * time.Second
	}
	if b.SearchContext == "" {
		b.SearchContext = "medium"
	}
	client := r.Client
	if client == nil {
		client = &http.Client{Timeout: b.Timeout}
	}
	prompt := fmt.Sprintf(`You are the bounded historical-research sub-agent for a simulation of Japanese PC communications.
World date: %s
Topic: %s
Question: %s
Previous case context, if any:
%s

Research only the concrete fact needed by the caller; do not expand into a general essay. Use web search only when evidence is needed. Prefer contemporary primary sources, manuals, magazines, archives, advertisements and contemporary records, then later retrospective sources. Never use later knowledge as if people on the world date already knew it. Distinguish announcement, release, availability and later retrospective claims. Explicitly report what could not be verified. Do not fabricate missing evidence. Stop when the requested fact is adequately supported or the tool budget is exhausted.

Populate the requested structured result. confidence must be 0..1.`, worldDate, topic, question, priorContext)
	payload := map[string]any{
		"model":             r.Model,
		"input":             prompt,
		"tools":             []map[string]any{{"type": "web_search", "search_context_size": b.SearchContext}},
		"max_tool_calls":    b.MaxToolCalls,
		"max_output_tokens": b.MaxOutputTokens,
		"text":              researchResultTextConfig(),
	}
	body, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.openai.com/v1/responses", bytes.NewReader(body))
	if err != nil {
		return ResearchResult{}, err
	}
	req.Header.Set("Authorization", "Bearer "+r.APIKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return ResearchResult{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return ResearchResult{}, fmt.Errorf("openai responses API returned %s", resp.Status)
	}
	var decoded struct {
		Status            string `json:"status"`
		IncompleteDetails *struct {
			Reason string `json:"reason"`
		} `json:"incomplete_details"`
		Output []struct {
			Content []struct {
				Type        string `json:"type"`
				Text        string `json:"text"`
				Annotations []struct {
					Type  string `json:"type"`
					URL   string `json:"url"`
					Title string `json:"title"`
				} `json:"annotations"`
			} `json:"content"`
		} `json:"output"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		return ResearchResult{}, err
	}
	if decoded.Status == "incomplete" {
		reason := "unknown"
		if decoded.IncompleteDetails != nil && strings.TrimSpace(decoded.IncompleteDetails.Reason) != "" {
			reason = strings.TrimSpace(decoded.IncompleteDetails.Reason)
		}
		return ResearchResult{}, fmt.Errorf("openai research response incomplete: %s", reason)
	}
	var text string
	var sources []SourceEvidence
	seen := map[string]bool{}
	for _, out := range decoded.Output {
		for _, c := range out.Content {
			if c.Type == "output_text" && strings.TrimSpace(c.Text) != "" {
				text += c.Text
			}
			for _, a := range c.Annotations {
				if a.URL != "" && !seen[a.URL] {
					seen[a.URL] = true
					sources = append(sources, SourceEvidence{URL: a.URL, Title: a.Title})
				}
			}
		}
	}
	if text == "" {
		return ResearchResult{}, errors.New("no output_text in OpenAI response")
	}
	var result ResearchResult
	if err := json.Unmarshal([]byte(strings.TrimSpace(text)), &result); err != nil {
		return ResearchResult{}, fmt.Errorf("decode structured research JSON: %w", err)
	}
	if result.Confidence < 0 {
		result.Confidence = 0
	}
	if result.Confidence > 1 {
		result.Confidence = 1
	}
	result.Sources = sources
	return result, nil
}

func researchResultTextConfig() map[string]any {
	return map[string]any{
		"verbosity": "low",
		"format": map[string]any{
			"type":        "json_schema",
			"name":        "historical_research_result",
			"description": "Bounded historical research result for a persistent world fact.",
			"strict":      true,
			"schema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"summary":           map[string]any{"type": "string"},
					"provisionalAnswer": map[string]any{"type": "string"},
					"missingInfo": map[string]any{
						"type":  "array",
						"items": map[string]any{"type": "string"},
					},
					"confidence": map[string]any{"type": "number"},
				},
				"required":             []string{"summary", "provisionalAnswer", "missingInfo", "confidence"},
				"additionalProperties": false,
			},
		},
	}
}
