package worldrepo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"

	"zutto-pccom/apps/server/internal/bbsengine"
	"zutto-pccom/apps/server/internal/llm"
	"zutto-pccom/apps/server/internal/titleshape"
	"zutto-pccom/apps/server/internal/world"
	"zutto-pccom/apps/server/internal/worldengine"
)

const (
	productionSituationChunkSize     = 5
	productionSituationChunkAttempts = 3
	productionSituationConcurrency   = 4
	productionTitleChunkSize         = 20
	// productionPostGoal is deliberately generic. The world layer selects no
	// post type (question, experience, tip...); the form of a post follows from
	// the board name and scope.
	productionPostGoal = "write the post exactly as the canonical situation describes it"
)

type productionSituationSeed struct {
	eventID string
	slot    bbsengine.Slot
	// anchorKey is the board's identity only. It carries no topic meaning and
	// is never derived from board wording, host-specific vocabulary or the
	// author's interests; what may be posted is decided from the board name
	// and its scope.
	anchorKey string
	// kind is internal provenance, not a selected topic or LLM input.
	kind    string
	profile string
	facts   []string
}

func (p repositoryBBSBatchPlanner) planSituationFirstBatch(
	ctx context.Context,
	materializer LLMMaterializer,
	decision worldengine.EvidenceDecision,
	draftPlanner llm.BBSRootDraftPlanner,
	req bbsengine.BatchRequest,
	worldDate string,
) ([]bbsengine.PlannedPost, error) {
	planned := make(map[int]bbsengine.PlannedPost, len(req.Slots))
	rootSlots := make([]bbsengine.Slot, 0, len(req.Slots))
	batchReplies := make([]bbsengine.Slot, 0)

	for _, slot := range req.Slots {
		switch {
		case slot.ReplyToPostID == 0 && slot.ReplyToSlotIndex == 0:
			rootSlots = append(rootSlots, slot)
		case slot.ReplyToPostID != 0:
			planned[slot.Index] = bbsengine.PlannedPost{
				SlotIndex:        slot.Index,
				Subject:          strings.TrimSpace(slot.ReplyToSubject),
				Topic:            strings.TrimSpace(slot.ReplyToSubject),
				Motivation:       "reply_to_existing_thread",
				Goal:             "respond to the existing thread",
				DiscourseMode:    "reply",
				SituationKind:    "reply_to_existing",
				SituationSummary: fmt.Sprintf("%s が %s の件名「%s」の既存記事へ返信する", slot.Author, slot.ReplyToAuthor, slot.ReplyToSubject),
			}
		default:
			batchReplies = append(batchReplies, slot)
		}
	}

	if len(rootSlots) > 0 {
		roots, err := p.planSituationFirstRoots(ctx, materializer, decision, draftPlanner, req, rootSlots, worldDate)
		if err != nil {
			return nil, err
		}
		for _, root := range roots {
			planned[root.SlotIndex] = root
		}
	}

	for _, slot := range batchReplies {
		target, ok := planned[slot.ReplyToSlotIndex]
		if !ok || strings.TrimSpace(target.Subject) == "" {
			return nil, fmt.Errorf("same-window reply slot %d cannot resolve root slot %d", slot.Index, slot.ReplyToSlotIndex)
		}
		planned[slot.Index] = bbsengine.PlannedPost{
			SlotIndex:        slot.Index,
			Subject:          target.Subject,
			Topic:            target.Subject,
			Motivation:       "reply_to_same_window_thread",
			Goal:             "respond to the earlier thread in this board history",
			DiscourseMode:    "reply",
			SituationKind:    "reply_to_same_window",
			SituationSummary: fmt.Sprintf("%s が %s の件名「%s」の少し前の記事へ返信する", slot.Author, slot.ReplyToAuthor, target.Subject),
		}
	}

	out := make([]bbsengine.PlannedPost, 0, len(req.Slots))
	for _, slot := range req.Slots {
		post, ok := planned[slot.Index]
		if !ok {
			return nil, fmt.Errorf("situation-first batch omitted slot %d", slot.Index)
		}
		out = append(out, post)
	}
	return out, nil
}

