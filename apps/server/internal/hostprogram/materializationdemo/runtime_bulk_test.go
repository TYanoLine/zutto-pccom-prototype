package materializationdemo

import (
	"fmt"
	"strings"
	"testing"

	"zutto-pccom/apps/server/internal/world"
)

type bulkRuntimeStore struct {
	*world.MemoryStore
	boards   []world.Board
	accesses []string
}

func (s *bulkRuntimeStore) HostWasMaterialized(string) bool       { return false }
func (s *bulkRuntimeStore) PopulationWasMaterialized(string) bool { return false }
func (s *bulkRuntimeStore) MaterializationPersonas(world.Host) ([]world.Persona, bool) {
	return nil, false
}
func (s *bulkRuntimeStore) MaterializationBoards(world.Host) ([]world.Board, bool) {
	return append([]world.Board(nil), s.boards...), false
}
func (s *bulkRuntimeStore) MaterializationPersonaArticleHeaders(host world.Host, board world.Board) ([]world.Post, bool) {
	out := make([]world.Post, 0)
	for _, post := range s.ListPosts(host.ID) {
		if post.BoardID == board.ID {
			out = append(out, post)
		}
	}
	return out, false
}
func (s *bulkRuntimeStore) MaterializationArticleWithDebug(host world.Host, board world.Board, postID int64) (world.Post, bool, bool, string) {
	s.accesses = append(s.accesses, fmt.Sprintf("%s:%d", board.ID, postID))
	for _, post := range s.ListPosts(host.ID) {
		if post.BoardID != board.ID || post.ID != postID {
			continue
		}
		created := false
		if strings.TrimSpace(post.Body) == "" {
			post.Body = fmt.Sprintf("body-%d", post.ID)
			s.UpdatePost(host.ID, post)
			created = true
		}
		return post, true, created, ""
	}
	return world.Post{}, false, false, "not found"
}
func (s *bulkRuntimeStore) MaterializationPlanningDiagnostic(string, string) string { return "" }
func (s *bulkRuntimeStore) MaterializationUsageTotalText() string                   { return "" }
func (s *bulkRuntimeStore) ResetMaterializationConversation(world.Host) (int, int, bool) {
	return 0, 0, true
}

func TestAllBodyCommandRendersEveryEnvelopeThroughRandomizedAccessRequests(t *testing.T) {
	base := world.NewMemoryStore()
	host := world.Host{ID: "bulk-debug-host", Name: "BULK DEBUG"}
	boards := []world.Board{{ID: "1", Name: "フリートーク"}, {ID: "2", Name: "通信"}, {ID: "3", Name: "地域"}}
	store := &bulkRuntimeStore{MemoryStore: base, boards: boards}

	for _, boardID := range []string{"1", "1", "2", "3"} {
		store.AddPost(host.ID, world.Post{BoardID: boardID, Author: "TEST", Subject: "subject"})
	}

	runtime := New(host, store)
	output, disconnected := runtime.HandleLine("ALLBODY")
	if disconnected {
		t.Fatal("ALLBODY unexpectedly disconnected")
	}
	if len(store.accesses) != 4 {
		t.Fatalf("accesses=%d want=4: %v", len(store.accesses), store.accesses)
	}
	seen := map[string]bool{}
	for _, access := range store.accesses {
		if seen[access] {
			t.Fatalf("duplicate access request: %s", access)
		}
		seen[access] = true
	}
	for _, post := range store.ListPosts(host.ID) {
		if strings.TrimSpace(post.Body) == "" {
			t.Fatalf("post %d body was not rendered", post.ID)
		}
	}
	for _, want := range []string{
		"envelopes=4",
		"generated=4",
		"complete=4",
		"failures=0",
		"[DEV] ACCESS SEED",
		"[DEV] REQUEST ORDER",
		"[DEV] THREAD ORDER",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("ALLBODY output missing %q:\n%s", want, output)
		}
	}
}

func TestBulkTargetShuffleIsSeededPermutation(t *testing.T) {
	targets := []bulkBodyTarget{
		{board: world.Board{ID: "1"}, postID: 10},
		{board: world.Board{ID: "2"}, postID: 20},
		{board: world.Board{ID: "3"}, postID: 30},
		{board: world.Board{ID: "1"}, postID: 40},
		{board: world.Board{ID: "2"}, postID: 50},
	}
	first := shuffledBulkBodyTargets(targets, 19660829)
	second := shuffledBulkBodyTargets(targets, 19660829)
	if len(first) != len(targets) || len(second) != len(targets) {
		t.Fatalf("shuffle changed target count")
	}
	for i := range first {
		if bulkBodyTargetKey(first[i]) != bulkBodyTargetKey(second[i]) {
			t.Fatalf("same seed produced different order: %v vs %v", first, second)
		}
	}
	seen := map[string]bool{}
	for _, target := range first {
		key := bulkBodyTargetKey(target)
		if seen[key] {
			t.Fatalf("shuffle duplicated %s", key)
		}
		seen[key] = true
	}
	if len(seen) != len(targets) {
		t.Fatalf("shuffle lost targets: got=%d want=%d", len(seen), len(targets))
	}
	if targets[0].postID != 10 || targets[1].postID != 20 {
		t.Fatal("shuffle mutated caller slice")
	}
}

func TestWelcomeAndHelpAdvertiseAllBodyMode(t *testing.T) {
	store := &bulkRuntimeStore{MemoryStore: world.NewMemoryStore()}
	runtime := New(world.Host{ID: "bulk-debug-host"}, store)
	if !strings.Contains(runtime.Welcome(), "[ALLBODY]") {
		t.Fatal("welcome does not advertise ALLBODY")
	}
	help, _ := runtime.HandleLine("HELP")
	if !strings.Contains(help, "ALLBODY") || !strings.Contains(help, "ランダム") {
		t.Fatalf("help does not explain ALLBODY random access mode: %s", help)
	}
}
