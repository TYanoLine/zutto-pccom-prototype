package llm

import (
	"strings"
	"testing"
)

func TestBBSWorldSituationHistoricalPolicyModelMemory(t *testing.T) {
	got := bbsWorldSituationHistoricalPolicy(BBSWorldSituationProposalRequest{AllowModelHistoricalMemory: true})
	for _, want := range []string{"世界日時点", "実在名", "材料"} {
		if !strings.Contains(got, want) {
			t.Fatalf("model-memory material guidance missing %q: %s", want, got)
		}
	}
	if strings.Contains(got, "historical material または existing_facts") {
		t.Fatalf("dictionary-only boundary leaked into model-memory: %s", got)
	}
}

func TestBBSWorldSituationHistoricalPolicyDefault(t *testing.T) {
	got := bbsWorldSituationHistoricalPolicy(BBSWorldSituationProposalRequest{})
	for _, want := range []string{"historical material", "existing_facts", "材料"} {
		if !strings.Contains(got, want) {
			t.Fatalf("default sourced boundary missing %q: %s", want, got)
		}
	}
}

func TestBBSWorldSituationHistoricalPolicyConcreteNamePreference(t *testing.T) {
	got := bbsWorldSituationHistoricalPolicy(BBSWorldSituationProposalRequest{
		AllowModelHistoricalMemory:    true,
		PreferConcreteHistoricalNames: true,
	})
	for _, want := range []string{"実在の対象が自然なら", "具体名", "世界日時点"} {
		if !strings.Contains(got, want) {
			t.Fatalf("concrete-name material guidance missing %q: %s", want, got)
		}
	}
	if len([]rune(got)) > 80 {
		t.Fatalf("historical material guidance became verbose: %s", got)
	}
}
