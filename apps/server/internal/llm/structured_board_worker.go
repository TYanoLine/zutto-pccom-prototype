package llm

import (
	"context"
	"strings"
)

const legacyDenseArticleDetailContract = "article_detail_contract=The article_detail facts are canonical article-local specifics selected after title/persona/Era adoption. Materially express at least two distinct supplied details. A detail must add information beyond the title/summary; never collapse it back into vague wording. Do not add external historical/product/game facts, durable biography, or unexplained causes beyond canonical context."
const sparseArticleDetailContract = "article_detail_contract=The article_detail facts are optional canonical article-local specifics, not a prose checklist. Reveal only the detail that this person would naturally mention now; do not enumerate all details, add a conclusion, or turn a small post into an explanatory article. Do not add external historical/product/game facts, durable biography, or unexplained causes beyond canonical context."

func prepareStructuredBoardPostRequest(req BoardPostRequest) BoardPostRequest {
	// Older title-first world rows carry the dense Article Detail contract. Keep
	// those rows readable, but translate the rendering instruction at the worker
	// boundary so already-materialized debug worlds do not force checklist prose.
	if strings.Contains(req.PostIntent, legacyDenseArticleDetailContract) {
		req.PostIntent = strings.ReplaceAll(req.PostIntent, legacyDenseArticleDetailContract, sparseArticleDetailContract)
	}
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
