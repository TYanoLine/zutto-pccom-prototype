package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"
)

var _ BBSWorldWindowProducer = StructuredOpenAIProvider{}

const worldWindowProducerHTTPTimeout = 390 * time.Second

type bbsArticleBriefWire struct {
	Subject         string               `json:"subject"`
	Episode         string               `json:"episode"`
	Referents       []string             `json:"referents"`
	ActorKnowledge  []string             `json:"actor_knowledge"`
	AudienceContext []string             `json:"audience_context"`
	Contribution    []string             `json:"contribution"`
	MustNot         []string             `json:"must_not"`
	Topic           string               `json:"topic"`
	Motivation      string               `json:"motivation"`
	Stance          string               `json:"stance"`
	Goal            string               `json:"goal"`
	Facts           []BBSIntentFactDraft `json:"facts"`
}

type bbsWorldWindowProductionWire struct {
	Briefs map[string]bbsArticleBriefWire `json:"briefs"`
}

// GenerateBBSWorldWindowProduction is the semantic producer pass. Unlike the
// older board-local timeline planner, it sees the complete bounded host window
// across boards and personas before any article prose is rendered. It cannot add
// or remove actions; it only turns world-selected shells into mutually coherent,
// detailed article briefs.
func (p StructuredOpenAIProvider) GenerateBBSWorldWindowProduction(ctx context.Context, req BBSWorldWindowProductionRequest) (BBSWorldWindowProductionDraft, error) {
	if err := validateBBSWorldWindowEventIDs(req.Events); err != nil {
		return BBSWorldWindowProductionDraft{}, err
	}
	eventsJSON, err := json.Marshal(req.Events)
	if err != nil {
		return BBSWorldWindowProductionDraft{}, err
	}
	recent := strings.TrimSpace(req.RecentBBSState)
	if recent == "" {
		recent = "(no earlier canonical BBS state supplied)"
	}

	prompt := fmt.Sprintf(`You are the PRODUCER for a bounded slice of a fictional Japanese grass-roots BBS world.

Your job is NOT to write article bodies. Your job is to issue a detailed, mutually coherent production brief for every already-selected article event in the whole host window. Later article workers will write each article from your brief.

ABSOLUTE WORLD BOUNDARY:
- The world engine already decided every event's existence, actor, timestamp, board, root/reply topology, source event, routing domain and cause kind.
- You MUST NOT add, delete, merge, move or retarget an event.
- You MUST NOT make the human-controlled member the center of the world.
- Treat the supplied event shells as immutable production slots whose causes already exist.
- The database, not your prose, is the eventual source of truth. Make the briefs consistent enough to be committed as canonical semantic state.
- A relationship between current-window events exists ONLY when parent_event_id or source_event_id explicitly names it. Same board, same actor, nearby time, similar topic, or editorial convenience does NOT create a relationship.
- If action=thread_start and both parent_event_id/source_event_id are absent, it is a STANDALONE ROOT. Do not describe it as reading, replying to, continuing, or sharing the same occurrence/referent with another selected current-window event. Its audience_context MUST be empty and its subject MUST NOT begin with Re:.
- For a reply/continuation, only the explicitly named parent_event_id/source_event_id may supply current-window causal/thread context. Never borrow a different event merely because it would make a nicer story.

PRODUCER RESPONSIBILITIES:
- Coordinate the WHOLE WINDOW across all boards and personas before any article worker writes prose.
- Make each event's small episode concrete enough that a worker does not have to invent why the post exists.
- Reuse the same referent wording only when the immutable topology explicitly relates the events, or earlier canonical BBS state already establishes that identity. Never merge two standalone current-window roots into one occurrence.
- Track what the actor actually knows at that moment and what earlier BBS context makes safe to leave implicit.
- For replies, make contribution describe the NEW contribution to the selected source/thread, not a restatement of the root.
- For returning participants, respect cause_summary literally: a newer contribution is the reason they can speak again.
- Prefer sparse, mundane causality. A two-week BBS window is not a TV drama and does not need an arc for every person.
- Cross-board coherence matters: the same person's life, possessions, current activities and known facts must not mutate just because the board changed.
- KEEP THE BRIEFS COMPACT: each list should normally have 0-2 short items and never more than 4; each scalar field should normally be one short sentence. Do not spend tokens restating the supplied event shell.

SPECIFICITY BOUNDARY:
- Specificity must come from supplied canonical state, earlier BBS state, or safe fictional local detail that does not masquerade as an external historical fact.
- Do NOT invent a named commercial game, product, modem, software package, railway station, real shop, price, release date, exact technical specification or historical event unless that exact real-world referent is already supplied.
- If an exact external name is unavailable, keep the external identity unnamed but make the episode concrete in other ways (what happened, where within the already-known context, what was tried, what changed, what information is sought).
- Do not solve missing specificity by inventing nostalgia, a recent purchase, upgrade, hiatus, rediscovery, compatibility surprise, move, maintenance event or membership change.

DIEGETIC PRESENT:
%s

HOST WINDOW:
- world date: %s
- window start: %s
- window end: %s
- host: %s
- region: %s
- host software family: %s

EARLIER CANONICAL BBS STATE BEFORE THIS WINDOW/PLAN:
%s

WORLD-SELECTED EVENTS ACROSS ALL BOARDS (JSON):
%s

Return briefs as ONE JSON object keyed by the exact supplied event_id strings. Every supplied event_id is a required object key, and no other key is allowed. Do NOT repeat event_id inside a brief; the application owns identity and will attach it from the object key.

Brief fields:
- subject: exact subject this actor would type. Replies may still be canonicalized by the application to Re: root subject.
- episode: one concise description of the concrete contemporaneous occurrence/state difference that makes this exact post worth writing now.
- referents: zero or more concrete referents that the worker must keep stable. Reuse wording only inside an explicitly related event component. Do not invent unsupported named real-world entities.
- actor_knowledge: facts this actor is entitled to know when writing this article. Do not include omniscient producer knowledge.
- audience_context: facts legitimately established in the explicitly selected thread/source that make natural ellipsis such as 「あの面」 or 「さっきの件」 understandable. Standalone roots must return an empty array.
- contribution: the information/reaction/question this article must actually add. Replies should react to their source, and repeat participants need a genuinely newer contribution.
- must_not: event-specific prohibitions that prevent fact theft, unsupported specificity, retrospective framing or contradiction.
- topic/motivation/stance/goal: compact semantic state for the article worker. These must describe this exact event, not choose a different topic.
- facts: zero or one durable FICTIONAL PERSONAL fact only if the already-selected episode genuinely requires persistence. Most briefs should have none.

Additional rules:
- Internal anchor_key values are routing metadata, never resident vocabulary, but the episode must remain inside that routing domain. A games root cannot become a connection incident; a communications root cannot become BBS etiquette merely because communication articles are involved.
- Do not manufacture a story merely to connect unrelated posts. Cross-window consistency is more important than forced interconnection.
- A reply actor must not appropriate another person's first-person experience. Put ownership in actor_knowledge/must_not clearly when needed.
- Board placement is semantic. The brief must make sense on the exact supplied board without inventing a bridge.
- Subject lines may be terse/contextual like period BBS subjects, but contextual ellipsis is only allowed when audience_context actually establishes the referent.
- Never mention AI, simulation, prompts, databases, web searches, social media, smartphones or anything after the world date.`, withDiegeticWorldFrame(req.EraRules), req.WorldDate, req.WindowStart, req.WindowEnd, req.HostName, req.HostRegion, req.HostSoftware, recent, string(eventsJSON))

	maxTokens := 1800 + len(req.Events)*360
	if maxTokens > 12000 {
		maxTokens = 12000
	}
	producer := p.withWorldWindowHTTPTimeout()
	result, err := producer.responseTextWithJSONSchema(ctx, prompt, "low", maxTokens, "bbs_world_window_production", bbsWorldWindowProductionSchema(req.Events))
	if err != nil {
		return BBSWorldWindowProductionDraft{}, err
	}
	var wire bbsWorldWindowProductionWire
	if err := json.Unmarshal([]byte(strings.TrimSpace(result.Text)), &wire); err != nil {
		return BBSWorldWindowProductionDraft{}, fmt.Errorf("decode BBS world-window production JSON: %w", err)
	}
	draft, err := bbsWorldWindowDraftFromWire(req, wire)
	if err != nil {
		return BBSWorldWindowProductionDraft{}, err
	}
	if err := validateBBSWorldWindowProduction(req, draft); err != nil {
		return BBSWorldWindowProductionDraft{}, err
	}
	for i := range draft.Briefs {
		b := &draft.Briefs[i]
		b.EventID = strings.TrimSpace(b.EventID)
		b.Subject = strings.TrimSpace(b.Subject)
		b.Episode = strings.TrimSpace(b.Episode)
		b.Topic = strings.TrimSpace(b.Topic)
		b.Motivation = strings.TrimSpace(b.Motivation)
		b.Stance = strings.TrimSpace(b.Stance)
		b.Goal = strings.TrimSpace(b.Goal)
		b.Referents = cleanStringList(b.Referents)
		b.ActorKnowledge = cleanStringList(b.ActorKnowledge)
		b.AudienceContext = cleanStringList(b.AudienceContext)
		b.Contribution = cleanStringList(b.Contribution)
		b.MustNot = cleanStringList(b.MustNot)
		for j := range b.Facts {
			b.Facts[j].Key = strings.TrimSpace(strings.ToLower(b.Facts[j].Key))
			b.Facts[j].Value = strings.TrimSpace(b.Facts[j].Value)
		}
	}
	draft.Usage = result.Usage
	return draft, nil
}

