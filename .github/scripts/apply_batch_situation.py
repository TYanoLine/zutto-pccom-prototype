from pathlib import Path

# provider types / interface
p = Path('apps/server/internal/llm/provider.go')
s = p.read_text()
marker = 'type Provider interface {\n'
insert = r'''// BBSWorldSituationProposalRequest asks for small canonical world-situation
// proposals for a bounded set of already-selected standalone root slots. Unlike
// BBSWorldWindowProduction, this pass does not plan article prose/editorial
// briefs; it only proposes what concretely happened before prose is rendered.
type BBSWorldSituationProposalRequest struct {
	HostName        string
	HostRegion      string
	HostSoftware    string
	WorldDate       string
	WindowStart     string
	WindowEnd       string
	EraRules        string
	RecentBBSState  string
	Events          []BBSWorldWindowEvent
	AvoidSituations []string
}

type BBSWorldSituationDraft struct {
	EventID          string   `json:"event_id"`
	ObjectClass      string   `json:"object_class"`
	ChangeClass      string   `json:"change_class"`
	Occurrence       string   `json:"occurrence"`
	ActorObservation string   `json:"actor_observation"`
	Impact           string   `json:"impact"`
	Uncertainty      string   `json:"uncertainty"`
	NoveltyKey       string   `json:"novelty_key"`
	MustNot          []string `json:"must_not"`
}

type BBSWorldSituationProposalDraft struct {
	Situations []BBSWorldSituationDraft `json:"situations"`
	Usage      TokenUsage                `json:"-"`
}

// BBSWorldSituationProposer sees multiple independent roots at once so it can
// propose diverse concrete world facts in one call. The world layer validates
// and commits accepted proposals before any article worker writes prose.
type BBSWorldSituationProposer interface {
	GenerateBBSWorldSituationProposals(context.Context, BBSWorldSituationProposalRequest) (BBSWorldSituationProposalDraft, error)
}

'''
assert marker in s
s = s.replace(marker, insert + marker, 1)
p.write_text(s)

# OpenAI batch situation proposer
Path('apps/server/internal/llm/openai_world_situation_proposer.go').write_text(r'''package llm

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
- Technical roots should describe observable terminal/call/session behavior without guessing protocols, carrier causes, hardware faults or services.
- Do not introduce new real product/work/service/company/person/place/event names unless supplied canonical evidence explicitly contains them.
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

Never mention AI, prompts, databases, social media, smartphones or anything after the world date.`, withDiegeticWorldFrame(req.EraRules), req.WorldDate, req.WindowStart, req.WindowEnd, req.HostName, req.HostRegion, req.HostSoftware, recent, string(eventsJSON), string(avoidJSON))

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
			"required": []string{"object_class", "change_class", "occurrence", "actor_observation", "impact", "uncertainty", "novelty_key", "must_not"},
			"additionalProperties": false,
		}
		required = append(required, id)
	}
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"situations": map[string]any{
				"type": "object",
				"properties": properties,
				"required": required,
				"additionalProperties": false,
			},
		},
		"required": []string{"situations"},
		"additionalProperties": false,
	}
}
''')

