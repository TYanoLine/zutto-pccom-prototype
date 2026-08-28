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

type Researcher struct {
	APIKey string
	Model  string
	Client *http.Client
}

func (r Researcher) Research(ctx context.Context, topic, question, worldDate, priorContext string) (ResearchResult, error) {
	if r.APIKey == "" { return ResearchResult{}, errors.New("OPENAI_API_KEY is not set") }
	client := r.Client
	if client == nil { client = &http.Client{Timeout: 75 * time.Second} }
	prompt := fmt.Sprintf(`Research a historical fact or cultural context for a simulation of Japanese PC communications.
World date: %s
Topic: %s
Question: %s
Previous case context, if any:
%s

Use web search. Prefer contemporary primary sources, manuals, magazines, archives, contemporary records, then later retrospective sources. Never use later knowledge as if people on the world date already knew it. Explicitly report what could not be verified. Do not fabricate missing evidence.

Return ONLY valid JSON with this shape:
{"summary":"short Japanese summary","provisionalAnswer":"best usable Japanese provisional answer","missingInfo":["unverified point"],"confidence":0.0}
Confidence must be 0..1.`, worldDate, topic, question, priorContext)
	payload := map[string]any{
		"model": r.Model,
		"input": prompt,
		"tools": []map[string]any{{"type":"web_search","search_context_size":"medium"}},
		"text": map[string]any{"verbosity":"low"},
	}
	body, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx,http.MethodPost,"https://api.openai.com/v1/responses",bytes.NewReader(body))
	if err != nil { return ResearchResult{}, err }
	req.Header.Set("Authorization","Bearer "+r.APIKey)
	req.Header.Set("Content-Type","application/json")
	resp, err := client.Do(req)
	if err != nil { return ResearchResult{}, err }
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 { return ResearchResult{}, fmt.Errorf("openai responses API returned %s",resp.Status) }
	var decoded struct {
		Output []struct {
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
				Annotations []struct {
					Type string `json:"type"`
					URL string `json:"url"`
					Title string `json:"title"`
				} `json:"annotations"`
			} `json:"content"`
		} `json:"output"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil { return ResearchResult{}, err }
	var text string
	var sources []SourceEvidence
	seen := map[string]bool{}
	for _, out := range decoded.Output {
		for _, c := range out.Content {
			if c.Type == "output_text" && strings.TrimSpace(c.Text) != "" { text += c.Text }
			for _, a := range c.Annotations {
				if a.URL != "" && !seen[a.URL] { seen[a.URL]=true; sources=append(sources,SourceEvidence{URL:a.URL,Title:a.Title}) }
			}
		}
	}
	if text == "" { return ResearchResult{}, errors.New("no output_text in OpenAI response") }
	text = strings.TrimSpace(text)
	text = strings.TrimPrefix(text,"```json")
	text = strings.TrimPrefix(text,"```")
	text = strings.TrimSuffix(text,"```")
	var result ResearchResult
	if err := json.Unmarshal([]byte(strings.TrimSpace(text)),&result); err != nil { return ResearchResult{}, fmt.Errorf("decode research JSON: %w",err) }
	if result.Confidence < 0 { result.Confidence=0 }
	if result.Confidence > 1 { result.Confidence=1 }
	result.Sources = sources
	return result,nil
}
