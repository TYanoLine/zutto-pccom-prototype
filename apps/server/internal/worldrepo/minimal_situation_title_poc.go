package worldrepo

import (
	"time"

	"zutto-pccom/apps/server/internal/world"
)

// DevelopmentMinimalRootSlot exposes the world-selected, non-persisted root shell
// for the minimal situation->title/body PoC. It deliberately contains only
// canonical world selection data plus the cheap deterministic sparse situation.
type DevelopmentMinimalRootSlot struct {
	EventID          string   `json:"event_id"`
	BoardID          string   `json:"board_id"`
	BoardName        string   `json:"board_name"`
	AuthorHandle     string   `json:"author_handle"`
	CreatedAt        string   `json:"created_at"`
	Action           string   `json:"action"`
	AnchorKey        string   `json:"anchor_key"`
	CauseKind        string   `json:"cause_kind"`
	CauseSummary     string   `json:"cause_summary"`
	DiscourseMode    string   `json:"discourse_mode"`
	PersonaProfile   string   `json:"persona_profile"`
	SituationKind    string   `json:"situation_kind"`
	SituationSummary string   `json:"situation_summary"`
	SituationFacts   []string `json:"situation_facts"`
}

// DevelopmentMinimalRootSlots recreates the current board's world-selected root
// slots without committing posts or invoking any title/body planner.
func (r *Repository) DevelopmentMinimalRootSlots(host world.Host, board world.Board, limit int) []DevelopmentMinimalRootSlot {
	return r.DevelopmentMinimalRootSlotsWindow(host, board, limit, developmentInteractiveActivityLookbackDays, developmentMaxPostsPerBoardCatchup)
}

// DevelopmentMinimalRootSlotsWindow is diagnostic-only. It reuses the same
// deterministic world visit/write/topology selection as production while allowing
// a larger observation window and catch-up cap for statistical PoCs. Production
// callers keep the normal 120-day/28-post bounds above.
func (r *Repository) DevelopmentMinimalRootSlotsWindow(host world.Host, board world.Board, limit, lookbackDays, maxPosts int) []DevelopmentMinimalRootSlot {
	if limit < 1 {
		return nil
	}
	if limit > 100 {
		limit = 100
	}
	if lookbackDays < 1 {
		lookbackDays = 1
	}
	if lookbackDays > 730 {
		lookbackDays = 730
	}
	if maxPosts < developmentMaxPostsPerBoardCatchup {
		maxPosts = developmentMaxPostsPerBoardCatchup
	}
	if maxPosts > 600 {
		maxPosts = 600
	}
	personas, _ := r.MaterializationPersonas(host)
	if len(personas) == 0 {
		return nil
	}
	behaviorAdvice := r.developmentJevBehaviorAdvice(host, []world.Board{board}, personas)
	visits := developmentVisitsForBoardDaysWithAdvice(host, board, personas, r.WorldDate, lookbackDays, behaviorAdvice)
	shells, _ := selectDevelopmentTimelineShellsWithBehaviorAdviceLimit(host, board, visits, behaviorAdvice, maxPosts)
	prior := filterBoard(r.Base.ListPosts(host.ID), board.ID)

	out := make([]DevelopmentMinimalRootSlot, 0, limit)
	for _, shell := range shells {
		if shell.action != "thread_start" || shell.parentIndex != 0 || shell.sourceIndex != 0 {
			continue
		}
		situation := developmentSituationForShell(host, board, shell, prior, nil)
		out = append(out, DevelopmentMinimalRootSlot{
			EventID:          developmentWindowEventID(board.ID, shell.index),
			BoardID:          board.ID,
			BoardName:        board.Name,
			AuthorHandle:     shell.persona.Handle,
			CreatedAt:        shell.createdAt.Format(time.RFC3339),
			Action:           shell.action,
			AnchorKey:        shell.anchorKey,
			CauseKind:        shell.causeKind,
			CauseSummary:     shell.causeSummary,
			DiscourseMode:    shell.discourseMode,
			PersonaProfile:   personaSummary(shell.persona),
			SituationKind:    situation.kind,
			SituationSummary: situation.summary,
			SituationFacts:   append([]string(nil), situation.facts...),
		})
		if len(out) >= limit {
			break
		}
	}
	return out
}