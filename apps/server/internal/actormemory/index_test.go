package actormemory

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"zutto-pccom/apps/server/internal/llm"
	"zutto-pccom/apps/server/internal/world"
)

// fakeEmbedder is a deterministic EmbeddingClient that records its calls.
type fakeEmbedder struct {
	mu       sync.Mutex
	calls    int
	sizes    []int
	failNext int
	block    chan struct{}
	vecs     map[string][]float32
}

func (f *fakeEmbedder) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	f.mu.Lock()
	f.calls++
	f.sizes = append(f.sizes, len(texts))
	fail := f.failNext > 0
	if fail {
		f.failNext--
	}
	block := f.block
	f.mu.Unlock()
	if block != nil {
		select {
		case <-block:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	if fail {
		return nil, errors.New("embedding unavailable")
	}
	out := make([][]float32, len(texts))
	for i, t := range texts {
		if v, ok := f.vecs[t]; ok {
			out[i] = v
		} else {
			out[i] = []float32{0, 0, 1}
		}
	}
	return out, nil
}

func (f *fakeEmbedder) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls
}

func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}

func item(host string, id int, text string) Item {
	return Item{
		EventID:    fmt.Sprintf("post:%s:%d", host, id),
		HostID:     host,
		PostID:     int64(id),
		BoardID:    "1",
		Scope:      "public_board:" + host + ":1",
		Author:     world.ActorRef{Kind: "persona", ID: "p1"},
		Text:       text,
		OccurredAt: time.Date(1996, 8, 29, 12, 0, id, 0, time.UTC),
	}
}

func TestIndexBatchesAtBatchSize(t *testing.T) {
	emb := &fakeEmbedder{}
	ix := NewIndex(emb, IndexOptions{BatchSize: 10, FlushInterval: time.Hour})
	defer ix.Close()
	for i := 1; i <= 10; i++ {
		if !ix.Enqueue(item("h", i, "t")) {
			t.Fatalf("enqueue %d rejected", i)
		}
	}
	waitFor(t, "10 items embedded", func() bool { return ix.Len() == 10 })
	if emb.callCount() != 1 {
		t.Fatalf("embed calls=%d, want 1", emb.callCount())
	}
}

func TestIndexTwentyFiveItemsUseAtMostThreeCalls(t *testing.T) {
	emb := &fakeEmbedder{}
	ix := NewIndex(emb, IndexOptions{BatchSize: 10, FlushInterval: 20 * time.Millisecond})
	for i := 1; i <= 25; i++ {
		ix.Enqueue(item("h", i, "t"))
	}
	ix.Close()
	if ix.Len() != 25 {
		t.Fatalf("len=%d, want 25", ix.Len())
	}
	if emb.callCount() > 3 {
		t.Fatalf("embed calls=%d, want <=3 (sizes %v)", emb.callCount(), emb.sizes)
	}
}

func TestIndexFlushesPartialBatchAfterInterval(t *testing.T) {
	emb := &fakeEmbedder{}
	ix := NewIndex(emb, IndexOptions{BatchSize: 10, FlushInterval: 30 * time.Millisecond})
	defer ix.Close()
	ix.Enqueue(item("h", 1, "a"))
	ix.Enqueue(item("h", 2, "b"))
	waitFor(t, "partial batch flushed", func() bool { return ix.Len() == 2 })
	if emb.callCount() != 1 {
		t.Fatalf("embed calls=%d, want 1", emb.callCount())
	}
}

func TestIndexEnqueueDoesNotWaitForEmbedding(t *testing.T) {
	emb := &fakeEmbedder{block: make(chan struct{})}
	ix := NewIndex(emb, IndexOptions{BatchSize: 1, FlushInterval: time.Millisecond})
	done := make(chan struct{})
	go func() {
		for i := 1; i <= 5; i++ {
			ix.Enqueue(item("h", i, "t"))
		}
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Enqueue blocked while embedding was blocked")
	}
	close(emb.block)
	ix.Close()
	if ix.Len() != 5 {
		t.Fatalf("len=%d, want 5", ix.Len())
	}
}

func TestIndexSurvivesEmbedFailureAndAllowsRetry(t *testing.T) {
	emb := &fakeEmbedder{failNext: 1}
	ix := NewIndex(emb, IndexOptions{BatchSize: 2, FlushInterval: 10 * time.Millisecond})
	defer ix.Close()
	first := item("h", 1, "a")
	ix.Enqueue(first)
	ix.Enqueue(item("h", 2, "b"))
	waitFor(t, "failed batch attempted", func() bool { return emb.callCount() >= 1 })
	// Enqueue stays false while the item is still marked as seen, and becomes
	// true once the failed batch has been rolled back.
	waitFor(t, "failed item enqueueable again", func() bool { return ix.Enqueue(first) })
	if ix.Len() != 0 {
		t.Fatalf("failed batch must not be indexed, len=%d", ix.Len())
	}
	ix.Enqueue(item("h", 3, "c"))
	waitFor(t, "worker kept going", func() bool { return ix.Len() == 2 })
}

