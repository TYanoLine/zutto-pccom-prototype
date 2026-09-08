package worldrepo

import (
	"testing"
	"time"

	"zutto-pccom/apps/server/internal/world"
)

func TestDevelopmentSearchGroundingCandidatesPreferExternalReferentDomains(t *testing.T) {
	date := time.Date(1996, 8, 20, 12, 0, 0, 0, time.FixedZone("JST", 9*3600))
	roots := []developmentWindowShell{
		{eventID: "game", board: world.Board{ID: "4", Name: "ゲーム"}, shell: developmentTimelineShell{createdAt: date, anchorKey: "games"}},
		{eventID: "music", board: world.Board{ID: "5", Name: "音楽"}, shell: developmentTimelineShell{createdAt: date.Add(time.Hour), anchorKey: "music"}},
		{eventID: "local", board: world.Board{ID: "3", Name: "地域"}, shell: developmentTimelineShell{createdAt: date.Add(2 * time.Hour), anchorKey: "local"}},
	}
	accepted := map[string]developmentSituationProposal{
		"game":  {noveltyKey: "g"},
		"music": {noveltyKey: "m"},
		"local": {noveltyKey: "l"},
	}
	got := developmentSearchGroundingCandidates(roots, accepted, 6)
	if len(got) != 2 {
		t.Fatalf("candidates=%d want 2", len(got))
	}
	if got[0].eventID != "game" || got[1].eventID != "music" {
		t.Fatalf("candidate order=%v,%v", got[0].eventID, got[1].eventID)
	}
}

func TestDevelopmentSearchGroundingCompatiblePreservesSemanticIdentity(t *testing.T) {
	original := developmentSituationProposal{
		objectClass:      "ゲームセンターの対戦格闘ゲーム",
		occurrence:       "友人と対戦格闘ゲームを遊び、負けた後も再戦した。",
		actorObservation: "友人との対戦を続けた。",
		noveltyKey:       "対戦格闘ゲームの連続再戦",
	}
	refined := original
	refined.objectClass = "ゲームセンターのバーチャファイター2"
	refined.occurrence = "友人とバーチャファイター2を遊び、負けた後も再戦した。"
	if !developmentSearchGroundingCompatible(original, refined) {
		t.Fatal("expected named refinement to preserve semantic identity")
	}
	refined.noveltyKey = "別の出来事"
	if developmentSearchGroundingCompatible(original, refined) {
		t.Fatal("changed novelty key must be rejected")
	}
}
