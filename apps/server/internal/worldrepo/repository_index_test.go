package worldrepo

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"zutto-pccom/apps/server/internal/actormemory"
	"zutto-pccom/apps/server/internal/llm"
	"zutto-pccom/apps/server/internal/world"
)

type recordingIndexer struct {
	mu    sync.Mutex
	calls []world.Post
	hosts []string
}

func (r *recordingIndexer) IndexPost(hostID string, p world.Post) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls = append(r.calls, p)
	r.hosts = append(r.hosts, hostID)
}

func TestAddPostHandsTheSavedPostToTheIndexer(t *testing.T) {
	store := newTestStore()
	repo := New(store, nil, nil, "1996-08-29")
	ix := &recordingIndexer{}
	repo.SetPostIndexer(ix)

	saved := repo.AddPost(genericTestHostID, world.Post{BoardID: "1", Author: "MARI", AuthorPersonaID: "mari",
		Subject: "件名", Body: "本文", CreatedAt: time.Date(1996, 8, 29, 12, 0, 0, 0, time.UTC)})

	if len(ix.calls) != 1 || ix.hosts[0] != genericTestHostID {
		t.Fatalf("indexer calls=%d hosts=%v", len(ix.calls), ix.hosts)
	}
	if ix.calls[0].ID == 0 || ix.calls[0].ID != saved.ID {
		t.Fatalf("indexer must see the saved post with its assigned ID: got %d, saved %d", ix.calls[0].ID, saved.ID)
	}
	if got := store.ListPosts(genericTestHostID); len(got) != 1 || got[0].ID != saved.ID {
		t.Fatalf("canonical write missing: %+v", got)
	}
}

func TestAddPostWithoutIndexerBehavesAsBefore(t *testing.T) {
	store := newTestStore()
	repo := New(store, nil, nil, "1996-08-29")

	saved := repo.AddPost(genericTestHostID, world.Post{BoardID: "1", Author: "MARI", AuthorPersonaID: "mari", Subject: "件名", Body: "本文"})
	if saved.ID == 0 {
		t.Fatal("post was not saved")
	}
	if _, ok := store.GetInteractionEvent("post:" + genericTestHostID + ":" + itoa(saved.ID)); !ok {
		t.Fatal("ledger event must still be recorded without an indexer")
	}

	ix := &recordingIndexer{}
	repo.SetPostIndexer(ix)
	repo.SetPostIndexer(nil)
	repo.AddPost(genericTestHostID, world.Post{BoardID: "1", Author: "MARI", Subject: "二つ目", Body: "本文"})
	if len(ix.calls) != 0 {
		t.Fatalf("a removed indexer must not be called, got %d calls", len(ix.calls))
	}
}

func itoa(n int64) string {
	const digits = "0123456789"
	if n == 0 {
		return "0"
	}
	var b []byte
	for ; n > 0; n /= 10 {
		b = append([]byte{digits[n%10]}, b...)
	}
	return string(b)
}

// blockingEmbedder holds every Embed call until released, like a slow API.
type blockingEmbedder struct {
	release chan struct{}
	mu      sync.Mutex
	calls   int
}

func (b *blockingEmbedder) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	b.mu.Lock()
	b.calls++
	b.mu.Unlock()
	select {
	case <-b.release:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	out := make([][]float32, len(texts))
	for i := range texts {
		out[i] = []float32{1, 0, 0}
	}
	return out, nil
}

var _ llm.EmbeddingClient = (*blockingEmbedder)(nil)

// A slow embedding API must not slow AddPost; once it answers, the post is found
// by the memory tools — the whole path from save to recall.
func TestAddPostIsNotDelayedBySlowEmbeddingAndBecomesSearchable(t *testing.T) {
	store := newTestStore()
	repo := New(store, nil, nil, "1996-08-29")
	emb := &blockingEmbedder{release: make(chan struct{})}
	idx := actormemory.NewIndex(emb, actormemory.IndexOptions{BatchSize: 1, FlushInterval: time.Millisecond})
	defer idx.Close()
	repo.SetPostIndexer(actormemory.NewPostIndexer(idx))

	done := make(chan world.Post, 1)
	go func() {
		done <- repo.AddPost(genericTestHostID, world.Post{BoardID: "1", Author: "MARI", AuthorPersonaID: "mari",
			Subject: "グラボの設定", Body: "SW2の3番をONにしました。", CreatedAt: time.Date(1996, 8, 29, 12, 0, 0, 0, time.UTC)})
	}()
	var saved world.Post
	select {
	case saved = <-done:
	case <-time.After(time.Second):
		t.Fatal("AddPost waited for the embedding API")
	}
	if idx.Len() != 0 {
		t.Fatal("embedding cannot have finished while the API is blocked")
	}

	close(emb.release)
	deadline := time.Now().Add(3 * time.Second)
	for idx.Len() != 1 {
		if time.Now().After(deadline) {
			t.Fatal("post never became searchable")
		}
		time.Sleep(5 * time.Millisecond)
	}

	exec := actormemory.NewExecutor(genericTestHostID, store, idx)
	res, err := exec.Execute(context.Background(), "persona:mari", llm.ToolCall{
		ID: "c1", Name: llm.ToolSearchMyObservedEvents, Arguments: []byte(`{"query":"グラボ"}`)})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(res.Content, "#"+itoa(saved.ID)) || !strings.Contains(res.Content, "グラボの設定") {
		t.Fatalf("search result=%q", res.Content)
	}
}
