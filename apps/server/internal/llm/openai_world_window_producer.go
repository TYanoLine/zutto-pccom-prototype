package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

var _ BBSWorldWindowProducer = StructuredOpenAIProvider{}

// GenerateBBSWorldWindowProduction is the semantic producer pass. Unlike the
// older board-local timeline planner, it sees the complete bounded host window
// across boards and personas before any article prose is rendered. It cannot add
// or remove actions; it only turns world-selected shells into mutually coherent,
// detailed article briefs.
func (p StructuredOpenAIProvider) GenerateBBSWorldWindowProduction(ctx context.Context, req BBSWorldWindowProductionRequest) (BBSWorldWindowProductionDraft, error) {
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

PRODUCER RESPONSIBILITIES:
- Coordinate the WHOLE WINDOW across all boards and personas before any article worker writes prose.
- Make each event's small episode concrete enough that a worker does not have to invent why the post exists.
- Reuse the same referent consistently when multiple events concern the same thing. Referents may be compact descriptions or stable labels; they do not need to be public-facing IDs.
- Track what the actor actually knows at that moment and what earlier BBS context makes safe to leave implicit.
- For replies, make contribution describe the NEW contribution to the selected source/thread, not a restatement of the root.
- For returning participants, respect cause_summary literally: a newer contribution is the reason they can speak again.
- Prefer sparse, mundane causality. A two-week BBS window is not a TV drama and does not need an arc for every person.
- Cross-board coherence matters: the same person's life, possessions, current activities and known facts must not mutate just because the board changed.

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

For EACH event return one brief with exactly the same event_id.

Brief fields:
- subject: exact subject this actor would type. Replies may still be canonicalized by the application to Re: root subject.
- episode: one concise description of the concrete contemporaneous occurrence/state difference that makes this exact post worth writing now.
- referents: zero or more concrete referents that the worker must keep stable. Reuse wording across related briefs when it is the same object/place/problem. Do not invent unsupported named real-world entities.
- actor_knowledge: facts this actor is entitled to know when writing this article. Do not include omniscient producer knowledge.
- audience_context: facts legitimately established in this BBS/thread/window that make natural ellipsis such as 「あの面」 or 「さっきの件」 understandable. If there is no shared referent, do not pretend there is one.
- contribution: the information/reaction/question this article must actually add. Replies should react to their source, and repeat participants need a genuinely newer contribution.
- must_not: event-specific prohibitions that prevent fact theft, unsupported specificity, retrospective framing or contradiction.
- topic/motivation/stance/goal: compact semantic state for the article worker. These must describe this exact event, not choose a different topic.
- facts: zero or one durable FICTIONAL PERSONAL fact only if the already-selected episode genuinely requires persistence. Most briefs should have none.

Additional rules:
- Internal anchor_key values are routing metadata, never resident vocabulary.
- Do not manufacture a story merely to connect unrelated posts. Cross-window consistency is more important than forced interconnection.
- If two events plausibly share an already-supported referent, make that identity explicit in referents/audience_context so workers can use natural shorthand later.
- If they do not share a referent, keep them separate.
- A reply actor must not appropriate another person's first-person experience. Put ownership in actor_knowledge/must_not clearly when needed.
- Board placement is semantic. The brief must make sense on the exact supplied board without inventing a bridge.
- Subject lines may be terse/contextual like period BBS subjects, but contextual ellipsis is only allowed when audience_context actually establishes the referent.
- Never mention AI, simulation, prompts, databases, web searches, social media, smartphones or anything after the world date.
- Return exactly one brief for every supplied event_id and no extra briefs.`, withDiegeticWorldFrame(req.EraRules), req.WorldDate, req.WindowStart, req.WindowEnd, req.HostName, req.HostRegion, req.HostSoftware, recent, string(eventsJSON))

	maxTokens := 2200 + len(req.Events)*420
	if maxTokens > 16000 {
		maxTokens = 16000
	}
	result, err := p.responseTextWithJSONSchema(ctx, prompt, "low", maxTokens, "bbs_world_window_production", bbsWorldWindowProductionSchema())
	if err != nil {
		return BBSWorldWindowProductionDraft{}, err
	}
	var draft BBSWorldWindowProductionDraft
	if err := json.Unmarshal([]byte(strings.TrimSpace(result.Text)), &draft); err != nil {
		return BBSWorldWindowProductionDraft{}, fmt.Errorf("decode BBS world-window production JSON: %w", err)
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

func validateBBSWorldWindowProduction(req BBSWorldWindowProductionRequest, draft BBSWorldWindowProductionDraft) error {
	if len(draft.Briefs) != len(req.Events) {
		return fmt.Errorf("world-window producer returned %d briefs, want %d", len(draft.Briefs), len(req.Events))
	}
	want := make(map[string]bool, len(req.Events))
	for _, event := range req.Events {
		id := strings.TrimSpace(event.EventID)
		if id == "" {
			return fmt.Errorf("world-window request contains empty event_id")
		}
		if want[id] {
			return fmt.Errorf("world-window request contains duplicate event_id %q", id)
		}
		want[id] = true
	}
	seen := map[string]bool{}
	for _, brief := range draft.Briefs {
		id := strings.TrimSpace(brief.EventID)
		if !want[id] {
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
	}
	return nil
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

func bbsWorldWindowProductionSchema() map[string]any {
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
	brief := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"event_id":          map[string]any{"type": "string"},
			"subject":           map[string]any{"type": "string"},
			"episode":           map[string]any{"type": "string"},
			"referents":         stringArray(8),
			"actor_knowledge":   stringArray(8),
			"audience_context":  stringArray(8),
			"contribution":      stringArray(8),
			"must_not":          stringArray(8),
			"topic":             map[string]any{"type": "string"},
			"motivation":        map[string]any{"type": "string"},
			"stance":            map[string]any{"type": "string"},
			"goal":              map[string]any{"type": "string"},
			"facts":             map[string]any{"type": "array", "items": fact, "maxItems": 1},
		},
		"required": []string{"event_id", "subject", "episode", "referents", "actor_knowledge", "audience_context", "contribution", "must_not", "topic", "motivation", "stance", "goal", "facts"},
		"additionalProperties": false,
	}
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"briefs": map[string]any{"type": "array", "items": brief},
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
