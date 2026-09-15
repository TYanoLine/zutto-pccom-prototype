package materializationdemo

import (
	"strings"
	"sync"
	"testing"
	"time"

	"zutto-pccom/apps/server/internal/world"
)

type blockingIndexStore struct {
	*bulkRuntimeStore
	started chan struct{}
	release chan struct{}
	once    sync.Once
}

func (s *blockingIndexStore) MaterializationPersonaArticleHeaders(host world.Host, board world.Board) ([]world.Post, bool) {
	s.once.Do(func() { close(s.started) })
	<-s.release
	post := s.AddPost(host.ID, world.Post{BoardID: board.ID, Author: "TEST", Subject: "生成されたタイトル", CreatedAt: time.Now()})
	return []world.Post{post}, true
}

func TestInteractiveArticleIndexGenerationDoesNotBlockTerminal(t *testing.T) {
	host := world.Host{ID: "interactive-index-host", Name: "INDEX DEBUG"}
	core := &bulkRuntimeStore{
		MemoryStore: world.NewMemoryStore(),
		boards:      []world.Board{{ID: "1", Name: "フリートーク"}, {ID: "2", Name: "通信"}},
	}
	store := &blockingIndexStore{bulkRuntimeStore: core, started: make(chan struct{}), release: make(chan struct{})}
	defer func() {
		select {
		case <-store.release:
		default:
			close(store.release)
		}
	}()

	runtime := New(host, store)
	boards, disconnected := runtime.HandleLine("B")
	if disconnected || !strings.Contains(boards, "フリートーク") {
		t.Fatalf("B did not return board catalog: %s", boards)
	}

	startedAt := time.Now()
	out, disconnected := runtime.HandleLine("1")
	elapsed := time.Since(startedAt)
	if disconnected {
		t.Fatal("board selection unexpectedly disconnected")
	}
	if elapsed > 250*time.Millisecond {
		t.Fatalf("board selection blocked for %s", elapsed)
	}
	if !strings.Contains(out, "GENERATING IN BACKGROUND") {
		t.Fatalf("board selection did not return immediate progress UI: %s", out)
	}

	select {
	case <-store.started:
	case <-time.After(time.Second):
		t.Fatal("background article-index worker never started")
	}

	refreshAt := time.Now()
	progress, _ := runtime.HandleLine("R")
	if time.Since(refreshAt) > 250*time.Millisecond || !strings.Contains(progress, "GENERATING IN BACKGROUND") {
		t.Fatalf("refresh blocked while generation was running: %s", progress)
	}

	close(store.release)
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		out, _ = runtime.HandleLine("R")
		if strings.Contains(out, "生成されたタイトル") {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("completed background index was not displayed: %s", out)
}
