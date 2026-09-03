package worldrepo

import (
	"fmt"
	"sort"
	"time"

	"zutto-pccom/apps/server/internal/world"
)

type demoScoredInformationSlot struct {
	slot  string
	score float64
}

// demoNaturalInformationSlots uses semantic slots only as lightweight novelty
// metadata. They do not form a checklist and they never force the next member to
// ask about the next uncovered dimension. Which personal details become concrete
// is sampled from the actor/topic/context and then persisted as normal PersonaFact
// world truth.
func demoNaturalInformationSlots(host world.Host, board world.Board, persona world.Persona, topic string, thread []world.Post, created time.Time) []string {
	available := demoAvailablePersonaSlots(persona, topic)
	if len(available) == 0 {
		return nil
	}
	covered := demoCoveredInformationSlots(thread)
	scored := make([]demoScoredInformationSlot, 0, len(available))
	for _, slot := range available {
		score := demoStableUnit(host.ID, board.ID, persona.ID, topic, slot, created.Format(time.RFC3339), "natural-slot-v1")
		// Novel information is a soft preference, not a requirement. A resident may
		// still repeat a dimension when their own experience is relevant.
		if !covered[slot] {
			score += .28
		}
		// Interests influence which part of a topic comes to mind without creating a
		// universal fixed order for the whole thread.
		switch slot {
		case "configuration":
			score += persona.Interests["software"] * .16
		case "modem":
			score += persona.Interests["modem"] * .18
		case "usage_pattern", "shared_machine", "other_usage":
			score += persona.Interests["pc98"] * .10
		}
		scored = append(scored, demoScoredInformationSlot{slot: slot, score: score})
	}
	sort.SliceStable(scored, func(i, j int) bool { return scored[i].score > scored[j].score })

	count := 1
	if len(scored) > 1 {
		secondChance := .22 + (1-persona.LurkerTendency)*.20
		if persona.Handle == "NORI" {
			secondChance += .18
		}
		if demoStableUnit(host.ID, board.ID, persona.ID, created.Format(time.RFC3339), "second-detail-v1") < secondChance {
			count = 2
		}
	}
	if count > len(scored) {
		count = len(scored)
	}
	out := make([]string, 0, count)
	for _, candidate := range scored[:count] {
		out = append(out, candidate.slot)
	}
	return out
}

func demoNaturalResponseAct(host world.Host, board world.Board, persona world.Persona, root world.Post, thread []world.Post, created time.Time) string {
	roll := demoStableUnit(host.ID, board.ID, persona.ID, fmt.Sprint(root.ID), created.Format(time.RFC3339), "response-act-v1")

	// These are actor tendencies, not a prescribed thread progression. Brief
	// agreement, a personal example, disagreement and an occasional question are
	// all valid; no action is obliged to reveal a new semantic dimension.
	if persona.Argumentativeness >= .35 && roll < .24 {
		return "different_view"
	}
	if persona.LurkerTendency >= .50 && roll < .42 {
		return "brief_reaction"
	}
	if persona.Handle == "NEKO" && roll > .82 {
		return "ask_naturally"
	}
	if persona.NewcomerOpenness >= .80 && roll > .70 {
		return "friendly_compare"
	}
	if len(thread) >= 3 && roll < .22 {
		return "pick_up_recent_point"
	}
	return "share_experience"
}

func (r *Repository) demoNaturalReplyEnvelopeWithThread(host world.Host, board world.Board, persona world.Persona, root world.Post, thread []world.Post, created time.Time) world.Post {
	target := demoLatestSemanticTarget(thread)
	if target.ID == 0 {
		target = root
	}
	slots := demoNaturalInformationSlots(host, board, persona, root.Intent.Topic, thread, created)
	facts := r.materializeDemoPersonaFactsForSlots(persona, root.Intent.Topic, slots, created)
	claims := demoClaimsFromPersonaFacts(facts)
	if len(claims) == 0 {
		claims = []string{fmt.Sprintf("%sについて、自分の経験からひとこと返したい", root.Subject)}
	}
	return world.Post{
		BoardID:         board.ID,
		ParentID:        root.ID,
		Author:          persona.Handle,
		AuthorPersonaID: persona.ID,
		Subject:         "Re: " + root.Subject,
		Intent: world.PostIntent{
			Action:           "reply",
			Topic:            root.Intent.Topic,
			Motivation:       demoReplyMotivation(persona, root),
			Stance:           demoPersonaStance(persona),
			Claims:           claims,
			RespondsToClaims: demoRespondsToClaims(target),
			RespondsToPostID: target.ID,
			ResponseAct:      demoNaturalResponseAct(host, board, persona, root, thread, created),
			InformationSlots: slots,
		},
		CreatedAt: created,
	}
}

func (r *Repository) demoNaturalRootEnvelope(host world.Host, board world.Board, persona world.Persona, seed demoTopicSeed, subject string, created time.Time) world.Post {
	action := "thread_start"
	if seed.role == "sysop" {
		action = "announcement"
	}
	slots := demoNaturalInformationSlots(host, board, persona, seed.key, nil, created)
	facts := r.materializeDemoPersonaFactsForSlots(persona, seed.key, slots, created)
	return world.Post{
		BoardID:         board.ID,
		Author:          persona.Handle,
		AuthorPersonaID: persona.ID,
		Subject:         subject,
		Intent: world.PostIntent{
			Action:           action,
			Topic:            seed.key,
			Motivation:       seed.motivation,
			Stance:           demoPersonaStance(persona),
			Claims:           demoClaimsFromPersonaFacts(facts),
			ResponseAct:      "thread_start",
			InformationSlots: slots,
		},
		CreatedAt: created,
	}
}
