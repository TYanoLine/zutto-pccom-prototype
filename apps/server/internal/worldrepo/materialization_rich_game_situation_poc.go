package worldrepo

import (
	"fmt"
	"strings"
	"time"

	"zutto-pccom/apps/server/internal/world"
)

// developmentModeSituationFacet is diagnostic vocabulary for the typed Situation
// v2 PoC. Unlike the production facet list, each facet declares which discourse
// modes can naturally realize it, so a question-shaped situation is not handed to
// an opinion/share slot merely because the broad domain matches.
type developmentModeSituationFacet struct {
	developmentSituationFacet
	modes map[string]bool
}

func developmentModeSet(values ...string) map[string]bool {
	out := make(map[string]bool, len(values))
	for _, value := range values {
		out[value] = true
	}
	return out
}

func developmentRichGameSituationFacets() []developmentModeSituationFacet {
	obsExp := developmentModeSet("share_observation", "share_experience")
	obsExpOpinion := developmentModeSet("share_observation", "share_experience", "state_opinion")
	expTip := developmentModeSet("share_experience", "share_tip")
	expAsk := developmentModeSet("share_experience", "ask_peers")
	allButAsk := developmentModeSet("share_observation", "share_experience", "state_opinion", "share_tip")
	opinionAsk := developmentModeSet("state_opinion", "ask_peers")
	tipAsk := developmentModeSet("share_tip", "ask_peers")
	all := developmentModeSet("share_observation", "share_experience", "state_opinion", "share_tip", "ask_peers")

	return []developmentModeSituationFacet{
		{developmentSituationFacet: developmentSituationFacet{
			kind: "games_progress_setback", focus: "a recent setback followed by another attempt",
			boundary: "Keep the play situation self-contained; no real title, character, machine, or hidden level name.",
			occurrences: []string{"The actor lost progress near the end of a difficult stretch, retried, and then got farther on the next attempt.", "A small mistake ended a promising attempt, but the immediate retry went more smoothly."},
		}, modes: obsExpOpinion},
		{developmentSituationFacet: developmentSituationFacet{
			kind: "games_stuck_point", focus: "a specific obstacle with visible conditions and already-tried actions",
			boundary: "Describe the obstacle and attempts in ordinary words; never rely on an unnamed title or 'this part'.",
			occurrences: []string{"The actor was stuck at one visible obstacle after trying two obvious actions.", "A repeated section still did not progress after the actor changed one obvious approach."},
		}, modes: expAsk},
		{developmentSituationFacet: developmentSituationFacet{
			kind: "games_route_choice", focus: "two clearly described next routes or actions",
			boundary: "State both options plainly; do not invent irreversible consequences as fact.",
			occurrences: []string{"Two clearly different routes were both available and the actor paused before choosing one.", "Two plausible next actions were visible and their practical order was unclear."},
		}, modes: opinionAsk},
		{developmentSituationFacet: developmentSituationFacet{
			kind: "games_score_retry", focus: "a repeated attempt to improve a score, time, or other visible result",
			boundary: "Keep the metric generic; report only the actor's observed result and changes they actually tried.",
			occurrences: []string{"Several attempts gave nearly the same result until one try improved it.", "A small timing or sequence change coincided with a noticeably better result on the next attempt."},
		}, modes: allButAsk},
		{developmentSituationFacet: developmentSituationFacet{
			kind: "games_save_decision", focus: "where to preserve progress before another risky attempt",
			boundary: "Do not invent a product-specific save system; keep the choice understandable in generic terms.",
			occurrences: []string{"After losing noticeable progress once, the actor became more deliberate about where to save before retrying.", "The actor reached an uncertain stretch and considered preserving progress before experimenting."},
		}, modes: opinionAsk},
		{developmentSituationFacet: developmentSituationFacet{
			kind: "games_naming_choice", focus: "choosing an in-game name or label",
			boundary: "No real title is needed; do not invent a product-specific name-change feature.",
			occurrences: []string{"The actor paused at a naming step because they wanted a memorable non-real-name choice.", "The actor chose a temporary name quickly and later found the repeated display of it slightly awkward."},
		}, modes: all},

		{developmentSituationFacet: developmentSituationFacet{
			kind: "games_manual_lookup", focus: "checking the packaged instructions after forgetting or overlooking one ordinary control or rule",
			boundary: "Treat the manual as part of this unnamed game; do not invent exact page numbers, diagrams, or product-specific commands.",
			occurrences: []string{"The actor stopped playing briefly to check the instructions for one rule they had been guessing about.", "A control the actor had been doing from memory turned out to be described more simply in the instructions."},
		}, modes: allButAsk},
		{developmentSituationFacet: developmentSituationFacet{
			kind: "games_note_taking", focus: "writing down a small piece of play information for later",
			boundary: "Keep the note mundane: a route, short sequence, score, item count, or other locally observed detail.",
			occurrences: []string{"The actor wrote down one short sequence because remembering it between attempts was becoming annoying.", "A tiny paper note beside the machine made the next retry less fiddly."},
		}, modes: expTip},
		{developmentSituationFacet: developmentSituationFacet{
			kind: "games_password_recording", focus: "copying or checking a game-provided password or code used to resume",
			boundary: "Do not invent the exact code or imply every game uses passwords; this situation is about one unnamed game that does.",
			occurrences: []string{"The actor copied a resume password and then checked it once because one character was easy to misread.", "A previously written resume code was hard to read, so the actor rewrote it more clearly before stopping."},
		}, modes: allButAsk},
		{developmentSituationFacet: developmentSituationFacet{
			kind: "games_save_slot_housekeeping", focus: "making room among a small number of stored game records",
			boundary: "Do not assert a specific device, battery, or slot count; the unnamed game simply has limited stored records.",
			occurrences: []string{"The actor had to decide which older stored record to overwrite before starting another run.", "Several saved records had become hard to distinguish, so the actor paused to identify which one was safe to replace."},
		}, modes: opinionAsk},
		{developmentSituationFacet: developmentSituationFacet{
			kind: "games_controller_handoff", focus: "passing control to another person during local play",
			boundary: "Keep the other person generic and present only for this play session; do not invent a durable relationship.",
			occurrences: []string{"After several failed attempts, the actor handed the controls over for one try and noticed a different approach.", "Two people alternated attempts at the same section and naturally settled into a turn-taking rhythm."},
		}, modes: obsExpOpinion},
		{developmentSituationFacet: developmentSituationFacet{
			kind: "games_multiplayer_rule", focus: "a simple local rule for taking turns or deciding when players switch",
			boundary: "Keep it to an informal household/friend-session rule, not an official game rule.",
			occurrences: []string{"The players had to decide whether one loss or several losses should end a person's turn.", "A local play session went more smoothly after everyone agreed on a simple switch-over rule."},
		}, modes: opinionAsk},
		{developmentSituationFacet: developmentSituationFacet{
			kind: "games_local_play_setup", focus: "getting an ordinary local multiplayer session started",
			boundary: "No specific hardware model or accessory may be invented; keep the setup at the level of seats, turns, available controllers, or who starts.",
			occurrences: []string{"A small setup detail delayed the start of a local play session until the group settled who would begin.", "The group adjusted where people sat and who held the controls first before playing."},
		}, modes: obsExp},
		{developmentSituationFacet: developmentSituationFacet{
			kind: "games_lending_return", focus: "lending or returning one game to someone the actor knows",
			boundary: "Keep the other person generic; do not invent a named friend, shop, price, or dispute.",
			occurrences: []string{"The actor checked the game and its accompanying items before returning it after a loan.", "Before lending a game out, the actor made a quick note so it would be easy to remember where it went."},
		}, modes: allButAsk},
		{developmentSituationFacet: developmentSituationFacet{
			kind: "games_storage_organization", focus: "organizing a small personal collection of games or their accompanying materials",
			boundary: "Do not assume a specific media format; organize only what this situation explicitly says the actor has.",
			occurrences: []string{"The actor gathered games and their accompanying materials that had ended up in several places and started putting them in a consistent order.", "Looking for one game took longer than expected, prompting the actor to reorganize how the collection was kept."},
		}, modes: obsExpOpinion},
		{developmentSituationFacet: developmentSituationFacet{
			kind: "games_shared_tv_time", focus: "fitting play around ordinary shared use of the household television or display",
			boundary: "Keep it a mundane scheduling choice; do not invent family conflict, programs, or ownership details.",
			occurrences: []string{"The actor stopped at a convenient point because the shared screen was going to be needed for something else.", "A shorter-than-usual play session made the actor think more carefully about where to stop next time."},
		}, modes: obsExpOpinion},
		{developmentSituationFacet: developmentSituationFacet{
			kind: "games_sound_volume", focus: "adjusting game sound for the time of day or other people nearby",
			boundary: "Keep it to ordinary volume/timing; no technical audio claims or household conflict.",
			occurrences: []string{"Playing later than usual made the actor lower the sound and notice different cues than before.", "The actor reduced the sound during a short session and found one part easier or harder to follow that way."},
		}, modes: obsExpOpinion},
		{developmentSituationFacet: developmentSituationFacet{
			kind: "games_choose_what_to_play", focus: "choosing one game from several already on hand for a short session",
			boundary: "Do not invent named titles or purchases; this is only a mundane choice among the actor's available games.",
			occurrences: []string{"With only a short amount of time, the actor compared a few games already on hand and picked one that was easy to stop.", "The actor changed their mind about what to play after noticing how much time was left in the evening."},
		}, modes: obsExpOpinion},
		{developmentSituationFacet: developmentSituationFacet{
			kind: "games_restart_or_continue", focus: "deciding whether to continue an existing run or start over",
			boundary: "Do not invent story specifics; keep the tradeoff to familiarity, forgotten progress, or wanting a clean start.",
			occurrences: []string{"The actor returned to an older run and spent a while remembering what they had been doing before deciding whether to continue.", "Starting over looked tempting because the actor no longer remembered a few earlier choices clearly."},
		}, modes: opinionAsk},
		{developmentSituationFacet: developmentSituationFacet{
			kind: "games_practice_focus", focus: "practicing one repeatable action instead of trying to clear the whole section",
			boundary: "Keep the practiced action generic and observable; no product-specific move name.",
			occurrences: []string{"Rather than trying to finish the whole section, the actor repeated one troublesome action several times on its own.", "A short practice session focused on one timing problem and became more consistent by the end."},
		}, modes: expTip},
		{developmentSituationFacet: developmentSituationFacet{
			kind: "games_watching_another_player", focus: "noticing a different approach while watching someone else play briefly",
			boundary: "The other player is generic and temporary; do not transfer their experience into the actor's first-person history.",
			occurrences: []string{"Watching another person's attempt revealed a much slower approach than the actor had been using.", "The actor noticed that another player checked the surroundings before acting, unlike the actor's usual rush."},
		}, modes: obsExpOpinion},
		{developmentSituationFacet: developmentSituationFacet{
			kind: "games_session_stop_point", focus: "choosing a sensible point to end a play session",
			boundary: "Keep it to ordinary stopping/continuing decisions; do not invent tomorrow's plans or external obligations.",
			occurrences: []string{"The actor reached a natural break after a long stretch and debated doing one more attempt before stopping.", "A late retry succeeded just enough to make stopping there feel better than risking another long attempt."},
		}, modes: obsExpOpinion},
		{developmentSituationFacet: developmentSituationFacet{
			kind: "games_simple_comparison", focus: "comparing two ordinary ways of approaching the same in-game task",
			boundary: "Both approaches must be described directly; no hidden item, character, or named level.",
			occurrences: []string{"The actor tried the same short task once cautiously and once quickly and noticed a practical difference.", "Two different orders of the same small actions produced noticeably different results."},
		}, modes: tipAsk},
	}
}

