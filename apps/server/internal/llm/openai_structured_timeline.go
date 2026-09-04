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

// StructuredOpenAIProvider keeps the normal OpenAIProvider behavior for prose
// rendering, but requires strict JSON Schema output for BBS semantic planning.
// This prevents malformed planner JSON from aborting an otherwise valid atomic
// timeline plan. The world layer still validates all returned semantics before
// anything becomes canonical world state.
type StructuredOpenAIProvider struct {
	OpenAIProvider
}

var _ BoardPostRenderer = StructuredOpenAIProvider{}
var _ BBSTimelineIntentPlanner = StructuredOpenAIProvider{}

func (p StructuredOpenAIProvider) GenerateBBSTimelineIntent(ctx context.Context, req BBSTimelineIntentRequest) (BBSTimelineIntentDraft, error) {
	eventsJSON, err := json.Marshal(req.Events)
	if err != nil {
		return BBSTimelineIntentDraft{}, err
	}
	recent := strings.TrimSpace(req.RecentBBSState)
	if recent == "" {
		recent = "(no earlier materialized BBS state supplied)"
	}
	prompt := fmt.Sprintf(`Propose semantic content for a bounded sequence of events in a fictional Japanese grass-roots BBS world.

The WORLD LAYER has already decided every event's actor, time, and whether it is a root post or reply. You MUST NOT change those facts. You are proposing content semantics only; the application validates and commits accepted results as world state.

There is intentionally NO fixed topic list, subject template bank, information-slot checklist, or response-act menu. Infer each concrete post from the board, the persistent persona, earlier BBS state, and the sequence itself.

SUBJECT-LINE CALIBRATION FROM PRESERVED PERIOD CORPORA:
%s

WORLD / BOARD:
- world date: %s
- host: %s
- region: %s
- host software family: %s
- board id: %s
- board name: %s
- era rules: %s

EARLIER BBS STATE:
%s

WORLD-SELECTED EVENT SHELLS (JSON):
%s

Rules for each event:
- Preserve index, author, timestamp, action, parent topology, and canonical_subject from the event shell.
- For a root post, follow the subject-line calibration above. The subject must come from this actor and this event, not from a canned list or generic headline-writing habit.
- For a reply, subject may simply follow the canonical reply subject supplied by the world layer; content should naturally continue the existing thread rather than starting an unrelated topic.
- topic is a short free-form semantic summary, not an enum or catalog key.
- motivation, stance, and goal are free-form descriptions of this exact event. goal should say what the actor is trying to communicate or react to; it does not have to advance the conversation.
- facts contains zero to three durable FICTIONAL PERSONAL facts that genuinely need to become concrete for this post. Do not manufacture a fact merely to add color. A short reaction may have zero facts.
- Each fact key is an open lowercase semantic path such as household.shared_computer or computer.communication_usage. It is NOT chosen from a predefined schema. Use the same key when the same personal dimension already appears in existing_facts.
- If existing_facts contains a key, never contradict its value. Prefer reusing it when relevant.
- Facts may describe this fictional person's ordinary ownership, habits, preferences, household situation, or experience. They must NOT assert unprovided real-world hardware limits, exact product specifications, release dates, prices, historical events, or other external historical facts.
- Do not invent a question merely to keep the thread alive. Ask only if that is naturally the actor's actual goal.
- Repetition, silence-like brevity, disagreement, or a mundane tangent can be natural. Do not optimize every post for information density.
- The human-controlled member is not special and need not be mentioned.
- Never mention AI, simulation, prompts, databases, web searches, social media, smartphones, or anything after the world date.
- Keep the sequence mutually coherent: later events can react to earlier proposed semantics, but do not make every event mechanically answer the previous one.
- Return exactly one event object for every supplied event index.`, historicalBBSSubjectCalibration, req.WorldDate, req.HostName, req.HostRegion, req.HostSoftware, req.BoardID, req.BoardName, req.EraRules, recent, string(eventsJSON))

	maxTokens := 1800 + len(req.Events)*220
	if maxTokens > 6000 {
		maxTokens = 6000
	}
	result, err := p.responseTextWithJSONSchema(ctx, prompt, "low", maxTokens, "bbs_timeline_intent", bbsTimelineIntentSchema())
	if err != nil {
		return BBSTimelineIntentDraft{}, err
	}
	var draft BBSTimelineIntentDraft
	if err := json.Unmarshal([]byte(strings.TrimSpace(result.Text)), &draft); err != nil {
		return BBSTimelineIntentDraft{}, fmt.Errorf("decode structured BBS timeline intent JSON: %w", err)
	}
	if err := validateBBSTimelineIntentDraft(req, draft); err != nil {
		return BBSTimelineIntentDraft{}, err
	}
	for i := range draft.Events {
		draft.Events[i].Subject = strings.TrimSpace(draft.Events[i].Subject)
		draft.Events[i].Topic = strings.TrimSpace(draft.Events[i].Topic)
		draft.Events[i].Motivation = strings.TrimSpace(draft.Events[i].Motivation)
		draft.Events[i].Stance = strings.TrimSpace(draft.Events[i].Stance)
		draft.Events[i].Goal = strings.TrimSpace(draft.Events[i].Goal)
		for j := range draft.Events[i].Facts {
			draft.Events[i].Facts[j].Key = strings.TrimSpace(strings.ToLower(draft.Events[i].Facts[j].Key))
			draft.Events[i].Facts[j].Value = strings.TrimSpace(draft.Events[i].Facts[j].Value)
		}
	}
	draft.Usage = result.Usage
	return draft, nil
}

