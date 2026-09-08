package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

var _ BBSWorldSituationProposer = StructuredOpenAIProvider{}

type bbsWorldSituationWire struct {
	ObjectClass      string   `json:"object_class"`
	ChangeClass      string   `json:"change_class"`
	Occurrence       string   `json:"occurrence"`
	ActorObservation string   `json:"actor_observation"`
	Impact           string   `json:"impact"`
	Uncertainty      string   `json:"uncertainty"`
	NoveltyKey       string   `json:"novelty_key"`
	MustNot          []string `json:"must_not"`
}

type bbsWorldSituationProposalWire struct {
	Situations map[string]bbsWorldSituationWire `json:"situations"`
}

func (p StructuredOpenAIProvider) GenerateBBSWorldSituationProposals(ctx context.Context, req BBSWorldSituationProposalRequest) (BBSWorldSituationProposalDraft, error) {
	if err := validateBBSWorldWindowEventIDs(req.Events); err != nil {
		return BBSWorldSituationProposalDraft{}, err
	}
	if len(req.Events) == 0 {
		return BBSWorldSituationProposalDraft{}, nil
	}
	for _, event := range req.Events {
		if !isStandaloneWorldWindowRoot(event) {
			return BBSWorldSituationProposalDraft{}, fmt.Errorf("situation proposer received non-standalone root %q", event.EventID)
		}
	}
	eventsJSON, err := json.Marshal(req.Events)
	if err != nil {
		return BBSWorldSituationProposalDraft{}, err
	}
	avoidJSON, err := json.Marshal(req.AvoidSituations)
	if err != nil {
		return BBSWorldSituationProposalDraft{}, err
	}
	historicalFacts := "(none supplied)"
	if len(req.HistoricalFacts) > 0 {
		historicalFacts = "- " + strings.Join(req.HistoricalFacts, "\n- ")
	}
	recent := strings.TrimSpace(req.RecentBBSState)
	if recent == "" {
		recent = "(no earlier canonical BBS state supplied)"
	}
	prompt := fmt.Sprintf(`You propose WORLD FACTS for a bounded slice of a fictional Japanese grass-roots BBS world around 1996.

This is NOT article writing. Do not write subject lines or post bodies. For each already-selected standalone root slot, propose exactly one small concrete contemporaneous situation that the world engine could accept as canonical BEFORE prose is rendered.

IMMUTABLE INPUT:
- event existence, actor, time, board, routing domain, cause kind and discourse mode are already fixed.
- every supplied event is an independent standalone root. Do not connect two roots into one incident, conversation, place, object or hidden shared event.
- routing domains are broad constraints, not topic menus. Invent a situation freely inside the domain; do not choose from a predefined event/facet catalog.

BATCH DIVERSITY:
- You see the whole root batch specifically so you can avoid repetition.
- Within the same board, use meaningfully different concrete objects/activities and different changes/occurrences.
- Across the host window, do not repeat the same distinctive occurrence with paraphrased wording.
- object_class and change_class are FREE descriptive labels, not enums. Make object_class concrete enough to distinguish, for example, one practical local object/activity from another; do not return only the broad routing domain.
- novelty_key is a short normalized semantic key for duplicate detection. Unrelated roots must have different novelty_key values.

WORLD-TRUTH BOUNDARY:
- Propose only observable or modestly inferable facts. Do not invent that a SYSOP checked logs, a machine failed internally, a phone network caused something, or an earlier post existed unless supplied canonical state establishes it.
- Technical roots should describe observable behavior appropriate to the selected board/domain; software roots need not become terminal/call/session incidents. Do not guess protocols, carrier causes or internal hardware faults.
- Do not introduce new real product/work/service/company/person/place/event names unless SUPPLIED HISTORICAL TEXTURE below explicitly permits them or supplied canonical evidence contains them.
- SUPPLIED HISTORICAL TEXTURE is permission and contemporaneous background, not a topic menu. Use a supplied concrete name when it genuinely sharpens an already-plausible situation; do not mechanically insert names into every root. A name may identify the ordinary object of conversation; no exceptional comparison or malfunction is required. Decide relevance from the selected board, routing domain and actor interests. Do not use one favored name across unrelated roots; unnamed everyday subjects remain valid. Never extrapolate release dates, prices, specifications, plot/results, popularity rankings or other facts that the supplied line does not state.
- Keep events mundane. Do not manufacture upgrades, purchases, nostalgia, rediscovery, membership changes, maintenance, outages or dramatic incidents merely to make a post interesting.
- impact and uncertainty may be empty strings if none are needed.

DISCOURSE MODES:
- share_observation: a concrete observation, not an invitation for replies.
- share_experience: a concrete firsthand experience/result.
- state_opinion: a modest opinion grounded in the proposed situation.
- share_tip: a small firsthand practical habit/result; avoid unsupported universal claims.
- ask_peers: a concrete uncertainty another member can answer from the eventual article without guessing a hidden title/place/product/device/choice.

DIEGETIC PRESENT:
%s

HOST WINDOW:
world date: %s
window: %s .. %s
host: %s
region: %s
software family: %s

SUPPLIED HISTORICAL TEXTURE — ALLOWED CONTEMPORARY REFERENTS, NOT REQUIRED TOPICS:
%s

EARLIER CANONICAL BBS STATE:
%s

WORLD-SELECTED ROOT SLOTS (JSON):
%s

SITUATIONS ALREADY ACCEPTED OR OTHERWISE FORBIDDEN FOR THIS RETRY (JSON):
%s

Return one JSON object keyed by every exact event_id and no other keys. Each situation contains:
- object_class: concise free label for the concrete object/activity/state being observed
- change_class: concise free label for what happened/changed/was decided
- occurrence: one sentence stating the canonical occurrence
- actor_observation: what this actor directly observed/experienced/learned from supplied state
- impact: small immediate consequence, or empty string
- uncertainty: unresolved question/unknown, or empty string
- novelty_key: short semantic duplicate-detection key, unique among unrelated roots
- must_not: 0-2 short situation-specific constraints preventing unsupported facts or confusion

Never mention AI, prompts, databases, social media, smartphones or anything after the world date.`, withDiegeticWorldFrame(req.EraRules), req.WorldDate, req.WindowStart, req.WindowEnd, req.HostName, req.HostRegion, req.HostSoftware, historicalFacts, recent, string(eventsJSON), string(avoidJSON))

	maxTokens := 700 + len(req.Events)*240
	if maxTokens > 10000 {
		maxTokens = 10000
	}
	producer := p.withWorldWindowHTTPTimeout()
	result, err := producer.responseTextWithJSONSchema(ctx, prompt, "low", maxTokens, "bbs_world_situation_proposals", bbsWorldSituationProposalSchema(req.Events))
	if err != nil {
		return BBSWorldSituationProposalDraft{}, err
	}
	var wire bbsWorldSituationProposalWire
	if err := json.Unmarshal([]byte(strings.TrimSpace(result.Text)), &wire); err != nil {
		return BBSWorldSituationProposalDraft{}, fmt.Errorf("decode BBS world-situation proposal JSON: %w", err)
	}
	if len(wire.Situations) != len(req.Events) {
		return BBSWorldSituationProposalDraft{}, fmt.Errorf("situation proposer returned %d keyed situations, want %d", len(wire.Situations), len(req.Events))
	}
	out := make([]BBSWorldSituationDraft, 0, len(req.Events))
	for _, event := range req.Events {
		id := strings.TrimSpace(event.EventID)
		value, ok := wire.Situations[id]
		if !ok {
			return BBSWorldSituationProposalDraft{}, fmt.Errorf("situation proposer omitted required event key %q", id)
		}
		out = append(out, BBSWorldSituationDraft{
			EventID:          id,
			ObjectClass:      strings.TrimSpace(value.ObjectClass),
			ChangeClass:      strings.TrimSpace(value.ChangeClass),
			Occurrence:       strings.TrimSpace(value.Occurrence),
			ActorObservation: strings.TrimSpace(value.ActorObservation),
			Impact:           strings.TrimSpace(value.Impact),
			Uncertainty:      strings.TrimSpace(value.Uncertainty),
			NoveltyKey:       strings.TrimSpace(value.NoveltyKey),
			MustNot:          cleanStringList(value.MustNot),
		})
	}
	for id := range wire.Situations {
		found := false
		for _, event := range req.Events {
			if strings.TrimSpace(event.EventID) == id {
				found = true
				break
			}
		}
		if !found {
			return BBSWorldSituationProposalDraft{}, fmt.Errorf("situation proposer returned unknown event key %q", id)
		}
	}
	return BBSWorldSituationProposalDraft{Situations: out, Usage: result.Usage}, nil
}

func bbsWorldSituationProposalSchema(events []BBSWorldWindowEvent) map[string]any {
	properties := make(map[string]any, len(events))
	required := make([]string, 0, len(events))
	for _, event := range events {
		id := strings.TrimSpace(event.EventID)
		properties[id] = map[string]any{
			"type": "object",
			"properties": map[string]any{
				"object_class":      map[string]any{"type": "string"},
				"change_class":      map[string]any{"type": "string"},
				"occurrence":        map[string]any{"type": "string"},
				"actor_observation": map[string]any{"type": "string"},
				"impact":            map[string]any{"type": "string"},
				"uncertainty":       map[string]any{"type": "string"},
				"novelty_key":       map[string]any{"type": "string"},
				"must_not":          map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "maxItems": 2},
			},
			"required":             []string{"object_class", "change_class", "occurrence", "actor_observation", "impact", "uncertainty", "novelty_key", "must_not"},
			"additionalProperties": false,
		}
		required = append(required, id)
	}
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"situations": map[string]any{
				"type":                 "object",
				"properties":           properties,
				"required":             required,
				"additionalProperties": false,
			},
		},
		"required":             []string{"situations"},
		"additionalProperties": false,
	}
}
