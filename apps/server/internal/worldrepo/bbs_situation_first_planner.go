package worldrepo

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"
	"unicode"

	"zutto-pccom/apps/server/internal/bbsengine"
	"zutto-pccom/apps/server/internal/llm"
	"zutto-pccom/apps/server/internal/world"
	"zutto-pccom/apps/server/internal/worldengine"
)

const (
	productionSituationChunkSize     = 6
	productionSituationChunkAttempts = 3
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
	proposer llm.BBSWorldSituationProposer,
	titlePlanner llm.BBSSituationTitlePlanner,
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
		roots, err := p.planSituationFirstRoots(ctx, materializer, decision, proposer, titlePlanner, req, rootSlots, worldDate)
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
	proposer llm.BBSWorldSituationProposer,
	titlePlanner llm.BBSSituationTitlePlanner,
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
		profile, personaFacts := p.personaTitleContext(slot.AuthorPersonaID)
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
	byEvent := make(map[string]llm.BBSWorldSituationDraft, len(seeds))
	noveltyOwners := map[string]string{}
	// Historical facet labels are internal legacy metadata, not material
	// for any board's open-topic Situation. Pass observed subjects only.
	recentContext := productionRecentSubjectContext(req.RecentPosts)
	avoidSituations := productionRecentSubjectAvoid(req.RecentPosts)

	// Successful chunks are accepted exactly once. If the provider exhausts its
	// output budget, retry fewer pending event shells rather than repeating the
	// same oversized request. Accepted chunks remain available for novelty checks.
	chunkSize := productionSituationChunkSize
	for chunkStart := 0; chunkStart < len(events); {
		chunkEnd := chunkStart + chunkSize
		if chunkEnd > len(events) {
			chunkEnd = len(events)
		}
		chunkEvents := events[chunkStart:chunkEnd]
		accepted := false
		budgetTruncated := false
		var lastErr error
		for attempt := 1; attempt <= productionSituationChunkAttempts && !accepted; attempt++ {
			situationDraft, err := proposer.GenerateBBSWorldSituationProposals(ctx, llm.BBSWorldSituationProposalRequest{
				HostName:                       req.Host.Name,
				HostRegion:                     req.Host.Region,
				HostSoftware:                   req.Host.Software,
				WorldDate:                      worldDate,
				WindowStart:                    windowStart.Format(time.RFC3339),
				WindowEnd:                      windowEnd.Format(time.RFC3339),
				AllowModelHistoricalMemory:     materializer.ModelHistoricalMemory,
				PreferConcreteHistoricalNames:  materializer.PreferConcreteHistoricalNames,
				RecentBBSState:                 recentContext,
				Events:                         chunkEvents,
				AvoidSituations:                append([]string(nil), avoidSituations...),
			})
			if err != nil {
				lastErr = fmt.Errorf("chunk %d..%d attempt %d: %w", chunkStart, chunkEnd, attempt, err)
				if len(chunkEvents) > 1 && errors.Is(err, llm.ErrBBSWorldSituationOutputTruncated) {
					budgetTruncated = true
					break
				}
				continue
			}
			storeDevelopmentPlanningUsage(p.repo, req.Host.ID, "bbs-situation", GenerationUsage{
				InputTokens: situationDraft.Usage.InputTokens, CachedInputTokens: situationDraft.Usage.CachedInputTokens,
				OutputTokens: situationDraft.Usage.OutputTokens, ReasoningTokens: situationDraft.Usage.ReasoningTokens,
				TotalTokens: situationDraft.Usage.TotalTokens, Model: situationDraft.Usage.Model,
			})
			if len(situationDraft.Situations) != len(chunkEvents) {
				lastErr = fmt.Errorf("chunk %d..%d returned %d situations, want %d", chunkStart, chunkEnd, len(situationDraft.Situations), len(chunkEvents))
				continue
			}

			chunkByEvent := make(map[string]llm.BBSWorldSituationDraft, len(chunkEvents))
			chunkNovelty := map[string]string{}
			valid := true
			for _, value := range situationDraft.Situations {
				eventID := strings.TrimSpace(value.EventID)
				_, known := seedByEvent[eventID]
				if eventID == "" || !known {
					lastErr = fmt.Errorf("chunk %d..%d returned invalid event %q", chunkStart, chunkEnd, eventID)
					valid = false
					break
				}
				if _, exists := chunkByEvent[eventID]; exists {
					lastErr = fmt.Errorf("chunk %d..%d duplicated event %q", chunkStart, chunkEnd, eventID)
					valid = false
					break
				}
				if err := validateProductionSituation(value); err != nil {
					lastErr = fmt.Errorf("%s: %w", eventID, err)
					valid = false
					break
				}
				key := normalizeSituationNoveltyKey(value.NoveltyKey)
				if key == "" {
					lastErr = fmt.Errorf("%s returned empty normalized novelty key", eventID)
					valid = false
					break
				}
				if owner := noveltyOwners[key]; owner != "" {
					lastErr = fmt.Errorf("situation novelty duplicates earlier chunk: %s and %s", owner, eventID)
					avoidSituations = append(avoidSituations, "DO NOT REUSE novelty_key="+key)
					valid = false
					break
				}
				if owner := chunkNovelty[key]; owner != "" {
					lastErr = fmt.Errorf("situation novelty duplicates same chunk: %s and %s", owner, eventID)
					avoidSituations = append(avoidSituations, "DO NOT REUSE novelty_key="+key)
					valid = false
					break
				}
				chunkByEvent[eventID] = value
				chunkNovelty[key] = eventID
			}
			if !valid {
				continue
			}
			for _, event := range chunkEvents {
				value, ok := chunkByEvent[event.EventID]
				if !ok {
					lastErr = fmt.Errorf("chunk %d..%d omitted %s", chunkStart, chunkEnd, event.EventID)
					valid = false
					break
				}
				byEvent[event.EventID] = value
				key := normalizeSituationNoveltyKey(value.NoveltyKey)
				noveltyOwners[key] = event.EventID
				avoidSituations = append(avoidSituations,
					"already accepted novelty_key="+key+" occurrence="+strings.TrimSpace(value.Occurrence),
				)
			}
			if valid {
				accepted = true
			}
		}
		if budgetTruncated {
			chunkSize = (len(chunkEvents) + 1) / 2
			log.Printf("BBS Situation output exhausted budget; splitting remaining chunk: host=%s board=%s start=%d previous=%d next=%d", req.Host.ID, req.Board.ID, chunkStart, len(chunkEvents), chunkSize)
			continue
		}
		if !accepted {
			return nil, fmt.Errorf("plan BBS world Situations: %w", lastErr)
		}
		// Keep the smaller size for the rest of this board's batch once a
		// provider response shows that the original chunk was too large.
		chunkStart = chunkEnd
	}

	titleSeeds := make([]llm.BBSSituationTitleSeed, 0, len(seeds))
	type rootState struct {
		seed    productionSituationSeed
		draft   llm.BBSWorldSituationDraft
		summary string
		facts   []string
	}
	states := make(map[string]rootState, len(seeds))
	for _, seed := range seeds {
		draft, ok := byEvent[seed.eventID]
		if !ok {
			return nil, fmt.Errorf("situation planner omitted %s", seed.eventID)
		}
		summary, facts := productionSituationCanonicalState(draft)
		states[seed.eventID] = rootState{seed: seed, draft: draft, summary: summary, facts: facts}
		titleSeeds = append(titleSeeds, llm.BBSSituationTitleSeed{
			EventID:          seed.eventID,
			AuthorHandle:     seed.slot.Author,
			CreatedAt:        seed.slot.CreatedAt.Format(time.RFC3339),
			PersonaProfile:   seed.profile,
			SituationKind:    seed.kind,
			SituationSummary: summary,
			SituationFacts:   facts,
		})
	}

	titleByEvent := make(map[string]string, len(titleSeeds))
	recentSubjects := append([]string(nil), rootSubjects(req.RecentPosts)...)
	for chunkStart := 0; chunkStart < len(titleSeeds); chunkStart += productionTitleChunkSize {
		chunkEnd := chunkStart + productionTitleChunkSize
		if chunkEnd > len(titleSeeds) {
			chunkEnd = len(titleSeeds)
		}
		titleDraft, err := titlePlanner.GenerateBBSSituationTitles(ctx, llm.BBSSituationTitleRequest{
			HostName:       req.Host.Name,
			HostRegion:     req.Host.Region,
			BoardID:        req.Board.ID,
			BoardName:      req.Board.Name,
			BoardScope:     req.Board.SemanticScope,
			WorldDate:      worldDate,
			RecentSubjects: append([]string(nil), recentSubjects...),
			Articles:       titleSeeds[chunkStart:chunkEnd],
		})
		if err != nil {
			return nil, fmt.Errorf("word BBS Situation titles chunk %d..%d: %w", chunkStart, chunkEnd, err)
		}
		storeDevelopmentPlanningUsage(p.repo, req.Host.ID, "bbs-situation-title", GenerationUsage{
			InputTokens: titleDraft.Usage.InputTokens, CachedInputTokens: titleDraft.Usage.CachedInputTokens,
			OutputTokens: titleDraft.Usage.OutputTokens, ReasoningTokens: titleDraft.Usage.ReasoningTokens,
			TotalTokens: titleDraft.Usage.TotalTokens, Model: titleDraft.Usage.Model,
		})
		if len(titleDraft.Titles) != chunkEnd-chunkStart {
			return nil, fmt.Errorf("title chunk %d..%d returned %d titles, want %d", chunkStart, chunkEnd, len(titleDraft.Titles), chunkEnd-chunkStart)
		}
		for _, title := range titleDraft.Titles {
			eventID := strings.TrimSpace(title.EventID)
			subject := strings.TrimSpace(title.Subject)
			if eventID == "" || subject == "" {
				return nil, fmt.Errorf("title chunk %d..%d returned empty event or subject", chunkStart, chunkEnd)
			}
			if _, exists := titleByEvent[eventID]; exists {
				return nil, fmt.Errorf("title planner duplicated event %s", eventID)
			}
			titleByEvent[eventID] = subject
			recentSubjects = append(recentSubjects, subject)
		}
	}

	out := make([]bbsengine.PlannedPost, 0, len(seeds))
	for _, seed := range seeds {
		state := states[seed.eventID]
		subject := titleByEvent[seed.eventID]
		if subject == "" {
			return nil, fmt.Errorf("situation title planner omitted %s", seed.eventID)
		}
		out = append(out, bbsengine.PlannedPost{
			SlotIndex:        seed.slot.Index,
			Subject:          subject,
			Topic:            state.draft.ObjectClass,
			Motivation:       "world_selected_situation",
			Goal:             productionPostGoal,
			AnchorKey:        seed.anchorKey,
			SituationKind:    seed.kind,
			SituationSummary: state.summary,
			SituationFacts:   state.facts,
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

// validateProductionSituation checks only that the proposal is complete. It
// does not depend on any post type: every root has the same fields.
func validateProductionSituation(d llm.BBSWorldSituationDraft) error {
	if strings.TrimSpace(d.ObjectClass) == "" || strings.TrimSpace(d.Occurrence) == "" ||
		strings.TrimSpace(d.PostContent) == "" || strings.TrimSpace(d.NoveltyKey) == "" {
		return fmt.Errorf("required Situation identity is empty")
	}
	return nil
}

func productionSituationCanonicalState(d llm.BBSWorldSituationDraft) (string, []string) {
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
