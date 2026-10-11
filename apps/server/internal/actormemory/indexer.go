package actormemory

import (
	"fmt"

	"zutto-pccom/apps/server/internal/world"
)

// PostIndexer adapts an Index to worldrepo.PostIndexer: it turns a saved post into
// an index item and hands it over without waiting for any embedding.
type PostIndexer struct {
	idx *Index
}

// NewPostIndexer wraps idx. A nil idx makes IndexPost a no-op.
func NewPostIndexer(idx *Index) *PostIndexer { return &PostIndexer{idx: idx} }

// IndexPost enqueues one post. Posts with neither subject nor body are skipped.
// The event ID matches the interaction ledger's ("post:<host>:<id>").
func (p *PostIndexer) IndexPost(hostID string, post world.Post) {
	if p == nil || p.idx == nil || (post.Subject == "" && post.Body == "") {
		return
	}
	p.idx.Enqueue(Item{
		EventID:    fmt.Sprintf("post:%s:%d", hostID, post.ID),
		HostID:     hostID,
		PostID:     post.ID,
		BoardID:    post.BoardID,
		Scope:      fmt.Sprintf("public_board:%s:%s", hostID, post.BoardID),
		Author:     postActor(post),
		Subject:    post.Subject,
		Text:       post.Body,
		OccurredAt: post.CreatedAt,
	})
}
