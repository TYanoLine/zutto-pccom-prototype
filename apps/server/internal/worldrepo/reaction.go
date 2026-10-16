package worldrepo

import (
	"fmt"
	"strings"

	"zutto-pccom/apps/server/internal/hostprogram"
	"zutto-pccom/apps/server/internal/world"
)

func (r *Repository) planHumanPostReactions(hostID string, saved world.Post) {
	if r == nil || r.Base == nil || strings.TrimSpace(hostID) == "" || strings.TrimSpace(saved.BoardID) == "" {
		return
	}
	// Only human-authored posts trigger immediate reaction planning in AddPost.
	if saved.AuthorPersonaID != "" {
		return
	}
	if saved.Intent.Action == "bbs-world-catchup" || saved.Intent.Action == "world-catchup" {
		return
	}

	planStore, ok := r.Base.(world.ReactionPlanStore)
	if !ok {
		return
	}
	if _, exists := planStore.ReactionPlan(hostID, saved.ID); exists {
		return
	}

	personaSource, ok := r.Base.(world.PersonaStore)
	if !ok {
		return
	}
	personas := personaSource.ListHostPersonas(hostID)
	if len(personas) == 0 {
		return
	}

	host := r.resolveHostByID(hostID)
	board := r.resolveBoardByID(host, saved.BoardID)

	factsByPersona := make(map[string][]world.PersonaFact, len(personas))
	if factStore, ok := r.Base.(world.PersonaFactStore); ok {
		for _, p := range personas {
			if facts := factStore.ListPersonaFacts(p.ID); len(facts) > 0 {
				factsByPersona[p.ID] = facts
			}
		}
	}

	allBoardPosts := filterBoard(r.Base.ListPosts(hostID), saved.BoardID)
	ordinal := r.nextReactionOrdinal(host, board, allBoardPosts)
	replyRate := board.ReplyRate
	if replyRate <= 0 && board.Name == "" {
		if state, ok := r.BoardActivity(host, board); ok && state.ReplyRate > 0 {
			replyRate = state.ReplyRate
		} else {
			replyRate = 1.35
		}
	}

	threadState := buildThreadStateForPost(allBoardPosts, saved, board.WelcomesNewcomers)
	plannedAt := saved.CreatedAt
	if plannedAt.IsZero() {
		plannedAt = r.currentWorldTime()
	}

	in := world.PlanInput{
		Host:           host,
		Board:          board,
		TargetPost:     saved,
		Ordinal:        ordinal,
		ReplyRate:      replyRate,
		PlannedAt:      plannedAt,
		Personas:       personas,
		FactsByPersona: factsByPersona,
		BoardPrior:     board.Topics,
		Lexicon:        world.DefaultLexicon(),
		RhythmLayers:   world.DefaultRhythmLayers(),
		Thread:         threadState,
	}

	plan := world.PlanReactions(in)
	planStore.SaveReactionPlan(hostID, plan)
}

func (r *Repository) resolveHostByID(hostID string) world.Host {
	r.mu.Lock()
	h, ok := r.hosts[hostID]
	r.mu.Unlock()
	if ok && h.ID != "" {
		return h
	}
	return world.Host{ID: hostID}
}

func (r *Repository) resolveBoardByID(host world.Host, boardID string) world.Board {
	if bs, ok := r.Base.(world.BoardStore); ok {
		for _, b := range bs.ListBoards(host.ID) {
			if b.ID == boardID {
				return b
			}
		}
	}
	if ds, ok := r.Base.(world.HostDetailStore); ok {
		if detail, ok := ds.HostDetail(host.ID); ok && detail.ErikaK != nil {
			for _, eb := range detail.ErikaK.Boards {
				if eb.Path == boardID {
					return world.Board{
						ID:                   eb.Path,
						Name:                 eb.Name,
						SemanticScope:        eb.Scope,
						RootAuthorPolicy:     eb.RootAuthorPolicy,
						ActivityWeight:       eb.ActivityWeight,
						ReplyRate:            eb.ReplyRate,
						RetainedRootCap:      eb.RetainedRootCap,
						VerifiedReferentRate: eb.VerifiedReferentRate,
						Topics:               eb.Topics,
						WelcomesNewcomers:    eb.WelcomesNewcomers,
						Rhythm:               eb.Rhythm,
					}
				}
			}
		}
	}
	return world.Board{ID: boardID}
}

func (r *Repository) nextReactionOrdinal(host world.Host, board world.Board, boardPosts []world.Post) int {
	baseOrdinal := 0
	if state, ok := r.BoardActivity(host, board); ok && state.TotalRoots > 0 {
		baseOrdinal = state.TotalRoots
	}
	rootCount := 0
	for _, p := range boardPosts {
		if world.IsSemanticRoot(p) {
			rootCount++
		}
	}
	ord := baseOrdinal + rootCount
	if ord < 1 {
		ord = 1
	}
	return ord
}