# worldrepo batch planner/validator
Path('apps/server/internal/worldrepo/materialization_batch_situation.go').write_text(r'''package worldrepo

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"

	"zutto-pccom/apps/server/internal/llm"
	"zutto-pccom/apps/server/internal/world"
)

var developmentBatchSituationPoC sync.Map
var developmentBatchSituationTelemetry sync.Map

type developmentBatchSituationKey struct {
	repo   *Repository
	hostID string
}

type developmentBatchSituationStats struct {
	Roots         int
	ProposalCalls int
	Rejected      int
	RepairCalls   int
}

type developmentSituationProposal struct {
	eventID          string
	objectClass      string
	changeClass      string
	occurrence       string
	actorObservation string
	impact           string
	uncertainty      string
	noveltyKey       string
	mustNot          []string
}

type developmentWorldSituationPlan struct {
	proposals map[string]developmentSituationProposal
	usage     GenerationUsage
}

type developmentWorldSituationPlanner interface {
	PlanDevelopmentWorldSituations(context.Context, world.Host, string, []developmentWindowShell, map[string][]world.PersonaFact, string, []string) (developmentWorldSituationPlan, error)
}

func (r *Repository) EnableDevelopmentBatchSituationPoC() {
	developmentBatchSituationPoC.Store(r, true)
}

func developmentBatchSituationPoCEnabled(r *Repository) bool {
	_, ok := developmentBatchSituationPoC.Load(r)
	return ok
}

func (r *Repository) DevelopmentBatchSituationDiagnostic(hostID string) string {
	value, ok := developmentBatchSituationTelemetry.Load(developmentBatchSituationKey{repo: r, hostID: hostID})
	if !ok {
		return ""
	}
	stats := value.(developmentBatchSituationStats)
	return fmt.Sprintf("roots=%d proposal_calls=%d rejected=%d repair_calls=%d", stats.Roots, stats.ProposalCalls, stats.Rejected, stats.RepairCalls)
}

func (m LLMMaterializer) PlanDevelopmentWorldSituations(ctx context.Context, host world.Host, worldDate string, roots []developmentWindowShell, factsByPersona map[string][]world.PersonaFact, recentBBS string, avoid []string) (developmentWorldSituationPlan, error) {
	proposer, ok := m.Renderer.(llm.BBSWorldSituationProposer)
	if !ok {
		return developmentWorldSituationPlan{}, fmt.Errorf("configured renderer does not implement BBS world-situation proposals")
	}
	if len(roots) == 0 {
		return developmentWorldSituationPlan{proposals: map[string]developmentSituationProposal{}}, nil
	}
	events := make([]llm.BBSWorldWindowEvent, 0, len(roots))
	windowStart := roots[0].shell.createdAt
	windowEnd := roots[0].shell.createdAt
	for _, item := range roots {
		shell := item.shell
		if shell.createdAt.Before(windowStart) {
			windowStart = shell.createdAt
		}
		if shell.createdAt.After(windowEnd) {
			windowEnd = shell.createdAt
		}
		existingFacts := make([]string, 0, len(factsByPersona[shell.persona.ID]))
		for _, fact := range factsByPersona[shell.persona.ID] {
			existingFacts = append(existingFacts, "BACKGROUND ONLY: "+fact.Key+"="+fact.Value)
		}
		events = append(events, llm.BBSWorldWindowEvent{
			EventID:        item.eventID,
			BoardID:        item.board.ID,
			BoardName:      item.board.Name,
			AuthorHandle:   shell.persona.Handle,
			CreatedAt:      shell.createdAt.Format(time.RFC3339),
			Action:         shell.action,
			AnchorKey:      shell.anchorKey,
			CauseKind:      shell.causeKind,
			CauseSummary:   shell.causeSummary,
			DiscourseMode:  shell.discourseMode,
			PersonaProfile: personaSummary(shell.persona),
			ExistingFacts:  existingFacts,
		})
	}
	draft, err := proposer.GenerateBBSWorldSituationProposals(ctx, llm.BBSWorldSituationProposalRequest{
		HostName:        host.Name,
		HostRegion:      host.Region,
		HostSoftware:    host.Software,
		WorldDate:       worldDate,
		WindowStart:     windowStart.Format(time.RFC3339),
		WindowEnd:       windowEnd.Format(time.RFC3339),
		EraRules:        m.eraRules(),
		RecentBBSState:  recentBBS,
		Events:          events,
		AvoidSituations: append([]string(nil), avoid...),
	})
	if err != nil {
		return developmentWorldSituationPlan{}, err
	}
	if len(draft.Situations) != len(roots) {
		return developmentWorldSituationPlan{}, fmt.Errorf("situation proposer returned %d situations, want %d", len(draft.Situations), len(roots))
	}
	proposals := make(map[string]developmentSituationProposal, len(draft.Situations))
	for _, situation := range draft.Situations {
		proposals[situation.EventID] = developmentSituationProposal{
			eventID:          strings.TrimSpace(situation.EventID),
			objectClass:      strings.TrimSpace(situation.ObjectClass),
			changeClass:      strings.TrimSpace(situation.ChangeClass),
			occurrence:       strings.TrimSpace(situation.Occurrence),
			actorObservation: strings.TrimSpace(situation.ActorObservation),
			impact:           strings.TrimSpace(situation.Impact),
			uncertainty:      strings.TrimSpace(situation.Uncertainty),
			noveltyKey:       strings.TrimSpace(situation.NoveltyKey),
			mustNot:          append([]string(nil), situation.MustNot...),
		}
	}
	return developmentWorldSituationPlan{proposals: proposals, usage: GenerationUsage{
		InputTokens: draft.Usage.InputTokens, CachedInputTokens: draft.Usage.CachedInputTokens,
		OutputTokens: draft.Usage.OutputTokens, ReasoningTokens: draft.Usage.ReasoningTokens,
		TotalTokens: draft.Usage.TotalTokens, Model: draft.Usage.Model,
	}}, nil
}

func (r *Repository) developmentPlanBatchSituations(host world.Host, window []developmentWindowShell, personas []world.Persona) (map[string]developmentSparseSituation, error) {
	planner, ok := r.Materializer.(developmentWorldSituationPlanner)
	if !ok {
		return nil, fmt.Errorf("configured materializer does not implement batch world-situation planning")
	}
	roots := make([]developmentWindowShell, 0, len(window))
	for _, item := range window {
		if item.shell.action == "thread_start" && item.shell.parentIndex == 0 && item.shell.sourceIndex == 0 {
			roots = append(roots, item)
		}
	}
	stats := developmentBatchSituationStats{Roots: len(roots)}
	if len(roots) == 0 {
		developmentBatchSituationTelemetry.Store(developmentBatchSituationKey{repo: r, hostID: host.ID}, stats)
		return map[string]developmentSparseSituation{}, nil
	}

	factsByPersona := r.existingPersonaFactsByID(personas)
	ctx, cancel := context.WithTimeout(context.Background(), developmentBatchSituationTimeout(len(roots)))
	defer cancel()
	first, err := planner.PlanDevelopmentWorldSituations(ctx, host, r.WorldDate, roots, factsByPersona, planningBBSState(r.Base.ListPosts(host.ID), 24), nil)
	stats.ProposalCalls++
	if err != nil {
		developmentBatchSituationTelemetry.Store(developmentBatchSituationKey{repo: r, hostID: host.ID}, stats)
		return nil, err
	}
	accepted, rejected := developmentValidateSituationProposalBatch(roots, first.proposals, nil)
	usage := first.usage
	stats.Rejected += len(rejected)

	if len(rejected) > 0 {
		retryRoots := make([]developmentWindowShell, 0, len(rejected))
		for _, item := range roots {
			if _, bad := rejected[item.eventID]; bad {
				retryRoots = append(retryRoots, item)
			}
		}
		avoid := developmentSituationAvoidList(roots, accepted)
		for id, reason := range rejected {
			avoid = append(avoid, "REJECTED "+id+": "+reason)
		}
		sort.Strings(avoid)
		repair, repairErr := planner.PlanDevelopmentWorldSituations(ctx, host, r.WorldDate, retryRoots, factsByPersona, planningBBSState(r.Base.ListPosts(host.ID), 24), avoid)
		stats.ProposalCalls++
		stats.RepairCalls++
		if repairErr != nil {
			developmentBatchSituationTelemetry.Store(developmentBatchSituationKey{repo: r, hostID: host.ID}, stats)
			return nil, repairErr
		}
		usage = addDevelopmentGenerationUsage(usage, repair.usage)
		repaired, stillRejected := developmentValidateSituationProposalBatch(retryRoots, repair.proposals, accepted)
		if len(stillRejected) > 0 {
			developmentBatchSituationTelemetry.Store(developmentBatchSituationKey{repo: r, hostID: host.ID}, stats)
			return nil, fmt.Errorf("batch situation repair still rejected %d roots: %v", len(stillRejected), stillRejected)
		}
		for id, proposal := range repaired {
			accepted[id] = proposal
		}
	}

	storeDevelopmentPlanningUsage(r, host.ID, "situation-window", usage)
	developmentBatchSituationTelemetry.Store(developmentBatchSituationKey{repo: r, hostID: host.ID}, stats)
	out := make(map[string]developmentSparseSituation, len(accepted))
	for _, item := range roots {
		proposal, found := accepted[item.eventID]
		if !found {
			return nil, fmt.Errorf("batch situation planning omitted accepted root %q", item.eventID)
		}
		out[item.eventID] = developmentSparseSituationFromProposal(item.shell, proposal)
	}
	return out, nil
}

func developmentBatchSituationTimeout(rootCount int) time.Duration {
	if rootCount > 30 {
		return 360 * time.Second
	}
	if rootCount > 15 {
		return 300 * time.Second
	}
	return 240 * time.Second
}

func developmentValidateSituationProposalBatch(roots []developmentWindowShell, proposals map[string]developmentSituationProposal, seed map[string]developmentSituationProposal) (map[string]developmentSituationProposal, map[string]string) {
	accepted := make(map[string]developmentSituationProposal, len(seed)+len(proposals))
	for id, proposal := range seed {
		accepted[id] = proposal
	}
	rejected := map[string]string{}
	rootByID := make(map[string]developmentWindowShell, len(roots))
	for _, root := range roots {
		rootByID[root.eventID] = root
	}

	noveltyOwner := map[string]string{}
	boardObjectOwner := map[string]map[string]string{}
	boardOccurrences := map[string][]string{}
	for id, proposal := range accepted {
		if root, ok := rootByID[id]; ok {
			developmentSeedProposalIndexes(root, proposal, noveltyOwner, boardObjectOwner, boardOccurrences, id)
		}
	}
	// Seed proposals may belong to roots outside the retry slice. Index them by
	// parsing their own stable event board id when the root is not present here.
	for id, proposal := range accepted {
		if _, ok := rootByID[id]; ok {
			continue
		}
		boardID := developmentBoardIDFromEventID(id)
		developmentSeedProposalIndexes(developmentWindowShell{eventID: id, board: world.Board{ID: boardID}}, proposal, noveltyOwner, boardObjectOwner, boardOccurrences, id)
	}

	for _, root := range roots {
		proposal, ok := proposals[root.eventID]
		if !ok {
			rejected[root.eventID] = "missing proposal"
			continue
		}
		if reason := developmentValidateOneSituationProposal(root, proposal, noveltyOwner, boardObjectOwner, boardOccurrences); reason != "" {
			rejected[root.eventID] = reason
			continue
		}
		accepted[root.eventID] = proposal
		developmentSeedProposalIndexes(root, proposal, noveltyOwner, boardObjectOwner, boardOccurrences, root.eventID)
	}
	// Return only newly accepted proposals plus seed; caller can merge safely.
	return accepted, rejected
}

func developmentValidateOneSituationProposal(root developmentWindowShell, p developmentSituationProposal, noveltyOwner map[string]string, boardObjectOwner map[string]map[string]string, boardOccurrences map[string][]string) string {
	if strings.TrimSpace(p.objectClass) == "" || strings.TrimSpace(p.changeClass) == "" || strings.TrimSpace(p.occurrence) == "" || strings.TrimSpace(p.actorObservation) == "" || strings.TrimSpace(p.noveltyKey) == "" {
		return "required situation field is empty"
	}
	novelty := developmentNormalizeSituationKey(p.noveltyKey)
	if owner := noveltyOwner[novelty]; owner != "" && owner != root.eventID {
		return "novelty_key duplicates unrelated root " + owner
	}
	object := developmentNormalizeSituationKey(p.objectClass)
	if boardObjectOwner[root.board.ID] != nil {
		if owner := boardObjectOwner[root.board.ID][object]; owner != "" && owner != root.eventID {
			return "object_class duplicates another root on the same board " + owner
		}
	}
	for _, prior := range boardOccurrences[root.board.ID] {
		if developmentSituationTextSimilarity(prior, p.occurrence) >= .68 {
			return "occurrence is too similar to another root on the same board"
		}
	}
	return ""
}

func developmentSeedProposalIndexes(root developmentWindowShell, p developmentSituationProposal, noveltyOwner map[string]string, boardObjectOwner map[string]map[string]string, boardOccurrences map[string][]string, id string) {
	novelty := developmentNormalizeSituationKey(p.noveltyKey)
	if novelty != "" {
		noveltyOwner[novelty] = id
	}
	object := developmentNormalizeSituationKey(p.objectClass)
	if boardObjectOwner[root.board.ID] == nil {
		boardObjectOwner[root.board.ID] = map[string]string{}
	}
	if object != "" {
		boardObjectOwner[root.board.ID][object] = id
	}
	if strings.TrimSpace(p.occurrence) != "" {
		boardOccurrences[root.board.ID] = append(boardOccurrences[root.board.ID], p.occurrence)
	}
}

func developmentBoardIDFromEventID(id string) string {
	const prefix = "board-"
	if !strings.HasPrefix(id, prefix) {
		return ""
	}
	rest := strings.TrimPrefix(id, prefix)
	if idx := strings.Index(rest, ":event-"); idx >= 0 {
		return rest[:idx]
	}
	return ""
}

func developmentSituationAvoidList(roots []developmentWindowShell, accepted map[string]developmentSituationProposal) []string {
	rootByID := make(map[string]developmentWindowShell, len(roots))
	for _, root := range roots {
		rootByID[root.eventID] = root
	}
	out := make([]string, 0, len(accepted))
	for id, proposal := range accepted {
		boardID := developmentBoardIDFromEventID(id)
		if root, ok := rootByID[id]; ok {
			boardID = root.board.ID
		}
		out = append(out, fmt.Sprintf("ACCEPTED board=%s object=%s change=%s novelty=%s occurrence=%s", boardID, proposal.objectClass, proposal.changeClass, proposal.noveltyKey, proposal.occurrence))
	}
	return out
}

func developmentSparseSituationFromProposal(shell developmentTimelineShell, p developmentSituationProposal) developmentSparseSituation {
	parts := []string{"BATCH-PROPOSED CANONICAL SITUATION: " + p.occurrence, "Actor observation: " + p.actorObservation}
	if p.impact != "" {
		parts = append(parts, "Immediate impact: "+p.impact)
	}
	if p.uncertainty != "" {
		parts = append(parts, "Uncertainty: "+p.uncertainty)
	}
	facts := []string{
		"world_fact_status=accepted_batch_proposal_before_prose",
		"object_class=" + p.objectClass,
		"change_class=" + p.changeClass,
		"occurrence=" + p.occurrence,
		"actor_observation=" + p.actorObservation,
		"impact=" + p.impact,
		"uncertainty=" + p.uncertainty,
		"novelty_key=" + p.noveltyKey,
		"root_independence=This accepted situation belongs only to this root unless an explicit source edge later references it.",
	}
	for _, value := range p.mustNot {
		if strings.TrimSpace(value) != "" {
			facts = append(facts, "must_not="+strings.TrimSpace(value))
		}
	}
	if shell.discourseMode == "ask_peers" {
		facts = append(facts, "answerability=State the accepted uncertainty/observable detail clearly enough for another member to answer without hidden context.")
	}
	return developmentSparseSituation{kind: "batch_proposed", summary: strings.Join(parts, " "), facts: facts}
}

func developmentNormalizeSituationKey(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var b strings.Builder
	for _, r := range value {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func developmentSituationTextSimilarity(a, b string) float64 {
	aSet := developmentSituationBigrams(developmentNormalizeSituationKey(a))
	bSet := developmentSituationBigrams(developmentNormalizeSituationKey(b))
	if len(aSet) == 0 || len(bSet) == 0 {
		if developmentNormalizeSituationKey(a) == developmentNormalizeSituationKey(b) && developmentNormalizeSituationKey(a) != "" {
			return 1
		}
		return 0
	}
	intersections := 0
	for key := range aSet {
		if _, ok := bSet[key]; ok {
			intersections++
		}
	}
	union := len(aSet) + len(bSet) - intersections
	if union == 0 {
		return 0
	}
	return float64(intersections) / float64(union)
}

func developmentSituationBigrams(value string) map[string]struct{} {
	runes := []rune(value)
	out := map[string]struct{}{}
	if len(runes) < 2 {
		if len(runes) == 1 {
			out[string(runes)] = struct{}{}
		}
		return out
	}
	for i := 0; i+1 < len(runes); i++ {
		out[string(runes[i:i+2])] = struct{}{}
	}
	return out
}

func addDevelopmentGenerationUsage(a, b GenerationUsage) GenerationUsage {
	model := a.Model
	if model == "" {
		model = b.Model
	}
	return GenerationUsage{
		InputTokens: a.InputTokens + b.InputTokens,
		CachedInputTokens: a.CachedInputTokens + b.CachedInputTokens,
		OutputTokens: a.OutputTokens + b.OutputTokens,
		ReasoningTokens: a.ReasoningTokens + b.ReasoningTokens,
		TotalTokens: a.TotalTokens + b.TotalTokens,
		Model: model,
	}
}
''')

