package worldrepo

import (
	"context"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"
	"unicode"

	"zutto-pccom/apps/server/internal/bbsengine"
	"zutto-pccom/apps/server/internal/historicalkb"
	"zutto-pccom/apps/server/internal/llm"
	"zutto-pccom/apps/server/internal/world"
	"zutto-pccom/apps/server/internal/worldengine"
)

const (
	// Contextual production generation uses one large structured pool. A second
	// pool is allowed only as recovery if fit/duplicate/historical attrition leaves
	// world-selected roots unresolved.
	sharedTitlePoolMaxSize            = 100
	sharedTitlePoolMinSize            = 60
	sharedTitlePoolReserve            = 24
	sharedTitleLargePoolMaxAttempts   = 2
	sharedTitleFitBatchSize           = 20
	sharedTitleLegacyPoolTargetSize   = 20
	sharedTitleLegacyPoolMaxAttempts  = 6
	sharedTitleResearchBudget         = 6 * time.Second
	sharedTitleBackgroundResearchJobs = 12
)

func sharedContextualTitlePoolSize(remaining int) int {
	if remaining < 0 {
		remaining = 0
	}
	size := remaining + sharedTitlePoolReserve
	if size < sharedTitlePoolMinSize {
		size = sharedTitlePoolMinSize
	}
	if size > sharedTitlePoolMaxSize {
		size = sharedTitlePoolMaxSize
	}
	return size
}

func sharedLegacyTitlePoolAttemptLimit(rootCount int) int {
	required := 0
	if rootCount > 0 {
		required = (rootCount + sharedTitleLegacyPoolTargetSize - 1) / sharedTitleLegacyPoolTargetSize
	}
	attempts := required + 1
	if attempts < 2 {
		attempts = 2
	}
	if attempts > sharedTitleLegacyPoolMaxAttempts {
		attempts = sharedTitleLegacyPoolMaxAttempts
	}
	return attempts
}

func sharedTitleResearchRemaining(deadline *time.Time, now time.Time) time.Duration {
	if deadline.IsZero() {
		*deadline = now.Add(sharedTitleResearchBudget)
	}
	remaining := deadline.Sub(now)
	if remaining < 0 {
		return 0
	}
	return remaining
}

type repositoryBBSBatchPlanner struct {
	repo *Repository
}

func (r *Repository) sharedBBSArticleEngineEnabled(host world.Host) bool {
	if r == nil || host.SoftwareID == "materialization-demo" {
		return false
	}
	var materializer LLMMaterializer
	switch m := r.Materializer.(type) {
	case LLMMaterializer:
		materializer = m
	case *LLMMaterializer:
		materializer = *m
	default:
		return false
	}
	_, hasSituation := materializer.Renderer.(llm.BBSWorldSituationProposer)
	_, hasSituationTitles := materializer.Renderer.(llm.BBSSituationTitlePlanner)
	if hasSituation && hasSituationTitles {
		return true
	}
	_, hasLegacyTitles := materializer.Renderer.(llm.BBSTitleCandidatePlanner)
	return hasLegacyTitles
}

