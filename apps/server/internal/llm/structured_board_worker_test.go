package llm

import (
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