# conversation view: configurable shell limit + batch plan before commit
p = Path('apps/server/internal/worldrepo/materialization_conversation_view.go')
s = p.read_text()
old = 'var developmentConversationViewPoC sync.Map\n'
new = '''var developmentConversationViewPoC sync.Map\nvar developmentConversationShellLimits sync.Map\n'''
assert old in s
s = s.replace(old, new, 1)
marker = '''func developmentConversationViewPoCEnabled(r *Repository) bool {\n\t_, ok := developmentConversationViewPoC.Load(r)\n\treturn ok\n}\n'''
addition = marker + r'''

func (r *Repository) SetDevelopmentConversationShellLimit(limit int) {
	if limit < 1 {
		limit = 1
	}
	if limit > 10 {
		limit = 10
	}
	developmentConversationShellLimits.Store(r, limit)
}

func developmentConversationShellLimit(r *Repository) int {
	if value, ok := developmentConversationShellLimits.Load(r); ok {
		return value.(int)
	}
	return developmentWorldWindowPoCMaxShellsPerBoard
}

func limitDevelopmentShellsForConversation(r *Repository, shells []developmentTimelineShell, stats developmentSelectionStats) ([]developmentTimelineShell, developmentSelectionStats) {
	limit := developmentConversationShellLimit(r)
	if len(shells) <= limit {
		return shells, stats
	}
	limited := append([]developmentTimelineShell(nil), shells[:limit]...)
	stats.Posts = len(limited)
	stats.Roots = 0
	stats.Replies = 0
	for _, shell := range limited {
		if shell.action == "reply" {
			stats.Replies++
		} else {
			stats.Roots++
		}
	}
	stats.ROM = stats.Visits - stats.Posts
	if stats.ROM < 0 {
		stats.ROM = 0
	}
	return limited, stats
}
'''
assert marker in s
s = s.replace(marker, addition, 1)
s = s.replace('windowShells := make([]developmentWindowShell, 0, len(boards)*developmentWorldWindowPoCMaxShellsPerBoard)', 'windowShells := make([]developmentWindowShell, 0, len(boards)*developmentConversationShellLimit(r))', 1)
s = s.replace('shells, stats = limitDevelopmentShellsForProducer(shells, stats)', 'shells, stats = limitDevelopmentShellsForConversation(r, shells, stats)', 1)
needle = '''\tsort.SliceStable(windowShells, func(i, j int) bool {\n\t\tif windowShells[i].shell.createdAt.Equal(windowShells[j].shell.createdAt) {\n\t\t\treturn windowShells[i].eventID < windowShells[j].eventID\n\t\t}\n\t\treturn windowShells[i].shell.createdAt.Before(windowShells[j].shell.createdAt)\n\t})\n\n\tcommittedByEventID := map[string]world.Post{}'''
replacement = '''\tsort.SliceStable(windowShells, func(i, j int) bool {\n\t\tif windowShells[i].shell.createdAt.Equal(windowShells[j].shell.createdAt) {\n\t\t\treturn windowShells[i].eventID < windowShells[j].eventID\n\t\t}\n\t\treturn windowShells[i].shell.createdAt.Before(windowShells[j].shell.createdAt)\n\t})\n\n\tbatchSituations := map[string]developmentSparseSituation{}\n\tif developmentBatchSituationPoCEnabled(r) {\n\t\tplanned, err := r.developmentPlanBatchSituations(host, windowShells, personas)\n\t\tif err != nil {\n\t\t\tfor _, board := range boards {\n\t\t\t\tstoreDevelopmentPlanningError(r, host.ID, board.ID, err)\n\t\t\t}\n\t\t\treturn nil, false\n\t\t}\n\t\tbatchSituations = planned\n\t}\n\n\tcommittedByEventID := map[string]world.Post{}'''
assert needle in s
s = s.replace(needle, replacement, 1)
old = '\t\tsituation := r.developmentConversationSituationForShell(host, item.board, shell, out, source)\n'
new = '''\t\tsituation := r.developmentConversationSituationForShell(host, item.board, shell, out, source)\n\t\tif proposed, ok := batchSituations[item.eventID]; ok {\n\t\t\tsituation = proposed\n\t\t}\n'''
assert old in s
s = s.replace(old, new, 1)
p.write_text(s)

