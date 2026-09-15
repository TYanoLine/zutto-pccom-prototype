package worldrepo

import (
	"context"
	"fmt"
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

// materializeInteractiveTitleArticleDetails moves the expensive article-local
// detail pass to first article open. The index only needs accepted subjects; it
// must not wait for body-only detail materialization.
func (r *Repository) materializeInteractiveTitleArticleDetails(host world.Host, board world.Board, selected world.Post) (world.Post, string, error) {
	if !developmentInteractiveTitleFirstEnabled(r) || titleFirstSubject(selected.Intent.SituationFacts) == "" {
		return selected, "", nil
	}
	if repaired, changed := repairInteractiveArticleDetailFacts(selected.Intent.SituationFacts); changed {
		selected.Intent.SituationFacts = repaired
		if updater, ok := r.Base.(world.PostUpdater); ok {
			if updated, ok := updater.UpdatePost(host.ID, selected); ok {
				selected = updated
			}
		}
	}
	if hasInteractiveArticleDetails(selected.Intent.SituationFacts) {
		return selected, "", nil
	}

	var m LLMMaterializer
	switch x := r.Materializer.(type) {
	case LLMMaterializer:
		m = x
	case *LLMMaterializer:
		m = *x
	default:
		return selected, "", fmt.Errorf("interactive title detail requires LLMMaterializer")
	}
	planner, ok := m.Renderer.(llm.BBSTitleArticleDetailPlanner)
	if !ok {
		return selected, "", fmt.Errorf("renderer does not support title article details")
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
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	draft, err := planner.MaterializeBBSTitleArticleDetails(ctx, llm.BBSTitleArticleDetailRequest{
		BoardName:      board.Name,
		WorldDate:      selected.CreatedAt.Format("2006-01-02"),
		RecentBBSState: planningBBSState(filterBoard(r.Base.ListPosts(host.ID), board.ID), 48),
		Articles: []llm.BBSTitleArticleDetailSeed{{
			EventID:        eventID,
			Subject:        selected.Subject,
			Summary:        worldAdoptedSummary(selected.Intent.SituationFacts, selected.Intent.SituationSummary),
			AuthorHandle:   selected.Author,
			CreatedAt:      selected.CreatedAt.Format(time.RFC3339),
			DiscourseMode:  selected.Intent.DiscourseMode,
			PersonaProfile: personaProfile,
			ExistingFacts:  existingFacts,
		}},
	})
	usage := GenerationUsage{InputTokens: draft.Usage.InputTokens, CachedInputTokens: draft.Usage.CachedInputTokens, OutputTokens: draft.Usage.OutputTokens, ReasoningTokens: draft.Usage.ReasoningTokens, TotalTokens: draft.Usage.TotalTokens, Model: draft.Usage.Model}
	storeDevelopmentPlanningUsage(r, host.ID, fmt.Sprintf("article-detail-%d", selected.ID), usage)
	if err != nil {
		return selected, formatGenerationError("article-detail", err), err
	}
	if len(draft.Articles) != 1 || draft.Articles[0].EventID != eventID {
		err := fmt.Errorf("article detail result did not match selected post")
		return selected, formatGenerationError("article-detail", err), err
	}

	for _, detail := range draft.Articles[0].Details {
		encoded := strings.TrimSpace(detail.Kind) + ":" + strings.TrimSpace(detail.Fact)
		selected.Intent.SituationFacts = append(selected.Intent.SituationFacts, "article_detail="+encoded)
	}
	selected.Intent.SituationFacts = append(selected.Intent.SituationFacts,
		"article_detail_contract=The article_detail facts are canonical article-local specifics selected after title/persona/Era adoption. Materially express at least two distinct supplied details. A detail must add information beyond the title/summary; never collapse it back into vague wording. Do not add external historical/product/game facts, durable biography, or unexplained causes beyond canonical context.",
	)
	if updater, ok := r.Base.(world.PostUpdater); ok {
		if updated, ok := updater.UpdatePost(host.ID, selected); ok {
			selected = updated
		}
	}
	return selected, formatGenerationUsage(usage), nil
}
