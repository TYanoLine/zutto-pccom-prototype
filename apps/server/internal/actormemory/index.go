// Package actormemory gives an actor read access to what it has observed.
//
// Everything here is derived from the canonical world store: the vector index is
// a rebuildable cache, and no generation LLM is called by any of it.
package actormemory

import (
	"context"
	"errors"
	"log"
	"sort"
	"sync"
	"time"

	"zutto-pccom/apps/server/internal/llm"
	"zutto-pccom/apps/server/internal/world"
)

// ErrIndexUnavailable is returned by Search when no embedding client is configured.
var ErrIndexUnavailable = errors.New("embedding index is unavailable")

// Item is one observed event that can be recalled by topic.
type Item struct {
	EventID    string
	HostID     string
	PostID     int64
	BoardID    string
	Scope      string
	Author     world.ActorRef
	Addressed  []world.ActorRef
	Subject    string
	Text       string
	OccurredAt time.Time
}

// IndexOptions tunes the asynchronous micro-batching. Zero values use defaults.
type IndexOptions struct {
	BatchSize     int           // flush when this many items are waiting (default 10)
	FlushInterval time.Duration // flush this long after the first waiting item (default 200ms)
	QueueSize     int           // capacity of the intake channel (default 1024)
	EmbedTimeout  time.Duration // per Embed call (default 30s)
}

func (o IndexOptions) withDefaults() IndexOptions {
	if o.BatchSize <= 0 {
		o.BatchSize = 10
	}
	if o.FlushInterval <= 0 {
		o.FlushInterval = 200 * time.Millisecond
	}
	if o.QueueSize <= 0 {
		o.QueueSize = 1024
	}
	if o.EmbedTimeout <= 0 {
		o.EmbedTimeout = 30 * time.Second
	}
	return o
}

// Hit is a search result.
type Hit struct {
	Item  Item
	Score float64
}

// SearchQuery selects what to recall. Visible must restrict results to what the
// calling actor may observe; a nil Visible matches nothing extra (host filter only).
type SearchQuery struct {
	HostID  string
	Text    string
	Limit   int
	Visible func(Item) bool
}

type entry struct {
	item Item
	vec  []float32
}

// Index embeds items in the background, in micro-batches, and searches them by
// cosine similarity. Enqueue never blocks and never fails the caller.
type Index struct {
	client llm.EmbeddingClient
	opts   IndexOptions

	in   chan Item
	done chan struct{}

	mu      sync.RWMutex
	closed  bool
	seen    map[string]bool
	entries map[string]entry
}

// NewIndex starts the background worker. A nil client yields an inert index:
// Enqueue is a no-op and Search returns ErrIndexUnavailable.
func NewIndex(client llm.EmbeddingClient, opts IndexOptions) *Index {
	opts = opts.withDefaults()
	ix := &Index{
		client:  client,
		opts:    opts,
		in:      make(chan Item, opts.QueueSize),
		done:    make(chan struct{}),
		seen:    map[string]bool{},
		entries: map[string]entry{},
	}
	if client == nil {
		close(ix.done)
		return ix
	}
	go ix.run()
	return ix
}

// Enqueue hands an item to the background worker. It returns false when the item
// was not accepted: no client, already seen, closed, or the queue is full.
func (ix *Index) Enqueue(item Item) bool {
	if ix == nil || ix.client == nil || item.EventID == "" {
		return false
	}
	ix.mu.Lock()
	defer ix.mu.Unlock()
	if ix.closed || ix.seen[item.EventID] {
		return false
	}
	select {
	case ix.in <- item:
		ix.seen[item.EventID] = true
		return true
	default:
		return false
	}
}

// Len reports how many items have been embedded.
func (ix *Index) Len() int {
	if ix == nil {
		return 0
	}
	ix.mu.RLock()
	defer ix.mu.RUnlock()
	return len(ix.entries)
}