# Scale-only board routing metadata for isolated lab boards 4-6.
p = Path('apps/server/internal/worldrepo/materialization_action_seed.go')
s = p.read_text()
old = '''\tcase "3": // 地域の話題\n\t\tif key == "local" {\n\t\t\treturn 1\n\t\t}\n\t\treturn 0\n\tdefault: // フリートーク has intentionally broad root scope.'''
new = '''\tcase "3": // 地域の話題\n\t\tif key == "local" {\n\t\t\treturn 1\n\t\t}\n\t\treturn 0\n\tcase "4": // fresh Lab scale fixture: ゲーム\n\t\tif key == "games" {\n\t\t\treturn 1\n\t\t}\n\t\treturn 0\n\tcase "5": // fresh Lab scale fixture: 音楽\n\t\tif key == "music" {\n\t\t\treturn 1\n\t\t}\n\t\treturn 0\n\tcase "6": // fresh Lab scale fixture: ソフトウェア\n\t\tif key == "software" {\n\t\t\treturn 1\n\t\t}\n\t\treturn 0\n\tdefault: // フリートーク has intentionally broad root scope.'''
assert old in s
s = s.replace(old, new, 1)
old = '''\tcase "3": // 地域の話題\n\t\tswitch key {\n\t\tcase "local":\n\t\t\treturn 1\n\t\tcase "chat":\n\t\t\treturn .55\n\t\tcase "music":\n\t\t\treturn .18\n\t\tcase "games":\n\t\t\treturn .15\n\t\tcase "bbs":\n\t\t\treturn .10\n\t\tdefault:\n\t\t\treturn 0\n\t\t}\n\tdefault: // フリートーク'''
new = '''\tcase "3": // 地域の話題\n\t\tswitch key {\n\t\tcase "local":\n\t\t\treturn 1\n\t\tcase "chat":\n\t\t\treturn .55\n\t\tcase "music":\n\t\t\treturn .18\n\t\tcase "games":\n\t\t\treturn .15\n\t\tcase "bbs":\n\t\t\treturn .10\n\t\tdefault:\n\t\t\treturn 0\n\t\t}\n\tcase "4": // fresh Lab scale fixture: ゲーム\n\t\tif key == "games" { return 1 }; return 0\n\tcase "5": // fresh Lab scale fixture: 音楽\n\t\tif key == "music" { return 1 }; return 0\n\tcase "6": // fresh Lab scale fixture: ソフトウェア\n\t\tif key == "software" { return 1 }; return 0\n\tdefault: // フリートーク'''
assert old in s
s = s.replace(old, new, 1)
p.write_text(s)

