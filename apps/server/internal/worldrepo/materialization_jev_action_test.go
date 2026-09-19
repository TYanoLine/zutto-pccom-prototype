package worldrepo

import "testing"

func TestDemoBlendWriteProbabilityIsConservative(t *testing.T) {
	if got := demoBlendWriteProbability(.20, .80); got != .41 {
		t.Fatalf("blend=%v, want .41", got)
	}
	if got := demoBlendWriteProbability(.20, 2); got != .48 {
		t.Fatalf("high advice clamp=%v, want .48", got)
	}
	if got := demoBlendWriteProbability(.20, -1); got != .13 {
		t.Fatalf("low advice clamp=%v, want .13", got)
	}
}
