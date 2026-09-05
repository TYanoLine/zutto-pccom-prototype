package worldrepo

import (
	"context"
	"math"
	"sort"
	"time"

	"zutto-pccom/apps/server/internal/world"
)

type demoActivityDay struct {
	persona     world.Persona
	day         time.Time
	probability float64
	score       float64
}

// MaterializationPersonaArticleHeaders first selects plausible board visits over
// the catch-up window. A visit is intentionally NOT a post. The causal action
// selector then decides ROM/no-op vs write, root-vs-reply, and a world anchor
// before any LLM call. Only those already-causal write events are semantically
// realized and persisted.
func (r *Repository) MaterializationPersonaArticleHeaders(host world.Host, board world.Board) ([]world.Post, bool) {
	if existing := filterBoard(r.Base.ListPosts(host.ID), board.ID); len(existing) > 0 {
		return existing, false
	}

	personas, _ := r.MaterializationPersonas(host)
	stamp := worldTime(r.WorldDate)
	visits := make([]demoPostCandidate, 0, 32)

	for _, persona := range personas {
		days := make([]demoActivityDay, 0, 14)
		expected := 0.0
		for dayBack := 13; dayBack >= 0; dayBack-- {
			day := stamp.AddDate(0, 0, -dayBack)
			probability := demoActivityProbability(persona, board)
			if day.Weekday() == time.Saturday || day.Weekday() == time.Sunday {
				probability += .025
				if persona.Handle == "MARI" {
					probability += .04
				}
			}
			probability = clamp01(probability)
			expected += probability
			roll := demoStableUnit(host.ID, board.ID, persona.ID, day.Format("2006-01-02"), "activity-day-v3")
			score := roll / math.Max(probability, .01)
			days = append(days, demoActivityDay{persona: persona, day: day, probability: probability, score: score})
		}

		target := int(math.Round(expected))
		if target < 0 {
			target = 0
		}
		if target > len(days) {
			target = len(days)
		}
		sort.SliceStable(days, func(i, j int) bool {
			if days[i].score == days[j].score {
				return days[i].day.Before(days[j].day)
			}
			return days[i].score < days[j].score
		})
		for _, chosen := range days[:target] {
			created := demoPersonaTimestampForDay(chosen.day, persona, host.ID, board.ID)
			if created.After(stamp) {
				continue
			}
			visits = append(visits, demoPostCandidate{persona: persona, createdAt: created})
		}
	}

	sort.SliceStable(visits, func(i, j int) bool { return visits[i].createdAt.Before(visits[j].createdAt) })
	return r.materializePersonaCandidates(host, board, personas, visits)
}

func (r *Repository) materializePersonaCandidates(host world.Host, board world.Board, personas []world.Persona, visits []demoPostCandidate) ([]world.Post, bool) {
	planner, ok := r.Materializer.(developmentTimelinePlanner)
	if !ok || len(visits) == 0 {
		return nil, false
	}

	// Cheap world simulation ends here. The selected shells already contain a
	// causal reason to exist; semantic planning cannot create posts that the world
	// layer did not select.
	shells, selectionStats := selectDevelopmentTimelineShells(host, board, visits)
	storeDevelopmentSelectionStats(r, host.ID, board.ID, selectionStats)
	if len(shells) == 0 {
		return nil, false
	}

	// Planning is split into small API calls. The deadline covers the complete
	// atomic plan, while each individual HTTP request retains its own tighter
	// client timeout.
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	factsByPersona := r.existingPersonaFactsByID(personas)
	plan, err := planner.PlanDevelopmentTimeline(ctx, host, board, r.WorldDate, shells, factsByPersona, planningBBSState(filterBoard(r.Base.ListPosts(host.ID), board.ID), 12))
	if err != nil || len(plan.events) != len(shells) {
		storeDevelopmentPlanningError(r, host.ID, board.ID, err)
		return nil, false
	}
	storeDevelopmentPlanningUsage(r, host.ID, board.ID, plan.usage)
	clearDevelopmentPlanningError(r, host.ID, board.ID)

	plans := make(map[int]developmentTimelinePlanEvent, len(plan.events))
	for _, event := range plan.events {
		plans[event.index] = event
	}

	committedByIndex := map[int]world.Post{}
	out := make([]world.Post, 0, len(shells))
	for _, shell := range shells {
		semantic, found := plans[shell.index]
		if !found {
			continue
		}
		subject := semantic.subject
		parentID := int64(0)
		respondsToID := int64(0)
		sourcePostID := int64(0)
		if shell.sourceIndex != 0 {
			if source, sourceFound := committedByIndex[shell.sourceIndex]; sourceFound {
				sourcePostID = source.ID
			}
		}
		if shell.action == "reply" {
			parent, parentFound := committedByIndex[shell.parentIndex]
			if !parentFound {
				continue
			}
			parentID = parent.ID
			sourcePostID = parent.ID
			subject = "Re: " + parent.Subject
			if target, targetFound := latestPostInThread(parent.ID, out); targetFound {
				respondsToID = target.ID
			} else {
				respondsToID = parent.ID
			}
		}
		claims := r.commitPlannedFacts(shell.persona, semantic.topic, semantic.facts, shell.createdAt)
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
				SourcePostID:     sourcePostID,
				Topic:            semantic.topic,
				Motivation:       semantic.motivation,
				Stance:           semantic.stance,
				Goal:             semantic.goal,
				Claims:           claims,
				RespondsToPostID: respondsToID,
			},
			CreatedAt: shell.createdAt,
		}
		post = r.Base.AddPost(host.ID, post)
		committedByIndex[shell.index] = post
		out = append(out, post)
	}
	return out, len(out) > 0
}

func latestPostInThread(rootID int64, posts []world.Post) (world.Post, bool) {
	var latest world.Post
	found := false
	for _, post := range posts {
		if post.ID != rootID && post.ParentID != rootID {
			continue
		}
		if !found || post.CreatedAt.After(latest.CreatedAt) || (post.CreatedAt.Equal(latest.CreatedAt) && post.ID > latest.ID) {
			latest = post
			found = true
		}
	}
	return latest, found
}