# fresh Lab mode + scaling params/diagnostics
p = Path('apps/server/cmd/server/materialization_lab_fresh.go')
s = p.read_text()
s = s.replace('"net/http"\n', '"net/http"\n\t"strconv"\n', 1)
s = s.replace('\tSituationMode      string                        `json:"situation_mode,omitempty"`\n\tCreatedAt', '\tSituationMode      string                        `json:"situation_mode,omitempty"`\n\tBoardCount         int                           `json:"board_count,omitempty"`\n\tShellLimit         int                           `json:"shell_limit,omitempty"`\n\tSituationDiagnostic string                       `json:"situation_diagnostic,omitempty"`\n\tCreatedAt', 1)
s = s.replace('''\tcase "facetless":\n\t\treturn "facetless", true\n\tdefault:''', '''\tcase "facetless":\n\t\treturn "facetless", true\n\tcase "batch":\n\t\treturn "batch", true\n\tdefault:''', 1)
marker = 'func (l *materializationLab) handleFreshStart(w http.ResponseWriter, r *http.Request) {'
helper = r'''func freshIntParam(raw string, fallback, min, max int) (int, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return fallback, true
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value < min || value > max {
		return 0, false
	}
	return value, true
}

func freshScaleBoards(count int) []world.Board {
	catalog := []world.Board{
		{ID: "1", Name: "フリートーク"},
		{ID: "2", Name: "パソコン通信・モデム"},
		{ID: "3", Name: "地域の話題"},
		{ID: "4", Name: "ゲーム"},
		{ID: "5", Name: "音楽"},
		{ID: "6", Name: "ソフトウェア"},
	}
	if count < 3 { count = 3 }
	if count > len(catalog) { count = len(catalog) }
	return append([]world.Board(nil), catalog[:count]...)
}

'''
assert marker in s
s = s.replace(marker, helper + marker, 1)
old = '''\tsituationMode, ok := normalizeFreshSituationMode(r.URL.Query().Get("situation_mode"))\n\tif !ok {\n\t\tw.WriteHeader(http.StatusBadRequest)\n\t\t_ = json.NewEncoder(w).Encode(map[string]string{"error": "situation_mode must be facets or facetless"})\n\t\treturn\n\t}\n\tif !publicLabAdmission.start(w, r, phone, 1) {'''
new = '''\tsituationMode, ok := normalizeFreshSituationMode(r.URL.Query().Get("situation_mode"))\n\tif !ok {\n\t\tw.WriteHeader(http.StatusBadRequest)\n\t\t_ = json.NewEncoder(w).Encode(map[string]string{"error": "situation_mode must be facets, facetless or batch"})\n\t\treturn\n\t}\n\tboardCount, ok := freshIntParam(r.URL.Query().Get("board_count"), 3, 3, 6)\n\tif !ok {\n\t\tw.WriteHeader(http.StatusBadRequest)\n\t\t_ = json.NewEncoder(w).Encode(map[string]string{"error": "board_count must be 3..6"})\n\t\treturn\n\t}\n\tshellLimit, ok := freshIntParam(r.URL.Query().Get("shell_limit"), 5, 1, 10)\n\tif !ok {\n\t\tw.WriteHeader(http.StatusBadRequest)\n\t\t_ = json.NewEncoder(w).Encode(map[string]string{"error": "shell_limit must be 1..10"})\n\t\treturn\n\t}\n\tif !publicLabAdmission.start(w, r, phone, 1) {'''
assert old in s
s = s.replace(old, new, 1)
old = 'job := &materializationFreshJob{ID: id, Status: "queued", Phone: phone, SituationMode: situationMode, CreatedAt: time.Now().UTC()}'
new = 'job := &materializationFreshJob{ID: id, Status: "queued", Phone: phone, SituationMode: situationMode, BoardCount: boardCount, ShellLimit: shellLimit, CreatedAt: time.Now().UTC()}'
assert old in s
s = s.replace(old, new, 1)
old = '''\tsnapshot.Posts = nil\n\tsnapshot.PersonaFacts = map[string][]world.PersonaFact{}\n\tbase := world.NewMemoryStore()'''
new = '''\tsnapshot.Posts = nil\n\tsnapshot.PersonaFacts = map[string][]world.PersonaFact{}\n\tif snapshot.Host.SoftwareID == "materialization-demo" {\n\t\tsnapshot.Boards = freshScaleBoards(job.BoardCount)\n\t}\n\tbase := world.NewMemoryStore()'''
assert old in s
s = s.replace(old, new, 1)
old = '''\trepo := worldrepo.New(base, l.engine, l.materializer, l.worldDate)\n\trepo.EnableDevelopmentConversationViewPoC()\n\tif job.SituationMode == "facetless" {\n\t\trepo.EnableDevelopmentFacetlessSituationPoC()\n\t}\n\thost, err := repo.HostByPhone(job.Phone)'''
new = '''\trepo := worldrepo.New(base, l.engine, l.materializer, l.worldDate)\n\trepo.EnableDevelopmentConversationViewPoC()\n\trepo.SetDevelopmentConversationShellLimit(job.ShellLimit)\n\tif job.SituationMode == "facetless" {\n\t\trepo.EnableDevelopmentFacetlessSituationPoC()\n\t}\n\tif job.SituationMode == "batch" {\n\t\trepo.EnableDevelopmentBatchSituationPoC()\n\t}\n\thost, err := repo.HostByPhone(job.Phone)'''
assert old in s
s = s.replace(old, new, 1)
s = s.replace('deadline := time.Now().Add(10 * time.Minute)', 'deadlineMinutes := 10\n\tif job.BoardCount > 3 || job.ShellLimit > 5 { deadlineMinutes = 15 }\n\tdeadline := time.Now().Add(time.Duration(deadlineMinutes) * time.Minute)', 1)
old = '''\tjob.Usage = repo.MaterializationUsageTotalText()\n\tjob.Articles = articles'''
new = '''\tjob.Usage = repo.MaterializationUsageTotalText()\n\tjob.SituationDiagnostic = repo.DevelopmentBatchSituationDiagnostic(host.ID)\n\tjob.Articles = articles'''
assert old in s
s = s.replace(old, new, 1)
p.write_text(s)

