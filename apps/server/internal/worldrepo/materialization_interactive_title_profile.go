package worldrepo

import (
	"context"

	"zutto-pccom/apps/server/internal/llm"
)

type developmentTitleCandidatePlannerAdapter struct {
	generate func(context.Context, string, string) (llm.BBSTitleCandidates, error)
	review   func(context.Context, llm.BBSTitleReviewRequest) (llm.BBSTitleReview, error)
}

func (a developmentTitleCandidatePlannerAdapter) GenerateBBSTitleCandidates(ctx context.Context, date, board string) (llm.BBSTitleCandidates, error) {
	return a.generate(ctx, date, board)
}

func (a developmentTitleCandidatePlannerAdapter) ReviewBBSTitleCandidates(ctx context.Context, req llm.BBSTitleReviewRequest) (llm.BBSTitleReview, error) {
	return a.review(ctx, req)
}

type developmentTitleEraValidatorAdapter struct {
	validate func(context.Context, llm.BBSTitleEraRequest) (llm.BBSTitleEraReview, error)
}

func (a developmentTitleEraValidatorAdapter) ValidateBBSTitleEra(ctx context.Context, req llm.BBSTitleEraRequest) (llm.BBSTitleEraReview, error) {
	return a.validate(ctx, req)
}

func developmentInteractiveTitlePlanner(renderer any, fallback llm.BBSTitleCandidatePlanner) llm.BBSTitleCandidatePlanner {
	fast, ok := renderer.(llm.BBSTitleInteractiveCandidatePlanner)
	if !ok {
		return fallback
	}
	return developmentTitleCandidatePlannerAdapter{
		generate: fast.GenerateInteractiveBBSTitleCandidates,
		review:   fast.ReviewInteractiveBBSTitleCandidates,
	}
}

func developmentInteractiveTitleEraValidator(renderer any, fallback llm.BBSTitleEraValidator) llm.BBSTitleEraValidator {
	fast, ok := renderer.(llm.BBSTitleInteractiveEraValidator)
	if !ok {
		return fallback
	}
	return developmentTitleEraValidatorAdapter{validate: fast.ValidateInteractiveBBSTitleEra}
}
