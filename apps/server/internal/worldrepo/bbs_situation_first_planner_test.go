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
