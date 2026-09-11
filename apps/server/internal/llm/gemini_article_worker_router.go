package llm

import "context"

// GeminiArticleWorkerRouter keeps the existing OpenAI-backed planning capabilities
// (title candidates, era validation, article details, and other development
// planners) while routing the final article prose worker to Gemini.
//
// Embedding StructuredOpenAIProvider is intentional: worldrepo discovers optional
// planning capabilities through interface assertions on the renderer. Replacing
// the renderer with StructuredGeminiProvider directly would therefore disable
// title-first and related debug flows until those planner interfaces also have a
// native Gemini implementation.
type GeminiArticleWorkerRouter struct {
	StructuredOpenAIProvider
	ArticleWorker StructuredGeminiProvider
}

func (p GeminiArticleWorkerRouter) GenerateBoardPost(ctx context.Context, req BoardPostRequest) (BoardPostDraft, error) {
	return p.ArticleWorker.GenerateBoardPost(ctx, req)
}

var _ BoardPostRenderer = GeminiArticleWorkerRouter{}
var _ BBSTitleCandidatePlanner = GeminiArticleWorkerRouter{}
var _ BBSTitleEraValidator = GeminiArticleWorkerRouter{}
var _ BBSTitleArticleDetailPlanner = GeminiArticleWorkerRouter{}
