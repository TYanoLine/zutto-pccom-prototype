package worldrepo

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"zutto-pccom/apps/server/internal/world"
)

const developmentMaxPostsPerBoardCatchup = 28

type developmentSelectionStats struct {
	Visits  int
	Posts   int
	ROM     int
	Roots   int
	Replies int
}

type developmentRootCause struct {
	anchorKey    string
	causeKind    string
	causeSummary string
	sourceIndex  int
}

type developmentWeightedVisit struct {
	candidate demoPostCandidate
	score     float64
}

type developmentAnchorCandidate struct {
	key    string
	weight float64
}

// selectDevelopmentTimelineShells is the causal boundary between cheap world
// simulation and expensive semantic realization. The input candidates mean
// "this persona plausibly visited this board around this time", not "a post must
// exist". Most visits are allowed to resolve to ROM/no-op before any LLM call.
//
// For an actual write, the world layer also fixes root-vs-reply and a causal
// anchor. The LLM is therefore never asked to invent a topic merely because the
// scheduler happened to allocate a posting slot.
func selectDevelopmentTimelineShells(host world.Host, board world.Board, visits []demoPostCandidate) ([]developmentTimelineShell, developmentSelectionStats) {
	stats := developmentSelectionStats{Visits: len(visits)}
	if len(visits) == 0 {
		return nil, stats
	}

	writerVisits := selectDevelopmentWriterVisits(host, board, visits)
	shells := make([]developmentTimelineShell, 0, len(writerVisits))
	roots := make([]developmentTimelineShell, 0, len(writerVisits))

	for visitOrdinal, candidate := range visits {
		if len(shells) >= developmentMaxPostsPerBoardCatchup {
			break
		}
		if !writerVisits[developmentVisitKey(candidate)] {
			continue
		}

		postOrdinal := len(shells) + 1
		if len(roots) > 0 && demoShouldReply(host, board, candidate.persona, candidate.createdAt, visitOrdinal) {
			if parent, found := demoChooseCausalReplyRoot(host, board, candidate.persona, candidate.createdAt, roots, visitOrdinal); found {
				shell := developmentTimelineShell{
					index:        postOrdinal,
					persona:      candidate.persona,
					createdAt:    candidate.createdAt,
					action:       "reply",
					parentIndex:  parent.index,
					anchorKey:    parent.anchorKey,
					causeKind:    "observed_thread",
					causeSummary: fmt.Sprintf("The actor read the existing thread rooted at event %04d and independently chose to respond. Keep the response inside that thread's world-selected anchor %q; do not replace it with a more salient persona fact.", parent.index, parent.anchorKey),
					sourceIndex:  parent.index,
				}
				shells = append(shells, shell)
				stats.Replies++
				continue
			}
		}

		cause, found := demoSelectRootCause(host, board, candidate.persona, candidate.createdAt, roots, visitOrdinal)
		if !found {
			// A write-capable visit can still end without a post when this persona has
			// no plausible causal anchor for this board at this moment.
			continue
		}
		shell := developmentTimelineShell{
			index:        postOrdinal,
			persona:      candidate.persona,
			createdAt:    candidate.createdAt,
			action:       "thread_start",
			anchorKey:    cause.anchorKey,
			causeKind:    cause.causeKind,
			causeSummary: cause.causeSummary,
			sourceIndex:  cause.sourceIndex,
		}
		shells = append(shells, shell)
		roots = append(roots, shell)
		stats.Roots++
	}

	// The per-persona write sampler intentionally marks some plausible visits as
	// potential writers before topology/anchor checks. If a low-affinity writer has
	// no sensible root or reply target, that visit remains ROM as well.
	stats.Posts = len(shells)
	stats.ROM = stats.Visits - stats.Posts
	if stats.ROM < 0 {
		stats.ROM = 0
	}
	return shells, stats
}

