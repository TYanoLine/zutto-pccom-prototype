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

func TestBBSWorldSituationHistoricalPolicyConcreteNamePreference(t *testing.T) {
	got := bbsWorldSituationHistoricalPolicy(BBSWorldSituationProposalRequest{
		AllowModelHistoricalMemory:    true,
		PreferConcreteHistoricalNames: true,
	})
	if !strings.Contains(got, "PREFER that concrete historical name") {
		t.Fatalf("concrete-name preference missing: %s", got)
	}
	if !strings.Contains(got, "not a quota") {
		t.Fatalf("anti-quota boundary missing: %s", got)
	}
	if !strings.Contains(got, "creates new canonical world state") {
		t.Fatalf("canonicalization boundary missing: %s", got)
	}
	if !strings.Contains(got, "no earlier actor-use fact is required") {
		t.Fatalf("new actor occurrence permission missing: %s", got)
	}
	if !strings.Contains(got, "Do not put a generic 'do not name") {
		t.Fatalf("must_not anti-suppression rule missing: %s", got)
	}
}
