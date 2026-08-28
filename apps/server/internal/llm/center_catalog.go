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

	prompt := fmt.Sprintf(`Create exactly %d fictional names for independent Japanese dial-up personal BBS host stations that could plausibly appear together in one Japanese BBS telephone directory around %s.

This is historical-fiction test data. Model the population of a messy real telephone directory, not a curated list of attractive retro names. These stations were independently named by unrelated SYSOPs, so the set should look as if 100 different people chose names without coordinating with one another.

Naming goals:
- Make the naming logic highly heterogeneous. Some names may be Japanese, some English/roman letters, some mixed, some abbreviations, coined words, nicknames, hobby references, computer references, local jokes, or opaque proper-name-like strings.
- Include a few plain, awkward, nerdy, amateurish, eccentric, or slightly uncool names. Do not make every station poetic, cute, nostalgic, or aesthetically pleasing.
- Personal/hobby stations should be common. Technical, machine-oriented, radio, game, music, illustration, local-community and other hobby flavors may appear naturally, but do not make every name advertise its subject.
- Period suffixes/forms such as NET, Network, BBS, Station, Club, House, 通信, ネット, 倶楽部, ～の部屋 may occur, but many names should have no generic BBS suffix at all.
- Vary capitalization, spacing, punctuation and Japanese/Latin mixing naturally where plausible for the period.
- A small number of names may sound ambitious or polished, but most should not resemble companies, products, websites or modern brands.

Avoid corpus-wide AI patterns:
- Do NOT fill the list with poetic nature/season/time-of-day nouns followed by 通信, BBS, NET, Station, 倶楽部, or ～の部屋.
- Do NOT repeatedly use animals, stars, moon, sky, wind, dreams, sunset, flowers, cozy rooms, cafes, hideaways, or similarly sentimental motifs just to obtain variety.
- Do NOT cycle through suffixes or deliberately make adjacent entries follow a balanced pattern.
- Do NOT divide the output into obvious blocks such as Japanese names first and English names later. Mix styles irregularly throughout the list.
- Do NOT make every name semantically self-explanatory. Real directories contain names whose origin would be known only to the SYSOP or regular members.
- Do not systematically use prefecture/city names. Local references are fine only occasionally.
- Do not copy famous real BBS station names. All names must be fictional.
- Do not include telephone numbers, baud rates, descriptions, quotation marks, bullets, numbering, or commentary in names.
- Respect a 1996 knowledge/cultural ceiling. No later internet/SNS terminology or later products/culture.
- Prefer names that fit comfortably in a Japanese 80-column communications-software center list (roughly <= 30 display cells).

Before producing the final JSON, internally review the whole set as one telephone directory and remove names that make the corpus feel templated, overly tasteful, or generated from a small set of suffixes/motifs.

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
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, readErr := io.ReadAll(io.LimitReader(resp.Body, 16*1024))
		detail := strings.TrimSpace(string(b))
		if readErr != nil { return nil, fmt.Errorf("openai center catalog returned %s (read error body: %v)", resp.Status, readErr) }
		if detail == "" { return nil, fmt.Errorf("openai center catalog returned %s", resp.Status) }
		return nil, fmt.Errorf("openai center catalog returned %s: %s", resp.Status, detail)
	}
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
