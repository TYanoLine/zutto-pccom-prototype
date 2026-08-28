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

type CenterName struct { Name string `json:"name"` }

type CenterCatalogGenerator struct { APIKey string; Model string; Client *http.Client }

func (g CenterCatalogGenerator) Generate(ctx context.Context, count int, worldDate string) ([]CenterName, error) {
	prompt := fmt.Sprintf(`Create exactly %d fictional names for independent Japanese dial-up personal BBS host stations that could plausibly appear together in one Japanese BBS telephone directory around %s.

This is historical-fiction test data. Model the population of a messy real telephone directory, not a curated list of attractive retro names. These stations were independently named by unrelated SYSOPs, so the set should look as if many different people chose names without coordinating with one another.

Naming goals:
- Make the naming logic highly heterogeneous. Some names may be Japanese, some English/roman letters, some mixed, some abbreviations, coined words, nicknames, hobby references, computer references, local jokes, or opaque proper-name-like strings.
- Most names should feel sincerely chosen by their SYSOP, even when ordinary, amateurish, nerdy, idiosyncratic, or unfashionable.
- Include some plain, awkward, eccentric, self-deprecating, or slightly silly names, but keep those a minority rather than the dominant flavor of the directory.
- Balance the odd names with ordinary hobby names, earnest names, neutral proper-name-like names, modest technical names, and occasional local/community names.
- Period suffixes/forms such as NET, Network, BBS, Station, Club, House, 通信, ネット, 倶楽部, ～の部屋 may occur, but many names should have no generic BBS suffix at all.
- Do not turn poetic motifs, self-deprecation, jokes, nonsense, or generic BBS suffixes into recurring templates.
- Do not cycle through suffixes or divide output into obvious Japanese/English blocks.
- Do not copy famous real BBS station names. All names must be fictional.
- Respect a 1996 knowledge/cultural ceiling. No later internet/SNS terminology or later products/culture.
- Prefer names roughly <= 30 display cells.

Return JSON only as {"centers":[{"name":"..."}]}. There must be exactly %d unique entries.`, count, worldDate, count)
	return g.generateWithPrompt(ctx, count, prompt)
}

// GenerateRegional is an intentionally exaggerated experiment: every generated
// station gets regional/local flavor. Production world generation should later
// select only a small percentage of host skeletons for this treatment.
func (g CenterCatalogGenerator) GenerateRegional(ctx context.Context, count int, worldDate string) ([]CenterName, error) {
	prompt := fmt.Sprintf(`Create exactly %d fictional names for independent Japanese dial-up personal BBS host stations around %s. This is an experiment in regional flavor: EVERY entry must be imagined as a locally rooted station somewhere in Japan.

Treat the list as a messy nationwide BBS telephone directory assembled from unrelated SYSOPs in many different regions. Spread the imagined stations irregularly across Japan: Hokkaido, Tohoku, Kanto, Koshinetsu/Hokuriku, Tokai, Kansai, Chugoku, Shikoku, Kyushu and Okinawa. Do not make the distribution mechanically even.

For every station, let its local setting influence the naming somehow, but vary how visible that influence is:
- Some may directly use a city, district, station, river, mountain, bay, island, street, neighborhood, or old local place-name.
- Some may use local dialect, a locally familiar nickname, landscape, climate, railway, port, industry, university/school-club atmosphere, shopping street, radio-club culture, or other everyday local association.
- Some should be subtle or opaque: the name may only make sense to the SYSOP and local regulars. Regional flavor does NOT mean every name contains a prefecture or city name.
- Prefer ordinary resident-level associations over tourist-brochure stereotypes. Avoid making every Hokkaido name about snow, every Okinawa name about the sea, every Kyoto name about temples, etc.
- Do not invent claims about real organizations. All stations and names are fictional; real geographic names may be used only as setting flavor.

Keep the good heterogeneity of a real personal-BBS directory:
- Mix Japanese, roman letters, abbreviations, coined words, nicknames, technical/hobby names, plain names and opaque proper-name-like names.
- Most names should feel sincerely chosen by unrelated amateur SYSOPs. A minority may be awkward, nerdy, silly, or eccentric.
- Period forms such as NET, Network, BBS, Station, Club, House, 通信, ネット, 倶楽部, ～の部屋 may occur, but do not make them a template.
- Do not produce a neat tour of Japan in geographic order. Mix regions irregularly throughout the list.
- Do not make all regional references explicit, polished, poetic, nostalgic, or cute.
- Do not copy famous real BBS station names. All station identities are fictional.
- Respect a 1996 cultural/technology ceiling. No later internet/SNS terminology or later products/culture.
- Prefer names roughly <= 30 display cells.

Before returning, review the whole list: it should feel like many local SYSOPs around Japan named their own stations independently, not like one writer created a themed 'regional Japan BBS' collection.

Return JSON only as {"centers":[{"name":"..."}]}. There must be exactly %d unique entries.`, count, worldDate, count)
	return g.generateWithPrompt(ctx, count, prompt)
}

func (g CenterCatalogGenerator) generateWithPrompt(ctx context.Context, count int, prompt string) ([]CenterName, error) {
	if g.APIKey == "" { return nil, errors.New("OPENAI_API_KEY is not set") }
	if count <= 0 { return nil, errors.New("center count must be positive") }
	client := g.Client; if client == nil { client = &http.Client{Timeout: 60 * time.Second} }
	payload := map[string]any{
		"model": g.Model, "input": prompt, "reasoning": map[string]any{"effort": "medium"},
		"text": map[string]any{"verbosity": "low", "format": map[string]any{
			"type": "json_schema", "name": "center_catalog", "strict": true,
			"schema": map[string]any{"type": "object", "properties": map[string]any{"centers": map[string]any{
				"type": "array", "minItems": count, "maxItems": count,
				"items": map[string]any{"type": "object", "properties": map[string]any{"name": map[string]any{"type": "string"}}, "required": []string{"name"}, "additionalProperties": false},
			}}, "required": []string{"centers"}, "additionalProperties": false},
		}},
	}
	body, err := json.Marshal(payload); if err != nil { return nil, err }
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.openai.com/v1/responses", bytes.NewReader(body)); if err != nil { return nil, err }
	req.Header.Set("Authorization", "Bearer "+g.APIKey); req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req); if err != nil { return nil, err }; defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, readErr := io.ReadAll(io.LimitReader(resp.Body, 16*1024)); detail := strings.TrimSpace(string(b))
		if readErr != nil { return nil, fmt.Errorf("openai center catalog returned %s (read error body: %v)", resp.Status, readErr) }
		if detail == "" { return nil, fmt.Errorf("openai center catalog returned %s", resp.Status) }
		return nil, fmt.Errorf("openai center catalog returned %s: %s", resp.Status, detail)
	}
	var decoded struct { Output []struct { Content []struct { Type string `json:"type"`; Text string `json:"text"` } `json:"content"` } `json:"output"` }
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil { return nil, err }
	var text string; for _, out := range decoded.Output { for _, c := range out.Content { if c.Type == "output_text" { text += c.Text } } }
	if strings.TrimSpace(text) == "" { return nil, errors.New("no output_text in center catalog response") }
	var result struct { Centers []CenterName `json:"centers"` }; if err := json.Unmarshal([]byte(text), &result); err != nil { return nil, fmt.Errorf("decode center catalog: %w", err) }
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