func buildThreadStateForPost(boardPosts []world.Post, target world.Post, boardWelcomes bool) world.ThreadState {
	byID := make(map[int64]world.Post, len(boardPosts))
	for _, p := range boardPosts {
		byID[p.ID] = p
	}
	rootID := threadRootID(byID, target)
	rootPost := byID[rootID]

	priorCounts := map[string]int{}
	lastPersonaID := ""
	for _, p := range boardPosts {
		if p.ID == target.ID {
			continue
		}
		if threadRootID(byID, p) != rootID {
			continue
		}
		if p.ID != rootID && p.AuthorPersonaID != "" {
			priorCounts[p.AuthorPersonaID]++
		}
		if p.AuthorPersonaID != "" {
			lastPersonaID = p.AuthorPersonaID
		}
	}

	authorPostCount := 0
	targetActor := world.PostActor(target)
	for _, p := range boardPosts {
		if !targetActor.IsZero() && world.PostActor(p) == targetActor {
			authorPostCount++
		}
	}
	isNewcomer := boardWelcomes || authorPostCount <= 1

	return world.ThreadState{
		StarterPersonaID: rootPost.AuthorPersonaID,
		ParentPersonaID:  target.AuthorPersonaID,
		LastPersonaID:    lastPersonaID,
		PriorReplyCounts: priorCounts,
		IsNewcomerPost:   isNewcomer,
		Relationship:     world.ReactionRelationNeutral,
	}
}

// materializeDueReactions materializes header-only reply posts for any planned
// reaction on board whose DueAt <= worldNow and MaterializedPostID == 0.
// It is idempotent and never calls an LLM during header materialization.
func (r *Repository) materializeDueReactions(host world.Host, board world.Board) int {
	if r == nil || r.Base == nil || strings.TrimSpace(host.ID) == "" || strings.TrimSpace(board.ID) == "" {
		return 0
	}
	planStore, ok := r.Base.(world.ReactionPlanStore)
	if !ok {
		return 0
	}
	now := r.currentWorldTime()
	plans := planStore.ListReactionPlans(host.ID, board.ID)
	if len(plans) == 0 {
		return 0
	}

	allPosts := r.Base.ListPosts(host.ID)
	postsByID := make(map[int64]world.Post, len(allPosts))
	byEventID := make(map[string]world.Post)
	for _, p := range allPosts {
		postsByID[p.ID] = p
		if p.Intent.ProducerEventID != "" {
			byEventID[p.Intent.ProducerEventID] = p
		}
	}

	personaByID := map[string]world.Persona{}
	if ps, ok := r.Base.(world.PersonaStore); ok {
		for _, p := range ps.ListHostPersonas(host.ID) {
			personaByID[p.ID] = p
		}
	}

	createdCount := 0
	for _, plan := range plans {
		targetPost, exists := postsByID[plan.TargetPostID]
		if !exists {
			continue
		}
		planChanged := false
		for i := range plan.Replies {
			rep := &plan.Replies[i]
			if rep.MaterializedPostID != 0 {
				continue
			}
			if rep.DueAt.After(now) {
				continue
			}
			eventID := fmt.Sprintf("react:%d:%d", plan.TargetPostID, rep.Index)
			if existing, dup := byEventID[eventID]; dup {
				rep.MaterializedPostID = existing.ID
				planChanged = true
				continue
			}

			persona, hasPersona := personaByID[rep.PersonaID]
			handle := strings.TrimSpace(persona.Handle)
			if !hasPersona || handle == "" {
				handle = rep.PersonaID
			}

			proposedSubject := defaultReplySubject(targetPost.Subject)
			parentID := targetPost.ID
			subject := proposedSubject
			if projected, err := hostprogram.ProjectReply(host, targetPost, proposedSubject); err == nil {
				parentID = projected.ParentID
				subject = strings.TrimSpace(projected.Subject)
			}

			topics := world.ExtractPostTopics(targetPost, world.DefaultLexicon())
			topicStr := targetPost.Intent.Topic
			if topicStr == "" && len(topics) > 0 {
				topicStr = topics[0]
			}
			if topicStr == "" {
				topicStr = strings.TrimSpace(targetPost.Subject)
			}
			summary := fmt.Sprintf("%sの投稿「%s」に対する%sからの返信", targetPost.Author, targetPost.Subject, handle)

			newPost := world.Post{
				BoardID:         board.ID,
				ParentID:        parentID,
				Author:          handle,
				AuthorPersonaID: rep.PersonaID,
				Subject:         subject,
				Body:            "",
				CreatedAt:       rep.DueAt,
				Intent: world.PostIntent{
					Action:                     "persona-reaction",
					AnchorKey:                  targetPost.Intent.AnchorKey,
					CauseKind:                  "persona_reaction",
					DiscourseMode:              "reply",
					SourcePostID:               targetPost.ID,
					RespondsToPostID:           targetPost.ID,
					ProducerEventID:            eventID,
					Topic:                      topicStr,
					Topics:                     topics,
					SituationKind:              "reply",
					SituationSummary:           summary,
					ArticleDetailsMaterialized: true,
					SituationFacts: []string{
						fmt.Sprintf("responds_to_post_id=%d", targetPost.ID),
						"world_adopted_summary=" + summary,
					},
				},
			}
			saved := r.AddPost(host.ID, newPost)
			rep.MaterializedPostID = saved.ID
			postsByID[saved.ID] = saved
			byEventID[eventID] = saved
			planChanged = true
			createdCount++
		}
		if planChanged {
			planStore.SaveReactionPlan(host.ID, plan)
		}
	}
	return createdCount
}

func defaultReplySubject(parentSubject string) string {
	s := strings.TrimSpace(parentSubject)
	if s == "" {
		return "Re:"
	}
	upper := strings.ToUpper(s)
	if strings.HasPrefix(upper, "RE:") || strings.HasPrefix(upper, "RE^") {
		return s
	}
	return "Re: " + s
}
