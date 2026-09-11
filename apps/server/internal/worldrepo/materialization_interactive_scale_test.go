package worldrepo

import (
	"testing"

	"zutto-pccom/apps/server/internal/world"
)

func TestInteractiveMaterializationExpandsStoredBoardsToSix(t *testing.T) {
	base := world.NewMemoryStore()
	repo := New(base, nil, nil, "1996-08-29")
	host, err := repo.HostByPhone("0450000196")
	if err != nil {
		t.Fatal(err)
	}

	initial, created := repo.MaterializationBoards(host)
	if !created || len(initial) != 3 {
		t.Fatalf("initial boards created=%v len=%d, want 3", created, len(initial))
	}

	repo.EnableDevelopmentInteractiveTitleFirstPoC()
	expanded, changed := repo.MaterializationBoards(host)
	if !changed || len(expanded) != 6 {
		t.Fatalf("interactive boards changed=%v len=%d, want 6", changed, len(expanded))
	}
	want := []string{"フリートーク", "パソコン通信・モデム", "地域の話題", "ゲーム", "音楽", "ソフトウェア"}
	for i, name := range want {
		if expanded[i].Name != name {
			t.Fatalf("board[%d]=%q, want %q", i, expanded[i].Name, name)
		}
	}
	if got := developmentConversationShellLimit(repo); got != 12 {
		t.Fatalf("interactive shell limit=%d, want 12", got)
	}

	stored, changedAgain := repo.MaterializationBoards(host)
	if changedAgain || len(stored) != 6 {
		t.Fatalf("stored interactive boards changed=%v len=%d, want unchanged 6", changedAgain, len(stored))
	}
}

func TestInteractiveActivityWindowIsLongerThanDefaultLabWindow(t *testing.T) {
	base := world.NewMemoryStore()
	repo := New(base, nil, nil, "1996-08-29")
	host, err := repo.HostByPhone("0450000196")
	if err != nil {
		t.Fatal(err)
	}
	personas, _ := repo.MaterializationPersonas(host)
	board := world.Board{ID: "4", Name: "ゲーム"}

	labVisits := developmentVisitsForBoard(host, board, personas, repo.WorldDate)
	interactiveVisits := developmentVisitsForBoardDays(host, board, personas, repo.WorldDate, developmentInteractiveActivityLookbackDays)
	if len(interactiveVisits) <= len(labVisits) {
		t.Fatalf("interactive visits=%d, default visits=%d; longer observation window should add sample opportunities", len(interactiveVisits), len(labVisits))
	}
	if len(interactiveVisits) == 0 {
		t.Fatal("interactive visit sample is empty")
	}
	if !interactiveVisits[0].createdAt.Before(labVisits[0].createdAt) {
		t.Fatalf("interactive oldest=%s, default oldest=%s; longer window should expose earlier activity", interactiveVisits[0].createdAt, labVisits[0].createdAt)
	}
}
