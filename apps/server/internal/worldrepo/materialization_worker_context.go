package worldrepo

import (
	"fmt"
	"sort"
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
		fmt.Fprintf(&b, "[%s] %s\n", post.Author, semanticContextSubject(post))
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
		"object_class=", "observation=", "experience=", "result=", "stance=", "basis=", "attempted_actions=", "practical_point=", "question=",
		"source_object_class=", "source_observation=", "source_experience=", "source_result=", "source_stance=", "source_basis=", "source_attempted_actions=", "source_practical_point=", "source_question=",
		"continuation=", "source_fact=", "concrete_matter=", "subject_anchor=",
	} {
		if strings.HasPrefix(fact, prefix) {
			return true
		}
	}
	return false
}


type materializationAuthorHistoryCandidate struct {
	post  world.Post
	score int
}

// materializationAuthorHistoryContext supplies a bounded view of the selected
// actor's own earlier canonical posts. It is continuity evidence only: it must
// never become a topic queue, and it deliberately avoids materializing missing
// historical bodies just to build context.
func (r *Repository) materializationAuthorHistoryContext(host world.Host, board world.Board, selected world.Post, limit int) string {
	if strings.TrimSpace(selected.AuthorPersonaID) == "" || limit <= 0 {
		return ""
	}

	all := r.Base.ListPosts(host.ID)
	postsByID := make(map[int64]world.Post, len(all))
	for _, post := range all {
		postsByID[post.ID] = post
	}

	currentThreadRoot := int64(0)
	if world.ResponseTargetID(selected) != 0 {
		currentThreadRoot = threadRootID(postsByID, selected)
	}

	candidates := make([]materializationAuthorHistoryCandidate, 0, limit*2)
	for _, post := range all {
		if post.ID == selected.ID || post.AuthorPersonaID != selected.AuthorPersonaID || !postBefore(post, selected) {
			continue
		}
		if currentThreadRoot != 0 && threadRootID(postsByID, post) == currentThreadRoot {
			continue
		}
		score := 0
		if sameNonEmptyFold(post.Intent.Topic, selected.Intent.Topic) {
			score += 40
		}
		if sameNonEmptyFold(post.Intent.AnchorKey, selected.Intent.AnchorKey) {
			score += 20
		}
		if materializationSharedReferent(post.Intent.ProducerReferents, selected.Intent.ProducerReferents) {
			score += 30
		}
		if post.BoardID == board.ID {
			score += 10
		}
		if normalizeDemoSubject(semanticContextSubject(post)) != "" &&
			normalizeDemoSubject(semanticContextSubject(post)) == normalizeDemoSubject(semanticContextSubject(selected)) {
			score += 15
		}
		candidates = append(candidates, materializationAuthorHistoryCandidate{post: post, score: score})
	}
	if len(candidates) == 0 {
		return ""
	}

	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].score != candidates[j].score {
			return candidates[i].score > candidates[j].score
		}
		if !candidates[i].post.CreatedAt.Equal(candidates[j].post.CreatedAt) {
			return candidates[i].post.CreatedAt.After(candidates[j].post.CreatedAt)
		}
		return candidates[i].post.ID > candidates[j].post.ID
	})
	if len(candidates) > limit {
		candidates = candidates[:limit]
	}

	var b strings.Builder
	b.WriteString("AUTHOR'S OWN EARLIER CANONICAL POSTS — continuity evidence only; not a topic queue:\n")
	for _, candidate := range candidates {
		post := candidate.post
		b.WriteString("- ")
		if subject := semanticContextSubject(post); strings.TrimSpace(subject) != "" {
			b.WriteString(strings.TrimSpace(subject))
		} else {
			b.WriteString("(no subject)")
		}
		if body := strings.TrimSpace(post.Body); body != "" {
			b.WriteString("\n  said: ")
			b.WriteString(truncateDemoContext(body, 320))
		} else if summary := materializationAuthorHistorySemanticSummary(post); summary != "" {
			b.WriteString("\n  semantic: ")
			b.WriteString(summary)
		}
		b.WriteByte('\n')
	}
	return strings.TrimSpace(b.String())
}

func sameNonEmptyFold(a, b string) bool {
	a = strings.TrimSpace(a)
	b = strings.TrimSpace(b)
	return a != "" && b != "" && strings.EqualFold(a, b)
}

func materializationSharedReferent(a, b []string) bool {
	if len(a) == 0 || len(b) == 0 {
		return false
	}
	seen := make(map[string]bool, len(a))
	for _, value := range a {
		value = strings.TrimSpace(strings.ToLower(value))
		if value != "" {
			seen[value] = true
		}
	}
	for _, value := range b {
		value = strings.TrimSpace(strings.ToLower(value))
		if value != "" && seen[value] {
			return true
		}
	}
	return false
}

func materializationAuthorHistorySemanticSummary(post world.Post) string {
	parts := make([]string, 0, 4)
	if value := strings.TrimSpace(post.Intent.SituationSummary); value != "" {
		parts = append(parts, "event="+truncateDemoContext(value, 180))
	}
	if value := strings.TrimSpace(post.Intent.Topic); value != "" {
		parts = append(parts, "topic="+truncateDemoContext(value, 120))
	}
	if len(post.Intent.Claims) > 0 {
		parts = append(parts, "claims="+truncateDemoContext(strings.Join(post.Intent.Claims, " / "), 180))
	}
	if len(post.Intent.ProducerContribution) > 0 {
		parts = append(parts, "contribution="+truncateDemoContext(strings.Join(post.Intent.ProducerContribution, " / "), 180))
	}
	return strings.Join(parts, "; ")
}