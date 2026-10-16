package worldrepo

import (
	"fmt"
	"strings"
	"testing"

	"zutto-pccom/apps/server/internal/world"
)

func TestDerivedVoiceIsStablePerPersonaAndVaries(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 40; i++ {
		p := world.Persona{ID: fmt.Sprintf("host:p%02d", i)}
		v := derivedVoice(p)
		if v == "" || v != derivedVoice(p) {
			t.Fatalf("voice must be non-empty and deterministic for %s", p.ID)
		}
		seen[v] = true
	}
	if len(seen) < 15 {
		t.Fatalf("voices too uniform: %d distinct of 40", len(seen))
	}
	if derivedVoice(world.Persona{}) != "" {
		t.Fatal("persona without identity must get no derived voice")
	}
}

func TestFreeformPersonaSummaryPrefersExplicitWritingStyle(t *testing.T) {
	explicit := freeformPersonaSummary(world.Persona{ID: "a", WritingStyle: "関西風の軽い口調"})
	if !strings.Contains(explicit, "文体・口調: 関西風の軽い口調") || strings.Contains(explicit, "です・ます調を崩さない") {
		t.Fatalf("explicit style must win:\n%s", explicit)
	}
	derived := freeformPersonaSummary(world.Persona{ID: "a", Argumentativeness: 0.1})
	if !strings.Contains(derived, "文体・口調:") || strings.Contains(derived, "穏やかで和やか") {
		t.Fatalf("derived voice expected, blanket gentle line removed:\n%s", derived)
	}
}