func bbsWorldWindowDraftFromWire(req BBSWorldWindowProductionRequest, wire bbsWorldWindowProductionWire) (BBSWorldWindowProductionDraft, error) {
	if err := validateBBSWorldWindowEventIDs(req.Events); err != nil {
		return BBSWorldWindowProductionDraft{}, err
	}
	if len(wire.Briefs) != len(req.Events) {
		return BBSWorldWindowProductionDraft{}, fmt.Errorf("world-window producer returned %d keyed briefs, want %d", len(wire.Briefs), len(req.Events))
	}
	want := make(map[string]bool, len(req.Events))
	for _, event := range req.Events {
		want[strings.TrimSpace(event.EventID)] = true
	}
	for id := range wire.Briefs {
		if !want[id] {
			return BBSWorldWindowProductionDraft{}, fmt.Errorf("world-window producer returned unknown event key %q", id)
		}
	}

	briefs := make([]BBSArticleBriefDraft, 0, len(req.Events))
	for _, event := range req.Events {
		id := strings.TrimSpace(event.EventID)
		brief, ok := wire.Briefs[id]
		if !ok {
			return BBSWorldWindowProductionDraft{}, fmt.Errorf("world-window producer omitted required event key %q", id)
		}
		briefs = append(briefs, BBSArticleBriefDraft{
			EventID:         id,
			Subject:         brief.Subject,
			Episode:         brief.Episode,
			Referents:       append([]string(nil), brief.Referents...),
			ActorKnowledge:  append([]string(nil), brief.ActorKnowledge...),
			AudienceContext: append([]string(nil), brief.AudienceContext...),
			Contribution:    append([]string(nil), brief.Contribution...),
			MustNot:         append([]string(nil), brief.MustNot...),
			Topic:           brief.Topic,
			Motivation:      brief.Motivation,
			Stance:          brief.Stance,
			Goal:            brief.Goal,
			Facts:           append([]BBSIntentFactDraft(nil), brief.Facts...),
		})
	}
	return BBSWorldWindowProductionDraft{Briefs: briefs}, nil
}