# update fresh mode tests
p = Path('apps/server/cmd/server/materialization_lab_fresh_mode_test.go')
s = p.read_text()
s = s.replace('{" FACETLESS ", "facetless", true},\n\t\t{"other", "", false},', '{" FACETLESS ", "facetless", true},\n\t\t{" batch ", "batch", true},\n\t\t{"other", "", false},', 1)
s += r'''

func TestFreshIntParamAndScaleBoards(t *testing.T) {
	if got, ok := freshIntParam("", 5, 1, 10); !ok || got != 5 { t.Fatalf("default=(%d,%v)", got, ok) }
	if _, ok := freshIntParam("11", 5, 1, 10); ok { t.Fatal("out-of-range value should fail") }
	boards := freshScaleBoards(6)
	if len(boards) != 6 || boards[3].Name != "ゲーム" || boards[5].Name != "ソフトウェア" { t.Fatalf("unexpected boards: %#v", boards) }
}
'''
p.write_text(s)

# validator + schema tests
Path('apps/server/internal/worldrepo/materialization_batch_situation_test.go').write_text(r'''package worldrepo

import (
	"strings"
	"testing"
	"time"

	"zutto-pccom/apps/server/internal/world"
)

func TestDevelopmentBatchSituationValidatorRejectsSameBoardObject(t *testing.T) {
	roots := []developmentWindowShell{
		{eventID: "board-3:event-0001", board: world.Board{ID: "3"}, shell: developmentTimelineShell{createdAt: time.Now()}},
		{eventID: "board-3:event-0002", board: world.Board{ID: "3"}, shell: developmentTimelineShell{createdAt: time.Now().Add(time.Hour)}},
	}
	proposals := map[string]developmentSituationProposal{
		roots[0].eventID: {eventID: roots[0].eventID, objectClass: "資源回収", changeClass: "曜日変更", occurrence: "資源回収の曜日が変わった", actorObservation: "掲示を見た", noveltyKey: "resource-day"},
		roots[1].eventID: {eventID: roots[1].eventID, objectClass: "資源回収", changeClass: "場所変更", occurrence: "資源回収の場所が変わった", actorObservation: "掲示を見た", noveltyKey: "resource-place"},
	}
	accepted, rejected := developmentValidateSituationProposalBatch(roots, proposals, nil)
	if len(accepted) != 1 || len(rejected) != 1 { t.Fatalf("accepted=%d rejected=%v", len(accepted), rejected) }
	if !strings.Contains(rejected[roots[1].eventID], "object_class") { t.Fatalf("wrong rejection: %v", rejected) }
}

func TestDevelopmentBatchSituationValidatorAllowsDifferentObjects(t *testing.T) {
	roots := []developmentWindowShell{
		{eventID: "board-1:event-0001", board: world.Board{ID: "1"}},
		{eventID: "board-1:event-0002", board: world.Board{ID: "1"}},
	}
	proposals := map[string]developmentSituationProposal{
		roots[0].eventID: {eventID: roots[0].eventID, objectClass: "ゲーム内の名前", changeClass: "決めかねた", occurrence: "名前入力で少し迷った", actorObservation: "入力画面で手が止まった", noveltyKey: "game-name"},
		roots[1].eventID: {eventID: roots[1].eventID, objectClass: "帰宅時の雨", changeClass: "急に降った", occurrence: "帰り道で急に雨が強くなった", actorObservation: "歩いていて濡れた", noveltyKey: "rain-walk"},
	}
	accepted, rejected := developmentValidateSituationProposalBatch(roots, proposals, nil)
	if len(accepted) != 2 || len(rejected) != 0 { t.Fatalf("accepted=%d rejected=%v", len(accepted), rejected) }
}

func TestSparseSituationFromBatchProposalIsCanonicalBeforeProse(t *testing.T) {
	got := developmentSparseSituationFromProposal(developmentTimelineShell{discourseMode: "ask_peers"}, developmentSituationProposal{objectClass: "接続後の最初の画面", changeClass: "表示待ち", occurrence: "接続後に最初の画面が出るまで少し待った", actorObservation: "端末上で待ち時間を見た", uncertainty: "時間帯によるか不明", noveltyKey: "first-screen-wait"})
	joined := got.summary + "\n" + strings.Join(got.facts, "\n")
	for _, want := range []string{"batch_proposed", "accepted_batch_proposal_before_prose", "object_class=接続後の最初の画面", "answerability="} {
		if !strings.Contains(got.kind+"\n"+joined, want) { t.Fatalf("missing %q in %s", want, got.kind+"\n"+joined) }
	}
}
''')

