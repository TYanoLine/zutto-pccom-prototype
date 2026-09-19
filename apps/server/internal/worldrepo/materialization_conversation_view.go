package worldrepo

import (
	"fmt"
	"sort"
	"strings"
	"sync"

	"zutto-pccom/apps/server/internal/world"
)

const developmentConversationPendingSubject = "（本文生成時に決定）"

func developmentPendingSubject(subject string) bool {
	subject = strings.TrimSpace(subject)
	return subject == developmentConversationPendingSubject || subject == "Re: "+developmentConversationPendingSubject
}

func developmentReplySubject(parentSubject string) string {
	parentSubject = strings.TrimSpace(parentSubject)
	if parentSubject == "" || developmentPendingSubject(parentSubject) {
		return "Re: " + developmentConversationPendingSubject
	}
	if strings.HasPrefix(strings.ToLower(parentSubject), "re: ") {
		return parentSubject
	}
	return "Re: " + parentSubject
}

// repairDevelopmentPendingReplySubjects upgrades rows written by the older
// conversation-view PoC, where replies were persisted as
// "Re: （本文生成時に決定）" even though the parent title was already canonical.
// This is a data repair, not a display-only substitution: the DB remains the
// source of truth after the first read on the fixed build.
func (r *Repository) repairDevelopmentPendingReplySubjects(hostID string, posts []world.Post) []world.Post {
	out := append([]world.Post(nil), posts...)
	byID := make(map[int64]world.Post, len(out))
	for _, post := range out {
		byID[post.ID] = post
	}
	updater, canUpdate := r.Base.(world.PostUpdater)
	for i, post := range out {
		if post.ParentID == 0 || !developmentPendingSubject(post.Subject) {
			continue
		}
		parent, ok := byID[post.ParentID]
		if !ok {
			continue
		}
		corrected := developmentReplySubject(parent.Subject)
		if developmentPendingSubject(corrected) {
			continue
		}
		post.Subject = corrected
		if canUpdate {
			if saved, ok := updater.UpdatePost(hostID, post); ok {
				post = saved
			}
		}
		out[i] = post
		byID[post.ID] = post
	}
	return out
}

var developmentConversationViewPoC sync.Map
var developmentConversationShellLimits sync.Map

// EnableDevelopmentConversationViewPoC enables a development-only materialization
// experiment for this repository instance. The canonical database still owns
// topology, timing and actor identity, but the host-wide semantic producer is
// bypassed. Article prose is instead rendered from a transient conversation view
// reconstructed from canonical BBS records immediately before each write.
func (r *Repository) EnableDevelopmentConversationViewPoC() {
	developmentConversationViewPoC.Store(r, true)
}

func developmentConversationViewPoCEnabled(r *Repository) bool {
	_, ok := developmentConversationViewPoC.Load(r)
	return ok
}

func (r *Repository) SetDevelopmentConversationShellLimit(limit int) {
	if limit < 1 {
		limit = 1
	}
	if limit > 24 {
		limit = 24
	}
	developmentConversationShellLimits.Store(r, limit)
}

func developmentConversationShellLimit(r *Repository) int {
	if value, ok := developmentConversationShellLimits.Load(r); ok {
		return value.(int)
	}
	return developmentWorldWindowPoCMaxShellsPerBoard
}

func limitDevelopmentShellsForConversation(r *Repository, shells []developmentTimelineShell, stats developmentSelectionStats) ([]developmentTimelineShell, developmentSelectionStats) {
	limit := developmentConversationShellLimit(r)
	if len(shells) <= limit {
		return shells, stats
	}
	limited := append([]developmentTimelineShell(nil), shells[:limit]...)
	stats.Posts = len(limited)
	stats.Roots = 0
	stats.Replies = 0
	for _, shell := range limited {
		if shell.action == "reply" {
			stats.Replies++
		} else {
			stats.Roots++
		}
	}
	stats.ROM = stats.Visits - stats.Posts
	if stats.ROM < 0 {
		stats.ROM = 0
	}
	return limited, stats
}

