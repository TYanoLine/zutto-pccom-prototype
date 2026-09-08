package worldrepo

import "strings"

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

func TestDevelopmentSearchGroundingEvidenceRequestScopesMissingInfoToHistoricalCandidate(t *testing.T) {
	date := time.Date(1996, 8, 22, 12, 0, 0, 0, time.FixedZone("JST", 9*3600))
	root := developmentWindowShell{eventID: "game", board: world.Board{ID: "4", Name: "ゲーム"}, shell: developmentTimelineShell{createdAt: date, anchorKey: "games"}}
	proposal := developmentSituationProposal{objectClass: "ゲーム内の進行場面", occurrence: "手がかりを見落として同じ場所を調べた", actorObservation: "同じ場所を何度か調べた", noveltyKey: "clue"}
	req := developmentSearchGroundingEvidenceRequest(root, proposal)
	for _, want := range []string{"missingInfo", "NPCが実際に使った", "検索スコープ外", "空配列"} {
		if !strings.Contains(req.Need, want) {
			t.Fatalf("grounding Need missing %q: %s", want, req.Need)
		}
	}
}

func TestDevelopmentSearchGroundingEvidenceRequestRejectsArbitraryPeriodExamples(t *testing.T) {
	date := time.Date(1996, 8, 15, 12, 0, 0, 0, time.FixedZone("JST", 9*3600))
	root := developmentWindowShell{eventID: "music", board: world.Board{ID: "5", Name: "音楽"}, shell: developmentTimelineShell{createdAt: date, anchorKey: "music"}}
	proposal := developmentSituationProposal{objectClass: "曲を聴く音量", occurrence: "同じ曲を少し小さい音量で聴いた", actorObservation: "細かい音が聞きやすいと感じた", noveltyKey: "volume"}
	req := developmentSearchGroundingEvidenceRequest(root, proposal)
	for _, want := range []string{"任意の具体例", "識別的な手掛かり", "2つ以上", "具体化不要"} {
		if !strings.Contains(req.Need, want) {
			t.Fatalf("grounding Need missing discriminative rule %q: %s", want, req.Need)
		}
	}
}