func developmentModeFacetAllowed(f developmentModeSituationFacet, mode string) bool {
	if len(f.modes) == 0 {
		return true
	}
	return f.modes[mode]
}

func developmentSelectRootSituationV2(host world.Host, board world.Board, shell developmentTimelineShell, prior []world.Post) developmentSparseSituation {
	if strings.ToLower(strings.TrimSpace(shell.anchorKey)) != "games" {
		return developmentSelectRootSituation(host, board, shell, prior)
	}
	facets := developmentRichGameSituationFacets()
	candidates := make([]developmentSituationCandidate, 0, len(facets))
	hasUnblocked := false
	for _, rich := range facets {
		if !developmentModeFacetAllowed(rich, shell.discourseMode) {
			continue
		}
		weight, blocked := developmentSituationNovelty(rich.kind, board.ID, shell.persona.ID, shell.createdAt, prior)
		if !blocked {
			hasUnblocked = true
		}
		candidates = append(candidates, developmentSituationCandidate{facet: rich.developmentSituationFacet, weight: weight, blocked: blocked})
	}
	if len(candidates) == 0 {
		return developmentSelectRootSituation(host, board, shell, prior)
	}

	total := 0.0
	for _, candidate := range candidates {
		if hasUnblocked && candidate.blocked {
			continue
		}
		total += candidate.weight
	}
	if total <= 0 {
		total = float64(len(candidates))
		for i := range candidates {
			candidates[i].weight = 1
			candidates[i].blocked = false
		}
		hasUnblocked = true
	}

	roll := demoStableUnit(host.ID, board.ID, shell.persona.ID, shell.createdAt.Format(time.RFC3339), fmt.Sprintf("sparse-situation-v2-%d", shell.index)) * total
	selected := candidates[len(candidates)-1].facet
	for _, candidate := range candidates {
		if hasUnblocked && candidate.blocked {
			continue
		}
		if roll < candidate.weight {
			selected = candidate.facet
			break
		}
		roll -= candidate.weight
	}
	return developmentComposeSituation(host, board, shell, selected)
}