// materializeConversationWorldWindow persists only cheap world-selected shells.
// It intentionally does not invent a producer episode/topic/goal graph. The
// broad routing domain, sparse concrete situation and cause summary are retained
// as world-layer guardrails; natural subject/body wording is chosen later while
// looking at conversation history reconstructed from the DB.
func (r *Repository) materializeConversationWorldWindow(host world.Host) ([]world.Post, bool) {
	if existing := r.Base.ListPosts(host.ID); len(existing) > 0 {
		return existing, false
	}
	boards, _ := r.MaterializationBoards(host)
	personas, _ := r.MaterializationPersonas(host)
	if len(boards) == 0 || len(personas) == 0 {
		return nil, false
	}

	behaviorAdvice := r.developmentJevBehaviorAdvice(host, boards, personas)
	selectionStatsByBoard := map[string]developmentSelectionStats{}
	windowShells := make([]developmentWindowShell, 0, len(boards)*developmentConversationShellLimit(r))
	for _, board := range boards {
		visits := developmentVisitsForBoardWithAdvice(host, board, personas, r.WorldDate, behaviorAdvice)
		shells, stats := r.selectDevelopmentTimelineShellsWithAdvice(host, board, visits, behaviorAdvice)
		shells, stats = limitDevelopmentShellsForConversation(r, shells, stats)
		selectionStatsByBoard[board.ID] = stats
		storeDevelopmentSelectionStats(r, host.ID, board.ID, stats)
		clearDevelopmentPlanningError(r, host.ID, board.ID)
		for _, shell := range shells {
			windowShells = append(windowShells, developmentWindowShell{
				eventID: developmentWindowEventID(board.ID, shell.index),
				board:   board,
				shell:   shell,
			})
		}
	}
	sort.SliceStable(windowShells, func(i, j int) bool {
		if windowShells[i].shell.createdAt.Equal(windowShells[j].shell.createdAt) {
			return windowShells[i].eventID < windowShells[j].eventID
		}
		return windowShells[i].shell.createdAt.Before(windowShells[j].shell.createdAt)
	})

	batchSituations := map[string]developmentSparseSituation{}
	if developmentTitleFirstEnabled(r) {
		planned, err := r.developmentPlanTitleFirst(host, windowShells, personas)
		if err != nil {
			for _, board := range boards {
				storeDevelopmentPlanningError(r, host.ID, board.ID, err)
			}
			return nil, false
		}
		batchSituations = planned
	} else if developmentBatchSituationPoCEnabled(r) {
		planned, err := r.developmentPlanBatchSituations(host, windowShells, personas)
		if err != nil {
			for _, board := range boards {
				storeDevelopmentPlanningError(r, host.ID, board.ID, err)
			}
			return nil, false
		}
		batchSituations = planned
	}

	committedByEventID := map[string]world.Post{}
	titleFirstReplyCounts := map[int64]int{}
	out := make([]world.Post, 0, len(windowShells))
	for _, item := range windowShells {
		shell := item.shell
		if developmentTitleFirstEnabled(r) && shell.action == "thread_start" {
			// In title-first mode every root must come from an adopted title candidate.
			// Continuation/progress shells may still influence later simulation, but
			// they must not surface as a new generic thread with an unrelated title.
			if _, accepted := batchSituations[item.eventID]; !accepted {
				continue
			}
		}
		if developmentTitleFirstEnabled(r) && shell.sourceIndex != 0 {
			if _, exists := committedByEventID[developmentWindowEventID(item.board.ID, shell.sourceIndex)]; !exists {
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
			parentEventID := developmentWindowEventID(item.board.ID, shell.parentIndex)
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
			sourceEventID := developmentWindowEventID(item.board.ID, shell.sourceIndex)
			if source, ok := committedByEventID[sourceEventID]; ok {
				sourcePostID = source.ID
				respondsToID = source.ID
				sourcePost = source
				hasSource = true
			}
		}
		if developmentTitleFirstEnabled(r) && shell.action == "reply" {
			if parentID == 0 || titleFirstReplyCounts[parentID] >= 3 {
				continue
			}
		}

		var source *world.Post
		if hasSource {
			source = &sourcePost
		}
		situation := r.developmentConversationSituationForShell(host, item.board, shell, out, source)
		if proposed, ok := batchSituations[item.eventID]; ok {
			situation = proposed
		}

		if shell.parentIndex == 0 && shell.sourceIndex == 0 {
			if fixed := titleFirstSubject(situation.facts); fixed != "" {
				subject = fixed
			}
		} else if developmentTitleFirstEnabled(r) {
			filtered := []string{}
			for _, fact := range situation.facts {
				if !strings.HasPrefix(fact, "title_first_") && !strings.HasPrefix(fact, "subject_contract=") {
					filtered = append(filtered, fact)
				}
			}
			situation.facts = filtered
		}
		post := world.Post{
			BoardID:         item.board.ID,
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
		if developmentTitleFirstEnabled(r) {
			stats := selectionStatsByBoard[item.board.ID]
			if shell.action == "reply" && parentID != 0 {
				titleFirstReplyCounts[parentID]++
				stats.MaterializedReplies++
			} else if shell.action == "thread_start" {
				stats.MaterializedRoots++
			}
			selectionStatsByBoard[item.board.ID] = stats
			storeDevelopmentSelectionStats(r, host.ID, item.board.ID, stats)
		}
		out = append(out, post)
	}
	return out, len(out) > 0
}

func (r *Repository) materializationRenderContext(host world.Host, board world.Board, selected world.Post) (string, demoRenderContextStats) {
	if !developmentConversationViewPoCEnabled(r) {
		return r.materializationBBSRenderContext(host, board, selected)
	}
	return r.materializationConversationViewContext(host, board, selected)
}

func (r *Repository) materializationConversationViewContext(host world.Host, board world.Board, selected world.Post) (string, demoRenderContextStats) {
	canonical, stats := r.materializationBBSRenderContext(host, board, selected)
	var b strings.Builder
	b.WriteString("CONVERSATION VIEW POC — transient rendering input rebuilt from canonical DB records.\n")
	b.WriteString("Do not treat this view as new world memory; write as the selected member inside the conversation already shown.\n")
	fmt.Fprintf(&b, "CURRENT WORLD SLOT: action=%s; routing_domain=%s; cause_kind=%s; discourse_mode=%s\n",
		selected.Intent.Action, selected.Intent.AnchorKey, selected.Intent.CauseKind, selected.Intent.DiscourseMode)
	if strings.TrimSpace(selected.Intent.Motivation) != "" {
		fmt.Fprintf(&b, "WORLD-LAYER CAUSE BOUNDARY: %s\n", strings.TrimSpace(selected.Intent.Motivation))
	}
	if strings.TrimSpace(selected.Intent.SituationKind) != "" || strings.TrimSpace(selected.Intent.SituationSummary) != "" {
		fmt.Fprintf(&b, "WORLD SITUATION: kind=%s\n", strings.TrimSpace(selected.Intent.SituationKind))
		if strings.TrimSpace(selected.Intent.SituationSummary) != "" {
			b.WriteString(strings.TrimSpace(selected.Intent.SituationSummary))
			b.WriteString("\n")
		}
		for _, fact := range selected.Intent.SituationFacts {
			if strings.TrimSpace(fact) != "" {
				b.WriteString("- ")
				b.WriteString(strings.TrimSpace(fact))
				b.WriteString("\n")
			}
		}
	}
	if selected.ParentID == 0 && selected.Intent.SourcePostID == 0 {
		b.WriteString("ROOT ISOLATION: This root is its own world situation. Other roots and this actor's earlier posts are not causes or shared events. Do not borrow their concrete event details merely because they appear below.\n")
	}
	if selected.Intent.DiscourseMode == "ask_peers" {
		b.WriteString("ANSWERABILITY: The question must contain enough concrete referent/observable detail that another member can answer from this article without guessing an unnamed title, place, product, device or hidden choice.\n")
	}

	all := r.Base.ListPosts(host.ID)
	if selected.Intent.SourcePostID != 0 {
		if source, ok := developmentConversationFindPost(all, selected.Intent.SourcePostID); ok && postBefore(source, selected) {
			b.WriteString("\nEXPLICIT CANONICAL SOURCE FOR THIS WORLD SLOT:\n")
			writeDevelopmentConversationPost(&b, source)
		}
	}

	recent := developmentConversationRecentActorPosts(all, selected, 3)
	b.WriteString("\nTHIS ACTOR'S RECENT CANONICAL POSTS — continuity/style context only. DO NOT merge their events into the current situation unless an explicit source/continuation edge above says to do so:\n")
	if len(recent) == 0 {
		b.WriteString("(none)\n")
	}
	for _, post := range recent {
		writeDevelopmentConversationPost(&b, post)
	}

	b.WriteString("\nCANONICAL BOARD CONVERSATION:\n")
	b.WriteString(canonical)
	return b.String(), stats
}

func developmentConversationFindPost(posts []world.Post, id int64) (world.Post, bool) {
	for _, post := range posts {
		if post.ID == id {
			return post, true
		}
	}
	return world.Post{}, false
}

func developmentConversationRecentActorPosts(all []world.Post, selected world.Post, limit int) []world.Post {
	out := make([]world.Post, 0, limit)
	for _, post := range all {
		if post.ID == selected.ID || !postBefore(post, selected) {
			continue
		}
		sameActor := selected.AuthorPersonaID != "" && post.AuthorPersonaID == selected.AuthorPersonaID
		if !sameActor && !strings.EqualFold(post.Author, selected.Author) {
			continue
		}
		if post.ID == selected.Intent.SourcePostID || post.ID == selected.ParentID {
			continue
		}
		out = append(out, post)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].CreatedAt.Equal(out[j].CreatedAt) {
			return out[i].ID > out[j].ID
		}
		return out[i].CreatedAt.After(out[j].CreatedAt)
	})
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].CreatedAt.Equal(out[j].CreatedAt) {
			return out[i].ID < out[j].ID
		}
		return out[i].CreatedAt.Before(out[j].CreatedAt)
	})
	return out
}

func writeDevelopmentConversationPost(b *strings.Builder, post world.Post) {
	fmt.Fprintf(b, "[MSG %04d board=%s %s %s] %s\n", post.ID, post.BoardID, post.CreatedAt.Format("01/02 15:04"), post.Author, post.Subject)
	if strings.TrimSpace(post.Body) != "" {
		b.WriteString(truncateDemoContext(strings.TrimSpace(post.Body), 420))
		b.WriteString("\n")
		return
	}
	b.WriteString("semantic shell: ")
	b.WriteString(demoPostSemanticSummary(post))
	b.WriteString("\n")
}

func (r *Repository) developmentConversationRenderedSubject(hostID string, selected world.Post, generated string) string {
	generated = strings.TrimSpace(generated)
	if selected.ParentID == 0 {
		if generated != "" {
			return generated
		}
		return selected.Subject
	}
	if parent, ok := developmentConversationFindPost(r.Base.ListPosts(hostID), selected.ParentID); ok {
		subject := strings.TrimSpace(parent.Subject)
		if subject != "" && subject != developmentConversationPendingSubject {
			return "Re: " + normalizeDemoSubject(subject)
		}
	}
	if generated != "" {
		return generated
	}
	return selected.Subject
}
