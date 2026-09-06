package worldrepo

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"zutto-pccom/apps/server/internal/llm"
	"zutto-pccom/apps/server/internal/world"
)

type developmentWindowShell struct {
	eventID string
	board   world.Board
	shell   developmentTimelineShell
}

type developmentWorldWindowPlanEvent struct {
	eventID         string
	subject         string
	episode         string
	referents       []string
	actorKnowledge  []string
	audienceContext []string
	contribution    []string
	mustNot         []string
	topic           string
	motivation      string
	stance          string
	goal            string
	facts           []llm.BBSIntentFactDraft
}

type developmentWorldWindowPlan struct {
	events []developmentWorldWindowPlanEvent
	usage  GenerationUsage
}

type developmentWorldWindowPlanner interface {
	PlanDevelopmentWorldWindow(context.Context, world.Host, string, []developmentWindowShell, map[string][]world.PersonaFact, string) (developmentWorldWindowPlan, error)
}

// PlanDevelopmentWorldWindow is the producer layer between world action
// selection and per-article prose workers. It sees the entire bounded host window
// across boards/personas at once so referents, knowledge and episode ownership can
// be coordinated before any body is rendered.
func (m LLMMaterializer) PlanDevelopmentWorldWindow(ctx context.Context, host world.Host, worldDate string, shells []developmentWindowShell, factsByPersona map[string][]world.PersonaFact, recentBBS string) (developmentWorldWindowPlan, error) {
	producer, ok := m.Renderer.(llm.BBSWorldWindowProducer)
	if !ok {
		return developmentWorldWindowPlan{}, fmt.Errorf("configured renderer does not implement BBS world-window production")
	}
	if len(shells) == 0 {
		return developmentWorldWindowPlan{}, nil
	}

	events := make([]llm.BBSWorldWindowEvent, 0, len(shells))
	windowStart := shells[0].shell.createdAt
	windowEnd := shells[0].shell.createdAt
	for _, item := range shells {
		shell := item.shell
		if shell.createdAt.Before(windowStart) {
			windowStart = shell.createdAt
		}
		if shell.createdAt.After(windowEnd) {
			windowEnd = shell.createdAt
		}
		existingFacts := make([]string, 0, len(factsByPersona[shell.persona.ID]))
		for _, fact := range factsByPersona[shell.persona.ID] {
			existingFacts = append(existingFacts, "BACKGROUND ONLY: "+fact.Key+"="+fact.Value)
		}
		parentID := ""
		if shell.parentIndex != 0 {
			parentID = developmentWindowEventID(item.board.ID, shell.parentIndex)
		}
		sourceID := ""
		if shell.sourceIndex != 0 {
			sourceID = developmentWindowEventID(item.board.ID, shell.sourceIndex)
		}
		events = append(events, llm.BBSWorldWindowEvent{
			EventID:        item.eventID,
			BoardID:        item.board.ID,
			BoardName:      item.board.Name,
			AuthorHandle:   shell.persona.Handle,
			CreatedAt:      shell.createdAt.Format(time.RFC3339),
			Action:         shell.action,
			ParentEventID:  parentID,
			SourceEventID:  sourceID,
			AnchorKey:      shell.anchorKey,
			CauseKind:      shell.causeKind,
			CauseSummary:   shell.causeSummary,
			PersonaProfile: personaSummary(shell.persona),
			ExistingFacts:  existingFacts,
		})
	}

	draft, err := producer.GenerateBBSWorldWindowProduction(ctx, llm.BBSWorldWindowProductionRequest{
		HostName:       host.Name,
		HostRegion:     host.Region,
		HostSoftware:   host.Software,
		WorldDate:      worldDate,
		WindowStart:    windowStart.Format(time.RFC3339),
		WindowEnd:      windowEnd.Format(time.RFC3339),
		EraRules:       "世界時刻より未来の知識を使わない。外部世界の具体的な歴史事実・製品仕様は根拠なしに確定しない。架空住人の個人的事実と史実を区別する。\n" + llm.DiegeticWorldFrame,
		RecentBBSState: recentBBS,
		Events:         events,
	})
	if err != nil {
		return developmentWorldWindowPlan{}, err
	}
	if len(draft.Briefs) != len(shells) {
		return developmentWorldWindowPlan{}, fmt.Errorf("producer returned %d briefs, want %d", len(draft.Briefs), len(shells))
	}

	plans := make([]developmentWorldWindowPlanEvent, 0, len(draft.Briefs))
	for _, brief := range draft.Briefs {
		plans = append(plans, developmentWorldWindowPlanEvent{
			eventID:         strings.TrimSpace(brief.EventID),
			subject:         strings.TrimSpace(brief.Subject),
			episode:         strings.TrimSpace(brief.Episode),
			referents:       append([]string(nil), brief.Referents...),
			actorKnowledge:  append([]string(nil), brief.ActorKnowledge...),
			audienceContext: append([]string(nil), brief.AudienceContext...),
			contribution:    append([]string(nil), brief.Contribution...),
			mustNot:         append([]string(nil), brief.MustNot...),
			topic:           strings.TrimSpace(brief.Topic),
			motivation:      strings.TrimSpace(brief.Motivation),
			stance:          strings.TrimSpace(brief.Stance),
			goal:            strings.TrimSpace(brief.Goal),
			facts:           append([]llm.BBSIntentFactDraft(nil), brief.Facts...),
		})
	}
	return developmentWorldWindowPlan{
		events: plans,
		usage: GenerationUsage{
			InputTokens:       draft.Usage.InputTokens,
			CachedInputTokens: draft.Usage.CachedInputTokens,
			OutputTokens:      draft.Usage.OutputTokens,
			ReasoningTokens:   draft.Usage.ReasoningTokens,
			TotalTokens:       draft.Usage.TotalTokens,
			Model:             draft.Usage.Model,
		},
	}, nil
}