func (p repositoryBBSBatchPlanner) planSituationFirstRoots(
	ctx context.Context,
	materializer LLMMaterializer,
	decision worldengine.EvidenceDecision,
	draftPlanner llm.BBSRootDraftPlanner,
	req bbsengine.BatchRequest,
	rootSlots []bbsengine.Slot,
	worldDate string,
) ([]bbsengine.PlannedPost, error) {

	seeds := make([]productionSituationSeed, 0, len(rootSlots))
	events := make([]llm.BBSWorldWindowEvent, 0, len(rootSlots))

	for _, slot := range rootSlots {
		// World action/actor authorization must be fixed before the model
		// proposes any Situation. Do not mask an invalid actor in prose.
		if req.Board.RootAuthorPolicy == "sysop_only" && !strings.EqualFold(strings.TrimSpace(slot.Author), "SYSOP") {
			return nil, fmt.Errorf("board %s only permits SYSOP roots; got author %q", req.Board.ID, slot.Author)
		}
		profile, personaFacts := p.personaTitleContext(req.Host, slot.AuthorPersonaID, slot.Author)
		// The board is identified by its ID only. No topic domain is derived
		// from board wording or persona interests: hosts and boards are created
		// dynamically, so what belongs on a board is decided from the board name
		// and its (non-public) scope, which travel in the event shell below.
		anchorKey := req.Board.ID
		// World fixes the posting event (who, when, which board). It does not
		// select a post type: the Situation proposer decides the natural form of
		// the post, and its topic, inside the board name and scope. The author's
		// persisted context only shapes voice and background. No production
		// board receives an activity facet, subject catalog, topic quota or
		// post-type rotation.
		existing := productionOpenSituationMaterials(personaFacts)

		eventID := fmt.Sprintf("slot-%d", slot.Index)

		seeds = append(seeds, productionSituationSeed{
			eventID: eventID, slot: slot, anchorKey: anchorKey,
			kind: "open_topic", profile: profile, facts: existing,
		})
		events = append(events, llm.BBSWorldWindowEvent{
			EventID:        eventID,
			BoardID:        req.Board.ID,
			BoardName:      req.Board.Name,
			BoardScope:     req.Board.SemanticScope,
			AuthorHandle:   slot.Author,
			CreatedAt:      slot.CreatedAt.Format(time.RFC3339),
			Action:         "thread_start",
			AnchorKey:      anchorKey,
			CauseKind:      "board_activity_window",
			PersonaProfile: profile,
			ExistingFacts:  existing,
		})
	}

	windowStart, windowEnd := rootSlots[0].CreatedAt, rootSlots[0].CreatedAt
	for _, slot := range rootSlots[1:] {
		if slot.CreatedAt.Before(windowStart) {
			windowStart = slot.CreatedAt
		}
		if slot.CreatedAt.After(windowEnd) {
			windowEnd = slot.CreatedAt
		}
	}

	seedByEvent := make(map[string]productionSituationSeed, len(seeds))
	for _, seed := range seeds {
		seedByEvent[seed.eventID] = seed
	}

	recentContext := productionRecentSubjectContext(req.RecentPosts)
	avoidSituations := productionRecentSubjectAvoid(req.RecentPosts)
	recentSubjects := rootSubjects(req.RecentPosts)

	// Build top-level chunks of size productionSituationChunkSize (5).
	var initialChunks [][]llm.BBSWorldWindowEvent
	for chunkStart := 0; chunkStart < len(events); chunkStart += productionSituationChunkSize {
		chunkEnd := chunkStart + productionSituationChunkSize
		if chunkEnd > len(events) {
			chunkEnd = len(events)
		}
		initialChunks = append(initialChunks, events[chunkStart:chunkEnd])
	}

	type chunkTask struct {
		index  int
		events []llm.BBSWorldWindowEvent
	}
	type chunkResult struct {
		index  int
		drafts map[string]llm.BBSRootDraft
		usage  GenerationUsage
		err    error
	}

	// Helper to execute a chunk of events (with recursive halving on output budget exhaustion).
	var executeChunkEvents func(chunkEvents []llm.BBSWorldWindowEvent) (map[string]llm.BBSRootDraft, GenerationUsage, error)
	executeChunkEvents = func(chunkEvents []llm.BBSWorldWindowEvent) (map[string]llm.BBSRootDraft, GenerationUsage, error) {
		var lastErr error
		var totalUsage GenerationUsage
		formSeed := fmt.Sprintf("%s|%s|%s", req.Host.ID, req.Board.ID, chunkEvents[0].CreatedAt)
		promptSubjects := recentSubjects
		if len(promptSubjects) > titleCoveredPromptLimit {
			promptSubjects = promptSubjects[len(promptSubjects)-titleCoveredPromptLimit:]
		}
		coveredSubjects := titleshape.ShuffleSeeded(formSeed, promptSubjects)

		for attempt := 1; attempt <= productionSituationChunkAttempts; attempt++ {
			chunkStart := time.Now()
			draftResult, err := draftPlanner.GenerateBBSRootDrafts(ctx, llm.BBSRootDraftRequest{
				HostName:                      req.Host.Name,
				HostRegion:                    req.Host.Region,
				HostSoftware:                  req.Host.Software,
				BoardID:                       req.Board.ID,
				BoardName:                     req.Board.Name,
				BoardScope:                    req.Board.SemanticScope,
				WorldDate:                     worldDate,
				WindowStart:                   windowStart.Format(time.RFC3339),
				WindowEnd:                     windowEnd.Format(time.RFC3339),
				AllowModelHistoricalMemory:    materializer.ModelHistoricalMemory,
				PreferConcreteHistoricalNames: materializer.PreferConcreteHistoricalNames,
				HistoricalFacts:               materializer.historicalFacts(decision),
				RecentBBSState:                recentContext,
				Events:                        chunkEvents,
				AvoidSituations:               append([]string(nil), avoidSituations...),
				RecentSubjects:                coveredSubjects,
				RecentFormFacts:               titleshape.FormFacts(recentSubjects),
				FormSeed:                      formSeed,
				MultiVariant:                  p.repo != nil && p.repo.titleVariants,
			})
			log.Printf("BBS timing: host=%s board=%s phase=situation_chunk duration=%s events=%d",
				req.Host.ID, req.Board.ID, time.Since(chunkStart), len(chunkEvents))

			if err != nil {
				lastErr = fmt.Errorf("attempt %d: %w", attempt, err)
				if len(chunkEvents) > 1 && errors.Is(err, llm.ErrBBSWorldSituationOutputTruncated) {
					// Local recursive halving: process halves sequentially within this chunk.
					log.Printf("BBS Situation output exhausted budget; splitting local chunk: host=%s board=%s events=%d",
						req.Host.ID, req.Board.ID, len(chunkEvents))
					mid := (len(chunkEvents) + 1) / 2
					h1Drafts, u1, err1 := executeChunkEvents(chunkEvents[:mid])
					if err1 != nil {
						return nil, totalUsage, err1
					}
					h2Drafts, u2, err2 := executeChunkEvents(chunkEvents[mid:])
					if err2 != nil {
						return nil, totalUsage, err2
					}
					merged := make(map[string]llm.BBSRootDraft, len(chunkEvents))
					for k, v := range h1Drafts {
						merged[k] = v
					}
					for k, v := range h2Drafts {
						merged[k] = v
					}
					return merged, GenerationUsage{
						InputTokens:       u1.InputTokens + u2.InputTokens,
						CachedInputTokens: u1.CachedInputTokens + u2.CachedInputTokens,
						OutputTokens:      u1.OutputTokens + u2.OutputTokens,
						ReasoningTokens:   u1.ReasoningTokens + u2.ReasoningTokens,
						TotalTokens:       u1.TotalTokens + u2.TotalTokens,
						Model:             u1.Model,
					}, nil
				}
				continue
			}

			totalUsage = GenerationUsage{
				InputTokens:       draftResult.Usage.InputTokens,
				CachedInputTokens: draftResult.Usage.CachedInputTokens,
				OutputTokens:      draftResult.Usage.OutputTokens,
				ReasoningTokens:   draftResult.Usage.ReasoningTokens,
				TotalTokens:       draftResult.Usage.TotalTokens,
				Model:             draftResult.Usage.Model,
			}

			if len(draftResult.Roots) != len(chunkEvents) {
				lastErr = fmt.Errorf("returned %d root drafts, want %d", len(draftResult.Roots), len(chunkEvents))
				continue
			}

			valid := true
			for _, event := range chunkEvents {
				draft, ok := draftResult.Roots[event.EventID]
				if !ok {
					lastErr = fmt.Errorf("omitted event %s", event.EventID)
					valid = false
					break
				}
				if err := validateProductionDraft(draft); err != nil {
					lastErr = fmt.Errorf("%s: %w", event.EventID, err)
					valid = false
					break
				}
			}
			if !valid {
				continue
			}

			return draftResult.Roots, totalUsage, nil
		}
		return nil, totalUsage, fmt.Errorf("plan BBS root drafts: %w", lastErr)
	}

	results := make([]chunkResult, len(initialChunks))
	var wg sync.WaitGroup
	sem := make(chan struct{}, productionSituationConcurrency)

	for i, chunk := range initialChunks {
		wg.Add(1)
		go func(idx int, ch []llm.BBSWorldWindowEvent) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			drafts, usage, err := executeChunkEvents(ch)
			results[idx] = chunkResult{index: idx, drafts: drafts, usage: usage, err: err}
		}(i, chunk)
	}
	wg.Wait()

	// Check errors and record usage deterministically in chunk order.
	byEvent := make(map[string]llm.BBSRootDraft, len(events))
	noveltyOwners := map[string]string{}

	for _, res := range results {
		if res.err != nil {
			return nil, res.err
		}
		storeDevelopmentPlanningUsage(p.repo, req.Host.ID, "bbs-root-draft", res.usage)
		for eventID, draft := range res.drafts {
			key := normalizeSituationNoveltyKey(draft.NoveltyKey)
			if owner, exists := noveltyOwners[key]; exists && key != "" {
				log.Printf("BBS Situation novelty overlap: host=%s board=%s key=%s events=%s,%s",
					req.Host.ID, req.Board.ID, key, owner, eventID)
			} else if key != "" {
				noveltyOwners[key] = eventID
			}
			byEvent[eventID] = draft
		}
	}

	orderedEventIDs := make([]string, 0, len(seeds))
	for _, seed := range seeds {
		orderedEventIDs = append(orderedEventIDs, seed.eventID)
	}

	titleByEvent, diag := p.selectDraftTitles(req, orderedEventIDs, byEvent, recentSubjects)

	if p.repo.shouldLogGeneratedContent(req.Host) {
		items := make([]titleshape.Item, 0, len(seeds))
		for _, seed := range seeds {
			draft := byEvent[seed.eventID]
			summary, _ := productionDraftCanonicalState(draft)
			items = append(items, titleshape.Item{Subject: titleByEvent[seed.eventID], Summary: summary})
		}
		diag.Report = titleshape.Measure(items, nil)
		diag.Host, diag.Board = req.Host.ID, req.Board.ID
		if encoded, err := json.Marshal(diag); err == nil {
			log.Printf("BBS title shape: %s", encoded)
		}
	}

	out := make([]bbsengine.PlannedPost, 0, len(seeds))
	for _, seed := range seeds {
		draft := byEvent[seed.eventID]
		summary, facts := productionDraftCanonicalState(draft)
		subject := titleByEvent[seed.eventID]
		if subject == "" {
			return nil, fmt.Errorf("root draft planner omitted subject for %s", seed.eventID)
		}
		topics := append([]string(nil), draft.Topics...)
		if len(topics) > 5 {
			topics = topics[:5]
		}
		out = append(out, bbsengine.PlannedPost{
			SlotIndex:                  seed.slot.Index,
			Subject:                    subject,
			Topic:                      draft.ObjectClass,
			Topics:                     topics,
			Motivation:                 "world_selected_situation",
			Goal:                       productionPostGoal,
			AnchorKey:                  seed.anchorKey,
			SituationKind:              seed.kind,
			SituationSummary:           summary,
			SituationFacts:             facts,
			ArticleDetailsMaterialized: true,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].SlotIndex < out[j].SlotIndex })
	return out, nil
}

