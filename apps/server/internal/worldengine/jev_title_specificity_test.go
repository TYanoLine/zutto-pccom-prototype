package worldengine

import (
	"strings"
	"testing"
)

func TestTitleCandidateFitPolicyRejectsUnanchoredSpecificReferents(t *testing.T) {
	for _, want := range []string{
		"one specific work",
		"near-zero fit",
		"do not assume a later prose step will invent the missing target",
		"single-work board",
	} {
		if !strings.Contains(titleCandidateFitPolicy, want) {
			t.Fatalf("fit policy missing %q", want)
		}
	}
}