// DevelopmentMinimalRootSlotsWindowV2 is a diagnostic-only variant of the
// existing minimal root sampler. Actor, time, topology, routing and discourse mode
// are identical; only the game Situation vocabulary/selection changes.
func (r *Repository) DevelopmentMinimalRootSlotsWindowV2(host world.Host, board world.Board, limit, lookbackDays, maxPosts int) []DevelopmentMinimalRootSlot {
	if limit < 1 {
		return nil
	}
	if limit > 100 {
		limit = 100
	}
	if lookbackDays < 1 {
		lookbackDays = 1
	}
	if lookbackDays > 730 {
		lookbackDays = 730
	}
	if maxPosts < developmentMaxPostsPerBoardCatchup {
		maxPosts = developmentMaxPostsPerBoardCatchup
	}
	if maxPosts > 600 {
		maxPosts = 600
	}

	personas, _ := r.MaterializationPersonas(host)
	if len(personas) == 0 {
		return nil
	}
	behaviorAdvice := r.developmentJevBehaviorAdvice(host, []world.Board{board}, personas)
	visits := developmentVisitsForBoardDaysWithAdvice(host, board, personas, r.WorldDate, lookbackDays, behaviorAdvice)
	shells, _ := selectDevelopmentTimelineShellsWithBehaviorAdviceLimit(host, board, visits, behaviorAdvice, maxPosts)
	prior := filterBoard(r.Base.ListPosts(host.ID), board.ID)

	out := make([]DevelopmentMinimalRootSlot, 0, limit)
	for _, shell := range shells {
		if shell.action != "thread_start" || shell.parentIndex != 0 || shell.sourceIndex != 0 {
			continue
		}
		situation := developmentSelectRootSituationV2(host, board, shell, prior)
		out = append(out, DevelopmentMinimalRootSlot{
			EventID:          developmentWindowEventID(board.ID, shell.index),
			BoardID:          board.ID,
			BoardName:        board.Name,
			AuthorHandle:     shell.persona.Handle,
			CreatedAt:        shell.createdAt.Format(time.RFC3339),
			Action:           shell.action,
			AnchorKey:        shell.anchorKey,
			CauseKind:        shell.causeKind,
			CauseSummary:     shell.causeSummary,
			DiscourseMode:    shell.discourseMode,
			PersonaProfile:   personaSummary(shell.persona),
			SituationKind:    situation.kind,
			SituationSummary: situation.summary,
			SituationFacts:   append([]string(nil), situation.facts...),
		})
		if len(out) >= limit {
			break
		}
	}
	return out
}