// All live boards use the same open-topic inputs. Only persisted persona
// facts are supplied, as background for the author's voice; board identity
// and posting purpose travel separately in each canonical World event shell.
func productionOpenSituationMaterials(personaFacts []string) []string {
	facts := make([]string, 0, len(personaFacts))
	for _, fact := range personaFacts {
		facts = append(facts, "persona_context="+fact)
	}
	return facts
}

func productionRecentSubjectContext(posts []world.Post) string {
	lines := make([]string, 0, 20)
	for i := len(posts)-1; i>=0 && len(lines)<20; i-- {
		if post := posts[i]; world.IsSemanticRoot(post) && strings.TrimSpace(post.Subject)!="" {
			lines = append(lines, "subject="+strings.TrimSpace(post.Subject))
		}
	}
	if len(lines)==0 { return "(none supplied)" }
	return strings.Join(lines, "\n")
}

func productionRecentSubjectAvoid(posts []world.Post) []string {
	lines := make([]string, 0, 20)
	for i := len(posts)-1; i>=0 && len(lines)<20; i-- {
		if post := posts[i]; world.IsSemanticRoot(post) && strings.TrimSpace(post.Subject)!="" {
			lines = append(lines, "subject="+strings.TrimSpace(post.Subject))
		}
	}
	return lines
}

