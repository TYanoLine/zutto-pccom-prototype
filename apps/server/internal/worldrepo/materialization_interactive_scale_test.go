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
	if got := developmentConversationShellLimit(repo); got != 24 {
		t.Fatalf("interactive shell limit=%d, want 24", got)
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

func TestSpecializedBoardAffinityUsesMatchingInterest(t *testing.T) {
	p := world.Persona{Interests: map[string]float64{"games": .8, "music": .6, "software": .9}}
	tests := []struct {
		board world.Board
		want  float64
	}{
		{world.Board{ID: "4", Name: "ゲーム"}, .8},
		{world.Board{ID: "5", Name: "音楽"}, .6},
		{world.Board{ID: "6", Name: "ソフトウェア"}, .9},
	}
	for _, tc := range tests {
		if got := demoBoardAffinity(p, tc.board); got != tc.want {
			t.Fatalf("board %s affinity=%v want=%v", tc.board.ID, got, tc.want)
		}
	}
	unrelated := world.Persona{Interests: map[string]float64{"chat": 1, "local": 1}}
	if got := demoBoardAffinity(unrelated, world.Board{ID: "6", Name: "ソフトウェア"}); got != 0 {
		t.Fatalf("unrelated software affinity=%v want=0", got)
	}
}
