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
	"sync"
	"time"
)

// StructuredOpenAIProvider keeps the normal OpenAIProvider behavior for prose
// rendering, but requires strict JSON Schema output for BBS semantic realization.
// The world layer already selected whether an event exists and why; this layer
// must not turn persona background into new world actions.
type StructuredOpenAIProvider struct {
	OpenAIProvider
}

type structuredOpenAIAPIError struct {
	StatusCode        int
	Status            string
	Type              string
	Code              string
	Message           string
	RequestID         string
	RetryAfter        string
	LimitRequests     string
	RemainingRequests string
	ResetRequests     string
	LimitTokens       string
	RemainingTokens   string
	ResetTokens       string
}

func (e *structuredOpenAIAPIError) Error() string {
	if e == nil {
		return "openai structured API error"
	}
	parts := []string{fmt.Sprintf("openai structured responses API returned %s", e.Status)}
	if e.Type != "" {
		parts = append(parts, "type="+e.Type)
	}
	if e.Code != "" {
		parts = append(parts, "code="+e.Code)
	}
	if e.Message != "" {
		parts = append(parts, "message="+e.Message)
	}
	if e.RequestID != "" {
		parts = append(parts, "request_id="+e.RequestID)
	}
	if e.RetryAfter != "" {
		parts = append(parts, "retry_after="+e.RetryAfter)
	}
	if e.RemainingRequests != "" || e.ResetRequests != "" {
		parts = append(parts, "requests_remaining="+e.RemainingRequests, "requests_reset="+e.ResetRequests)
	}
	if e.RemainingTokens != "" || e.ResetTokens != "" {
		parts = append(parts, "tokens_remaining="+e.RemainingTokens, "tokens_reset="+e.ResetTokens)
	}
	return strings.Join(parts, " ")
}

func (e *structuredOpenAIAPIError) retryable() bool {
	if e == nil {
		return false
	}
	if e.StatusCode >= 500 && e.StatusCode <= 599 {
		return true
	}
	if e.StatusCode != http.StatusTooManyRequests {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(e.Code)) {
	case "credit_balance_exhausted", "organization_usage_limit_exceeded", "organization_spend_limit_exceeded", "project_spend_limit_exceeded":
		return false
	}
	if strings.EqualFold(strings.TrimSpace(e.Type), "insufficient_quota") {
		return false
	}
	return true
}

type structuredOpenAIGate struct {
	mu      sync.Mutex
	notBefore time.Time
}

var sharedStructuredOpenAIGate structuredOpenAIGate