// The server's shared renderer client intentionally uses a shorter timeout for
// ordinary article/timeline calls. A host-wide producer request is much larger
// and has a several-minute caller context in the PoC, so reusing the shared 90s
// HTTP deadline defeats that budget. Clone the client only for this request;
// cancellation from ctx remains authoritative and shorter than this ceiling.
func (p StructuredOpenAIProvider) withWorldWindowHTTPTimeout() StructuredOpenAIProvider {
	clone := p
	if p.Client == nil {
		clone.Client = &http.Client{Timeout: worldWindowProducerHTTPTimeout}
		return clone
	}
	client := *p.Client
	if client.Timeout <= 0 || client.Timeout < worldWindowProducerHTTPTimeout {
		client.Timeout = worldWindowProducerHTTPTimeout
	}
	clone.Client = &client
	return clone
}

func validateBBSWorldWindowEventIDs(events []BBSWorldWindowEvent) error {
	seen := make(map[string]bool, len(events))
	for _, event := range events {
		id := strings.TrimSpace(event.EventID)
		if id == "" {
			return fmt.Errorf("world-window request contains empty event_id")
		}
		if seen[id] {
			return fmt.Errorf("world-window request contains duplicate event_id %q", id)
		}
		seen[id] = true
	}
	for _, event := range events {
		id := strings.TrimSpace(event.EventID)
		for kind, ref := range map[string]string{"parent_event_id": event.ParentEventID, "source_event_id": event.SourceEventID} {
			ref = strings.TrimSpace(ref)
			if ref == "" {
				continue
			}
			if !seen[ref] {
				return fmt.Errorf("world-window event %q has unknown %s %q", id, kind, ref)
			}
			if ref == id {
				return fmt.Errorf("world-window event %q cannot reference itself as %s", id, kind)
			}
		}
	}
	return nil
}

