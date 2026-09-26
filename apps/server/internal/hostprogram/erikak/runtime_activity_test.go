package erikak

import (
	"strings"
	"testing"

	"zutto-pccom/apps/server/internal/world"
)

type preplannedActivityStore struct {
	*world.MemoryStore
	calls  []string
	states map[string]world.BoardActivityState
}

func (s *preplannedActivityStore) BoardActivity(_ world.Host, board world.Board) (world.BoardActivityState, bool) {
	s.calls = append(s.calls, board.ID)
	state, ok := s.states[board.ID]
	return state, ok
}

func TestBoardMenuShowsPreplannedCountsBeforeHeadersExist(t *testing.T) {
	base := world.NewMemoryStore()
	host, err := base.HostByPhone("0920000196")
	if err != nil {
		t.Fatal(err)
	}
	store := &preplannedActivityStore{
		MemoryStore: base,
		states: map[string]world.BoardActivityState{
			"4": {BoardID: "4", RetainedRoots: 37},
			"7": {BoardID: "7", RetainedRoots: 12},
		},
	}
	runtime := New(host, store)

	if posts := store.ListPosts(host.ID); len(posts) != 0 {
		t.Fatalf("test requires no materialized HAKATA articles, got %d", len(posts))
	}
	loginGuest(t, runtime)
	if len(store.calls) == 0 {
		t.Fatal("login did not preplan leaf-board activity")
	}
	if posts := store.ListPosts(host.ID); len(posts) != 0 {
		t.Fatalf("activity planning materialized %d posts; it must remain prose-free", len(posts))
	}

	out, disconnect := runtime.HandleLine("BM")
	if disconnect {
		t.Fatal("board menu disconnected")
	}
	if !strings.Contains(out, "[4] ふり～と～く 37") {
		t.Fatalf("preplanned free-talk count missing: %q", out)
	}
	if !strings.Contains(out, "[7] ＣＡＮＡＬ市場 12") {
		t.Fatalf("preplanned market count missing: %q", out)
	}
}
