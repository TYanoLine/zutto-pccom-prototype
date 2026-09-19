package main

import (
	"context"
	"fmt"
	"strings"

	"zutto-pccom/apps/server/internal/llm"
	"zutto-pccom/apps/server/internal/worldrepo"
)

// titleEraObserveOnlyRenderer is a Lab-only diagnostic wrapper. It keeps the
// normal title candidate/persona/detail planners and Article Worker unchanged,
// but converts the title-era routing result to OK after recording the original
// result in the reason. This lets the Lab inspect article quality without making
// the production title-era gate more permissive.
type titleEraObserveOnlyRenderer struct {
	postRenderer  llm.BoardPostRenderer
	titlePlanner  llm.BBSTitleCandidatePlanner
	eraValidator  llm.BBSTitleEraValidator
	detailPlanner llm.BBSTitleArticleDetailPlanner
}

func newTitleEraObserveOnlyRenderer(renderer llm.BoardPostRenderer) (titleEraObserveOnlyRenderer, error) {
	titlePlanner, ok := renderer.(llm.BBSTitleCandidatePlanner)
	if !ok {
		return titleEraObserveOnlyRenderer{}, fmt.Errorf("renderer does not support title candidates")
	}
	eraValidator, ok := renderer.(llm.BBSTitleEraValidator)
	if !ok {
		return titleEraObserveOnlyRenderer{}, fmt.Errorf("renderer does not support title era validation")
	}
	detailPlanner, ok := renderer.(llm.BBSTitleArticleDetailPlanner)
	if !ok {
		return titleEraObserveOnlyRenderer{}, fmt.Errorf("renderer does not support title article details")
	}
	return titleEraObserveOnlyRenderer{
		postRenderer:  renderer,
		titlePlanner:  titlePlanner,
		eraValidator:  eraValidator,
		detailPlanner: detailPlanner,
	}, nil
}

func withTitleEraObserveOnly(materializer worldrepo.Materializer) (worldrepo.Materializer, error) {
	wrap := func(m worldrepo.LLMMaterializer) (worldrepo.LLMMaterializer, error) {
		renderer, err := newTitleEraObserveOnlyRenderer(m.Renderer)
		if err != nil {
			return m, err
		}
		m.Renderer = renderer
		return m, nil
	}

	switch m := materializer.(type) {
	case worldrepo.LLMMaterializer:
		wrapped, err := wrap(m)
		if err != nil {
			return nil, err
		}
		return wrapped, nil
	case *worldrepo.LLMMaterializer:
		clone, err := wrap(*m)
		if err != nil {
			return nil, err
		}
		return &clone, nil
	default:
		return nil, fmt.Errorf("observe-only title era gate requires LLMMaterializer, got %T", materializer)
	}
}

func (r titleEraObserveOnlyRenderer) GenerateBoardPost(ctx context.Context, req llm.BoardPostRequest) (llm.BoardPostDraft, error) {
	return r.postRenderer.GenerateBoardPost(ctx, req)
}

func (r titleEraObserveOnlyRenderer) GenerateBBSTitleCandidates(ctx context.Context, worldDate, boardName string) (llm.BBSTitleCandidates, error) {
	return r.titlePlanner.GenerateBBSTitleCandidates(ctx, worldDate, boardName)
}

func (r titleEraObserveOnlyRenderer) ReviewBBSTitleCandidates(ctx context.Context, req llm.BBSTitleReviewRequest) (llm.BBSTitleReview, error) {
	return r.titlePlanner.ReviewBBSTitleCandidates(ctx, req)
}

func (r titleEraObserveOnlyRenderer) ValidateBBSTitleEra(ctx context.Context, req llm.BBSTitleEraRequest) (llm.BBSTitleEraReview, error) {
	review, err := r.eraValidator.ValidateBBSTitleEra(ctx, req)
	if err != nil {
		// In observe-only mode the validator is diagnostic, not a gate. A malformed
		// model response must not starve an entire board and prevent prose-quality
		// inspection. Preserve any token usage and the validator error in each
		// synthetic reason, while production strict mode remains unchanged.
		decisions := make([]llm.BBSTitleEraDecision, 0, len(req.Titles))
		for i := range req.Titles {
			decisions = append(decisions, llm.BBSTitleEraDecision{
				Candidate: i + 1,
				Status:    llm.BBSTitleEraOK,
				Reason:    fmt.Sprintf("LAB observe-only: validator_error=%v", err),
			})
		}
		review.Decisions = decisions
		return review, nil
	}
	for i := range review.Decisions {
		originalStatus := review.Decisions[i].Status
		originalReason := strings.TrimSpace(review.Decisions[i].Reason)
		review.Decisions[i].Status = llm.BBSTitleEraOK
		review.Decisions[i].Reason = fmt.Sprintf("LAB observe-only: original=%s; %s", originalStatus, originalReason)
	}
	return review, nil
}

func (r titleEraObserveOnlyRenderer) MaterializeBBSTitleArticleDetails(ctx context.Context, req llm.BBSTitleArticleDetailRequest) (llm.BBSTitleArticleDetailDraft, error) {
	return r.detailPlanner.MaterializeBBSTitleArticleDetails(ctx, req)
}

func (r titleEraObserveOnlyRenderer) TitleEraObserveOnly() bool { return true }

var _ llm.BoardPostRenderer = titleEraObserveOnlyRenderer{}
var _ llm.BBSTitleCandidatePlanner = titleEraObserveOnlyRenderer{}
var _ llm.BBSTitleEraValidator = titleEraObserveOnlyRenderer{}
var _ llm.BBSTitleArticleDetailPlanner = titleEraObserveOnlyRenderer{}