func validateBBSWorldWindowProduction(req BBSWorldWindowProductionRequest, draft BBSWorldWindowProductionDraft) error {
	if err := validateBBSWorldWindowEventIDs(req.Events); err != nil {
		return err
	}
	if len(draft.Briefs) != len(req.Events) {
		return fmt.Errorf("world-window producer returned %d briefs, want %d", len(draft.Briefs), len(req.Events))
	}
	eventByID := make(map[string]BBSWorldWindowEvent, len(req.Events))
	for _, event := range req.Events {
		eventByID[strings.TrimSpace(event.EventID)] = event
	}
	seen := map[string]bool{}
	for _, brief := range draft.Briefs {
		id := strings.TrimSpace(brief.EventID)
		event, wanted := eventByID[id]
		if !wanted {
			return fmt.Errorf("world-window producer returned unknown event_id %q", id)
		}
		if seen[id] {
			return fmt.Errorf("world-window producer returned duplicate event_id %q", id)
		}
		seen[id] = true
		if strings.TrimSpace(brief.Subject) == "" || strings.TrimSpace(brief.Episode) == "" || strings.TrimSpace(brief.Topic) == "" || strings.TrimSpace(brief.Motivation) == "" || strings.TrimSpace(brief.Stance) == "" || strings.TrimSpace(brief.Goal) == "" {
			return fmt.Errorf("world-window brief %q is missing required semantic content", id)
		}
		if len(brief.Facts) > 1 {
			return fmt.Errorf("world-window brief %q proposed %d durable facts, max 1", id, len(brief.Facts))
		}
		if isStandaloneWorldWindowRoot(event) {
			if hasReplySubjectPrefix(brief.Subject) {
				return fmt.Errorf("world-window standalone root %q used reply subject %q", id, strings.TrimSpace(brief.Subject))
			}
			if len(cleanStringList(brief.AudienceContext)) > 0 {
				return fmt.Errorf("world-window standalone root %q invented audience_context", id)
			}
		}
	}
	if err := validateBBSWorldWindowReferentIsolation(req.Events, draft.Briefs); err != nil {
		return err
	}
	return nil
}

func isStandaloneWorldWindowRoot(event BBSWorldWindowEvent) bool {
	return strings.EqualFold(strings.TrimSpace(event.Action), "thread_start") &&
		strings.TrimSpace(event.ParentEventID) == "" && strings.TrimSpace(event.SourceEventID) == ""
}

func hasReplySubjectPrefix(subject string) bool {
	s := strings.ToLower(strings.TrimSpace(subject))
	return strings.HasPrefix(s, "re:") || strings.HasPrefix(s, "re：") || strings.HasPrefix(s, "ｒｅ:") || strings.HasPrefix(s, "ｒｅ：")
}

func validateBBSWorldWindowReferentIsolation(events []BBSWorldWindowEvent, briefs []BBSArticleBriefDraft) error {
	componentByID, err := worldWindowRelationComponents(events)
	if err != nil {
		return err
	}
	componentByReferent := map[string]string{}
	eventByReferent := map[string]string{}
	for _, brief := range briefs {
		id := strings.TrimSpace(brief.EventID)
		component := componentByID[id]
		for _, referent := range cleanStringList(brief.Referents) {
			key := normalizeProducerReferent(referent)
			if key == "" {
				continue
			}
			if priorComponent, ok := componentByReferent[key]; ok && priorComponent != component {
				return fmt.Errorf("world-window producer reused referent %q across unrelated events %q and %q", referent, eventByReferent[key], id)
			}
			componentByReferent[key] = component
			eventByReferent[key] = id
		}
	}
	return nil
}

