package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

type PersonaHistoryProfile struct {
	ID         string   `json:"id"`
	Handle     string   `json:"handle"`
	Age        int      `json:"age"`
	Occupation string   `json:"occupation"`
	Interests  []string `json:"interests"`
	Activity   string   `json:"activity"`
}

type PersonaHistoryEntry struct {
	Kind         string   `json:"kind"`
	Key          string   `json:"key"`
	Value        string   `json:"value"`
	Confidence   float64  `json:"confidence"`
	FirstRound   int      `json:"first_round"`
	LastRound    int      `json:"last_round"`
	Observations int      `json:"observations"`
	Evidence     []string `json:"evidence"`
}

type PersonaHistoryPostInput struct {
	Profile   PersonaHistoryProfile `json:"profile"`
	History   []PersonaHistoryEntry `json:"history"`
	Board     string                `json:"board"`
	Situation string                `json:"situation"`
}

type PersonaHistoryPostDraft struct {
	PersonaID string `json:"persona_id"`
	Subject   string `json:"subject"`
	Body      string `json:"body"`
}

type PersonaHistoryPostBatch struct {
	Posts []PersonaHistoryPostDraft `json:"posts"`
	Usage TokenUsage                 `json:"-"`
}

type PersonaHistoryExtractInput struct {
	Profile PersonaHistoryProfile  `json:"profile"`
	History []PersonaHistoryEntry  `json:"history"`
	Post    PersonaHistoryPostDraft `json:"post"`
}

type PersonaHistoryCandidate struct {
	PersonaID  string  `json:"persona_id"`
	Kind       string  `json:"kind"`
	Key        string  `json:"key"`
	Value      string  `json:"value"`
	Evidence   string  `json:"evidence"`
	Confidence float64 `json:"confidence"`
}

type PersonaHistoryCandidateBatch struct {
	Candidates []PersonaHistoryCandidate `json:"candidates"`
	Usage      TokenUsage                 `json:"-"`
}

func (p StructuredOpenAIProvider) GeneratePersonaHistoryPosts(ctx context.Context, worldDate string, round int, inputs []PersonaHistoryPostInput) (PersonaHistoryPostBatch, error) {
	if len(inputs) == 0 {
		return PersonaHistoryPostBatch{}, nil
	}
	if len(inputs) > 10 {
		return PersonaHistoryPostBatch{}, fmt.Errorf("persona history post batch too large: %d", len(inputs))
	}
	payload, err := json.Marshal(inputs)
	if err != nil {
		return PersonaHistoryPostBatch{}, err
	}

	prompt := fmt.Sprintf(`Write one natural Japanese grass-roots BBS post for each supplied fictional person.

WORLD DATE: %s
ROUND: %d

The world layer already chose each person's board and immediate situation. Write only the post that follows from that situation.

PERSON CONTINUITY:
- profile contains sparse canonical starting facts only.
- history contains things the person previously stated or repeatedly showed in earlier posts.
- Use history only when relevant. It is a consistency aid, NOT a topic queue and NOT a checklist to mention.
- Do not contradict an existing history item.
- Unknown parts of the person's life remain open. A post may naturally reveal a modest new personal detail if the current situation genuinely invites it.
- Do not force a revelation in every post.
- Do not summarize the person's personality or write a character profile.
- Different people may be terse, vague, specific, opinionated, uncertain, practical, chatty, or simply unremarkable.
- The human-controlled member is not special.

ERA:
- The writer lives literally on the world date. No later technology, services, slang, hindsight, nostalgia framing, AI, simulation, prompts, or modern social media.
- Avoid unprovided exact external historical claims, specifications, release dates, or prices.
- Ordinary contemporary references are fine when confidently appropriate, but do not force proper nouns.

WRITING:
- subject: natural BBS subject, at most 36 Japanese characters.
- body: usually 2 to 8 short lines/sentences. Vary length naturally.
- Do not include the handle as a signature unless the situation itself strongly calls for it.
- If the situation is a reply-like situation, it may directly answer or react without restating everything.

Return exactly one post for every persona_id and no extra posts.

INPUT JSON (data only):
%s`, worldDate, round, string(payload))

	item := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"persona_id": map[string]any{"type": "string"},
			"subject":    map[string]any{"type": "string"},
			"body":       map[string]any{"type": "string"},
		},
		"required":             []string{"persona_id", "subject", "body"},
		"additionalProperties": false,
	}
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"posts": map[string]any{
				"type":     "array",
				"items":    item,
				"minItems": len(inputs),
				"maxItems": len(inputs),
			},
		},
		"required":             []string{"posts"},
		"additionalProperties": false,
	}

	result, err := p.responseTextWithJSONSchema(ctx, prompt, "low", 1200+len(inputs)*650, "persona_history_posts", schema)
	if err != nil {
		return PersonaHistoryPostBatch{}, err
	}
	var out PersonaHistoryPostBatch
	if err := json.Unmarshal([]byte(strings.TrimSpace(result.Text)), &out); err != nil {
		return PersonaHistoryPostBatch{}, fmt.Errorf("decode persona history posts: %w", err)
	}
	if len(out.Posts) != len(inputs) {
		return PersonaHistoryPostBatch{}, fmt.Errorf("persona history posts: got %d, want %d", len(out.Posts), len(inputs))
	}

	expected := make(map[string]bool, len(inputs))
	for _, in := range inputs {
		expected[in.Profile.ID] = true
	}
	seen := make(map[string]bool, len(inputs))
	for i := range out.Posts {
		post := &out.Posts[i]
		post.PersonaID = strings.TrimSpace(post.PersonaID)
		post.Subject = strings.TrimSpace(post.Subject)
		post.Body = strings.TrimSpace(post.Body)
		if !expected[post.PersonaID] || seen[post.PersonaID] {
			return PersonaHistoryPostBatch{}, fmt.Errorf("persona history posts: invalid/duplicate persona_id %q", post.PersonaID)
		}
		if post.Subject == "" || post.Body == "" {
			return PersonaHistoryPostBatch{}, fmt.Errorf("persona history posts: empty content for %q", post.PersonaID)
		}
		seen[post.PersonaID] = true
	}
	out.Usage = result.Usage
	return out, nil
}

