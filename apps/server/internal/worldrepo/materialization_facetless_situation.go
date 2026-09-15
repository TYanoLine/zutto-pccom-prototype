package worldrepo

import (
	"fmt"
	"strings"
	"sync"

	"zutto-pccom/apps/server/internal/world"
)

var developmentFacetlessSituationPoC sync.Map

// EnableDevelopmentFacetlessSituationPoC weakens only the fresh-Lab situation
// boundary for an A/B experiment. Root posts keep the world-selected actor,
// timing, topology, routing domain, cause and discourse mode, but no predefined
// situation facet or occurrence is chosen before prose rendering.
func (r *Repository) EnableDevelopmentFacetlessSituationPoC() {
	developmentFacetlessSituationPoC.Store(r, true)
}

func developmentFacetlessSituationPoCEnabled(r *Repository) bool {
	_, ok := developmentFacetlessSituationPoC.Load(r)
	return ok
}

func (r *Repository) developmentConversationSituationForShell(host world.Host, board world.Board, shell developmentTimelineShell, prior []world.Post, source *world.Post) developmentSparseSituation {
	if !developmentFacetlessSituationPoCEnabled(r) {
		return developmentSituationForShell(host, board, shell, prior, source)
	}
	return developmentFacetlessSituationForShell(shell, source)
}

func developmentFacetlessSituationForShell(shell developmentTimelineShell, source *world.Post) developmentSparseSituation {
	if source != nil {
		if shell.action == "reply" {
			return developmentSituationFromSource(*source, false)
		}
		if shell.causeKind == "continuation_progress" {
			return developmentSituationFromSource(*source, true)
		}
	}

	domain := strings.TrimSpace(shell.anchorKey)
	if domain == "" {
		domain = "the board's broad routing domain"
	}
	mode := strings.TrimSpace(shell.discourseMode)
	if mode == "" {
		mode = "ordinary contribution"
	}

	summary := fmt.Sprintf("FACETLESS A/B EXPERIMENT: no predefined situation facet or occurrence has been selected. The world fixes only routing domain %q and discourse mode %q for this independent root. During prose rendering, realize exactly one small, mundane, present-tense occurrence that plausibly gives this actor a reason to write on this board. The occurrence may be freely invented at wording time, but it must remain inside the routing domain, fit the actor and 1996 setting, and must not be borrowed from another root or from recent actor history.", domain, mode)
	facts := []string{
		"facetless_experiment=true",
		"routing_domain=" + domain,
		"discourse_mode=" + mode,
		"event_generation=Invent one small concrete occurrence during prose rendering; do not choose from a predefined situation category, occurrence bank, title bank, product bank, or place bank.",
		"novelty=Do not reuse the concrete event type, object, change, or distinctive detail of another independent root or of this actor's recent posts merely because they appear in context.",
		"root_independence=This root is its own event. Related/recent posts are duplicate-awareness context, not shared-world evidence unless SourcePostID explicitly links them.",
	}
	if shell.discourseMode == "ask_peers" {
		facts = append(facts, "answerability=Make the freely invented occurrence concrete enough that another member can answer without guessing an unnamed title, place, product, device, or hidden choice.")
	}
	return developmentSparseSituation{kind: "facetless_open", summary: summary, facts: facts}
}