func validateProductionDraft(d llm.BBSRootDraft) error {
	if strings.TrimSpace(d.ObjectClass) == "" || strings.TrimSpace(d.Occurrence) == "" ||
		strings.TrimSpace(d.PostContent) == "" || strings.TrimSpace(d.NoveltyKey) == "" {
		return fmt.Errorf("required Situation identity is empty")
	}
	hasValidTitle := false
	for _, t := range d.Titles {
		if strings.TrimSpace(t) != "" {
			hasValidTitle = true
			break
		}
	}
	if !hasValidTitle {
		return fmt.Errorf("no valid titles")
	}
	return nil
}

func productionDraftCanonicalState(d llm.BBSRootDraft) (string, []string) {
	occurrence := strings.TrimSpace(d.Occurrence)
	content := strings.TrimSpace(d.PostContent)
	facts := []string{
		"world_fact_status=accepted_situation_before_subject",
		"object_class=" + strings.TrimSpace(d.ObjectClass),
		"occurrence=" + occurrence,
		"post_content=" + content,
		"novelty_key=" + strings.TrimSpace(d.NoveltyKey),
	}
	for _, value := range d.MustNot {
		if value = strings.TrimSpace(value); value != "" {
			facts = append(facts, "must_not="+value)
		}
	}
	return strings.TrimSpace(occurrence + " " + content), facts
}

func normalizeSituationNoveltyKey(value string) string {
    value = strings.ToLower(strings.TrimSpace(value))
    var b strings.Builder
    for _,r := range value {
        if unicode.IsLetter(r) || unicode.IsDigit(r) { b.WriteRune(r) }
    }
    return b.String()
}
