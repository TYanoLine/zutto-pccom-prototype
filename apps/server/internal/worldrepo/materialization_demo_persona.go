package worldrepo

import (
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

// MaterializationPersonaArticleHeaders is the second-stage development PoC.
// It keeps the actor-first semantics from MaterializationDenseArticleHeaders but
// avoids letting a short run of lucky random rolls overwhelm persistent traits.
//
// For each persona, the expected visible-post count over the bounded 14-day
// observation window is derived from activity/lurker tendencies and board
// affinity. Deterministic weighted sampling then chooses which days actually
// become visible posts. This is not a production quota system: it is a bounded
// PoC approximation of latent activity that makes persona differences observable
// in a small sample while remaining deterministic/shared after first commit.
func (r *Repository) MaterializationPersonaArticleHeaders(host world.Host, board world.Board) ([]world.Post, bool) {
	if existing := filterBoard(r.Base.ListPosts(host.ID), board.ID); len(existing) > 0 {
		return existing, false
	}

	personas, _ := r.MaterializationPersonas(host)
	stamp := worldTime(r.WorldDate)
	selected := make([]demoPostCandidate, 0, 32)

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
			roll := demoStableUnit(host.ID, board.ID, persona.ID, day.Format("2006-01-02"), "activity-day-v2")
			// Dividing by probability makes high-affinity/high-activity days more
			// competitive without forcing a particular calendar pattern.
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
			selected = append(selected, demoPostCandidate{persona: persona, createdAt: created})
		}
	}

	sort.SliceStable(selected, func(i, j int) bool { return selected[i].createdAt.Before(selected[j].createdAt) })
	return r.materializePersonaCandidates(host, board, selected)
}

func (r *Repository) materializePersonaCandidates(host world.Host, board world.Board, selected []demoPostCandidate) ([]world.Post, bool) {
	topics := demoTopicsForBoard(board)
	topicLastUsed := map[string]time.Time{}
	personaTopicLastUsed := map[string]map[string]time.Time{}
	subjectLastUsed := map[string]time.Time{}
	roots := make([]world.Post, 0, len(selected))
	out := make([]world.Post, 0, len(selected))

	for i, candidate := range selected {
		persona := candidate.persona
		created := candidate.createdAt
		stance := demoPersonaStance(persona)

		var post world.Post
		if root, ok := demoChooseReplyTarget(host, board, persona, created, roots, topics, i); ok && demoShouldReply(host, board, persona, created, i) {
			post = world.Post{
				BoardID:         board.ID,
				ParentID:        root.ID,
				Author:          persona.Handle,
				AuthorPersonaID: persona.ID,
				Subject:         "Re: " + root.Subject,
				Intent: world.PostIntent{
					Action:     "reply",
					Topic:      root.Intent.Topic,
					Motivation: demoReplyMotivation(persona, root),
					Stance:     stance,
				},
				CreatedAt: created,
			}
		} else {
			seed := demoChooseTopic(host, board, persona, created, topics, topicLastUsed, personaTopicLastUsed)
			subject := demoChooseFreshSubject(host, board, persona, created, seed, subjectLastUsed)
			action := "thread_start"
			if seed.role == "sysop" {
				action = "announcement"
			}
			post = world.Post{
				BoardID:         board.ID,
				Author:          persona.Handle,
				AuthorPersonaID: persona.ID,
				Subject:         subject,
				Intent: world.PostIntent{
					Action:     action,
					Topic:      seed.key,
					Motivation: seed.motivation,
					Stance:     stance,
				},
				CreatedAt: created,
			}
			topicLastUsed[seed.key] = created
			if personaTopicLastUsed[persona.ID] == nil {
				personaTopicLastUsed[persona.ID] = map[string]time.Time{}
			}
			personaTopicLastUsed[persona.ID][seed.key] = created
			subjectLastUsed[subject] = created
		}

		post = r.Base.AddPost(host.ID, post)
		out = append(out, post)
		if post.ParentID == 0 {
			roots = append(roots, post)
		}
	}
	return out, true
}

// demoChooseFreshSubject makes exact repeated root titles a last resort inside
// this small observation window. Semantic topic recurrence remains allowed (and
// replies intentionally repeat the parent subject), but a returning topic first
// consumes another natural subject variant before showing the exact same title.
func demoChooseFreshSubject(host world.Host, board world.Board, persona world.Persona, created time.Time, seed demoTopicSeed, used map[string]time.Time) string {
	if len(seed.subjects) == 0 {
		return seed.key
	}
	start := demoStableIndex(len(seed.subjects), host.ID, board.ID, persona.ID, created.Format(time.RFC3339), seed.key, "fresh-subject")
	for offset := 0; offset < len(seed.subjects); offset++ {
		subject := seed.subjects[(start+offset)%len(seed.subjects)]
		if _, alreadyUsed := used[subject]; !alreadyUsed {
			return subject
		}
	}
	return demoChooseSubject(host, board, persona, created, seed, used)
}
