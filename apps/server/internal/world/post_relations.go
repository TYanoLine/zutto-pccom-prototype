package world

import "strings"

// ResponseTargetID returns the canonical world-level article that this post
// responds to.
//
// RespondsToPostID is authoritative for generated semantic state. SourcePostID
// is accepted for older reply envelopes. ParentID is only a final compatibility
// fallback for legacy/manual posts whose host-native topology was stored before
// semantic response links were explicit.
//
// A host program may legitimately keep ParentID == 0 for a semantic response
// (for example, a flat-message system) or use ParentID with no independent
// response subject (for example, an append-style system).
func ResponseTargetID(post Post) int64 {
	if post.Intent.RespondsToPostID != 0 {
		return post.Intent.RespondsToPostID
	}
	if post.Intent.SourcePostID != 0 && strings.EqualFold(strings.TrimSpace(post.Intent.DiscourseMode), "reply") {
		return post.Intent.SourcePostID
	}
	return post.ParentID
}

// IsSemanticRoot reports whether the post is not a response to another post,
// independent of how a particular host program projects article topology.
func IsSemanticRoot(post Post) bool {
	return ResponseTargetID(post) == 0
}
