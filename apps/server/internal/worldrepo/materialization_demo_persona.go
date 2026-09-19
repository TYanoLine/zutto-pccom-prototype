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

// MaterializationPersonaArticleHeaders prefers the host-wide producer path when
// the configured materializer supports it. The first board observation selects
// causal actions for ALL boards in the bounded window, sends the complete shell
// set to one producer, and persists detailed article briefs. Later board reads
// only expose the already-produced slice for that board.
//
// The older board-local planner remains as a compatibility path for tests and
// development materializers that do not implement developmentWorldWindowPlanner.
func (r *Repository) MaterializationPersonaArticleHeaders(host world.Host, board world.Board) ([]world.Post, bool) {
	hostPosts := r.Base.ListPosts(host.ID)
	if existing := filterBoard(hostPosts, board.ID); len(existing) > 0 {
		return r.repairDevelopmentPendingReplySubjects(host.ID, existing), false
	}
	if developmentConversationViewPoCEnabled(r) {
		if developmentInteractiveTitleFirstEnabled(r) {
			return r.materializeInteractiveConversationBoardWindow(host, board)
		}
		posts, created := r.materializeConversationWorldWindow(host)
		return filterBoard(posts, board.ID), created
	}
	if hasProducerMaterialization(hostPosts) {
		// A producer window may legitimately leave a board empty. Presence of any
		// producer event proves the host window was already planned, including its
		// silence, so do not manufacture a second board-local plan.
		return nil, false
	}
	if len(hostPosts) == 0 && developmentWorldWindowAvailable(r.Materializer) {
		posts, created := r.materializeProducerWorldWindow(host)
		return filterBoard(posts, board.ID), created
	}

	personas, _ := r.MaterializationPersonas(host)
	visits := developmentVisitsForBoard(host, board, personas, r.WorldDate)
	return r.materializePersonaCandidates(host, board, personas, visits)
}

const developmentDefaultActivityLookbackDays = 14
const developmentInteractiveActivityLookbackDays = 120

func developmentVisitsForBoard(host world.Host, board world.Board, personas []world.Persona, worldDate string) []demoPostCandidate {
	return developmentVisitsForBoardDays(host, board, personas, worldDate, developmentDefaultActivityLookbackDays)
}

func developmentVisitsForBoardDays(host world.Host, board world.Board, personas []world.Persona, worldDate string, lookbackDays int) []demoPostCandidate {
	if lookbackDays < 1 {
		lookbackDays = 1
	}
	stamp := worldTime(worldDate)
	visits := make([]demoPostCandidate, 0, len(personas)*lookbackDays)
	for _, persona := range personas {
		days := make([]demoActivityDay, 0, lookbackDays)
		expected := 0.0
		for dayBack := lookbackDays - 1; dayBack >= 0; dayBack-- {
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
	sort.SliceStable(visits, func(i, j int) bool {
		if visits[i].createdAt.Equal(visits[j].createdAt) {
			return visits[i].persona.ID < visits[j].persona.ID
		}
		return visits[i].createdAt.Before(visits[j].createdAt)
	})
	return visits
}

func (r *Repository) materializePersonaCandidates(host world.Host, board world.Board, personas []world.Persona, visits []demoPostCandidate) ([]world.Post, bool) {
	planner, ok := r.Materializer.(developmentTimelinePlanner)
	if !ok || len(visits) == 0 {
		return nil, false
	}

	// Cheap world simulation ends here. The selected shells already contain a
	// causal reason to exist; semantic planning cannot create posts that the world
	// layer did not select.
	shells, selectionStats := r.selectDevelopmentTimelineShells(host, board, visits)
	storeDevelopmentSelectionStats(r, host.ID, board.ID, selectionStats)
	if len(shells) == 0 {
		return nil, false
	}

	// Compatibility planner: production uses the host-wide producer above. This
	// board-local path stays batched so existing tests/fakes and non-producer
	// development callers remain usable.
	ctx, cancel := context.WithTimeout(context.Background(), developmentPlanningTimeout(len(shells)))
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
			subject = "Re: " + parent.Subject
			if sourcePostID == 0 {
				sourcePostID = parent.ID
			}
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
				DiscourseMode:    shell.discourseMode,
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

func developmentPlanningTimeout(shellCount int) time.Duration {
	batches := (shellCount + developmentPlanningBatchSize - 1) / developmentPlanningBatchSize
	if batches < 1 {
		batches = 1
	}
	return time.Duration(batches)*90*time.Second + 15*time.Second
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
