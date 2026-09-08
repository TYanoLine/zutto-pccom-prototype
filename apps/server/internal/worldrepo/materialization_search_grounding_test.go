package worldrepo

import "strings"

import (
	"fmt"
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

func TestDevelopmentSearchGroundingEvidenceRequestBuildsCandidatePool(t *testing.T) {
	date := time.Date(1996, 8, 22, 12, 0, 0, 0, time.FixedZone("JST", 9*3600))
	root := developmentWindowShell{eventID: "game", board: world.Board{ID: "4", Name: "ゲーム"}, shell: developmentTimelineShell{createdAt: date, anchorKey: "games"}}
	proposal := developmentSituationProposal{objectClass: "ゲーム内の進行場面", occurrence: "手がかりを見落として同じ場所を調べた", actorObservation: "同じ場所を何度か調べた", noveltyKey: "clue"}
	req := developmentSearchGroundingEvidenceRequest(root, proposal, nil)
	for _, want := range []string{"world engineが決定", "CANDIDATE:", "一意に名前を推理できる必要はありません", "2〜5件", "candidate-pool-v1"} {
		if !strings.Contains(req.Need+req.Subject, want) {
			t.Fatalf("grounding request missing %q: %s / %s", want, req.Need, req.Subject)
		}
	}
}

func TestDevelopmentGroundingCandidateParser(t *testing.T) {
	claim := "CANDIDATE: MYST || 1994年発売で探索停滞の出来事に適合\nCANDIDATE: 弟切草 || 1992年発売で選択肢のある遊びに適合"
	got := developmentParseGroundingCandidates(claim)
	if len(got) != 2 || got[0].Name != "MYST" || got[1].Name != "弟切草" {
		t.Fatalf("parsed candidates=%+v", got)
	}
}

func TestDevelopmentSelectGroundingCandidatePenalizesRecentReuse(t *testing.T) {
	date := time.Date(1996, 8, 22, 12, 0, 0, 0, time.FixedZone("JST", 9*3600))
	root := developmentWindowShell{eventID: "e1", shell: developmentTimelineShell{createdAt: date, persona: world.Persona{ID: "p1"}}}
	options := []developmentGroundingCandidate{{Name: "MYST", Evidence: "x", Rank: 0}, {Name: "弟切草", Evidence: "y", Rank: 1}}
	recent := []world.Post{{Subject: "MYSTの話", Body: "MYSTを遊んだ"}}
	selected, ok := developmentSelectGroundingCandidate("h", root, options, recent, map[string]int{})
	if !ok || selected.Name != "弟切草" {
		t.Fatalf("selected=%+v ok=%v", selected, ok)
	}
}

func TestDevelopmentSearchGroundingErrorCompactsAndBoundsMessage(t *testing.T) {
	got := developmentSearchGroundingError("event-1", fmt.Errorf("first   line %s", strings.Repeat("x", 300)))
	if !strings.HasPrefix(got, "event-1:first line") {
		t.Fatalf("unexpected compact error: %q", got)
	}
	if strings.Contains(got, "  ") {
		t.Fatalf("error should compact whitespace: %q", got)
	}
	if len([]rune(got)) > 240 {
		t.Fatalf("error not bounded: %d %q", len([]rune(got)), got)
	}
}