func bbsTimelineIntentSchema() map[string]any {
	fact := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"key":   map[string]any{"type": "string"},
			"value": map[string]any{"type": "string"},
		},
		"required":             []string{"key", "value"},
		"additionalProperties": false,
	}
	event := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"index":      map[string]any{"type": "integer"},
			"subject":    map[string]any{"type": "string"},
			"topic":      map[string]any{"type": "string"},
			"motivation": map[string]any{"type": "string"},
			"stance":     map[string]any{"type": "string"},
			"goal":       map[string]any{"type": "string"},
			"facts":      map[string]any{"type": "array", "items": fact},
		},
		"required":             []string{"index", "subject", "topic", "motivation", "stance", "goal", "facts"},
		"additionalProperties": false,
	}
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"events": map[string]any{"type": "array", "items": event},
		},
		"required":             []string{"events"},
		"additionalProperties": false,
	}
}

func (p StructuredOpenAIProvider) responseTextWithJSONSchema(ctx context.Context, prompt, verbosity string, maxOutputTokens int, schemaName string, schema map[string]any) (responseTextResult, error) {
	if p.APIKey == "" {
		return responseTextResult{}, errors.New("OPENAI_API_KEY is not set")
	}
	client := p.Client
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	if maxOutputTokens <= 0 {
		maxOutputTokens = 1200
	}
	payload := map[string]any{
		"model": p.Model,
		"input": prompt,
		"text": map[string]any{
			"verbosity": verbosity,
			"format": map[string]any{
				"type":   "json_schema",
				"name":   schemaName,
				"strict": true,
				"schema": schema,
			},
		},
		"max_output_tokens": maxOutputTokens,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return responseTextResult{}, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.openai.com/v1/responses", bytes.NewReader(body))
	if err != nil {
		return responseTextResult{}, err
	}
	httpReq.Header.Set("Authorization", "Bearer "+p.APIKey)
	httpReq.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(httpReq)
	if err != nil {
		return responseTextResult{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return responseTextResult{}, fmt.Errorf("openai structured responses API returned %s", resp.Status)
	}
	var decoded struct {
		Model  string `json:"model"`
		Output []struct {
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
		} `json:"output"`
		Usage struct {
			InputTokens       int `json:"input_tokens"`
			InputTokenDetails struct {
				CachedTokens int `json:"cached_tokens"`
			} `json:"input_tokens_details"`
			OutputTokens       int `json:"output_tokens"`
			OutputTokenDetails struct {
				ReasoningTokens int `json:"reasoning_tokens"`
			} `json:"output_tokens_details"`
			TotalTokens int `json:"total_tokens"`
		} `json:"usage"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		return responseTextResult{}, err
	}
	usage := TokenUsage{
		InputTokens:       decoded.Usage.InputTokens,
		CachedInputTokens: decoded.Usage.InputTokenDetails.CachedTokens,
		OutputTokens:      decoded.Usage.OutputTokens,
		ReasoningTokens:   decoded.Usage.OutputTokenDetails.ReasoningTokens,
		TotalTokens:       decoded.Usage.TotalTokens,
		Model:             decoded.Model,
	}
	if usage.Model == "" {
		usage.Model = p.Model
	}
	for _, out := range decoded.Output {
		for _, c := range out.Content {
			if c.Type == "output_text" && strings.TrimSpace(c.Text) != "" {
				return responseTextResult{Text: c.Text, Usage: usage}, nil
			}
		}
	}
	return responseTextResult{}, errors.New("no structured output_text in OpenAI response")
}
