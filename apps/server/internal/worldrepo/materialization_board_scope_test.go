package worldrepo

import (
	"testing"
	"time"

	"zutto-pccom/apps/server/internal/world"
)

func TestDevelopmentRootBoardScopeIsStricterThanGeneralAffinity(t *testing.T) {
	localBoard := world.Board{ID: "3", Name: "地域の話題"}

	// Adjacent interests may explain why somebody visits/reads/replies on a local
	// board, but without an explicit local bridge they cannot start a new root.
	for _, key := range []string{"chat", "music", "games", "bbs"} {
		if got := demoInterestBoardRelevance(localBoard, key); got <= 0 {
			t.Fatalf("general affinity for %q = %v, want > 0", key, got)
		}
		if got := demoRootBoardScopeRelevance(localBoard, key); got != 0 {
			t.Fatalf("root scope relevance for %q = %v, want 0", key, got)
		}
	}
	if got := demoRootBoardScopeRelevance(localBoard, "local"); got <= 0 {
		t.Fatalf("local root scope relevance = %v, want > 0", got)
	}
}

func TestDevelopmentLocalBoardRootSelectorCannotChooseAdjacentInterest(t *testing.T) {
	host := world.Host{ID: "scope-test-host"}
	board := world.Board{ID: "3", Name: "地域の話題"}
	persona := world.Persona{
		ID: "scope-test-persona",
		Interests: map[string]float64{
			"local": 0.61,
			"chat":  1.00,
			"music": 1.00,
			"games": 1.00,
			"bbs":   1.00,
		},
	}

	start := time.Date(1996, 8, 1, 20, 0, 0, 0, time.UTC)
	for i := 0; i < 80; i++ {
		at := start.Add(time.Duration(i) * 3 * time.Hour)
		anchor, found := demoSelectFreshAnchor(host, board, persona, at, nil, i)
		if !found {
			t.Fatalf("iteration %d: expected a local root anchor", i)
		}
		if anchor != "local" {
			t.Fatalf("iteration %d: local board root anchor=%q, want local", i, anchor)
		}
	}
}

func TestDevelopmentLocalBoardRootSelectorReturnsROMWhenOnlyAdjacentInterestsExist(t *testing.T) {
	host := world.Host{ID: "scope-test-host"}
	board := world.Board{ID: "3", Name: "地域の話題"}
	persona := world.Persona{
		ID: "scope-test-chat-only",
		Interests: map[string]float64{
			"chat":  1.0,
			"games": 0.9,
			"bbs":   0.8,
		},
	}

	anchor, found := demoSelectFreshAnchor(host, board, persona, time.Date(1996, 8, 20, 23, 0, 0, 0, time.UTC), nil, 1)
	if found || anchor != "" {
		t.Fatalf("off-scope local-board root selected: found=%v anchor=%q", found, anchor)
	}
}

func TestDevelopmentSpecializedBoardRootScopes(t *testing.T) {
	technical := world.Board{ID: "2", Name: "パソコン通信・モデム"}
	for _, key := range []string{"communications", "modem", "software", "bbs"} {
		if got := demoRootBoardScopeRelevance(technical, key); got <= 0 {
			t.Fatalf("technical root scope relevance for %q = %v, want > 0", key, got)
		}
	}
	for _, key := range []string{"games", "music", "local", "chat"} {
		if got := demoRootBoardScopeRelevance(technical, key); got != 0 {
			t.Fatalf("technical root scope relevance for %q = %v, want 0", key, got)
		}
	}

	freeTalk := world.Board{ID: "1", Name: "フリートーク"}
	if got := demoRootBoardScopeRelevance(freeTalk, "something-new"); got <= 0 {
		t.Fatalf("free-talk unknown interest relevance = %v, want > 0", got)
	}
}
