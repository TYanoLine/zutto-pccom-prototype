package worldrepo

import (
	"fmt"
	"strings"

	"zutto-pccom/apps/server/internal/world"
)

func (r *Repository) materializationArticleWorkerContext(host world.Host, board world.Board, selected world.Post) string {
	responseToID := world.ResponseTargetID(selected)
	if responseToID == 0 {
		return ""
	}
	all := r.Base.ListPosts(host.ID)
	seen := map[int64]bool{}
	var b strings.Builder
	b.WriteString("THREAD CONTEXT (canonical article content only):\n")
	add := func(post world.Post) {
		if post.ID == selected.ID || seen[post.ID] {
			return
		}
		seen[post.ID] = true
		fmt.Fprintf(&b, "[%s] %s\n", post.Author, strings.TrimSpace(post.Subject))
		if body := strings.TrimSpace(post.Body); body != "" {
			b.WriteString(truncateDemoContext(body, 420))
			b.WriteString("\n")
		} else if summary := strings.TrimSpace(post.Intent.SituationSummary); summary != "" {
			b.WriteString(summary)
			b.WriteString("\n")
		}
	}
	if source, ok := developmentConversationFindPost(all, responseToID); ok {
		add(source)
	}
	for _, prior := range r.materializationThreadPredecessors(host.ID, board.ID, selected) {
		add(prior)
	}
	return strings.TrimSpace(b.String())
}

func workerRelevantSituationFact(fact string) bool {
	fact = strings.TrimSpace(fact)
	for _, prefix := range []string{
		"article_detail=", "source_article_detail=", "world_adopted_summary=", "source_world_adopted_summary=",
		"focus=", "occurrence=", "scope_boundary=", "source_focus=", "source_occurrence=", "source_scope_boundary=",
		"continuation=", "source_fact=", "concrete_matter=", "subject_anchor=",
	} {
		if strings.HasPrefix(fact, prefix) {
			return true
		}
	}
	return false
}