func worldWindowRelationComponents(events []BBSWorldWindowEvent) (map[string]string, error) {
	if err := validateBBSWorldWindowEventIDs(events); err != nil {
		return nil, err
	}
	adj := make(map[string][]string, len(events))
	order := make([]string, 0, len(events))
	for _, event := range events {
		id := strings.TrimSpace(event.EventID)
		order = append(order, id)
		if _, ok := adj[id]; !ok {
			adj[id] = nil
		}
		for _, ref := range []string{event.ParentEventID, event.SourceEventID} {
			ref = strings.TrimSpace(ref)
			if ref == "" {
				continue
			}
			adj[id] = append(adj[id], ref)
			adj[ref] = append(adj[ref], id)
		}
	}
	componentByID := make(map[string]string, len(events))
	for _, start := range order {
		if _, seen := componentByID[start]; seen {
			continue
		}
		component := start
		stack := []string{start}
		componentByID[start] = component
		for len(stack) > 0 {
			id := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			for _, next := range adj[id] {
				if _, seen := componentByID[next]; seen {
					continue
				}
				componentByID[next] = component
				stack = append(stack, next)
			}
		}
	}
	return componentByID, nil
}

func normalizeProducerReferent(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	return strings.Join(strings.Fields(value), "")
}

func cleanStringList(values []string) []string {
	out := make([]string, 0, len(values))
	seen := map[string]bool{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}

func bbsWorldWindowProductionSchema(events []BBSWorldWindowEvent) map[string]any {
	stringArray := func(max int) map[string]any {
		return map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "maxItems": max}
	}
	fact := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"key":   map[string]any{"type": "string"},
			"value": map[string]any{"type": "string"},
		},
		"required":             []string{"key", "value"},
		"additionalProperties": false,
	}
	briefSchema := func(event BBSWorldWindowEvent) map[string]any {
		audienceMax := 4
		if isStandaloneWorldWindowRoot(event) {
			audienceMax = 0
		}
		return map[string]any{
			"type": "object",
			"properties": map[string]any{
				"subject":          map[string]any{"type": "string"},
				"episode":          map[string]any{"type": "string"},
				"referents":        stringArray(4),
				"actor_knowledge":  stringArray(4),
				"audience_context": stringArray(audienceMax),
				"contribution":     stringArray(4),
				"must_not":         stringArray(4),
				"topic":            map[string]any{"type": "string"},
				"motivation":       map[string]any{"type": "string"},
				"stance":           map[string]any{"type": "string"},
				"goal":             map[string]any{"type": "string"},
				"facts":            map[string]any{"type": "array", "items": fact, "maxItems": 1},
			},
			"required":             []string{"subject", "episode", "referents", "actor_knowledge", "audience_context", "contribution", "must_not", "topic", "motivation", "stance", "goal", "facts"},
			"additionalProperties": false,
		}
	}

	briefProperties := make(map[string]any, len(events))
	required := make([]string, 0, len(events))
	for _, event := range events {
		id := strings.TrimSpace(event.EventID)
		briefProperties[id] = briefSchema(event)
		required = append(required, id)
	}
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"briefs": map[string]any{
				"type":                 "object",
				"properties":           briefProperties,
				"required":             required,
				"additionalProperties": false,
			},
		},
		"required":             []string{"briefs"},
		"additionalProperties": false,
	}
}

// sortedWorldWindowEvents is useful in tests and future producer variants where
// deterministic ordering matters independently of the request construction.
func sortedWorldWindowEvents(events []BBSWorldWindowEvent) []BBSWorldWindowEvent {
	out := append([]BBSWorldWindowEvent(nil), events...)
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].CreatedAt == out[j].CreatedAt {
			return out[i].EventID < out[j].EventID
		}
		return out[i].CreatedAt < out[j].CreatedAt
	})
	return out
}
