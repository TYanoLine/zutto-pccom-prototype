package worldrepo

import (
	"strings"
	"testing"

	"zutto-pccom/apps/server/internal/world"
)

func TestFacetlessSituationPoCDoesNotSelectPresetFacetOrOccurrence(t *testing.T) {
	repo := &Repository{}
	repo.EnableDevelopmentFacetlessSituationPoC()
	shell := developmentTimelineShell{action: "thread_start", anchorKey: "local", discourseMode: "share_observation"}
	got := repo.developmentConversationSituationForShell(world.Host{}, world.Board{ID: "3"}, shell, nil, nil)
	if got.kind != "facetless_open" {
		t.Fatalf("kind=%q want facetless_open", got.kind)
	}
	joined := got.summary + "\n" + strings.Join(got.facts, "\n")
	for _, forbidden := range []string{"local_route_condition", "local_notice_change", "focus=", "occurrence="} {
		if strings.Contains(joined, forbidden) {
			t.Fatalf("facetless situation leaked preset facet/occurrence %q: %s", forbidden, joined)
		}
	}
	for _, want := range []string{"routing_domain=local", "facetless_experiment=true", "Invent one small concrete occurrence"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("facetless situation missing %q: %s", want, joined)
		}
	}
}

func TestFacetlessSituationPoCKeepsExplicitReplySource(t *testing.T) {
	repo := &Repository{}
	repo.EnableDevelopmentFacetlessSituationPoC()
	source := world.Post{ID: 42, Intent: world.PostIntent{SituationKind: "facetless_open", SituationSummary: "source situation", SituationFacts: []string{"facetless_experiment=true", "routing_domain=games"}}}
	shell := developmentTimelineShell{action: "reply", anchorKey: "games"}
	got := repo.developmentConversationSituationForShell(world.Host{}, world.Board{ID: "1"}, shell, nil, &source)
	if got.kind != "facetless_open" || !strings.Contains(got.summary, "source situation") {
		t.Fatalf("reply lost explicit source situation: %#v", got)
	}
	if !strings.Contains(strings.Join(got.facts, "\n"), "reply_binding=") {
		t.Fatalf("reply missing source binding: %#v", got.facts)
	}
}