func (g *structuredOpenAIGate) wait(ctx context.Context) error {
	g.mu.Lock()
	until := g.notBefore
	g.mu.Unlock()
	delay := time.Until(until)
	if delay <= 0 {
		return nil
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (g *structuredOpenAIGate) deferUntil(until time.Time) {
	if until.IsZero() {
		return
	}
	g.mu.Lock()
	if until.After(g.notBefore) {
		g.notBefore = until
	}
	g.mu.Unlock()
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

anchor_key is INTERNAL ROUTING METADATA. It is not the actor's wording, not a headline, and not evidence that the category itself is unusual. A broad domain such as communications, games, music, local life, BBS activity, or software must not become a generic "I used/did this thing" post merely because it was selected.

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
- Follow the DIEGETIC PRESENT / ERA NORMALITY rules literally. The actor lives inside the world date; never write from a later nostalgic, preservationist, retro, or historical-explainer viewpoint unless canonical state explicitly establishes that viewpoint.
- anchor_key and cause_summary constrain the event. Existing persona facts, interests, and everyday_baseline are BACKGROUND/CONSISTENCY context only. Their presence does not make them current topics or novelties.
- everyday_baseline is deliberately ordinary and normally UNMENTIONED. Do not turn "normally uses/does X" into "tried X", "could still use X", "returned to X", "rediscovered X", or "X was nostalgic" unless the supplied event explicitly establishes that change.
- For cause_kind=recent_salience, stay inside the supplied routing domain but make the post about a concrete contemporaneously meaningful difference, problem, decision, interaction, question, or observation. Ordinary participation/use of the domain itself is not the event.
- A recent_salience event does NOT authorize you to invent a purchase, upgrade, hiatus, rediscovery, compatibility surprise, new member, membership growth, maintenance, popularity change, move, or other durable world transition. Such transitions require explicit canonical support in cause_summary, existing facts, earlier BBS state, or supplied historical facts.
- Do not use retrospective shortcuts such as 「久しぶりに」「懐かしい」「〜からでも入れた」「まだ使える」「昔使っていた」 unless the supplied canonical state actually establishes the corresponding hiatus, nostalgia, compatibility doubt, age comparison, or past usage.
- When a technical distinction matters, name only a model/setup/version that is actually supplied or otherwise historically supported. If only a broad internal family/classification is known, normally leave that classification unspoken rather than turning it into a topic.
- For cause_kind=continuation_progress, there must be materially new progress/change/observation compared with the referenced earlier event. Do not merely restate the old preference, baseline condition, habit, or question in new words.
- For cause_kind=observed_thread, respond to the supplied parent/source thread. Do not start an unrelated root topic inside a reply, and do not add a new world event merely to make the reply interesting.
- For a root post, follow the subject-line calibration above. The subject must be the exact text this actor would type now, not a polished summary or generic headline.
- For a reply, the host program owns whether an independent reply subject exists and how it is represented. The semantic content must still be a genuine response to the selected parent/source thread.
- topic is a short free-form human-readable description of the already-selected causal content. It is not a new topic selection step and should describe the concrete matter, not merely repeat anchor_key.
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
	return p.responseTextWithJSONSchemaReasoning(ctx, prompt, verbosity, "", maxOutputTokens, schemaName, schema)
}

func (p StructuredOpenAIProvider) responseTextWithJSONSchemaReasoning(ctx context.Context, prompt, verbosity, reasoningEffort string, maxOutputTokens int, schemaName string, schema map[string]any) (responseTextResult, error) {
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
	if reasoningEffort = strings.TrimSpace(reasoningEffort); reasoningEffort != "" {
		payload["reasoning"] = map[string]any{"effort": reasoningEffort}
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
	resp, err := doStructuredOpenAIRequest(ctx, client, httpReq)
	if err != nil {
		return responseTextResult{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		apiErr := readStructuredOpenAIAPIError(resp)
		return responseTextResult{}, apiErr
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


func doStructuredOpenAIRequest(ctx context.Context, client *http.Client, req *http.Request) (*http.Response, error) {
	const maxAttempts = 3
	for attempt := 0; attempt < maxAttempts; attempt++ {
		if err := sharedStructuredOpenAIGate.wait(ctx); err != nil {
			return nil, err
		}

		current := req
		if attempt > 0 {
			current = req.Clone(ctx)
			if req.GetBody != nil {
				body, err := req.GetBody()
				if err != nil {
					return nil, err
				}
				current.Body = body
			}
		}
		resp, err := client.Do(current)
		if err != nil {
			return nil, err
		}
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return resp, nil
		}
		apiErr := readStructuredOpenAIAPIError(resp)
		if !apiErr.retryable() || attempt+1 >= maxAttempts {
			return nil, apiErr
		}

		delay := structuredOpenAIRetryDelay(resp.Header, attempt)
		sharedStructuredOpenAIGate.deferUntil(time.Now().Add(delay))
		if err := sharedStructuredOpenAIGate.wait(ctx); err != nil {
			return nil, err
		}
	}
	return nil, errors.New("openai structured responses retry loop exhausted")
}

func readStructuredOpenAIAPIError(resp *http.Response) *structuredOpenAIAPIError {
	if resp == nil {
		return &structuredOpenAIAPIError{Status: "unknown"}
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 16<<10))
	var decoded struct {
		Error struct {
			Message string `json:"message"`
			Type    string `json:"type"`
			Code    string `json:"code"`
		} `json:"error"`
	}
	_ = json.Unmarshal(body, &decoded)
	return &structuredOpenAIAPIError{
		StatusCode:        resp.StatusCode,
		Status:            resp.Status,
		Type:              strings.TrimSpace(decoded.Error.Type),
		Code:              strings.TrimSpace(decoded.Error.Code),
		Message:           compactOpenAIErrorMessage(decoded.Error.Message),
		RequestID:         strings.TrimSpace(resp.Header.Get("x-request-id")),
		RetryAfter:        strings.TrimSpace(resp.Header.Get("Retry-After")),
		LimitRequests:     strings.TrimSpace(resp.Header.Get("x-ratelimit-limit-requests")),
		RemainingRequests: strings.TrimSpace(resp.Header.Get("x-ratelimit-remaining-requests")),
		ResetRequests:     strings.TrimSpace(resp.Header.Get("x-ratelimit-reset-requests")),
		LimitTokens:       strings.TrimSpace(resp.Header.Get("x-ratelimit-limit-tokens")),
		RemainingTokens:   strings.TrimSpace(resp.Header.Get("x-ratelimit-remaining-tokens")),
		ResetTokens:       strings.TrimSpace(resp.Header.Get("x-ratelimit-reset-tokens")),
	}
}

func compactOpenAIErrorMessage(message string) string {
	message = strings.Join(strings.Fields(strings.TrimSpace(message)), " ")
	const max = 300
	if len(message) > max {
		return message[:max] + "…"
	}
	return message
}

func structuredOpenAIRetryDelay(header http.Header, attempt int) time.Duration {
	delay := time.Duration(1<<attempt) * time.Second
	if raw := strings.TrimSpace(header.Get("Retry-After")); raw != "" {
		if d, err := time.ParseDuration(raw + "s"); err == nil && d >= 0 {
			delay = d
		} else if when, err := http.ParseTime(raw); err == nil {
			if d := time.Until(when); d > 0 {
				delay = d
			}
		}
	}
	for _, key := range []string{"x-ratelimit-reset-requests", "x-ratelimit-reset-tokens"} {
		if d, ok := parseOpenAIRateReset(header.Get(key)); ok && d > delay {
			delay = d
		}
	}
	if delay < 500*time.Millisecond {
		delay = 500 * time.Millisecond
	}
	if delay > 60*time.Second {
		delay = 60 * time.Second
	}
	return delay
}

func parseOpenAIRateReset(raw string) (time.Duration, bool) {
	raw = strings.TrimSpace(strings.ToLower(raw))
	if raw == "" {
		return 0, false
	}
	// OpenAI rate-limit reset headers commonly use compact values such as 1s,
	// 200ms, 1m30s. time.ParseDuration supports these forms directly.
	d, err := time.ParseDuration(raw)
	if err != nil || d < 0 {
		return 0, false
	}
	return d, true
}
