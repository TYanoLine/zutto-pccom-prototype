package worldrepo

import (
	"sort"

	"zutto-pccom/apps/server/internal/world"
)

const developmentInteractiveTitleRootsPerPool = 4

// developmentPlanInteractiveTitleFirst keeps the original title-first invariant:
// every accepted title starts as uncommitted wording and becomes a world fact only
// after slot/persona/Era validation. The ordinary ATDT observation path simply
// gives later root slots another independent 20-title pool instead of asking one
// pool to cover an entire ~20-article board sample. Fresh Lab remains one-pool.
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
		batch := append([]developmentWindowShell(nil), roots[start:end]...)

		r.EnableDevelopmentTitleFirstPoC(history)
		stateValue, _ := developmentTitleFirst.Load(r)
		state := stateValue.(*developmentTitleFirstState)
		state.eraResearchUsed = researchUsed

		planned, err := r.developmentPlanTitleFirst(host, batch, personas)
		allRows = append(allRows, state.rows...)
		researchUsed = state.eraResearchUsed
		if err != nil {
			developmentTitleFirst.Store(r, &developmentTitleFirstState{
				history: append([]world.Post(nil), history...), attempted: true,
				result: merged, err: err, rows: allRows, eraResearchUsed: researchUsed,
			})
			return nil, err
		}
		for eventID, situation := range planned {
			merged[eventID] = situation
		}

		// Accepted earlier roots are canonical within this planning operation.
		// Feed only their accepted title/situation back as transient BBS history
		// so later pools can avoid obvious repetition without treating rejected
		// candidates as facts.
		for _, item := range batch {
			situation, ok := planned[item.eventID]
			if !ok {
				continue
			}
			subject := titleFirstSubject(situation.facts)
			if subject == "" {
				continue
			}
			history = append(history, world.Post{
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
	}

	developmentTitleFirst.Store(r, &developmentTitleFirstState{
		history: append([]world.Post(nil), history...), attempted: true,
		result: merged, rows: allRows, eraResearchUsed: researchUsed,
	})
	return merged, nil
}
