package worldrepo

import (
	"strings"
	"testing"

	"zutto-pccom/apps/server/internal/worldengine"
)

func TestHistoricalTextureIsPassedAsAllowedFactSupport(t *testing.T) {
	m := LLMMaterializer{HistoricalTexture: []string{"Windows 95 is an allowed contemporary referent."}}
	facts := m.historicalFacts(worldengine.EvidenceDecision{})
	if len(facts) != 1 || facts[0] != "Windows 95 is an allowed contemporary referent." {
		t.Fatalf("historicalFacts=%#v", facts)
	}
	rules := m.eraRules()
	if !strings.Contains(rules, "HISTORICAL_REFERENCES=ON") || !strings.Contains(rules, "historical texture") {
		t.Fatalf("era rules did not enable bounded supplied texture: %q", rules)
	}
}
