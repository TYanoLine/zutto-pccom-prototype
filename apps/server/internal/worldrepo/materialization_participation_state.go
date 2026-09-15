package worldrepo

import (
	"fmt"
	"math"
	"time"

	"zutto-pccom/apps/server/internal/world"
)

// developmentReplyTarget is the world-selected thread participation edge for a
// reply. root fixes the canonical thread; source fixes the particular already-
// existing contribution that made this reply possible now.
type developmentReplyTarget struct {
	root      developmentTimelineShell
	source    developmentTimelineShell
	returning bool
}

// demoParticipationReplySource enforces the development ParticipationState
// invariant without inspecting prose. A persona's first contribution may react
// to the latest contribution already present in the thread. A returning persona
// is eligible only when another actor has contributed after their own most recent
// contribution. Merely revisiting/re-reading the same unchanged thread is ROM.
func demoParticipationReplySource(p world.Persona, root developmentTimelineShell, shells []developmentTimelineShell) (developmentTimelineShell, bool, bool) {
	lastActorIndex := 0
	latestThreadContribution := root
	latestOtherAfterActor := developmentTimelineShell{}

	for _, shell := range shells {
		if shell.index < root.index {
			continue
		}
		if shell.index != root.index && shell.parentIndex != root.index {
			continue
		}
		if shell.index > latestThreadContribution.index {
			latestThreadContribution = shell
		}
		if shell.persona.ID == p.ID {
			if shell.index > lastActorIndex {
				lastActorIndex = shell.index
			}
			continue
		}
		if shell.index > latestOtherAfterActor.index {
			latestOtherAfterActor = shell
		}
	}

	if lastActorIndex == 0 {
		return latestThreadContribution, false, true
	}
	if latestOtherAfterActor.index <= lastActorIndex {
		return developmentTimelineShell{}, true, false
	}
	return latestOtherAfterActor, true, true
}

// demoShouldReturnToThread is intentionally sparse. New thread activity is a
// necessary condition for a second contribution, not an automatic summons. This
// keeps conversations from turning into obligatory ping-pong while still allowing
// natural back-and-forth when a later contribution actually reopens the thread for
// this actor.
func demoShouldReturnToThread(host world.Host, board world.Board, p world.Persona, at time.Time, rootIndex, sourceIndex, ordinal int) bool {
	chance := .05 + clamp01(p.ReplyTendency)*.18 + clamp01(p.NewcomerOpenness)*.05
	chance = math.Max(.06, math.Min(.26, chance))
	return demoStableUnit(
		host.ID,
		board.ID,
		p.ID,
		at.Format(time.RFC3339),
		fmt.Sprintf("thread-return-v1-%d-%d-%d", rootIndex, sourceIndex, ordinal),
	) < chance
}

// demoChooseCausalReplyTarget extends root selection with ParticipationState.
// It uses only already-selected shell topology, recency, and persona propensities;
// no semantic text is generated or inspected here.
func demoChooseCausalReplyTarget(host world.Host, board world.Board, p world.Persona, at time.Time, roots, shells []developmentTimelineShell, ordinal int) (developmentReplyTarget, bool) {
	bestScore := -10.0
	best := developmentReplyTarget{}
	found := false
	start := len(roots) - 1
	stop := start - 7
	if stop < 0 {
		stop = 0
	}

	for i := start; i >= stop; i-- {
		root := roots[i]
		age := at.Sub(root.createdAt)
		if age < 0 || age > 6*24*time.Hour {
			continue
		}

		source, returning, eligible := demoParticipationReplySource(p, root, shells)
		if !eligible {
			continue
		}
		if returning && !demoShouldReturnToThread(host, board, p, at, root.index, source.index, ordinal) {
			continue
		}

		recency := 1 - age.Hours()/(6*24)
		interest := clamp01(p.Interests[root.anchorKey])
		social := clamp01(p.NewcomerOpenness)
		randomTie := demoStableUnit(host.ID, board.ID, p.ID, fmt.Sprint(root.index), fmt.Sprintf("causal-reply-v2-%d", ordinal))
		score := recency*.55 + interest*.25 + social*.12 + randomTie*.18
		if root.persona.ID == p.ID {
			score -= .75
		}
		if returning {
			// Prefer a thread this actor has not already occupied when both are
			// otherwise plausible. Returning is possible, just deliberately rarer.
			score -= .18
		}
		if score > bestScore {
			bestScore = score
			best = developmentReplyTarget{root: root, source: source, returning: returning}
			found = true
		}
	}
	return best, found && bestScore > .12
}

func developmentReplyCauseSummary(target developmentReplyTarget) string {
	if target.returning {
		return fmt.Sprintf(
			"The actor has already contributed to the thread rooted at event %04d. Another actor later contributed event %04d after this actor's most recent contribution, and the sparse return gate passed. Respond specifically to that newer contribution inside routing domain %q. The reply must be a distinct reaction, answer, clarification, or newly relevant contribution prompted by event %04d; do not merely restate the actor's earlier contribution, reread the unchanged thread, or invent an unrelated new world event.",
			target.root.index,
			target.source.index,
			target.root.anchorKey,
			target.source.index,
		)
	}
	if target.source.index != target.root.index {
		return fmt.Sprintf(
			"The actor read the existing thread rooted at event %04d, including the later contribution event %04d, and independently chose to respond. Keep the response inside routing domain %q and make it responsive to the supplied source contribution; do not replace it with a more salient persona fact or invent an unrelated world event.",
			target.root.index,
			target.source.index,
			target.root.anchorKey,
		)
	}
	return fmt.Sprintf(
		"The actor read the existing thread rooted at event %04d and independently chose to respond. Keep the response inside that thread's world-selected routing domain %q; do not replace it with a more salient persona fact or narrate the routing label itself.",
		target.root.index,
		target.root.anchorKey,
	)
}
