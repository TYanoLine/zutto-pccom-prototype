package worldrepo

import (
	"strings"
	"testing"
)

func TestModelHistoricalMemoryUsesNoInjectedDictionary(t *testing.T) {
	m := LLMMaterializer{ModelHistoricalMemory: true}
	copy := m.withPeriodReferents("1996-08-29")
	if len(copy.HistoricalTexture) != 0 {
		t.Fatalf("model-memory injected texture: %#v", copy.HistoricalTexture)
	}
	rules := copy.planningEraRules()
	if !strings.Contains(rules, "MODEL_MEMORY_EXPERIMENT") {
		t.Fatalf("model-memory marker missing: %s", rules)
	}
	if strings.Contains(rules, "SUPPLIED HISTORICAL FACTS / TEXTURE") {
		t.Fatalf("model-memory unexpectedly contains supplied dictionary: %s", rules)
	}
}

func TestModelHistoricalMemoryDoesNotChangeOffMode(t *testing.T) {
	if rules := (LLMMaterializer{}).eraRules(); !strings.Contains(rules, "HISTORICAL_REFERENCES=OFF") {
		t.Fatalf("off mode changed: %s", rules)
	}
}

func TestModelHistoricalMemoryConcretePreferenceStillUsesNoDictionary(t *testing.T) {
	m := LLMMaterializer{ModelHistoricalMemory: true, PreferConcreteHistoricalNames: true}
	copy := m.withPeriodReferents("1996-08-29")
	if len(copy.HistoricalTexture) != 0 {
		t.Fatalf("concrete model-memory injected texture: %#v", copy.HistoricalTexture)
	}
	rules := copy.planningEraRules()
	if !strings.Contains(rules, "MODEL_MEMORY_CONCRETE_NAME_EXPERIMENT") {
		t.Fatalf("concrete-name marker missing: %s", rules)
	}
	if !strings.Contains(rules, "prefer that concrete historical name") {
		t.Fatalf("concrete-name preference missing: %s", rules)
	}
}
