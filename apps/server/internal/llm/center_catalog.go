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

type CenterName struct {
	Name string `json:"name"`
}

type CenterCatalogGenerator struct {
	APIKey string
	Model  string
	Client *http.Client
}

func (g CenterCatalogGenerator) Generate(ctx context.Context, count int, worldDate string) ([]CenterName, error) {
	if g.APIKey == "" { return nil, errors.New("OPENAI_API_KEY is not set") }
	if count <= 0 { return nil, errors.New("center count must be positive") }
	client := g.Client
	if client == nil { client = &http.Client{Timeout: 60 * time.Second} }

	prompt := fmt.Sprintf(`Create exactly %d fictional names for independent Japanese dial-up personal BBS host stations that could plausibly appear in a Japanese BBS telephone directory around %s.

This is historical-fiction test data. Names must feel like actual grass-roots Japanese PC communication stations of the mid-1990s, NOT modern websites, apps, social networks, startups, or generic AI-generated fantasy names.

Important naming distribution:
- Use a heterogeneous mixture. Some names should be Japanese, some English/roman letters, and some mixed Japanese + Latin letters.
- Natural period forms may include NET, Network, BBS, Station, Club, House, 通信, ネット, 倶楽部, 〜の部屋, or an idiosyncratic standalone name, but DO NOT force a suffix onto every name.
- Include personal/hobby/local/whimsical names as well as technical names. Small personal stations should outnumber grand corporate-sounding names.
- Avoid repetitive templates and numbered variants. Every name must be independently distinctive.
- Do not systematically use prefecture/city names. Local references are fine only occasionally.
- Do not copy famous real BBS station names. All names must be fictional.
- Do not include telephone numbers, baud rates, descriptions, quotation marks, bullets, numbering, or commentary in names.
- Respect a 1996 knowledge/cultural ceiling. No later internet/SNS terminology.
- Prefer names that fit comfortably in a Japanese 80-column communications-software center list (roughly <= 30 display cells).

Return JSON only, matching this shape exactly:
{"centers":[{"name":"..."}]}
There must be exactly %d entries and all names must be unique.`, count, worldDate, count)

	payload := map[string]any{
		"model": g.Model,
		"input": prompt,
		"reasoning": map[string]any{"effort": "medium"},
		"text": map[string]any{
			"verbosity": "low",
			"format": map[string]any{
				"type": "json_schema",
				"name": "center_catalog",
				"strict": true,
				"schema": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"centers": map[string]any{
							"type": "array", "minItems": count, "maxItems": count,
							"items": map[string]any{
								"type": "object",
								"properties": map[string]any{"name": map[string]any{"type": "string"}},
								"required": []string{"name"}, "additionalProperties": false,
							},
						},
					},
					"required": []string{"centers"}, "additionalProperties": false,
				},
			},
		},
	}
	body, err := json.Marshal(payload)
	if err != nil { return nil, err }
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.openai.com/v1/responses", bytes.NewReader(body))
	if err != nil { return nil, err }
	req.Header.Set("Authorization", "Bearer "+g.APIKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil { return nil, err }
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 { return nil, fmt.Errorf("openai center catalog returned %s", resp.Status) }
	var decoded struct { Output []struct { Content []struct { Type string `json:"type"`; Text string `json:"text"` } `json:"content"` } `json:"output"` }
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil { return nil, err }
	var text string
	for _, out := range decoded.Output { for _, c := range out.Content { if c.Type == "output_text" { text += c.Text } } }
	if strings.TrimSpace(text) == "" { return nil, errors.New("no output_text in center catalog response") }
	var result struct { Centers []CenterName `json:"centers"` }
	if err := json.Unmarshal([]byte(text), &result); err != nil { return nil, fmt.Errorf("decode center catalog: %w", err) }
	if len(result.Centers) != count { return nil, fmt.Errorf("expected %d centers, got %d", count, len(result.Centers)) }
	seen := make(map[string]struct{}, count)
	for i := range result.Centers {
		result.Centers[i].Name = strings.TrimSpace(result.Centers[i].Name)
		if result.Centers[i].Name == "" { return nil, fmt.Errorf("center %d has empty name", i) }
		if _, exists := seen[result.Centers[i].Name]; exists { return nil, fmt.Errorf("duplicate center name %q", result.Centers[i].Name) }
		seen[result.Centers[i].Name] = struct{}{}
	}
	return result.Centers, nil
}