func developmentWindowEventID(boardID string, index int) string {
	return fmt.Sprintf("board-%s:event-%04d", boardID, index)
}

func hasProducerMaterialization(posts []world.Post) bool {
	for _, post := range posts {
		if strings.TrimSpace(post.Intent.ProducerEventID) != "" {
			return true
		}
	}
	return false
}

// materializeProducerWorldWindow performs world action selection independently
// per board, then hands ALL selected shells to one host-wide producer pass. The
// producer therefore coordinates semantics across boards while BoardScope and
// ParticipationState remain world-engine constraints rather than LLM choices.
func (r *Repository) materializeProducerWorldWindow(host world.Host) ([]world.Post, bool) {
	planner, ok := r.Materializer.(developmentWorldWindowPlanner)
	if !ok {
		return nil, false
	}
	if existing := r.Base.ListPosts(host.ID); len(existing) > 0 {
		return existing, false
	}
	boards, _ := r.MaterializationBoards(host)
	personas, _ := r.MaterializationPersonas(host)
	if len(boards) == 0 || len(personas) == 0 {
		return nil, false
	}

	windowShells := make([]developmentWindowShell, 0, 48)
	for _, board := range boards {
		visits := developmentVisitsForBoard(host, board, personas, r.WorldDate)
		shells, stats := selectDevelopmentTimelineShells(host, board, visits)
		storeDevelopmentSelectionStats(r, host.ID, board.ID, stats)
		for _, shell := range shells {
			windowShells = append(windowShells, developmentWindowShell{
				eventID: developmentWindowEventID(board.ID, shell.index),
				board:   board,
				shell:   shell,
			})
		}
	}
	if len(windowShells) == 0 {
		return nil, false
	}
	sort.SliceStable(windowShells, func(i, j int) bool {
		if windowShells[i].shell.createdAt.Equal(windowShells[j].shell.createdAt) {
			return windowShells[i].eventID < windowShells[j].eventID
		}
		return windowShells[i].shell.createdAt.Before(windowShells[j].shell.createdAt)
	})

	ctx, cancel := context.WithTimeout(context.Background(), developmentWorldWindowPlanningTimeout(len(windowShells)))
	defer cancel()
	factsByPersona := r.existingPersonaFactsByID(personas)
	plan, err := planner.PlanDevelopmentWorldWindow(ctx, host, r.WorldDate, windowShells, factsByPersona, planningBBSState(r.Base.ListPosts(host.ID), 24))
	if err != nil || len(plan.events) != len(windowShells) {
		for _, board := range boards {
			storeDevelopmentPlanningError(r, host.ID, board.ID, err)
		}
		return nil, false
	}
	for _, board := range boards {
		clearDevelopmentPlanningError(r, host.ID, board.ID)
	}
	// Record the producer usage once under a diagnostic synthetic board key so it
	// is not accidentally triple-counted as though three independent calls ran.
	storeDevelopmentPlanningUsage(r, host.ID, "world-window", plan.usage)

	briefByID := make(map[string]developmentWorldWindowPlanEvent, len(plan.events))
	for _, event := range plan.events {
		briefByID[event.eventID] = event
	}

	committedByEventID := map[string]world.Post{}
	out := make([]world.Post, 0, len(windowShells))
	for _, item := range windowShells {
		shell := item.shell
		brief, found := briefByID[item.eventID]
		if !found {
			continue
		}
		subject := brief.subject
		parentID := int64(0)
		sourcePostID := int64(0)
		respondsToID := int64(0)
		respondsToClaims := []string(nil)

		if shell.parentIndex != 0 {
			parentEventID := developmentWindowEventID(item.board.ID, shell.parentIndex)
			parent, parentFound := committedByEventID[parentEventID]
			if !parentFound {
				continue
			}
			parentID = parent.ID
			subject = "Re: " + parent.Subject
			respondsToID = parent.ID
			sourcePostID = parent.ID
		}
		if shell.sourceIndex != 0 {
			sourceEventID := developmentWindowEventID(item.board.ID, shell.sourceIndex)
			if source, sourceFound := committedByEventID[sourceEventID]; sourceFound {
				sourcePostID = source.ID
				respondsToID = source.ID
				respondsToClaims = append([]string(nil), source.Intent.Claims...)
			}
		}

		claims := r.commitPlannedFacts(shell.persona, brief.topic, brief.facts, shell.createdAt)
		post := world.Post{
			BoardID:         item.board.ID,
			ParentID:        parentID,
			Author:          shell.persona.Handle,
			AuthorPersonaID: shell.persona.ID,
			Subject:         subject,
			Intent: world.PostIntent{
				Action:                  shell.action,
				AnchorKey:               shell.anchorKey,
				CauseKind:               shell.causeKind,
				SourcePostID:            sourcePostID,
				Topic:                   brief.topic,
				Motivation:              brief.motivation,
				Stance:                  brief.stance,
				Goal:                    brief.goal,
				Claims:                  claims,
				RespondsToClaims:        respondsToClaims,
				RespondsToPostID:        respondsToID,
				ProducerEventID:         item.eventID,
				ProducerEpisode:         brief.episode,
				ProducerReferents:       append([]string(nil), brief.referents...),
				ProducerActorKnowledge:  append([]string(nil), brief.actorKnowledge...),
				ProducerAudienceContext: append([]string(nil), brief.audienceContext...),
				ProducerContribution:    append([]string(nil), brief.contribution...),
				ProducerMustNot:         append([]string(nil), brief.mustNot...),
			},
			CreatedAt: shell.createdAt,
		}
		post = r.Base.AddPost(host.ID, post)
		committedByEventID[item.eventID] = post
		out = append(out, post)
	}
	return out, len(out) > 0
}

func developmentWorldWindowPlanningTimeout(eventCount int) time.Duration {
	// One structured producer request covers the whole bounded window. Give it a
	// larger budget than a six-event board-local batch without allowing an
	// unbounded background job.
	if eventCount > 40 {
		return 210 * time.Second
	}
	return 165 * time.Second
}
