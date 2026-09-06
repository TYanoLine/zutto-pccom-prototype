package materializationdemo

import (
	"strings"
	"sync"
	"testing"

	"zutto-pccom/apps/server/internal/world"
)

// lateWorldWindowStore simulates the failure mode observed in the live fresh lab:
// earlier board observations return no posts, then a later host-wide producer
// succeeds and commits canonical posts for every board at once.
type lateWorldWindowStore struct {
	*bulkRuntimeStore
	triggerBoard string
	once         sync.Once
}

func (s *lateWorldWindowStore) MaterializationPersonaArticleHeaders(host world.Host, board world.Board) ([]world.Post, bool) {
	if board.ID == s.triggerBoard {
		s.once.Do(func() {
			for _, b := range s.boards {
				s.AddPost(host.ID, world.Post{BoardID: b.ID, Author: "TEST", Subject: "subject-" + b.ID})
			}
		})
	}
	return s.bulkRuntimeStore.MaterializationPersonaArticleHeaders(host, board)
}

func TestAllBodyRescansCanonicalPostsAfterEnvelopePlanning(t *testing.T) {
	base := world.NewMemoryStore()
	host := world.Host{ID: "bulk-late-window-host", Name: "BULK LATE WINDOW"}
	boards := []world.Board{{ID: "1", Name: "フリートーク"}, {ID: "2", Name: "通信"}, {ID: "3", Name: "地域"}}
	core := &bulkRuntimeStore{MemoryStore: base, boards: boards}
	store := &lateWorldWindowStore{bulkRuntimeStore: core, triggerBoard: "3"}

	runtime := New(host, store)
	start, disconnected := runtime.HandleLine("ALLBODY")
	if disconnected {
		t.Fatal("ALLBODY unexpectedly disconnected")
	}
	if !strings.Contains(start, "BULK BODY STARTED") {
		t.Fatalf("unexpected start: %s", start)
	}

	status := waitForBulkState(t, runtime, "COMPLETED")
	for _, want := range []string{
		"BOARDS         : 3/3 / envelopes=3",
		"REQUESTS       : 3/3",
		"bodies=3/3",
		"failures=0",
	} {
		if !strings.Contains(status, want) {
			t.Fatalf("completed STATUS missing %q:\n%s", want, status)
		}
	}
	if strings.Contains(status, "EMPTY BOARDS") {
		t.Fatalf("canonical rescan must see posts created for earlier boards:\n%s", status)
	}

	accesses := store.accessSnapshot()
	if len(accesses) != 3 {
		t.Fatalf("accesses=%d want=3: %v", len(accesses), accesses)
	}
	for _, post := range store.ListPosts(host.ID) {
		if strings.TrimSpace(post.Body) == "" {
			t.Fatalf("post %d on board %s body was not rendered", post.ID, post.BoardID)
		}
	}
}
