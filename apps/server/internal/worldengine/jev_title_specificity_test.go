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


func TestTitleCandidateSpecificityPolicyNamesObservedGenericRoots(t *testing.T) {
	for _, want := range []string{
		"enough topic identity",
		"台詞の間が好き",
		"お気に入りの見開き",
		"次号の展開を予想",
		"クリア時間を比べたい",
		"proper noun is not required",
	} {
		if !strings.Contains(titleCandidateSpecificityPolicy, want) {
			t.Fatalf("specificity policy missing %q", want)
		}
	}
}
