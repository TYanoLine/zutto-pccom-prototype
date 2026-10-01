package worldrepo

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"zutto-pccom/apps/server/internal/bbsengine"
	"zutto-pccom/apps/server/internal/historicalkb"
	"zutto-pccom/apps/server/internal/llm"
	"zutto-pccom/apps/server/internal/world"
	"zutto-pccom/apps/server/internal/worldengine"
)

const (
	productionSituationChunkSize     = 6
	productionSituationChunkAttempts = 3
	productionTitleChunkSize         = 20
)

type productionSituationSeed struct {
	eventID string
	slot    bbsengine.Slot
	mode    string
	domain  string
	sparse  developmentSparseSituation
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
	counts := recentSituationKindCounts(req.RecentPosts)
	seeds := make([]productionSituationSeed, 0, len(rootSlots))
	events := make([]llm.BBSWorldWindowEvent, 0, len(rootSlots))

	for ordinal, slot := range rootSlots {
		profile, personaFacts := p.personaTitleContext(slot.AuthorPersonaID)
		persona := p.personaForSituation(slot)
		domain := productionBoardDomain(req.Board, persona)
		mode := demoSelectRootDiscourseMode(req.Host, req.Board, ordinal)
		facet, ok := chooseProductionSituationFacet(req.Host, req.Board, persona, slot.CreatedAt, slot.Index, domain, mode, counts)
		if !ok {
			return nil, fmt.Errorf("no Situation facet for board=%s domain=%s mode=%s", req.Board.ID, domain, mode)
		}
		// Production uses the World-selected activity focus, not the detailed
		// diagnostic PoC's anonymous-game example and exclusionary boundary.
		// The proposer chooses the concrete occurrence before it is canonical.
		sparse := productionSituationFocus(facet)
		counts[facet.kind]++

		eventID := fmt.Sprintf("slot-%d", slot.Index)
		existing := make([]string, 0, len(personaFacts)+len(sparse.facts)+2)
		for _, fact := range personaFacts {
			existing = append(existing, "persona_context="+fact)
		}
		existing = append(existing, "situation_kind="+facet.kind)
		existing = append(existing, sparse.facts...)
		for _, fact := range productionEventPeriodFacts(req.Host, req.Board, persona, domain, slot.CreatedAt) {
			existing = append(existing, "period_reference="+fact)
		}

		seeds = append(seeds, productionSituationSeed{
			eventID: eventID, slot: slot, mode: mode, domain: domain,
			sparse: sparse, profile: profile, facts: existing,
		})
		events = append(events, llm.BBSWorldWindowEvent{
			EventID:        eventID,
			BoardID:        req.Board.ID,
			BoardName:      req.Board.Name,
			AuthorHandle:   slot.Author,
			CreatedAt:      slot.CreatedAt.Format(time.RFC3339),
			Action:         "thread_start",
			AnchorKey:      domain,
			CauseKind:      "board_activity_window",
			DiscourseMode:  mode,
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
	avoidSituations := append([]string(nil), productionRecentSituationAvoid(req.RecentPosts)...)

	for chunkStart := 0; chunkStart < len(events); chunkStart += productionSituationChunkSize {
		chunkEnd := chunkStart + productionSituationChunkSize
		if chunkEnd > len(events) {
			chunkEnd = len(events)
		}
		chunkEvents := events[chunkStart:chunkEnd]
		accepted := false
		var lastErr error
		for attempt := 1; attempt <= productionSituationChunkAttempts && !accepted; attempt++ {
			situationDraft, err := proposer.GenerateBBSWorldSituationProposals(ctx, llm.BBSWorldSituationProposalRequest{
				HostName:                       req.Host.Name,
				HostRegion:                     req.Host.Region,
				HostSoftware:                   req.Host.Software,
				WorldDate:                      worldDate,
				WindowStart:                    windowStart.Format(time.RFC3339),
				WindowEnd:                      windowEnd.Format(time.RFC3339),
				HistoricalFacts:                productionHistoricalFacts(materializer, decision),
				AllowModelHistoricalMemory:     materializer.ModelHistoricalMemory,
				PreferConcreteHistoricalNames:  materializer.PreferConcreteHistoricalNames,
				RecentBBSState:                 productionRecentSituationContext(req.RecentPosts),
				Events:                         chunkEvents,
				AvoidSituations:                append([]string(nil), avoidSituations...),
			})
			if err != nil {
				lastErr = fmt.Errorf("chunk %d..%d attempt %d: %w", chunkStart, chunkEnd, attempt, err)
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
				seed, known := seedByEvent[eventID]
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
				if err := validateProductionTypedSituation(seed.mode, value); err != nil {
					lastErr = fmt.Errorf("%s: %w", eventID, err)
					valid = false
					break
				}
				key := developmentNormalizeSituationKey(value.NoveltyKey)
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
				key := developmentNormalizeSituationKey(value.NoveltyKey)
				noveltyOwners[key] = event.EventID
				avoidSituations = append(avoidSituations,
					"already accepted novelty_key="+key+" occurrence="+strings.TrimSpace(value.Occurrence),
				)
			}
			if valid {
				accepted = true
			}
		}
		if !accepted {
			return nil, fmt.Errorf("plan BBS world Situations: %w", lastErr)
		}
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
		summary, facts := productionSituationCanonicalState(seed, draft)
		states[seed.eventID] = rootState{seed: seed, draft: draft, summary: summary, facts: facts}
		titleSeeds = append(titleSeeds, llm.BBSSituationTitleSeed{
			EventID:          seed.eventID,
			AuthorHandle:     seed.slot.Author,
			CreatedAt:        seed.slot.CreatedAt.Format(time.RFC3339),
			DiscourseMode:    seed.mode,
			PersonaProfile:   seed.profile,
			SituationKind:    seed.sparse.kind,
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
			Stance:           state.draft.Stance,
			Goal:             productionDiscourseGoal(seed.mode),
			AnchorKey:        seed.domain,
			DiscourseMode:    seed.mode,
			SituationKind:    seed.sparse.kind,
			SituationSummary: state.summary,
			SituationFacts:   state.facts,
			ArticleDetailsMaterialized: true,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].SlotIndex < out[j].SlotIndex })
	return out, nil
}

func (p repositoryBBSBatchPlanner) personaForSituation(slot bbsengine.Slot) world.Persona {
	if store, ok := p.repo.Base.(world.PersonaStore); ok && slot.AuthorPersonaID != "" {
		if persona, found := store.PersonaByID(slot.AuthorPersonaID); found {
			return persona
		}
	}
	return world.Persona{ID: slot.AuthorPersonaID, Handle: slot.Author}
}

func productionHistoricalFacts(_ LLMMaterializer, decision worldengine.EvidenceDecision) []string {
	combined := append([]string(nil), usableClaims(decision)...)
	out := make([]string, 0, len(combined))
	seen := map[string]bool{}
	for _, fact := range combined {
		fact = strings.TrimSpace(fact)
		if fact == "" || seen[fact] {
			continue
		}
		seen[fact] = true
		out = append(out, fact)
		if len(out) >= 32 {
			break
		}
	}
	return out
}

func productionEventPeriodFacts(host world.Host, board world.Board, persona world.Persona, domain string, at time.Time) []string {
	items := historicalkb.PeriodReferents(at.Format(time.DateOnly))
	candidates := make([]string, 0, len(items))
	for _, item := range items {
		claim := strings.TrimSpace(item.Claim)
		lower := strings.ToLower(item.Name + " " + claim)
		relevant := false
		switch domain {
		case "games":
			for _, marker := range []string{"ゲーム", "ソフト", "rpg", "シューティング", "playstation", "セガサターン", "スーパーファミコン", "ゲームボーイ", "アクション"} {
				if strings.Contains(lower, strings.ToLower(marker)) { relevant = true; break }
			}
		case "software":
			for _, marker := range []string{"windows", "一太郎", "ワープロ", "ソフト"} {
				if strings.Contains(lower, marker) { relevant = true; break }
			}
		case "communications", "modem":
			for _, marker := range []string{"nifty", "パソコン通信", "pc-9801"} {
				if strings.Contains(lower, marker) { relevant = true; break }
			}
		case "hardware":
			for _, marker := range []string{"pc-9801", "セガサターン", "playstation", "音源機器"} {
				if strings.Contains(lower, marker) { relevant = true; break }
			}
		case "music":
			relevant = strings.Contains(lower, "sc-55") || strings.Contains(lower, "音源")
		}
		if relevant && claim != "" {
			candidates = append(candidates, claim)
		}
	}
	if len(candidates) <= 6 {
		return candidates
	}
	start := int(demoStableUnit(host.ID, board.ID, persona.ID, at.Format(time.RFC3339), "event-period-referents-v1") * float64(len(candidates)))
	if start >= len(candidates) { start = len(candidates)-1 }
	out := make([]string, 0, 6)
	for i := 0; i < 6; i++ {
		out = append(out, candidates[(start+i)%len(candidates)])
	}
	return out
}

func productionBoardDomain(board world.Board, persona world.Persona) string {
	text := strings.ToLower(strings.TrimSpace(board.Name + " " + board.SemanticScope))
	checks := []struct {
		words  []string
		domain string
	}{
		{[]string{"ゲーム", "game"}, "games"},
		{[]string{"音楽", "music"}, "music"},
		{[]string{"ソフト", "software"}, "software"},
		{[]string{"モデム", "modem"}, "modem"},
		{[]string{"パソコン通信", "通信", "bbs"}, "communications"},
		{[]string{"地域", "local"}, "local"},
		{[]string{"ハード", "hardware"}, "hardware"},
	}
	for _, check := range checks {
		for _, word := range check.words {
			if strings.Contains(text, word) {
				return check.domain
			}
		}
	}
	best, bestScore := "chat", 0.0
	for key, score := range persona.Interests {
		if score > bestScore {
			best, bestScore = key, score
		}
	}
	return best
}

func chooseProductionSituationFacet(host world.Host, board world.Board, persona world.Persona, at time.Time, slotIndex int, domain, mode string, counts map[string]int) (developmentSituationFacet, bool) {
	candidates := make([]developmentSituationFacet, 0)
	if domain == "games" {
		for _, rich := range append(developmentRichGameSituationFacets(), productionGameTopicFacets()...) {
			if developmentModeFacetAllowed(rich, mode) {
				candidates = append(candidates, rich.developmentSituationFacet)
			}
		}
	} else {
		candidates = append(candidates, developmentSituationFacets(domain)...)
	}
	if len(candidates) == 0 {
		candidates = append(candidates, developmentSituationFacets("generic")...)
	}
	if len(candidates) == 0 {
		return developmentSituationFacet{}, false
	}

	weights := make([]float64, len(candidates))
	total := 0.0
	for i, facet := range candidates {
		// Strongly downweight kinds already used in the same retained window.
		weight := 1.0 / float64(1+counts[facet.kind]*4)
		weights[i] = weight
		total += weight
	}
	roll := demoStableUnit(host.ID, board.ID, persona.ID, at.Format(time.RFC3339), fmt.Sprintf("production-situation-v2-%d", slotIndex)) * total
	for i, facet := range candidates {
		if roll < weights[i] {
			return facet, true
		}
		roll -= weights[i]
	}
	return candidates[len(candidates)-1], true
}

// productionGameTopicFacets are broad *activity* directions, not a named-title
// dictionary. They give ordinary BBS conversation about particular works a
// chance alongside gameplay-habit posts. The proposer still chooses any exact
// referent before the Situation is materialized, using event-date evidence.
func productionGameTopicFacets() []developmentModeSituationFacet {
	return []developmentModeSituationFacet{
		{
			developmentSituationFacet: developmentSituationFacet{
				kind:  "games_particular_work_impression",
				focus: "the member's impression of a game",
			},
			modes: developmentModeSet("share_observation", "share_experience", "state_opinion"),
		},
		{
			developmentSituationFacet: developmentSituationFacet{
				kind:  "games_particular_work_interest",
				focus: "interest in a game the member has encountered or heard about",
			},
			modes: developmentModeSet("share_observation", "state_opinion", "ask_peers"),
		},
		{
			developmentSituationFacet: developmentSituationFacet{
				kind:  "games_particular_work_choice",
				focus: "choosing or comparing games",
			},
			modes: developmentModeSet("state_opinion", "ask_peers"),
		},
		{
			developmentSituationFacet: developmentSituationFacet{
				kind:  "games_particular_work_tip",
				focus: "a practical finding from playing a game",
			},
			modes: developmentModeSet("share_experience", "share_tip"),
		},
	}
}

// Production passes the World-selected activity focus as material. Diagnostic
// example incidents and their wording constraints belong to the Lab only.
// Concrete occurrences are first proposed here, then become canonical state.
func productionSituationFocus(facet developmentSituationFacet) developmentSparseSituation {
	return developmentSparseSituation{
		kind:    facet.kind,
		summary: facet.focus,
		facts:   []string{"activity_focus=" + facet.focus},
	}
}

func recentSituationKindCounts(posts []world.Post) map[string]int {
	out := map[string]int{}
	for _, post := range posts {
		if !world.IsSemanticRoot(post) {
			continue
		}
		if kind := strings.TrimSpace(post.Intent.SituationKind); kind != "" && kind != "title_first" {
			out[kind]++
		}
	}
	return out
}

func productionRecentSituationContext(posts []world.Post) string {
	lines := make([]string, 0, 20)
	for i := len(posts) - 1; i >= 0 && len(lines) < 20; i-- {
		post := posts[i]
		if !world.IsSemanticRoot(post) {
			continue
		}
		lines = append(lines, fmt.Sprintf("subject=%s | situation_kind=%s", strings.TrimSpace(post.Subject), strings.TrimSpace(post.Intent.SituationKind)))
	}
	if len(lines) == 0 {
		return "(none supplied)"
	}
	return strings.Join(lines, "\n")
}

func productionRecentSituationAvoid(posts []world.Post) []string {
	out := make([]string, 0, 20)
	for i := len(posts) - 1; i >= 0 && len(out) < 20; i-- {
		post := posts[i]
		if !world.IsSemanticRoot(post) {
			continue
		}
		out = append(out, fmt.Sprintf("kind=%s subject=%s", post.Intent.SituationKind, post.Subject))
	}
	return out
}

func validateProductionTypedSituation(mode string, d llm.BBSWorldSituationDraft) error {
	if strings.TrimSpace(d.ObjectClass) == "" || strings.TrimSpace(d.Occurrence) == "" || strings.TrimSpace(d.NoveltyKey) == "" {
		return fmt.Errorf("required Situation identity is empty")
	}
	require := func(name, value string) error {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("%s is required for discourse_mode=%s", name, mode)
		}
		return nil
	}
	switch mode {
	case "share_observation":
		return require("observation", d.Observation)
	case "share_experience":
		if err := require("experience", d.Experience); err != nil { return err }
		return require("result", d.Result)
	case "state_opinion":
		if err := require("stance", d.Stance); err != nil { return err }
		return require("basis", d.Basis)
	case "share_tip":
		if err := require("attempted_actions", d.AttemptedActions); err != nil { return err }
		if err := require("result", d.Result); err != nil { return err }
		return require("practical_point", d.PracticalPoint)
	case "ask_peers":
		if err := require("attempted_actions", d.AttemptedActions); err != nil { return err }
		return require("question", d.Question)
	default:
		return fmt.Errorf("unknown discourse mode %q", mode)
	}
}

func productionSituationCanonicalState(seed productionSituationSeed, d llm.BBSWorldSituationDraft) (string, []string) {
	facts := []string{
		"world_fact_status=accepted_situation_before_subject",
		"object_class=" + strings.TrimSpace(d.ObjectClass),
		"occurrence=" + strings.TrimSpace(d.Occurrence),
	}
	parts := []string{strings.TrimSpace(d.Occurrence)}
	add := func(key, value string) {
		value = strings.TrimSpace(value)
		if value == "" { return }
		facts = append(facts, key+"="+value)
	}
	switch seed.mode {
	case "share_observation":
		add("observation", d.Observation)
		parts = append(parts, strings.TrimSpace(d.Observation))
	case "share_experience":
		add("experience", d.Experience)
		add("result", d.Result)
		parts = append(parts, strings.TrimSpace(d.Experience), strings.TrimSpace(d.Result))
	case "state_opinion":
		add("stance", d.Stance)
		add("basis", d.Basis)
		parts = append(parts, strings.TrimSpace(d.Stance), strings.TrimSpace(d.Basis))
	case "share_tip":
		add("attempted_actions", d.AttemptedActions)
		add("result", d.Result)
		add("practical_point", d.PracticalPoint)
		parts = append(parts, strings.TrimSpace(d.AttemptedActions), strings.TrimSpace(d.Result), strings.TrimSpace(d.PracticalPoint))
	case "ask_peers":
		add("attempted_actions", d.AttemptedActions)
		add("question", d.Question)
		parts = append(parts, strings.TrimSpace(d.AttemptedActions), strings.TrimSpace(d.Question))
	}
	add("novelty_key", d.NoveltyKey)
	for _, fact := range seed.sparse.facts {
		if strings.HasPrefix(fact, "scope_boundary=") {
			facts = append(facts, fact)
		}
	}
	for _, value := range d.MustNot {
		if value = strings.TrimSpace(value); value != "" {
			facts = append(facts, "must_not="+value)
		}
	}
	clean := parts[:0]
	for _, part := range parts {
		if part = strings.TrimSpace(part); part != "" {
			clean = append(clean, part)
		}
	}
	return strings.Join(clean, " "), facts
}

func productionDiscourseGoal(mode string) string {
	switch mode {
	case "share_observation":
		return "state the observation"
	case "share_experience":
		return "tell what happened"
	case "state_opinion":
		return "state the opinion"
	case "share_tip":
		return "share the small practical finding"
	case "ask_peers":
		return "ask the canonical question"
	default:
		return ""
	}
}