Path('apps/server/internal/llm/openai_world_situation_proposer_schema_test.go').write_text(r'''package llm

import "testing"

func TestBBSWorldSituationProposalSchemaKeysExactRoots(t *testing.T) {
	events := []BBSWorldWindowEvent{{EventID: "board-1:event-0001", Action: "thread_start"}, {EventID: "board-3:event-0002", Action: "thread_start"}}
	schema := bbsWorldSituationProposalSchema(events)
	props := schema["properties"].(map[string]any)["situations"].(map[string]any)["properties"].(map[string]any)
	if len(props) != 2 || props["board-1:event-0001"] == nil || props["board-3:event-0002"] == nil { t.Fatalf("unexpected properties: %#v", props) }
}
''')

# Docs
p = Path('docs/CONVERSATION_VIEW_POC.md')
s = p.read_text()
s += r'''

## Batched world-situation proposal PoC

The fresh Lab supports `situation_mode=batch`. World-selected standalone root shells across the bounded host window are sent to one compact Situation Proposer call. The proposer does not write subjects/bodies and does not choose actor/time/board/topology/routing/discourse. It returns free-form `object_class`, `change_class`, `occurrence`, `actor_observation`, `impact`, `uncertainty`, and `novelty_key` fields. These are not selected from a hand-written facet catalog.

The world layer validates proposals before committing them as `PostIntent.Situation*`. Within one board, duplicate object classes and highly similar occurrences are rejected; duplicate novelty keys are rejected host-wide. Rejected roots are retried together at most once while the already accepted situations are supplied as an avoid set. Replies are not separately proposed; they continue to bind to the actual canonical source situation/body. Only after accepted situations are stored does Conversation View render article prose.

For scale testing only, fresh can expand the isolated development snapshot from 3 to at most 6 boards and raise the Conversation View shell limit from 5 to at most 10 per board. These extra boards and posts are never written back to the saved demo world.
'''
p.write_text(s)

p = Path('docs/MATERIALIZATION_LAB.md')
s = p.read_text()
s = s.replace('`situation_mode=facets|facetless`', '`situation_mode=facets|facetless|batch`, `board_count=3..6`, `shell_limit=1..10`')
s += r'''

### fresh batch Situation / scale experiment

`situation_mode=batch` removes the hand-written situation facet/occurrence selection without moving concrete world truth into article prose. All independent roots in the bounded fresh window are proposed together, validated by the world layer, and accepted Situation fields are persisted before ALLBODY prose rendering. A validator rejection triggers at most one batched repair call for only the rejected roots.

`board_count` and `shell_limit` are fresh-isolated load-test controls. Defaults remain 3 boards and 5 shells per board. `board_count>3` adds development-only `ゲーム`, `音楽`, `ソフトウェア` boards to the copied MemoryStore snapshot. Nothing from these scaled runs is written back to the saved development world or ordinary runtime.
'''
p.write_text(s)
