package worldrepo

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"zutto-pccom/apps/server/internal/llm"
	"zutto-pccom/apps/server/internal/world"
)

func hasInteractiveArticleDetails(facts []string) bool {
	for _, fact := range facts {
		if strings.HasPrefix(fact, "article_detail=") {
			return true
		}
	}
	return false
}

func repairInteractiveArticleDetailFacts(facts []string) ([]string, bool) {
	bad := false
	for _, raw := range facts {
		if !strings.HasPrefix(raw, "article_detail=") {
			continue
		}
		encoded := strings.TrimSpace(strings.TrimPrefix(raw, "article_detail="))
		if colon := strings.Index(encoded, ":"); colon >= 0 {
			encoded = strings.TrimSpace(encoded[colon+1:])
		}
		if llm.ArticleDetailFactIsRenderingMetadata(encoded) {
			bad = true
			break
		}
	}
	if !bad {
		return facts, false
	}
	clean := make([]string, 0, len(facts))
	for _, raw := range facts {
		if strings.HasPrefix(raw, "article_detail=") || strings.HasPrefix(raw, "article_detail_contract=") {
			continue
		}
		clean = append(clean, raw)
	}
	return clean, true
}

func worldAdoptedSummary(facts []string, fallback string) string {
	for _, fact := range facts {
		if strings.HasPrefix(fact, "world_adopted_summary=") {
			if value := strings.TrimSpace(strings.TrimPrefix(fact, "world_adopted_summary=")); value != "" {
				return value
			}
		}
	}
	return strings.TrimSpace(fallback)
}

