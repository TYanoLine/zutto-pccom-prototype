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
// rendering, but requires strict JSON Schema output for BBS semantic realization.
// The world layer already selected whether an event exists and why; this layer
// must not turn persona background into new world actions.
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
	prompt := fmt.Sprintf(`Realize semantic wording for a bounded sequence of events in a fictional Japanese grass-roots BBS world.

CRITICAL CAUSAL BOUNDARY:
The WORLD LAYER has already decided whether each event exists, its actor, time, root/reply topology, source event, anchor_key, and cause_kind. These are canonical constraints, not suggestions. You MUST NOT choose a different topic because another persona fact looks more interesting. You MUST NOT invent another post or turn a background preference into a new posting reason.

The event shell's cause_summary explains why this exact event exists now. Realize that cause into a plausible subject + semantic intent. The application validates and commits accepted results as world state.

There is intentionally NO fixed prose topic list, subject template bank, information-slot checklist, or response-act menu. anchor_key is selected dynamically from this persona/world state; it is not a quota or a request to rotate categories.

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

WORLD-SELECTED CAUSAL EVENT SHELLS (JSON):
%s

Rules for each event:
- Preserve index, author, timestamp, action, parent/source topology, anchor_key, cause_kind, and canonical_subject from the event shell.
- anchor_key and cause_summary define the current cause. Existing persona facts and persona interests are BACKGROUND/CONSISTENCY context only. Their presence does not make them current topics.
- For cause_kind=recent_salience, stay inside the supplied anchor. Make one ordinary current experience/observation/thought in that area concrete enough to support the post, without turning unrelated background facts into the subject.
- For cause_kind=continuation_progress, there must be materially new progress/change/observation compared with the referenced earlier event. Do not merely restate the old preference, habit, or question in new words.
- For cause_kind=observed_thread, respond to the supplied parent/source thread. Do not start an unrelated root topic inside a reply.
- For a root post, follow the subject-line calibration above. The subject must be the exact text this actor would type now, not a polished summary or generic headline.
- For a reply, the application may canonicalize the subject to Re: <root subject>; the semantic content must still be a genuine response to the selected thread.
- topic is a short free-form human-readable description of the already-selected causal content. It is not a new topic selection step.
- motivation, stance, and goal describe this exact event. Motivation must follow cause_summary; do not fabricate a different reason for posting.
- facts contains zero or one durable FICTIONAL PERSONAL fact only when the realized post genuinely requires a new long-lived fact for consistency. Most ordinary reactions/observations should have zero facts.
- Never create a fact merely to justify why the post exists: the world-selected cause already justifies the post.
- Each fact key is an open lowercase semantic path, not a predefined schema. If existing_facts already contains the same dimension, never contradict it.
- existing_facts entries marked BACKGROUND ONLY are contradiction guards. Reuse one only when the selected cause truly requires it. Do not treat the list as a menu of possible subjects.
- Facts may describe this fictional person's ordinary ownership, habits, preferences, household situation, or experience. They must NOT assert unprovided real-world hardware limits, exact product specifications, release dates, prices, historical events, or other external historical facts.
- Do not invent a question merely to keep a thread alive. Ask only if asking is naturally part of the already-selected event's goal.
- Repetition can be natural inside an ongoing thread, but a continuation_progress event must add something new. Do not optimize every post for information density.
- The human-controlled member is not special and need not be mentioned.
- Never mention AI, simulation, prompts, databases, web searches, social media, smartphones, or anything after the world date.
- Keep the sequence mutually coherent. Later events may react to earlier realized semantics only where their world-selected topology/cause permits it.
- Return exactly one event object for every supplied event index.`, historicalBBSSubjectCalibration, req.WorldDate, req.HostName, req.HostRegion, req.HostSoftware, req.BoardID, req.BoardName, req.EraRules, recent, string(eventsJSON))

	maxTokens := 1500 + len(req.Events)*210
	if maxTokens > 5200 {
		maxTokens = 5200
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
			"facts":      map[string]any{"type": "array", "items": fact, "maxItems": 1},
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
