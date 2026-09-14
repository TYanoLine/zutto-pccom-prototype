package main

import (
	"testing"

	"zutto-pccom/apps/server/internal/world"
)

func TestNewRuntimeStoreSeedsExpandedDevelopmentBoardCatalog(t *testing.T) {
	store := newRuntimeStore("")
	host, err := store.HostByPhone(developmentMaterializationPhone)
	if err != nil {
		t.Fatal(err)
	}
	boards := store.ListBoards(host.ID)
	if got, want := len(boards), len(developmentGrassrootsBoardCatalog); got != want {
		t.Fatalf("development boards=%d want=%d", got, want)
	}
	for i, want := range developmentGrassrootsBoardCatalog {
		if boards[i] != want {
			t.Fatalf("board[%d]=%+v want=%+v", i, boards[i], want)
		}
	}
}

func TestEnsureDevelopmentBoardCatalogPreservesExistingBoards(t *testing.T) {
	store := world.NewMemoryStore()
	host, err := store.HostByPhone(developmentMaterializationPhone)
	if err != nil {
		t.Fatal(err)
	}
	store.SaveBoards(host.ID, []world.Board{
		{ID: "1", Name: "既存のフリートーク"},
		{ID: "99", Name: "局固有の隠し板"},
	})

	ensureDevelopmentBoardCatalog(store)
	boards := store.ListBoards(host.ID)
	if got, want := len(boards), len(developmentGrassrootsBoardCatalog)+1; got != want {
		t.Fatalf("development boards=%d want=%d", got, want)
	}
	if boards[0].Name != "既存のフリートーク" {
		t.Fatalf("existing board was replaced: %+v", boards[0])
	}
	if boards[1].ID != "99" || boards[1].Name != "局固有の隠し板" {
		t.Fatalf("custom board was not preserved: %+v", boards[1])
	}

	seen := map[string]bool{}
	for _, board := range boards {
		if seen[board.ID] {
			t.Fatalf("duplicate board id %s", board.ID)
		}
		seen[board.ID] = true
	}
	for _, want := range developmentGrassrootsBoardCatalog {
		if !seen[want.ID] {
			t.Fatalf("missing development board id %s (%s)", want.ID, want.Name)
		}
	}
}
