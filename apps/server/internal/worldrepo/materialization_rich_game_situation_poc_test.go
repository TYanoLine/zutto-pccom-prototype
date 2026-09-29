package worldrepo

import "testing"

func TestRichGameSituationFacetsAreBroadAndModeAware(t *testing.T) {
	facets := developmentRichGameSituationFacets()
	if len(facets) < 20 {
		t.Fatalf("expected at least 20 rich game facets, got %d", len(facets))
	}
	seen := map[string]bool{}
	modeCounts := map[string]int{}
	for _, facet := range facets {
		if facet.kind == "" {
			t.Fatal("facet kind must not be empty")
		}
		if seen[facet.kind] {
			t.Fatalf("duplicate facet kind %q", facet.kind)
		}
		seen[facet.kind] = true
		for mode := range facet.modes {
			modeCounts[mode]++
		}
	}
	for _, mode := range developmentRootDiscourseModes {
		if modeCounts[mode] < 5 {
			t.Fatalf("mode %q has only %d compatible facets", mode, modeCounts[mode])
		}
	}
}

func TestRichGameQuestionLikeFacetsDoNotLeakAcrossModes(t *testing.T) {
	facets := developmentRichGameSituationFacets()
	for _, facet := range facets {
		if developmentModeFacetAllowed(facet, "ask_peers") {
			continue
		}
		// Non-question-only facets may still support several sharing modes, but
		// compatibility must be explicit rather than universal by accident.
		if len(facet.modes) == 0 {
			t.Fatalf("facet %q has no explicit mode compatibility", facet.kind)
		}
	}
}
