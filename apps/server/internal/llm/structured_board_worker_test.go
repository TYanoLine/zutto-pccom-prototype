package llm

import (
	"context"
	"strings"
	"testing"
)

func TestPrepareStructuredBoardPostRequestReplacesDenseArticleDetailContract(t *testing.T) {
	req := prepareStructuredBoardPostRequest(BoardPostRequest{PostIntent: "title_first_subject=x\n" + legacyDenseArticleDetailContract})
	if strings.Contains(req.PostIntent, "Materially express at least two distinct supplied details") {
		t.Fatalf("dense contract survived: %s", req.PostIntent)
	}
	if !strings.Contains(req.PostIntent, "not a prose checklist") {
		t.Fatalf("sparse contract missing: %s", req.PostIntent)
	}
}

func TestGeminiArticleWorkerCompatibilityRouterUsesOpenAIForProse(t *testing.T) {
	router := GeminiArticleWorkerRouter{
		StructuredOpenAIProvider: StructuredOpenAIProvider{OpenAIProvider: OpenAIProvider{}},
		ArticleWorker: StructuredGeminiProvider{GeminiProvider: GeminiProvider{}},
	}
	_, err := router.GenerateBoardPost(context.Background(), BoardPostRequest{})
	if err == nil || !strings.Contains(err.Error(), "OPENAI_API_KEY") {
		t.Fatalf("final prose should be routed to OpenAI, got %v", err)
	}
}
