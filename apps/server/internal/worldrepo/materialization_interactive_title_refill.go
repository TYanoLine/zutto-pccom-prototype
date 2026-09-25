package worldrepo

import (
	"fmt"
	"sort"

	"zutto-pccom/apps/server/internal/world"
)

const developmentInteractiveTitleRootsPerPool = 4
const developmentInteractiveTitlePoolRetries = 4

// developmentPlanInteractiveTitleFirst keeps world-selected root slots immutable.
// Candidate wording may be regenerated/re-ranked, but title planning is never
// allowed to erase a root event that the World layer already selected.
func (r *Repository) developmentPlanInteractiveTitleFirst(host world.Host, window []developmentWindowShell, personas []world.Persona) (map[string]developmentSparseSituation, error) {
	roots := make([]developmentWindowShell, 0, len(window))
	for _, item := range window {
		shell := item.shell
		if shell.action == "thread_start" && shell.parentIndex == 0 && shell.sourceIndex == 0 {
			roots = append(roots, item)
		}
	}
	sort.SliceStable(roots, func(i, j int) bool {
		if roots[i].shell.createdAt.Equal(roots[j].shell.createdAt) {
			return roots[i].eventID < roots[j].eventID
		}
		return roots[i].shell.createdAt.Before(roots[j].shell.createdAt)
	})

	merged := map[string]developmentSparseSituation{}
	if len(roots) == 0 {
		return merged, nil
	}

	history := append([]world.Post(nil), r.Base.ListPosts(host.ID)...)
	allRows := make([]DevelopmentTitleCandidate, 0)
	researchUsed := 0

	for start := 0; start < len(roots); start += developmentInteractiveTitleRootsPerPool {
		end := start + developmentInteractiveTitleRootsPerPool
		if end > len(roots) {
			end = len(roots)
		}
		remaining := append([]developmentWindowShell(nil), roots[start:end]...)

		for attempt := 0; attempt < developmentInteractiveTitlePoolRetries && len(remaining) > 0; attempt++ {
			state := &developmentTitleFirstState{
				history:            append([]world.Post(nil), history...),
				preserveWorldRoots: true,
				eraResearchUsed:    researchUsed,
			}
			planned, err := r.developmentPlanTitleFirstWithState(host, remaining, personas, state)
			allRows = append(allRows, state.rows...)
			researchUsed = state.eraResearchUsed
			if err != nil {
				developmentTitleFirst.Store(r, &developmentTitleFirstState{
					history: append([]world.Post(nil), history...), attempted: true,
					result: merged, err: err, rows: allRows, eraResearchUsed: researchUsed,
				})
				return nil, err
			}

			next := make([]developmentWindowShell, 0, len(remaining))
			for _, item := range remaining {
				situation, ok := planned[item.eventID]
				if !ok {
					next = append(next, item)
					continue
				}
				merged[item.eventID] = situation
				history = appendInteractivePlannedRootHistory(history, item, situation)
			}
			remaining = next
		}

		if len(remaining) > 0 {
			err := fmt.Errorf("interactive title-first left %d roots unresolved after %d generated pools; canned title fallback is disabled", len(remaining), developmentInteractiveTitlePoolRetries)
			developmentTitleFirst.Store(r, &developmentTitleFirstState{
				history: append([]world.Post(nil), history...), preserveWorldRoots: true, attempted: true,
				result: merged, err: err, rows: allRows, eraResearchUsed: researchUsed,
			})
			return nil, err
		}
	}

	developmentTitleFirst.Store(r, &developmentTitleFirstState{
		history: append([]world.Post(nil), history...), preserveWorldRoots: true, attempted: true,
		result: merged, rows: allRows, eraResearchUsed: researchUsed,
	})
	return merged, nil
}

func appendInteractivePlannedRootHistory(history []world.Post, item developmentWindowShell, situation developmentSparseSituation) []world.Post {
	subject := titleFirstSubject(situation.facts)
	if subject == "" {
		return history
	}
	return append(history, world.Post{
		BoardID:         item.board.ID,
		Author:          item.shell.persona.Handle,
		AuthorPersonaID: item.shell.persona.ID,
		Subject:         subject,
		CreatedAt:       item.shell.createdAt,
		Intent: world.PostIntent{
			Action:           item.shell.action,
			AnchorKey:        item.shell.anchorKey,
			CauseKind:        item.shell.causeKind,
			DiscourseMode:    item.shell.discourseMode,
			SituationKind:    situation.kind,
			SituationSummary: situation.summary,
			SituationFacts:   append([]string(nil), situation.facts...),
			Topic:            item.shell.anchorKey,
			Motivation:       item.shell.causeSummary,
		},
	})
}
