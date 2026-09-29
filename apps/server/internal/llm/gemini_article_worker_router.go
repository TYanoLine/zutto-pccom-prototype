package llm

import "context"

// GeminiArticleWorkerRouter is retained as a compatibility wrapper while the
// server wiring is simplified. Planning capabilities and final article prose now
// both use the embedded OpenAI provider; ArticleWorker remains available to the
// dedicated A/B debug endpoint through its own materializer.
//
// Embedding StructuredOpenAIProvider is intentional: worldrepo discovers optional
// planning capabilities through interface assertions on the renderer.
type GeminiArticleWorkerRouter struct {
	StructuredOpenAIProvider
	ArticleWorker StructuredGeminiProvider
}

func (p GeminiArticleWorkerRouter) GenerateBoardPost(ctx context.Context, req BoardPostRequest) (BoardPostDraft, error) {
	return p.StructuredOpenAIProvider.GenerateBoardPost(ctx, req)
}

var _ BoardPostRenderer = GeminiArticleWorkerRouter{}
var _ BBSWorldSituationProposer = GeminiArticleWorkerRouter{}
var _ BBSSituationTitlePlanner = GeminiArticleWorkerRouter{}
var _ BBSTitleCandidatePlanner = GeminiArticleWorkerRouter{}
var _ BBSTitleEraValidator = GeminiArticleWorkerRouter{}
var _ BBSTitleArticleDetailPlanner = GeminiArticleWorkerRouter{}