// Close flushes what is waiting and stops the worker.
func (ix *Index) Close() {
	if ix == nil {
		return
	}
	ix.mu.Lock()
	if !ix.closed {
		ix.closed = true
		if ix.client != nil {
			close(ix.in)
		}
	}
	ix.mu.Unlock()
	<-ix.done
}

func (ix *Index) run() {
	defer close(ix.done)
	batch := make([]Item, 0, ix.opts.BatchSize)
	var timer <-chan time.Time
	for {
		select {
		case item, ok := <-ix.in:
			if !ok {
				ix.flush(batch)
				return
			}
			if len(batch) == 0 {
				timer = time.After(ix.opts.FlushInterval)
			}
			batch = append(batch, item)
			if len(batch) >= ix.opts.BatchSize {
				ix.flush(batch)
				batch = batch[:0]
				timer = nil
			}
		case <-timer:
			ix.flush(batch)
			batch = batch[:0]
			timer = nil
		}
	}
}

func (ix *Index) flush(batch []Item) {
	if len(batch) == 0 {
		return
	}
	texts := make([]string, len(batch))
	for i, item := range batch {
		texts[i] = embedText(item)
	}
	ctx, cancel := context.WithTimeout(context.Background(), ix.opts.EmbedTimeout)
	defer cancel()
	vecs, err := ix.client.Embed(ctx, texts)
	if err != nil || len(vecs) != len(batch) {
		if err == nil {
			err = errors.New("embedding count mismatch")
		}
		// Restore state first so a slow log sink never delays a retry.
		ix.forget(batch)
		log.Printf("actormemory: embed batch of %d failed: %v", len(batch), err)
		return
	}
	ix.mu.Lock()
	defer ix.mu.Unlock()
	for i, item := range batch {
		if len(vecs[i]) == 0 {
			delete(ix.seen, item.EventID)
			continue
		}
		ix.entries[item.EventID] = entry{item: item, vec: vecs[i]}
	}
}

// forget lets a failed batch be enqueued again.
func (ix *Index) forget(batch []Item) {
	ix.mu.Lock()
	defer ix.mu.Unlock()
	for _, item := range batch {
		delete(ix.seen, item.EventID)
	}
}

func embedText(item Item) string {
	if item.Subject == "" {
		return item.Text
	}
	return item.Subject + "\n" + item.Text
}

// Search embeds the query once and returns visible items by descending cosine
// similarity. Items scoring 0 or less are not returned.
func (ix *Index) Search(ctx context.Context, q SearchQuery) ([]Hit, error) {
	if ix == nil || ix.client == nil {
		return nil, ErrIndexUnavailable
	}
	if q.Text == "" {
		return nil, nil
	}
	vecs, err := ix.client.Embed(ctx, []string{q.Text})
	if err != nil {
		return nil, err
	}
	if len(vecs) != 1 || len(vecs[0]) == 0 {
		return nil, errors.New("embedding of the query is empty")
	}
	query := vecs[0]

	ix.mu.RLock()
	candidates := make([]entry, 0, len(ix.entries))
	for _, e := range ix.entries {
		if e.item.HostID != q.HostID {
			continue
		}
		if q.Visible != nil && !q.Visible(e.item) {
			continue
		}
		candidates = append(candidates, e)
	}
	ix.mu.RUnlock()

	hits := make([]Hit, 0, len(candidates))
	for _, e := range candidates {
		score := llm.CosineSimilarity(query, e.vec)
		if score <= 0 {
			continue
		}
		hits = append(hits, Hit{Item: e.item, Score: score})
	}
	sort.Slice(hits, func(i, j int) bool {
		if hits[i].Score != hits[j].Score {
			return hits[i].Score > hits[j].Score
		}
		if !hits[i].Item.OccurredAt.Equal(hits[j].Item.OccurredAt) {
			return hits[i].Item.OccurredAt.After(hits[j].Item.OccurredAt)
		}
		return hits[i].Item.EventID < hits[j].Item.EventID
	})
	if q.Limit > 0 && len(hits) > q.Limit {
		hits = hits[:q.Limit]
	}
	return hits, nil
}
