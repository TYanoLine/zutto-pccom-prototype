package worldrepo

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
		InputTokens:       a.InputTokens + b.InputTokens,
		CachedInputTokens: a.CachedInputTokens + b.CachedInputTokens,
		OutputTokens:      a.OutputTokens + b.OutputTokens,
		ReasoningTokens:   a.ReasoningTokens + b.ReasoningTokens,
		TotalTokens:       a.TotalTokens + b.TotalTokens,
		Model:             model,
	}
}