// selectDevelopmentWriterVisits keeps the cost bounded and makes silence normal.
// Each persona first gets deterministic board visits; only a fraction of those
// visits become write opportunities. Selection is quota-by-expectation per
// persona rather than one Bernoulli per event, so a two-week catch-up remains
// stable and does not accidentally put every post at one end of the window.
func selectDevelopmentWriterVisits(host world.Host, board world.Board, visits []demoPostCandidate) map[string]bool {
	byPersona := map[string][]demoPostCandidate{}
	for _, candidate := range visits {
		byPersona[candidate.persona.ID] = append(byPersona[candidate.persona.ID], candidate)
	}

	selected := map[string]bool{}
	personaIDs := make([]string, 0, len(byPersona))
	for personaID := range byPersona {
		personaIDs = append(personaIDs, personaID)
	}
	sort.Strings(personaIDs)

	for _, personaID := range personaIDs {
		candidates := byPersona[personaID]
		if len(candidates) == 0 {
			continue
		}
		chance := demoWriteProbability(candidates[0].persona, board)
		target := int(math.Round(float64(len(candidates)) * chance))
		if target < 0 {
			target = 0
		}
		if target > len(candidates) {
			target = len(candidates)
		}
		weighted := make([]developmentWeightedVisit, 0, len(candidates))
		for _, candidate := range candidates {
			roll := demoStableUnit(host.ID, board.ID, candidate.persona.ID, candidate.createdAt.Format(time.RFC3339), "write-slot-v1")
			weighted = append(weighted, developmentWeightedVisit{candidate: candidate, score: roll / math.Max(chance, .01)})
		}
		sort.SliceStable(weighted, func(i, j int) bool {
			if weighted[i].score == weighted[j].score {
				return weighted[i].candidate.createdAt.Before(weighted[j].candidate.createdAt)
			}
			return weighted[i].score < weighted[j].score
		})
		for _, value := range weighted[:target] {
			selected[developmentVisitKey(value.candidate)] = true
		}
	}
	return selected
}

func developmentVisitKey(candidate demoPostCandidate) string {
	return candidate.persona.ID + "|" + candidate.createdAt.Format(time.RFC3339Nano)
}

func demoWriteProbability(p world.Persona, board world.Board) float64 {
	affinity := demoBoardAffinity(p, board)
	chance := .18 + (1-p.LurkerTendency)*.35 + math.Max(p.ReplyTendency, p.ThreadStartTendency)*.15
	// Visiting a board is not the same as having enough board-relevant motivation
	// to write there. Low affinity still leaves room for occasional replies, while
	// preventing every active persona from manufacturing an off-topic root.
	chance *= .55 + affinity*.45
	if strings.EqualFold(p.Handle, "SYSOP") {
		chance += .04
	}
	return math.Max(.08, math.Min(.72, chance))
}

func demoSelectRootCause(host world.Host, board world.Board, p world.Persona, at time.Time, roots []developmentTimelineShell, ordinal int) (developmentRootCause, bool) {
	if prior, found := demoContinuationSource(host, board, p, at, roots, ordinal); found {
		return developmentRootCause{
			anchorKey:   prior.anchorKey,
			causeKind:   "continuation_progress",
			sourceIndex: prior.index,
			causeSummary: fmt.Sprintf("A new development occurred since this actor's earlier root event %04d about anchor %q. The new post must add a materially new observation/progress/change instead of restating the earlier preference or background fact.", prior.index, prior.anchorKey),
		}, true
	}

	anchor, found := demoSelectFreshAnchor(host, board, p, at, roots, ordinal)
	if !found {
		return developmentRootCause{}, false
	}
	return developmentRootCause{
		anchorKey: anchor,
		causeKind: "recent_salience",
		causeSummary: fmt.Sprintf("After activity, board-visit, and write sampling, the world layer selected persistent interest key %q as the current causal anchor. A small recent experience, observation, or thought in this area became salient enough to mention now. Existing persona facts are consistency background only and are not themselves a reason to revisit a topic.", anchor),
	}, true
}

// demoContinuationSource is a cheap OpenLoop approximation for the development
// fixture. It can revisit one of the actor's own recent root threads only when a
// deterministic progress gate fires. Merely having a persistent fact or having
// mentioned a topic before does not create another post.
func demoContinuationSource(host world.Host, board world.Board, p world.Persona, at time.Time, roots []developmentTimelineShell, ordinal int) (developmentTimelineShell, bool) {
	if demoStableUnit(host.ID, board.ID, p.ID, at.Format(time.RFC3339), fmt.Sprintf("continuation-gate-%d", ordinal)) >= .12 {
		return developmentTimelineShell{}, false
	}
	for i := len(roots) - 1; i >= 0; i-- {
		root := roots[i]
		if root.persona.ID != p.ID || root.anchorKey == "" || root.causeKind == "continuation_progress" {
			continue
		}
		age := at.Sub(root.createdAt)
		if age >= 48*time.Hour && age <= 10*24*time.Hour {
			return root, true
		}
	}
	return developmentTimelineShell{}, false
}