func TestIndexEnqueueIsIdempotentPerEvent(t *testing.T) {
	emb := &fakeEmbedder{}
	ix := NewIndex(emb, IndexOptions{BatchSize: 10, FlushInterval: 10 * time.Millisecond})
	defer ix.Close()
	it := item("h", 1, "a")
	if !ix.Enqueue(it) {
		t.Fatal("first enqueue rejected")
	}
	if ix.Enqueue(it) {
		t.Fatal("duplicate enqueue accepted")
	}
	waitFor(t, "indexed", func() bool { return ix.Len() == 1 })
	if ix.Enqueue(it) {
		t.Fatal("duplicate after indexing accepted")
	}
	time.Sleep(30 * time.Millisecond)
	if ix.Len() != 1 || emb.callCount() != 1 {
		t.Fatalf("len=%d calls=%d, want 1/1", ix.Len(), emb.callCount())
	}
}

func TestIndexSearchRanksFiltersAndDropsNonPositive(t *testing.T) {
	emb := &fakeEmbedder{vecs: map[string][]float32{
		"グラボの設定":  {1, 0, 0},
		"ほぼ同じ話題":  {0.9, 0.1, 0},
		"少し近い話題":  {0.5, 0.5, 0},
		"無関係":     {0, 1, 0},
		"反対":      {-1, 0, 0},
		"他人の私信":   {1, 0, 0},
		"別の局の記事":  {1, 0, 0},
		"PC-98の画面": {1, 0, 0}, // the query
	}}
	ix := NewIndex(emb, IndexOptions{BatchSize: 20, FlushInterval: 10 * time.Millisecond})
	defer ix.Close()
	ix.Enqueue(item("h", 1, "グラボの設定"))
	ix.Enqueue(item("h", 2, "ほぼ同じ話題"))
	ix.Enqueue(item("h", 3, "少し近い話題"))
	ix.Enqueue(item("h", 4, "無関係"))
	ix.Enqueue(item("h", 5, "反対"))
	secret := item("h", 6, "他人の私信")
	secret.Scope = "mail:h:x"
	ix.Enqueue(secret)
	ix.Enqueue(item("other", 7, "別の局の記事"))
	waitFor(t, "all indexed", func() bool { return ix.Len() == 7 })

	visible := func(it Item) bool { return it.Scope != "mail:h:x" }
	hits, err := ix.Search(context.Background(), SearchQuery{HostID: "h", Text: "PC-98の画面", Limit: 10, Visible: visible})
	if err != nil {
		t.Fatal(err)
	}
	var got []int64
	for _, h := range hits {
		got = append(got, h.Item.PostID)
	}
	want := []int64{1, 2, 3}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("hit post ids=%v, want %v", got, want)
	}

	limited, err := ix.Search(context.Background(), SearchQuery{HostID: "h", Text: "PC-98の画面", Limit: 2, Visible: visible})
	if err != nil || len(limited) != 2 {
		t.Fatalf("limited=%d err=%v", len(limited), err)
	}
}

func TestIndexWithoutClientIsUnavailable(t *testing.T) {
	ix := NewIndex(nil, IndexOptions{})
	if ix.Enqueue(item("h", 1, "a")) {
		t.Fatal("nil client must not accept items")
	}
	if _, err := ix.Search(context.Background(), SearchQuery{HostID: "h", Text: "q"}); !errors.Is(err, ErrIndexUnavailable) {
		t.Fatalf("err=%v, want ErrIndexUnavailable", err)
	}
	ix.Close()
}

func TestIndexCloseFlushesAndRejectsLaterItems(t *testing.T) {
	emb := &fakeEmbedder{}
	ix := NewIndex(emb, IndexOptions{BatchSize: 100, FlushInterval: time.Hour})
	ix.Enqueue(item("h", 1, "a"))
	ix.Enqueue(item("h", 2, "b"))
	ix.Close()
	if ix.Len() != 2 {
		t.Fatalf("Close must flush waiting items, len=%d", ix.Len())
	}
	if ix.Enqueue(item("h", 3, "c")) {
		t.Fatal("enqueue after Close accepted")
	}
	ix.Close() // idempotent
}

var _ llm.EmbeddingClient = (*fakeEmbedder)(nil)
