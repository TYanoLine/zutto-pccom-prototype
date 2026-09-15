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
	historicalPolicy := bbsWorldSituationHistoricalPolicy(req)
	recent := strings.TrimSpace(req.RecentBBSState)
	if recent == "" {
		recent = "(no earlier canonical BBS state supplied)"
	}
	prompt := fmt.Sprintf(`You propose WORLD FACTS for a bounded slice of a fictional Japanese grass-roots BBS world around 1996.

This is NOT article writing. Do not write subject lines or post bodies. For each already-selected standalone root slot, propose exactly one small concrete contemporaneous situation that the world engine could accept as canonical BEFORE prose is rendered.

IMMUTABLE INPUT:
- event existence, actor, time, board, routing domain, cause kind and discourse mode are already fixed.
- every supplied event is an independent standalone root. Do not connect two roots into one incident, conversation or hidden shared event. Two independent actors may discuss the same publicly available work/product without sharing an experience or reading each other's posts.
- routing domains are broad constraints, not topic menus. Invent a situation freely inside the domain; do not choose from a predefined event/facet catalog.

BATCH DIVERSITY:
- You see the whole root batch specifically so you can avoid repetition.
- Within the same board, vary the matters people discuss, not just names. The same work/product may recur with genuinely different questions or impressions; do not reuse an occurrence.
- Across the host window, do not repeat the same distinctive occurrence with paraphrased wording.
- object_class and change_class are FREE descriptive labels, not enums. Make object_class concrete enough to distinguish, for example, one practical local object/activity from another; do not return only the broad routing domain.
- novelty_key is a short normalized semantic key for duplicate detection. Unrelated roots must have different novelty_key values.

WORLD-TRUTH BOUNDARY:
- TARGET BEFORE MATTER: identify the contemporary subject within the selected board/domain FIRST, then propose this actor's modest involvement and conversational purpose about it. Use only historically permitted names. Do not first invent an anonymous puzzle or malfunction and attach an arbitrary name afterwards.
- A supplied TOPIC TARGET SELECTED BY WORLD is immutable for this root. Establish the new occurrence around that exact name and supplied evidence. Include it in object_class and occurrence. The evidence establishes public availability, not ownership or personal history. Do not invent a platform/edition, plot, mechanic, release status or product detail not supported by the evidence.
- An already-selected posting action may be motivated by an impression, preference, ordinary curiosity or wish to discuss the target. No malfunction, state change or dramatic novelty is required. This permission does not create extra actions or override the fixed discourse_mode.
- Propose only observable or modestly inferable facts. Do not invent that a SYSOP checked logs, a machine failed internally, a phone network caused something, or an earlier post existed unless supplied canonical state establishes it.
- Technical roots should describe observable behavior appropriate to the selected board/domain; software roots need not become terminal/call/session incidents. Do not guess protocols, carrier causes or internal hardware faults.
%s
- SEARCH-GROUNDED RETRY: an event may contain existing_facts beginning "GROUNDING ORIGINAL SITUATION - PRESERVE SEMANTICS:" and "GROUNDING WORLD-SELECTED REFERENT - MUST USE:". For such an event, the search stage has already produced historically valid candidates and the Go world layer has already selected exactly one referent as the new world fact for this event. You MUST use that selected referent to replace the generic object while preserving the original activity, change, actor observation, impact, uncertainty and novelty_key. Do not choose an alternative candidate and do not fall back to a generic label merely because the original Situation did not uniquely imply the name; this is creation of a new canonical fact, not reconstruction of a hidden past fact. The selected actor may modestly play, use, hear, watch, read, visit or discuss that referent as required by the original occurrence. This does not establish ownership, purchase history, long-term fandom, unrelated biography or extra product facts. Never emit a must_not rule forbidding the selected referent; use must_not only to prevent unsupported extra details.
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

Never mention AI, prompts, databases, social media, smartphones or anything after the world date.`, historicalPolicy, withDiegeticWorldFrame(req.EraRules), req.WorldDate, req.WindowStart, req.WindowEnd, req.HostName, req.HostRegion, req.HostSoftware, historicalFacts, recent, string(eventsJSON), string(avoidJSON))

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

func bbsWorldSituationHistoricalPolicy(req BBSWorldSituationProposalRequest) string {
	if req.AllowModelHistoricalMemory {
		if req.PreferConcreteHistoricalNames {
			return "- This model-memory concrete-name experiment intentionally supplies no proper-noun dictionary or referent list. IMPORTANT CANONICALIZATION RULE: this Situation-proposal pass is itself the step that creates new canonical world state for each standalone root. Therefore, when the already-selected situation naturally corresponds to a real product/work/service/company/person/place/event/news/cultural reference that you are confident existed and was knowable in Japan by the event date, PREFER that concrete historical name over a generic label. It is valid for this new canonical occurrence to establish that the selected actor played, used, read, watched, heard, visited, or talked about that named thing when that involvement is the modest event being proposed; no earlier actor-use fact is required for that new occurrence. This does NOT establish persistent ownership, purchase history, long-term preference, compatibility, or unrelated biography. This is a concretization preference, not a quota: never change the selected topic in order to use a name, never add names as period decoration, and do not repeat one favorite name across unrelated roots. If uncertain about existence, timing, identity or details, stay generic. Do not put a generic 'do not name the title/product/device' constraint in must_not merely because no dictionary was supplied; if you choose a real name, use must_not only to prevent unsupported extra details about it."
		}
		return "- This model-memory experiment intentionally supplies no proper-noun dictionary. You may introduce real product/work/service/company/person/place/event/news/cultural names from your own historical knowledge only when confident they existed and were knowable in Japan by the world date. If uncertain, stay generic. Do not invent details merely because you recognize a name."
	}
	return "- Do not introduce new real product/work/service/company/person/place/event names unless SUPPLIED HISTORICAL TEXTURE below explicitly permits them or supplied canonical evidence contains them."
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