func demoSelectFreshAnchor(host world.Host, board world.Board, p world.Persona, at time.Time, roots []developmentTimelineShell, ordinal int) (string, bool) {
	keys := make([]string, 0, len(p.Interests))
	for key := range p.Interests {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	choices := make([]developmentAnchorCandidate, 0, len(keys))
	total := 0.0
	for _, key := range keys {
		strength := clamp01(p.Interests[key])
		relevance := demoInterestBoardRelevance(board, key)
		if strength <= 0 || relevance <= 0 {
			continue
		}
		weight := strength * relevance * demoAnchorNoveltyFactor(p, key, at, roots)
		if weight <= .001 {
			continue
		}
		choices = append(choices, developmentAnchorCandidate{key: key, weight: weight})
		total += weight
	}
	if total < .08 || len(choices) == 0 {
		return "", false
	}

	roll := demoStableUnit(host.ID, board.ID, p.ID, at.Format(time.RFC3339), fmt.Sprintf("root-anchor-v2-%d", ordinal)) * total
	for _, choice := range choices {
		if roll < choice.weight {
			return choice.key, true
		}
		roll -= choice.weight
	}
	return choices[len(choices)-1].key, true
}

// demoInterestBoardRelevance is fixture-level routing metadata, not a content
// catalog. The actual anchor keys come from each persona's persisted Interests
// map. It only answers whether one of those already-existing interests belongs
// naturally on this development board.
func demoInterestBoardRelevance(board world.Board, key string) float64 {
	key = strings.ToLower(strings.TrimSpace(key))
	switch board.ID {
	case "2": // パソコン通信・モデム
		switch key {
		case "modem":
			return 1
		case "software":
			return .95
		case "pc98":
			return .90
		case "bbs":
			return .55
		default:
			return 0
		}
	case "3": // 地域の話題
		switch key {
		case "local":
			return 1
		case "chat":
			return .55
		case "music":
			return .18
		case "games":
			return .15
		case "bbs":
			return .10
		default:
			return 0
		}
	default: // フリートーク
		switch key {
		case "chat":
			return 1
		case "music":
			return .85
		case "games":
			return .85
		case "local":
			return .65
		case "bbs":
			return .35
		case "pc98":
			return .18
		case "modem", "software":
			return .12
		default:
			// Unknown future persona interests are not banned from free talk. Giving
			// them moderate relevance preserves extensibility without inventing a
			// global topic taxonomy.
			return .45
		}
	}
}

func demoAnchorNoveltyFactor(p world.Persona, anchor string, at time.Time, roots []developmentTimelineShell) float64 {
	factor := 1.0
	recentGlobal := 0
	for i := len(roots) - 1; i >= 0; i-- {
		root := roots[i]
		if root.anchorKey != anchor {
			continue
		}
		age := at.Sub(root.createdAt)
		if age < 0 {
			continue
		}
		if age <= 24*time.Hour {
			factor *= .45
		}
		if root.persona.ID == p.ID {
			switch {
			case age < 48*time.Hour:
				factor *= .04
			case age < 5*24*time.Hour:
				factor *= .18
			case age < 10*24*time.Hour:
				factor *= .42
			default:
				factor *= .70
			}
		}
		recentGlobal++
		if recentGlobal >= 6 {
			break
		}
	}
	for i := 0; i < recentGlobal; i++ {
		factor *= .78
	}
	return math.Max(.01, factor)
}

// demoChooseCausalReplyRoot uses only already-selected world anchors and
// recency/social propensity. It does not inspect or generate prose. A low-topic-
// affinity persona may still reply socially, but it no longer needs to create an
// unrelated root on a specialized board just because they were online.
func demoChooseCausalReplyRoot(host world.Host, board world.Board, p world.Persona, at time.Time, roots []developmentTimelineShell, ordinal int) (developmentTimelineShell, bool) {
	bestScore := -10.0
	best := developmentTimelineShell{}
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
		recency := 1 - age.Hours()/(6*24)
		interest := clamp01(p.Interests[root.anchorKey])
		social := clamp01(p.NewcomerOpenness)
		randomTie := demoStableUnit(host.ID, board.ID, p.ID, fmt.Sprint(root.index), fmt.Sprintf("causal-reply-%d", ordinal))
		score := recency*.55 + interest*.25 + social*.12 + randomTie*.18
		if root.persona.ID == p.ID {
			score -= .75
		}
		if score > bestScore {
			bestScore = score
			best = root
			found = true
		}
	}
	return best, found && bestScore > .12
}
