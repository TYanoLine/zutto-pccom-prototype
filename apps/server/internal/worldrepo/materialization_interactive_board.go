package worldrepo

import (
	"sort"
	"strings"

	"zutto-pccom/apps/server/internal/world"
)

// materializeInteractiveConversationBoardWindow plans only the board the caller
// actually opened. The fresh Lab intentionally plans a whole bounded host window
// for evaluation, but doing that synchronously in the dial-up UI makes a simple
// article-index request wait for unrelated boards.
func (r *Repository) materializeInteractiveConversationBoardWindow(host world.Host, board world.Board) ([]world.Post, bool) {
	if existing := filterBoard(r.Base.ListPosts(host.ID), board.ID); len(existing) > 0 {
		return existing, false
	}

	planningMu := developmentInteractiveTitleFirstPlanningMutex(r)
	planningMu.Lock()
	defer planningMu.Unlock()
	// Another session may have completed this board while this caller waited for
	// the shared title planner lock.
	if existing := filterBoard(r.Base.ListPosts(host.ID), board.ID); len(existing) > 0 {
		return existing, false
	}

	personas, _ := r.MaterializationPersonas(host)
	if len(personas) == 0 {
		return nil, false
	}

	visits := developmentVisitsForBoard(host, board, personas, r.WorldDate)
	shells, stats := selectDevelopmentTimelineShells(host, board, visits)
	shells, stats = limitDevelopmentShellsForConversation(r, shells, stats)
	storeDevelopmentSelectionStats(r, host.ID, board.ID, stats)
	clearDevelopmentPlanningError(r, host.ID, board.ID)
	if len(shells) == 0 {
		return nil, false
	}

	windowShells := make([]developmentWindowShell, 0, len(shells))
	for _, shell := range shells {
		windowShells = append(windowShells, developmentWindowShell{
			eventID: developmentWindowEventID(board.ID, shell.index),
			board:   board,
			shell:   shell,
		})
	}
	sort.SliceStable(windowShells, func(i, j int) bool {
		if windowShells[i].shell.createdAt.Equal(windowShells[j].shell.createdAt) {
			return windowShells[i].eventID < windowShells[j].eventID
		}
		return windowShells[i].shell.createdAt.Before(windowShells[j].shell.createdAt)
	})

	// The Lab keeps one host-wide planning state. Interactive browsing is board
	// local: rearm title planning with already-committed posts as conversation
	// history so opening board 2 never forces board 1 to be regenerated.
	r.EnableDevelopmentTitleFirstPoC(r.Base.ListPosts(host.ID))
	batchSituations, err := r.developmentPlanTitleFirst(host, windowShells, personas)
	if err != nil {
		storeDevelopmentPlanningError(r, host.ID, board.ID, err)
		return nil, false
	}

	committedByEventID := map[string]world.Post{}
	out := make([]world.Post, 0, len(windowShells))
	for _, item := range windowShells {
		shell := item.shell
		if shell.parentIndex == 0 && shell.sourceIndex == 0 {
			if _, accepted := batchSituations[item.eventID]; !accepted {
				continue
			}
		}
		if shell.sourceIndex != 0 {
			if _, exists := committedByEventID[developmentWindowEventID(board.ID, shell.sourceIndex)]; !exists {
				continue
			}
		}

		parentID := int64(0)
		sourcePostID := int64(0)
		respondsToID := int64(0)
		subject := developmentConversationPendingSubject
		var sourcePost world.Post
		hasSource := false

		if shell.parentIndex != 0 {
			parentEventID := developmentWindowEventID(board.ID, shell.parentIndex)
			parent, ok := committedByEventID[parentEventID]
			if !ok {
				continue
			}
			parentID = parent.ID
			subject = developmentReplySubject(parent.Subject)
			sourcePostID = parent.ID
			respondsToID = parent.ID
			sourcePost = parent
			hasSource = true
		}
		if shell.sourceIndex != 0 {
			sourceEventID := developmentWindowEventID(board.ID, shell.sourceIndex)
			if source, ok := committedByEventID[sourceEventID]; ok {
				sourcePostID = source.ID
				respondsToID = source.ID
				sourcePost = source
				hasSource = true
			}
		}

		var source *world.Post
		if hasSource {
			source = &sourcePost
		}
		situation := r.developmentConversationSituationForShell(host, board, shell, out, source)
		if proposed, ok := batchSituations[item.eventID]; ok {
			situation = proposed
		}

		if shell.parentIndex == 0 && shell.sourceIndex == 0 {
			if fixed := titleFirstSubject(situation.facts); fixed != "" {
				subject = fixed
			}
		} else {
			filtered := make([]string, 0, len(situation.facts))
			for _, fact := range situation.facts {
				if !strings.HasPrefix(fact, "title_first_") && !strings.HasPrefix(fact, "subject_contract=") {
					filtered = append(filtered, fact)
				}
			}
			situation.facts = filtered
		}

		post := world.Post{
			BoardID:         board.ID,
			ParentID:        parentID,
			Author:          shell.persona.Handle,
			AuthorPersonaID: shell.persona.ID,
			Subject:         subject,
			Intent: world.PostIntent{
				Action:           shell.action,
				AnchorKey:        shell.anchorKey,
				CauseKind:        shell.causeKind,
				DiscourseMode:    shell.discourseMode,
				SourcePostID:     sourcePostID,
				SituationKind:    situation.kind,
				SituationSummary: situation.summary,
				SituationFacts:   append([]string(nil), situation.facts...),
				Topic:            shell.anchorKey,
				Motivation:       shell.causeSummary,
				RespondsToPostID: respondsToID,
			},
			CreatedAt: shell.createdAt,
		}
		post = r.Base.AddPost(host.ID, post)
		committedByEventID[item.eventID] = post
		out = append(out, post)
	}
	return out, len(out) > 0
}
