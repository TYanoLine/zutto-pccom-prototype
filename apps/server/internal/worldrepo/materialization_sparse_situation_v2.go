package worldrepo

import (
	"fmt"
	"strings"
	"time"

	"zutto-pccom/apps/server/internal/world"
)

// developmentSituationForShellV2 keeps the sparse situation model deliberately
// local while tightening two invariants found by the first live PoC run:
// independent roots should not recycle the same facet while plenty of alternatives
// exist, and reply chains must inherit only the source situation's world facts,
// not renderer/discourse guards accumulated by earlier replies.
func developmentSituationForShellV2(host world.Host, board world.Board, shell developmentTimelineShell, prior []world.Post, source *world.Post) developmentSparseSituation {
	if source != nil {
		if shell.action == "reply" {
			return developmentSituationFromSourceV2(*source, false)
		}
		if shell.causeKind == "continuation_progress" {
			return developmentSituationFromSourceV2(*source, true)
		}
	}
	return developmentSelectRootSituationV2(host, board, shell, prior)
}

func developmentSituationFromSourceV2(source world.Post, continuation bool) developmentSparseSituation {
	kind := strings.TrimSpace(source.Intent.SituationKind)
	if kind == "" {
		kind = "source_thread_context"
	}

	facts := developmentCoreSituationFacts(source.Intent.SituationFacts)
	focus := developmentSituationFactValue(facts, "focus=")
	occurrence := developmentSituationFactValue(facts, "occurrence=")
	boundary := developmentSituationFactValue(facts, "scope_boundary=")

	var summary string
	if continuation {
		summary = "A materially new development occurred inside the canonical source situation."
	} else {
		summary = "This contribution is a reply inside the canonical source situation."
	}
	if occurrence != "" {
		summary += " Source occurrence: " + occurrence
	} else if fallback := strings.TrimSpace(source.Intent.SituationSummary); fallback != "" {
		summary += " Source situation: " + fallback
	} else {
		summary += fmt.Sprintf(" Source post: %04d.", source.ID)
	}
	if focus != "" {
		summary += " Focus: " + focus
	}
	if boundary != "" {
		summary += " Scope boundary: " + boundary
	}

	if continuation {
		facts = append(facts, "continuation=Add a genuinely new development inside this same canonical situation; do not restate the earlier root or merely agree with it.")
	} else {
		facts = append(facts, "reply_binding=Respond to the explicit canonical source contribution inside this situation; do not replace it with another topic or event.")
	}
	return developmentSparseSituation{kind: kind, summary: strings.TrimSpace(summary), facts: facts}
}

func developmentCoreSituationFacts(facts []string) []string {
	out := make([]string, 0, 3)
	seen := map[string]bool{}
	for _, raw := range facts {
		fact := strings.TrimSpace(raw)
		prefix := ""
		switch {
		case strings.HasPrefix(fact, "focus="):
			prefix = "focus="
		case strings.HasPrefix(fact, "occurrence="):
			prefix = "occurrence="
		case strings.HasPrefix(fact, "scope_boundary="):
			prefix = "scope_boundary="
		default:
			continue
		}
		if fact == prefix || seen[prefix] {
			continue
		}
		seen[prefix] = true
		out = append(out, fact)
	}
	return out
}

func developmentSituationFactValue(facts []string, prefix string) string {
	for _, fact := range facts {
		fact = strings.TrimSpace(fact)
		if strings.HasPrefix(fact, prefix) {
			return strings.TrimSpace(strings.TrimPrefix(fact, prefix))
		}
	}
	return ""
}

func developmentSelectRootSituationV2(host world.Host, board world.Board, shell developmentTimelineShell, prior []world.Post) developmentSparseSituation {
	facets := developmentSituationFacets(shell.anchorKey)
	if len(facets) == 0 {
		facets = developmentSituationFacets("generic")
	}
	candidates := make([]developmentSituationCandidate, 0, len(facets))
	hasUnblocked := false
	for _, facet := range facets {
		weight, blocked := developmentSituationNoveltyV2(facet.kind, board.ID, shell.persona.ID, shell.createdAt, prior)
		if !blocked {
			hasUnblocked = true
		}
		candidates = append(candidates, developmentSituationCandidate{facet: facet, weight: weight, blocked: blocked})
	}

	total := 0.0
	for _, candidate := range candidates {
		if hasUnblocked && candidate.blocked {
			continue
		}
		total += candidate.weight
	}
	if total <= 0 {
		total = float64(len(candidates))
		for i := range candidates {
			candidates[i].weight = 1
			candidates[i].blocked = false
		}
		hasUnblocked = true
	}

	roll := demoStableUnit(host.ID, board.ID, shell.persona.ID, shell.createdAt.Format(time.RFC3339), fmt.Sprintf("sparse-situation-v2-%d", shell.index)) * total
	selected := candidates[len(candidates)-1].facet
	for _, candidate := range candidates {
		if hasUnblocked && candidate.blocked {
			continue
		}
		if roll < candidate.weight {
			selected = candidate.facet
			break
		}
		roll -= candidate.weight
	}
	return developmentComposeSituation(host, board, shell, selected)
}

// V2 treats a board facet as temporarily exhausted for one week when alternatives
// exist. This is still a local novelty gate, not a global event scheduler. The
// same actor keeps the stricter ten-day no-repeat window. If all facets are
// exhausted, the selector still falls back to weighted reuse instead of failing.
func developmentSituationNoveltyV2(kind, boardID, personaID string, at time.Time, prior []world.Post) (float64, bool) {
	weight := 1.0
	blocked := false
	for i := len(prior) - 1; i >= 0; i-- {
		post := prior[i]
		if post.ParentID != 0 || post.BoardID != boardID || post.Intent.SituationKind != kind {
			continue
		}
		age := at.Sub(post.CreatedAt)
		if age < 0 {
			continue
		}
		switch {
		case age < 7*24*time.Hour:
			blocked = true
			weight *= .08
		case age < 10*24*time.Hour:
			weight *= .35
		default:
			weight *= .72
		}
		if post.AuthorPersonaID == personaID {
			if age < 10*24*time.Hour {
				blocked = true
			}
			weight *= .10
		}
	}
	if weight < .001 {
		weight = .001
	}
	return weight, blocked
}