// materializeArticleDetails fixes article-local facts before prose generation.
// It is shared by normal host reads, development inspection and isolated Lab runs.
func (r *Repository) materializeArticleDetails(host world.Host, board world.Board, selected world.Post) (world.Post, string, error) {
	if strings.TrimSpace(selected.Body) != "" || selected.Intent.ArticleDetailsMaterialized {
		return selected, "", nil
	}
	if repaired, changed := repairInteractiveArticleDetailFacts(selected.Intent.SituationFacts); changed {
		selected.Intent.SituationFacts = repaired
		updater, ok := r.Base.(world.PostUpdater)
		if !ok {
			err := fmt.Errorf("article detail repair requires post updater")
			return selected, formatGenerationError("article-detail-save", err), err
		}
		updated, ok := updater.UpdatePost(host.ID, selected)
		if !ok {
			err := fmt.Errorf("could not persist repaired article details")
			return selected, formatGenerationError("article-detail-save", err), err
		}
		selected = updated
	}
	// Pre-feature title-first articles may already contain validated details but
	// lack the explicit completion bit. Preserve those canonical facts and migrate
	// them to the new state without asking a planner to replace them.
	if hasInteractiveArticleDetails(selected.Intent.SituationFacts) {
		selected.Intent.ArticleDetailsMaterialized = true
		updater, ok := r.Base.(world.PostUpdater)
		if !ok {
			err := fmt.Errorf("article detail completion requires post updater")
			return selected, formatGenerationError("article-detail-save", err), err
		}
		updated, ok := updater.UpdatePost(host.ID, selected)
		if !ok {
			err := fmt.Errorf("could not persist article detail completion")
			return selected, formatGenerationError("article-detail-save", err), err
		}
		return updated, "", nil
	}
	planner := r.ArticleDetailPlanner
	if planner == nil {
		err := fmt.Errorf("article detail planner is not configured")
		return selected, formatGenerationError("article-detail-setup", err), err
	}

	existingFacts := []string{}
	personaProfile := ""
	if ps, ok := r.Base.(world.PersonaStore); ok && selected.AuthorPersonaID != "" {
		if persona, found := ps.PersonaByID(selected.AuthorPersonaID); found {
			personaProfile = personaSummary(persona)
			facts := r.existingPersonaFactsByID([]world.Persona{persona})
			for _, fact := range facts[persona.ID] {
				if fact.MaterializedAt.IsZero() || !fact.MaterializedAt.After(selected.CreatedAt) {
					existingFacts = append(existingFacts, fact.Key+"="+fact.Value)
				}
			}
		}
	}

	eventID := fmt.Sprintf("interactive-post-%d", selected.ID)
	semanticSubject := strings.TrimSpace(selected.Subject)
	if semanticSubject == "" {
		if sourceID := world.ResponseTargetID(selected); sourceID != 0 {
			if source, ok := developmentConversationFindPost(r.Base.ListPosts(host.ID), sourceID); ok {
				semanticSubject = semanticContextSubject(source)
			}
		}
	}
	if semanticSubject == "" {
		semanticSubject = semanticContextSubject(selected)
	}
	if semanticSubject == "" {
		semanticSubject = board.Name
	}
	threadContext := ""
	if world.ResponseTargetID(selected) != 0 {
		threadContext = r.materializationArticleWorkerContext(host, board, selected)
	}
	authorHistory := r.materializationAuthorHistoryContext(host, board, selected, 6)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	request := llm.BBSTitleArticleDetailRequest{
		BoardName:      board.Name,
		WorldDate:      selected.CreatedAt.Format("2006-01-02"),
		RecentBBSState: planningBBSState(filterBoard(r.Base.ListPosts(host.ID), board.ID), 48),
		Articles: []llm.BBSTitleArticleDetailSeed{{
			EventID:        eventID,
			IsReply:        world.ResponseTargetID(selected) != 0,
			Subject:        semanticSubject,
			Summary:        worldAdoptedSummary(selected.Intent.SituationFacts, selected.Intent.SituationSummary),
			AuthorHandle:   selected.Author,
			CreatedAt:      selected.CreatedAt.Format(time.RFC3339),
			DiscourseMode:  selected.Intent.DiscourseMode,
			PersonaProfile: personaProfile,
			ExistingFacts:  existingFacts,
			ThreadContext:  threadContext,
			AuthorHistory:  authorHistory,
		}},
	}
	var draft llm.BBSTitleArticleDetailDraft
	var err error
	for attempt := 0; attempt < 2; attempt++ {
		draft, err = planner.MaterializeBBSTitleArticleDetails(ctx, request)
		if err == nil {
			err = llm.ValidateBBSTitleArticleDetails(request, draft)
		}
		if err == nil || ctx.Err() != nil {
			break
		}
	}
	usage := GenerationUsage{InputTokens: draft.Usage.InputTokens, CachedInputTokens: draft.Usage.CachedInputTokens, OutputTokens: draft.Usage.OutputTokens, ReasoningTokens: draft.Usage.ReasoningTokens, TotalTokens: draft.Usage.TotalTokens, Model: draft.Usage.Model}
	storeDevelopmentPlanningUsage(r, host.ID, fmt.Sprintf("article-detail-%d", selected.ID), usage)
	referentRequirement, referentStatus, referentGrounding := "", "", ""
	if len(draft.Articles) == 1 {
		referentRequirement = draft.Articles[0].ReferentRequirement
		referentStatus = draft.Articles[0].ReferentStatus
		referentGrounding = draft.Articles[0].ReferentGrounding
	}
	log.Printf("BBS article detail grounding: host=%s board=%s post=%d is_reply=%t referent_requirement=%s referent_status=%s referent_grounding=%s web_search_calls=%d web_sources=%d forced_search_retry=%t", host.ID, board.ID, selected.ID, world.ResponseTargetID(selected) != 0, referentRequirement, referentStatus, referentGrounding, draft.WebSearchCalls, len(draft.WebSearchSources), draft.ForcedWebSearchRetry)
	if err != nil {
		return selected, formatGenerationError("article-detail", err), err
	}
	if len(draft.Articles) != 1 || draft.Articles[0].EventID != eventID {
		err := fmt.Errorf("article detail result did not match selected post")
		return selected, formatGenerationError("article-detail", err), err
	}
	if r.debugBBSArticleDetailLoggingEnabled() {
		if encodedDetails, marshalErr := json.Marshal(draft.Articles[0].Details); marshalErr == nil {
			log.Printf("BBS article detail canonical(debug): host=%s board=%s post=%d details=%s", host.ID, board.ID, selected.ID, encodedDetails)
		} else {
			log.Printf("BBS article detail canonical(debug): host=%s board=%s post=%d marshal_error=%v", host.ID, board.ID, selected.ID, marshalErr)
		}
	}

	requiredReferent := ""
	for _, detail := range draft.Articles[0].Details {
		kind := strings.TrimSpace(detail.Kind)
		fact := strings.TrimSpace(detail.Fact)
		encoded := kind + ":" + fact
		selected.Intent.SituationFacts = append(selected.Intent.SituationFacts, "article_detail="+encoded)
		if kind == "referent" {
			requiredReferent = fact
		}
	}
	isReply := world.ResponseTargetID(selected) != 0
	if !isReply && strings.TrimSpace(draft.Articles[0].ReferentRequirement) == "required" && requiredReferent != "" {
		selected.Intent.SituationFacts = append(selected.Intent.SituationFacts, "article_referent_required="+requiredReferent)
	}
	selected.Intent.SituationFacts = append(selected.Intent.SituationFacts,
		"article_detail_contract=The article_detail facts are canonical article-local specifics selected before prose. Use the naturally relevant supplied detail instead of collapsing the post into generic advice or a paraphrase of earlier replies. Do not enumerate details, force a conclusion, add external historical/product/game facts, durable biography, or unexplained causes beyond canonical context.",
	)
	selected.Intent.ArticleDetailsMaterialized = true
	updater, ok := r.Base.(world.PostUpdater)
	if !ok {
		err := fmt.Errorf("article detail persistence requires post updater")
		return selected, formatGenerationError("article-detail-save", err), err
	}
	updated, ok := updater.UpdatePost(host.ID, selected)
	if !ok || !updated.Intent.ArticleDetailsMaterialized {
		err := fmt.Errorf("could not persist article detail result")
		return selected, formatGenerationError("article-detail-save", err), err
	}
	return updated, formatGenerationUsage(usage), nil
}