// PlanBBSBatch intentionally keeps the World/wording boundary narrow.
//
// World Engine: actor, time and root/reply topology.
// Title candidate model: proposes a large structured pool of uncommitted subjects
// and marks only real-world/time-dependent claims that need historical checking.
// Jev/OpenAI review: ranks/maps candidates to already-selected root slots.
// Historical gate: verifies only selected claim-bearing titles.
// World: adopts the winning title/summary as canonical.
// Body prose: remains lazy until the article is read.
func (p repositoryBBSBatchPlanner) PlanBBSBatch(ctx context.Context, req bbsengine.BatchRequest) ([]bbsengine.PlannedPost, error) {
	totalStarted := time.Now()
	defer func() {
		log.Printf("BBS timing: host=%s board=%s phase=planner_total duration=%s slots=%d", req.Host.ID, req.Board.ID, time.Since(totalStarted), len(req.Slots))
	}()
	if p.repo == nil {
		return nil, fmt.Errorf("bbs batch planner repository is nil")
	}
	var materializer LLMMaterializer
	switch m := p.repo.Materializer.(type) {
	case LLMMaterializer:
		materializer = m
	case *LLMMaterializer:
		materializer = *m
	default:
		return nil, fmt.Errorf("shared BBS article engine requires LLMMaterializer")
	}
	situationProposer, hasSituationProposer := materializer.Renderer.(llm.BBSWorldSituationProposer)
	situationTitlePlanner, hasSituationTitles := materializer.Renderer.(llm.BBSSituationTitlePlanner)
	titlePlanner, hasLegacyTitles := materializer.Renderer.(llm.BBSTitleCandidatePlanner)
	if !(hasSituationProposer && hasSituationTitles) && !hasLegacyTitles {
		return nil, fmt.Errorf("configured renderer supports neither situation-first nor legacy title planning")
	}

	worldDate := req.WorldNow.Format(time.DateOnly)
	titleAsOf := worldDate
	for _, slot := range req.Slots {
		if slot.CreatedAt.IsZero() {
			continue
		}
		date := slot.CreatedAt.Format(time.DateOnly)
		if date < titleAsOf {
			titleAsOf = date
		}
	}
	// A catch-up batch may represent several weeks of history. Calibrate shared
	// title vocabulary and historical verification to the earliest event date so
	// no later release can leak backward into an earlier article.
	materializer = materializer.withPeriodReferents(worldDate, titleAsOf)
	decision := worldengine.EvidenceDecision{}
	historicalVerificationDisabled := p.repo.debugBBSTitleHistoricalVerificationDisabled()
	if historicalVerificationDisabled {
		log.Printf("BBS DEBUG: host=%s board=%s title_historical_verification=disabled; claim-bearing titles may be adopted without Historical KB/research", req.Host.ID, req.Board.ID)
	}
	if p.repo.Engine != nil && !historicalVerificationDisabled {
		evidenceStarted := time.Now()
		var err error
		decision, err = p.repo.Engine.ResolveEvidence(ctx, worldengine.EvidenceRequest{
			Kind:        historicalkb.KnowledgeCulturalSignal,
			Subject:     req.Board.Name,
			WorldDate:   titleAsOf,
			Region:      req.Host.Region,
			Audience:    []string{"Japanese dial-up BBS users"},
			Need:        fmt.Sprintf("%s の「%s」で、その時点のBBS件名候補に使ってよい時代背景・参照対象", req.Host.Name, req.Board.Name),
			Persistence: true,
			Importance:  .3,
			Specificity: .3,
		})
		log.Printf("BBS timing: host=%s board=%s phase=evidence duration=%s model_first=%t facts=%d", req.Host.ID, req.Board.ID, time.Since(evidenceStarted), decision.ModelFirst, len(decision.Knowledge.Facts))
		if err != nil {
			return nil, fmt.Errorf("resolve BBS title historical context: %w", err)
		}
	}

	if hasSituationProposer && hasSituationTitles {
		planned, err := p.planSituationFirstBatch(ctx, materializer, decision, situationProposer, situationTitlePlanner, req, titleAsOf)
		if err == nil {
			return planned, nil
		}
		if !hasLegacyTitles {
			return nil, err
		}
		// Situation-first is the preferred planner, but a transient structured-output
		// failure must not make an otherwise readable BBS board inaccessible. Fall
		// back atomically to the retained legacy planner for this observation only.
		// No partial Situation-first posts have been committed at this point.
		log.Printf("BBS situation-first fallback: host=%s board=%s err=%v", req.Host.ID, req.Board.ID, err)
	}

	planned := make(map[int]bbsengine.PlannedPost, len(req.Slots))
	rootSlots := make([]bbsengine.Slot, 0, len(req.Slots))
	batchReplySlots := make([]bbsengine.Slot, 0)
	for _, slot := range req.Slots {
		switch {
		case slot.ReplyToPostID == 0 && slot.ReplyToSlotIndex == 0:
			rootSlots = append(rootSlots, slot)
		case slot.ReplyToPostID != 0:
			planned[slot.Index] = bbsengine.PlannedPost{
				SlotIndex:        slot.Index,
				// This is a host-neutral proposed response subject. Host-program
				// projection may keep it, transform it, or discard it entirely.
				Subject:          strings.TrimSpace(slot.ReplyToSubject),
				Topic:            strings.TrimSpace(slot.ReplyToSubject),
				Motivation:       "reply_to_existing_thread",
				Goal:             "respond to the existing thread",
				SituationSummary: fmt.Sprintf("%s が %s の件名「%s」の既存記事へ返信する", slot.Author, slot.ReplyToAuthor, slot.ReplyToSubject),
			}
		default:
			batchReplySlots = append(batchReplySlots, slot)
		}
	}

	if len(rootSlots) > 0 {
		roots, err := p.planRootTitles(ctx, materializer, decision, titlePlanner, req, rootSlots, titleAsOf)
		if err != nil {
			return nil, err
		}
		for _, root := range roots {
			planned[root.SlotIndex] = root
		}
	}
	for _, slot := range batchReplySlots {
		target, ok := planned[slot.ReplyToSlotIndex]
		if !ok || strings.TrimSpace(target.Subject) == "" {
			return nil, fmt.Errorf("same-window reply slot %d cannot resolve root slot %d", slot.Index, slot.ReplyToSlotIndex)
		}
		planned[slot.Index] = bbsengine.PlannedPost{
			SlotIndex:        slot.Index,
			// Keep semantic topic available even for hosts (such as Erika-K)
			// whose native append representation has no independent subject.
			Subject:          target.Subject,
			Topic:            target.Subject,
			Motivation:       "reply_to_same_window_thread",
			Goal:             "respond to the earlier thread in this board history",
			SituationSummary: fmt.Sprintf("%s が %s の件名「%s」の少し前の記事へ返信する", slot.Author, slot.ReplyToAuthor, target.Subject),
		}
	}

	out := make([]bbsengine.PlannedPost, 0, len(req.Slots))
	for _, slot := range req.Slots {
		post, ok := planned[slot.Index]
		if !ok {
			return nil, fmt.Errorf("title-first batch omitted slot %d", slot.Index)
		}
		out = append(out, post)
	}
	return out, nil
}

func (p repositoryBBSBatchPlanner) planRootTitles(
	ctx context.Context,
	materializer LLMMaterializer,
	decision worldengine.EvidenceDecision,
	titlePlanner llm.BBSTitleCandidatePlanner,
	req bbsengine.BatchRequest,
	rootSlots []bbsengine.Slot,
	worldDate string,
) ([]bbsengine.PlannedPost, error) {
	recentState := planningBBSStateWithBodyExcerpts(req.RecentPosts, 48, 8)
	recentSubjects := rootSubjects(req.RecentPosts)
	avoid := append([]string(nil), recentSubjects...)

	events := make([]llm.BBSWorldWindowEvent, 0, len(rootSlots))
	slotByEvent := map[string]bbsengine.Slot{}
	for _, slot := range rootSlots {
		eventID := fmt.Sprintf("slot-%d", slot.Index)
		profile, facts := p.personaTitleContext(slot.AuthorPersonaID)
		event := llm.BBSWorldWindowEvent{
			EventID:        eventID,
			BoardID:        req.Board.ID,
			BoardName:      req.Board.Name,
			AuthorHandle:   slot.Author,
			CreatedAt:      slot.CreatedAt.Format(time.RFC3339),