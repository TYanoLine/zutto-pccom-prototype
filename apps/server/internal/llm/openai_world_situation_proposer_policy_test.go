package llm

import (
	"strings"
	"testing"
)

func TestBBSWorldSituationHistoricalPolicyModelMemory(t *testing.T) {
	got := bbsWorldSituationHistoricalPolicy(BBSWorldSituationProposalRequest{AllowModelHistoricalMemory: true})
	if !strings.Contains(got, "own historical knowledge") {
		t.Fatalf("model-memory permission missing: %s", got)
	}
	if strings.Contains(got, "unless SUPPLIED HISTORICAL TEXTURE") {
		t.Fatalf("dictionary-only rule leaked into model-memory: %s", got)
	}
}

func TestBBSWorldSituationHistoricalPolicyDefault(t *testing.T) {
	got := bbsWorldSituationHistoricalPolicy(BBSWorldSituationProposalRequest{})
	if !strings.Contains(got, "unless SUPPLIED HISTORICAL TEXTURE") {
		t.Fatalf("default sourced boundary changed: %s", got)
	}
}
