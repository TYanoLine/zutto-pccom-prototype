package worldrepo

import (
	"context"
	"testing"

	"zutto-pccom/apps/server/internal/llm"
	"zutto-pccom/apps/server/internal/worldengine"
)

func TestJevEraValidatorClaimFreeSkipsResearch(t *testing.T) {
	v := developmentJevTitleEraValidator{
		advice:    worldengine.TitleCandidateAdviceDecision{Era: map[int]worldengine.TitleEraProbabilities{1: {}, 2: {}}},
		claimFree: map[int]bool{1: true},
	}
	review, err := v.ValidateBBSTitleEra(context.Background(), llm.BBSTitleEraRequest{Titles: []string{"a", "b"}})
	if err != nil {
		t.Fatal(err)
	}
	if review.Decisions[0].Status != llm.BBSTitleEraOK || review.Decisions[1].Status != llm.BBSTitleEraResearch {
		t.Fatalf("unexpected %+v", review.Decisions)
	}
}