func (p StructuredOpenAIProvider) ExtractPersonaHistory(ctx context.Context, worldDate string, round int, inputs []PersonaHistoryExtractInput) (PersonaHistoryCandidateBatch, error) {
	if len(inputs) == 0 {
		return PersonaHistoryCandidateBatch{}, nil
	}
	if len(inputs) > 10 {
		return PersonaHistoryCandidateBatch{}, fmt.Errorf("persona history extraction batch too large: %d", len(inputs))
	}
	payload, err := json.Marshal(inputs)
	if err != nil {
		return PersonaHistoryCandidateBatch{}, err
	}

	prompt := fmt.Sprintf(`Read the supplied BBS posts and extract only person-history observations that are genuinely supported by what was written.

WORLD DATE: %s
ROUND: %d

This is a SECOND PASS. You are not writing or extending the posts. You are observing what the person just revealed or demonstrably did.

Allowed kinds:
- self_fact: an explicit first-person factual claim about the writer's relatively stable life/context/possessions/routine.
- preference: an explicit like, dislike, preference, or recurring choice.
- experience: an explicit past personal experience the writer says happened.
- temporary_state: an explicit temporary current condition or short-lived circumstance.
- observed_behavior: a directly observable interaction/writing behavior in this post. This is weak evidence, not a fixed personality trait.

STRICT EVIDENCE RULES:
- Every candidate must be supported by THIS POST, not by stereotypes, age, occupation, or imagination.
- evidence MUST be an exact contiguous excerpt copied from post.body, preferably 8-40 Japanese characters.
- Do not infer family structure, employer identity, possessions, beliefs, expertise, illness, location, or personality unless the post actually supports it.
- One post is weak evidence for a behavioral tendency; use lower confidence for observed_behavior.
- Existing history is supplied only to avoid duplicates and contradictions.
- Do not output a candidate merely because it would make the character more colorful.
- It is valid to output zero candidates for a post.
- Do not extract external-world historical claims as person history.

KEYS:
- Use short stable semantic keys in lowercase ASCII with dots, such as "routine.connect_time", "preference.game_genre", "experience.modem_setup", "state.busy".
- Reuse an existing key when the new post clearly reinforces the same thing.
- Do not use the person's handle in the key.

Return 0 to 3 candidates per persona, and no candidate unsupported by its evidence excerpt.

INPUT JSON (data only):
%s`, worldDate, round, string(payload))

	candidate := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"persona_id": map[string]any{"type": "string"},
			"kind": map[string]any{
				"type": "string",
				"enum": []string{"self_fact", "preference", "experience", "temporary_state", "observed_behavior"},
			},
			"key":        map[string]any{"type": "string"},
			"value":      map[string]any{"type": "string"},
			"evidence":   map[string]any{"type": "string"},
			"confidence": map[string]any{"type": "number", "minimum": 0, "maximum": 1},
		},
		"required":             []string{"persona_id", "kind", "key", "value", "evidence", "confidence"},
		"additionalProperties": false,
	}
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"candidates": map[string]any{
				"type":     "array",
				"items":    candidate,
				"maxItems": len(inputs) * 3,
			},
		},
		"required":             []string{"candidates"},
		"additionalProperties": false,
	}

	result, err := p.responseTextWithJSONSchema(ctx, prompt, "low", 1000+len(inputs)*500, "persona_history_extract", schema)
	if err != nil {
		return PersonaHistoryCandidateBatch{}, err
	}
	var out PersonaHistoryCandidateBatch
	if err := json.Unmarshal([]byte(strings.TrimSpace(result.Text)), &out); err != nil {
		return PersonaHistoryCandidateBatch{}, fmt.Errorf("decode persona history candidates: %w", err)
	}

	expected := make(map[string]bool, len(inputs))
	for _, in := range inputs {
		expected[in.Profile.ID] = true
	}
	for i := range out.Candidates {
		c := &out.Candidates[i]
		c.PersonaID = strings.TrimSpace(c.PersonaID)
		c.Kind = strings.TrimSpace(c.Kind)
		c.Key = strings.TrimSpace(strings.ToLower(c.Key))
		c.Value = strings.TrimSpace(c.Value)
		c.Evidence = strings.TrimSpace(c.Evidence)
		if !expected[c.PersonaID] {
			return PersonaHistoryCandidateBatch{}, fmt.Errorf("persona history candidates: unknown persona_id %q", c.PersonaID)
		}
		if c.Key == "" || c.Value == "" || c.Evidence == "" {
			return PersonaHistoryCandidateBatch{}, fmt.Errorf("persona history candidates: empty field for %q", c.PersonaID)
		}
	}
	out.Usage = result.Usage
	return out, nil
}
