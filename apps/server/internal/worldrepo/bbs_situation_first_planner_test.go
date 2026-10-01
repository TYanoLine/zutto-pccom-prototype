package worldrepo

import (
	"strings"
	"testing"
	"time"

	"zutto-pccom/apps/server/internal/world"
)

func TestProductionEventPeriodFactsRespectEventDate(t *testing.T) {
	host := world.Host{ID: "h"}
	board := world.Board{ID: "game", Name: "ゲーム"}
	persona := world.Persona{ID: "p", Handle: "P"}
	before := productionEventPeriodFacts(host, board, persona, "games", time.Date(1995, 8, 20, 20, 0, 0, 0, time.Local))
	after := productionEventPeriodFacts(host, board, persona, "games", time.Date(1996, 4, 20, 20, 0, 0, 0, time.Local))

	contains := func(values []string, marker string) bool {
		for _, value := range values {
			if strings.Contains(value, marker) {
				return true
			}
		}
		return false
	}
	if contains(before, "ポケットモンスター") || contains(before, "バイオハザード") {
		t.Fatalf("future referent leaked into 1995 event: %#v", before)
	}
	// Stable six-item rotation need not include every later title, so check the
	// historical catalog through multiple event times/identities.
	foundPokemon := contains(after, "ポケットモンスター")
	for i := 0; i < 12 && !foundPokemon; i++ {
		persona.ID = string(rune('a' + i))
		values := productionEventPeriodFacts(host, board, persona, "games", time.Date(1996, 4, 20, 20, i, 0, 0, time.Local))
		foundPokemon = contains(values, "ポケットモンスター")
	}
	if !foundPokemon {
		t.Fatal("date-valid 1996 game referents never became available")
	}
}

func TestProductionBoardDomainUsesBoardMeaningBeforePersonaInterest(t *testing.T) {
	persona := world.Persona{Interests: map[string]float64{"music": .99, "games": .1}}
	board := world.Board{Name: "ＧＡＭＥ", SemanticScope: "ゲームの感想や相談"}
	if got := productionBoardDomain(board, persona); got != "games" {
		t.Fatalf("domain=%q, want games", got)
	}
}

func TestProductionGameFacetSelectionSpreadsKindsWithinWindow(t *testing.T) {
	host := world.Host{ID: "h"}
	board := world.Board{ID: "g", Name: "ゲーム"}
	persona := world.Persona{ID: "p", Handle: "P"}
	counts := map[string]int{}
	seen := map[string]int{}
	base := time.Date(1996, 3, 1, 20, 0, 0, 0, time.Local)
	for i := 0; i < 50; i++ {
		mode := developmentRootDiscourseModes[i%len(developmentRootDiscourseModes)]
		facet, ok := chooseProductionSituationFacet(host, board, persona, base.Add(time.Duration(i)*time.Hour), i+1, "games", mode, counts)
		if !ok {
			t.Fatalf("no facet for i=%d mode=%s", i, mode)
		}
		counts[facet.kind]++
		seen[facet.kind]++
	}
	if len(seen) < 15 {
		t.Fatalf("only %d distinct game Situation kinds across 50 roots: %#v", len(seen), seen)
	}
	max := 0
	for _, count := range seen {
		if count > max { max = count }
	}
	if max > 8 {
		t.Fatalf("one Situation kind dominated 50-root window: max=%d kinds=%#v", max, seen)
	}
}

func TestProductionSituationUsesFocusWithoutDiagnosticAnonymousExample(t *testing.T) {
	rich := developmentRichGameSituationFacets()
	var password developmentSituationFacet
	for _, candidate := range rich {
		if candidate.kind == "games_password_recording" {
			password = candidate.developmentSituationFacet
			break
		}
	}
	if password.kind == "" {
		t.Fatal("expected diagnostic password facet")
	}
	// The diagnostic PoC intentionally describes an unnamed game, but that
	// experimental constraint must not leak into production generation.
	if !strings.Contains(password.boundary, "unnamed game") {
		t.Fatal("diagnostic fixture unexpectedly changed")
	}
	got := productionSituationFocus(password)
	if got.kind != password.kind || got.summary != password.focus {
		t.Fatalf("focus not preserved: %+v", got)
	}
	// The production material has one World-selected focus, not a list of
	// instructions controlling the model's topic or prose.
	if len(got.facts) != 1 || got.facts[0] != "activity_focus="+password.focus {
		t.Fatalf("production material includes diagnostic constraints: %#v", got.facts)
	}
	if strings.Contains(strings.Join(got.facts, "\n"), password.boundary) {
		t.Fatalf("diagnostic anonymous-game condition leaked into production: %#v", got.facts)
	}
}

func TestProductionGameTopicFacetsAreBroadAndModeCompatible(t *testing.T) {
	extra := productionGameTopicFacets()
	if len(extra) < 3 {
		t.Fatalf("too few production work-specific activity focuses: %d", len(extra))
	}
	for _, candidate := range extra {
		if candidate.kind == "" || candidate.focus == "" {
			t.Fatalf("topic facet lacks a World activity focus: %+v", candidate)
		}
		if len(candidate.occurrences) != 0 || candidate.boundary != "" {
			t.Fatalf("production-only activity focus is prescribing a diagnostic incident: %+v", candidate)
		}
		if len(candidate.modes) == 0 {
			t.Fatalf("topic facet missing mode compatibility: %+v", candidate)
		}
	}
}

func TestProductionFocusIsIdenticalMaterialAcrossPostingModes(t *testing.T) {
	facet := developmentSituationFacet{kind: "chat_hobby", focus: "a short hobby update"}
	got := productionSituationFocus(facet)
	if got.kind != "chat_hobby" || got.summary != "a short hobby update" {
		t.Fatalf("lost World-selected direction: %+v", got)
	}
	if len(got.facts) != 1 || got.facts[0] != "activity_focus=a short hobby update" {
		t.Fatalf("production added an unnecessary prompt rule: %#v", got.facts)
	}
}
