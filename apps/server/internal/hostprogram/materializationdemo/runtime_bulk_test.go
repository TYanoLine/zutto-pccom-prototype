package materializationdemo

import (
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"zutto-pccom/apps/server/internal/world"
)

type bulkRuntimeStore struct {
	*world.MemoryStore
	boards []world.Board

	mu            sync.Mutex
	accesses      []string
	blockFirst    bool
	accessStarted chan struct{}
	accessRelease chan struct{}
	startOnce     sync.Once
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
	s.mu.Lock()
	s.accesses = append(s.accesses, fmt.Sprintf("%s:%d", board.ID, postID))
	isFirst := len(s.accesses) == 1
	s.mu.Unlock()

	if s.blockFirst && isFirst {
		s.startOnce.Do(func() { close(s.accessStarted) })
		<-s.accessRelease
	}

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

func (s *bulkRuntimeStore) accessSnapshot() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.accesses...)
}

func waitForBulkState(t *testing.T, runtime *Runtime, want string) string {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		status, _ := runtime.HandleLine("STATUS")
		if strings.Contains(status, "BULK STATUS    : "+want) {
			return status
		}
		time.Sleep(5 * time.Millisecond)
	}
	status, _ := runtime.HandleLine("STATUS")
	t.Fatalf("bulk state did not reach %s: %s", want, status)
	return ""
}

func TestAllBodyCommandRunsInBackgroundAndReportsProgress(t *testing.T) {
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
	if !strings.Contains(output, "BULK BODY STARTED") || !strings.Contains(output, "STATUS=進捗表示") || !strings.Contains(output, "CANCEL=中断要求") {
		t.Fatalf("ALLBODY did not return immediate async controls: %s", output)
	}

	status := waitForBulkState(t, runtime, "COMPLETED")
	for _, want := range []string{
		"BOARDS         : 3/3",
		"REQUESTS       : 4/4",
		"bodies=4/4",
		"generated=4",
		"failures=0",
		"[DEV] ACCESS SEED",
		"[DEV] REQUEST ORDER",
		"[DEV] THREAD ORDER",
	} {
		if !strings.Contains(status, want) {
			t.Fatalf("completed STATUS missing %q:\n%s", want, status)
		}
	}

	accesses := store.accessSnapshot()
	if len(accesses) != 4 {
		t.Fatalf("accesses=%d want=4: %v", len(accesses), accesses)
	}
	seen := map[string]bool{}
	for _, access := range accesses {
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
}

func TestAllBodyCancelStopsBeforeNextArticle(t *testing.T) {
	base := world.NewMemoryStore()
	host := world.Host{ID: "bulk-cancel-host", Name: "BULK CANCEL"}
	boards := []world.Board{{ID: "1", Name: "フリートーク"}}
	store := &bulkRuntimeStore{
		MemoryStore:    base,
		boards:         boards,
		blockFirst:     true,
		accessStarted:  make(chan struct{}),
		accessRelease:  make(chan struct{}),
	}
	for i := 0; i < 4; i++ {
		store.AddPost(host.ID, world.Post{BoardID: "1", Author: "TEST", Subject: "subject"})
	}

	runtime := New(host, store)
	start, _ := runtime.HandleLine("ALLBODY")
	if !strings.Contains(start, "BULK BODY STARTED") {
		t.Fatalf("unexpected start: %s", start)
	}

	select {
	case <-store.accessStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("first article access never started")
	}

	cancel, _ := runtime.HandleLine("CANCEL")
	if !strings.Contains(cancel, "BULK CANCEL REQUESTED") || !strings.Contains(cancel, "cannot be preempted") {
		t.Fatalf("unexpected cancel response: %s", cancel)
	}
	blockedReset, _ := runtime.HandleLine("RESET")
	if !strings.Contains(blockedReset, "RESET BLOCKED") {
		t.Fatalf("RESET should be blocked while cancel is pending: %s", blockedReset)
	}

	close(store.accessRelease)
	status := waitForBulkState(t, runtime, "CANCELLED")
	if !strings.Contains(status, "REQUESTS       : 1/4") {
		t.Fatalf("cancelled status should retain partial progress: %s", status)
	}
	if accesses := store.accessSnapshot(); len(accesses) != 1 {
		t.Fatalf("cancel should stop before second article, accesses=%v", accesses)
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

func TestWelcomeAndHelpAdvertiseAllBodyControls(t *testing.T) {
	store := &bulkRuntimeStore{MemoryStore: world.NewMemoryStore()}
	runtime := New(world.Host{ID: "bulk-debug-host"}, store)
	welcome := runtime.Welcome()
	for _, want := range []string{"[ALLBODY]", "[STATUS]", "[CANCEL]"} {
		if !strings.Contains(welcome, want) {
			t.Fatalf("welcome does not advertise %s", want)
		}
	}
	help, _ := runtime.HandleLine("HELP")
	for _, want := range []string{"ALLBODY", "ランダム", "STATUS", "CANCEL", "バックグラウンド"} {
		if !strings.Contains(help, want) {
			t.Fatalf("help does not explain %s: %s", want, help)
		}
	}
}
