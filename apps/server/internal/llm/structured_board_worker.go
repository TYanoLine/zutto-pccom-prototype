package llm

import (
	"context"
	"strings"
)

func prepareStructuredBoardPostRequest(req BoardPostRequest) BoardPostRequest {
	if strings.Contains(req.PostIntent, "producer_event_id=") {
		req.PostIntent = `ARTICLE WORKER CONTRACT:
The host-window PRODUCER already coordinated this article with the rest of the world window. All producer_* fields below are canonical production instructions, not suggestions.
Use them as content facts, but do not narrate these field names or any internal planning metadata.
Do not replace producer_episode, invent a different referent, give the actor knowledge outside producer_actor_knowledge/context, omit producer_required_contribution, or violate producer_must_not.
If producer_audience_context says a referent is not shared, do not use unexplained shorthand as though readers already know it.
If a concrete external product/place name is absent from the brief and historical facts, do not invent one merely for specificity.

` + req.PostIntent
	}
	return req
}

func (p StructuredOpenAIProvider) GenerateBoardPost(ctx context.Context, req BoardPostRequest) (BoardPostDraft, error) {
	return p.OpenAIProvider.GenerateBoardPost(ctx, prepareStructuredBoardPostRequest(req))
}

type StructuredGeminiProvider struct{ GeminiProvider }

func (p StructuredGeminiProvider) GenerateBoardPost(ctx context.Context, req BoardPostRequest) (BoardPostDraft, error) {
	return p.GeminiProvider.GenerateBoardPost(ctx, prepareStructuredBoardPostRequest(req))
}
