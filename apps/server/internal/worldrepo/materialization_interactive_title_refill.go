package worldrepo

import (
	"fmt"
	"sort"
	"strings"

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

		// This path should be rare: multiple 20-title pools produced no safe title
		// for a world-selected root. Preserve the event with an explicitly generic,
		// date-safe local title rather than deleting canonical world activity.
		for _, item := range remaining {
			situation := developmentInteractiveRootFallback(item)
			merged[item.eventID] = situation
			history = appendInteractivePlannedRootHistory(history, item, situation)
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

func developmentInteractiveRootFallback(item developmentWindowShell) developmentSparseSituation {
	board := strings.TrimSpace(item.board.Name)
	if board == "" {
		board = "この話題"
	}
	var subject string
	switch item.shell.discourseMode {
	case "ask_peers":
		subject = board + "について教えてください"
	case "share_tip":
		subject = board + "の情報交換"
	case "share_observation":
		subject = board + "について"
	case "announce":
		subject = board + "のお知らせ"
	default:
		subject = board + "について"
	}
	runes := []rune(subject)
	if len(runes) > 36 {
		subject = string(runes[:36])
	}
	summary := fmt.Sprintf("この人物が「%s」を話題にする", subject)
	return developmentSparseSituation{
		kind:    "title_first",
		summary: summary,
		facts: []string{
			"title_first_subject=" + subject,
			"title_first_original=" + subject,
			"title_first_review=World-selected root preservation fallback after candidate pools were exhausted",
			"world_adoption=title_fallback",
			"world_adopted_summary=" + summary,
			"historical_check=generic_no_external_claim",
			"subject_contract=Keep the accepted title verbatim. The accepted title and world_adopted_summary are canonical world facts for this post. Article-local specifics will be added only by the post-adoption Article Detail Materializer.",
		},
	}
}
