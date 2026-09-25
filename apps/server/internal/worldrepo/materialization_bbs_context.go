package worldrepo

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"zutto-pccom/apps/server/internal/world"
)

type demoRetrievedPost struct {
	post  world.Post
	score float64
}

type demoRenderContextStats struct {
	threadPosts     int
	threadBodies    int
	threadEnvelopes int
	relatedPosts    int
}

func (s demoRenderContextStats) String() string {
	return fmt.Sprintf("context=thread:%d(body:%d/envelope:%d),related:%d", s.threadPosts, s.threadBodies, s.threadEnvelopes, s.relatedPosts)
}

// materializationBBSRenderContext builds prompt context from canonical BBS data
// every time a body is rendered. It deliberately does not use provider-side chat
// history as world memory. Earlier materialized bodies are included verbatim-ish
// (bounded), while still-lazy posts contribute their committed free-form semantic
// envelope. A tiny related-post retrieval demonstrates the future RAG boundary
// without making a search index authoritative world state.
func (r *Repository) materializationBBSRenderContext(host world.Host, board world.Board, selected world.Post) (string, demoRenderContextStats) {
	all := r.Base.ListPosts(host.ID)
	postsByID := make(map[int64]world.Post, len(all))
	for _, post := range all {
		postsByID[post.ID] = post
	}
	rootID := threadRootID(postsByID, selected)

	thread := make([]world.Post, 0, 8)
	threadIDs := map[int64]bool{}
	for _, post := range all {
		if post.BoardID != board.ID || post.ID == selected.ID || !postBefore(post, selected) {
			continue
		}
		if threadRootID(postsByID, post) == rootID {
			thread = append(thread, post)
			threadIDs[post.ID] = true
		}
	}
	sort.SliceStable(thread, func(i, j int) bool {
		if thread[i].CreatedAt.Equal(thread[j].CreatedAt) {
			return thread[i].ID < thread[j].ID
		}
		return thread[i].CreatedAt.Before(thread[j].CreatedAt)
	})
	thread = boundedThreadContext(thread, 6)

	related := make([]demoRetrievedPost, 0, 4)
	for _, post := range all {
		if post.BoardID != board.ID || post.ID == selected.ID || threadIDs[post.ID] || !postBefore(post, selected) {
			continue
		}
		score := 0.0
		if selected.Intent.Topic != "" && strings.EqualFold(strings.TrimSpace(post.Intent.Topic), strings.TrimSpace(selected.Intent.Topic)) {
			score += 1.0
		}
		if normalizeDemoSubject(post.Subject) == normalizeDemoSubject(selected.Subject) {
			score += .35
		}
		age := selected.CreatedAt.Sub(post.CreatedAt)
		if age >= 0 && age < 30*24*time.Hour {
			score += .25 * (1 - age.Hours()/(30*24))
		}
		if score >= .70 {
			related = append(related, demoRetrievedPost{post: post, score: score})
		}
	}
	sort.SliceStable(related, func(i, j int) bool {
		if related[i].score == related[j].score {
			return related[i].post.CreatedAt.After(related[j].post.CreatedAt)
		}
		return related[i].score > related[j].score
	})
	if len(related) > 3 {
		related = related[:3]
	}

	stats := demoRenderContextStats{threadPosts: len(thread), relatedPosts: len(related)}
	var b strings.Builder
	b.WriteString("THREAD SO FAR — canonical BBS records this reply may naturally continue:\n")
	if len(thread) == 0 {
		b.WriteString("(no earlier post in this thread)\n")
	}
	for _, post := range thread {
		fmt.Fprintf(&b, "\n[MSG %04d %s %s] %s\n", post.ID, post.CreatedAt.Format("01/02 15:04"), post.Author, post.Subject)
		if strings.TrimSpace(post.Body) != "" {
			stats.threadBodies++
			b.WriteString("body:\n")
			b.WriteString(truncateDemoContext(strings.TrimSpace(post.Body), 420))
			b.WriteString("\n")
		} else {
			stats.threadEnvelopes++
			b.WriteString("semantic envelope (body not materialized yet):\n")
			b.WriteString(demoPostSemanticSummary(post))
			b.WriteString("\n")
		}
	}

	b.WriteString("\nRELATED EARLIER POSTS RETRIEVED FROM THIS BOARD:\n")
	b.WriteString("These are retrieval hints for duplicate-topic awareness, not proof that the actor personally read them. Do not refer to them as remembered/read unless the thread context supports that.\n")
	if len(related) == 0 {
		b.WriteString("(none retrieved)\n")
	}
	for _, candidate := range related {
		post := candidate.post
		fmt.Fprintf(&b, "- MSG %04d %s %s: %s", post.ID, post.CreatedAt.Format("01/02"), post.Author, post.Subject)
		if post.Intent.Topic != "" {
			fmt.Fprintf(&b, " — topic: %s", truncateDemoContext(post.Intent.Topic, 120))
		}
		if len(post.Intent.Claims) > 0 {
			fmt.Fprintf(&b, " — %s", truncateDemoContext(strings.Join(post.Intent.Claims, " / "), 180))
		}
		b.WriteString("\n")
	}
	return b.String(), stats
}

func postBefore(candidate, selected world.Post) bool {
	if candidate.CreatedAt.Before(selected.CreatedAt) {
		return true
	}
	return candidate.CreatedAt.Equal(selected.CreatedAt) && candidate.ID < selected.ID
}

func threadRootID(postsByID map[int64]world.Post, post world.Post) int64 {
	rootID := post.ID
	current := post
	seen := map[int64]bool{post.ID: true}
	for {
		targetID := world.ResponseTargetID(current)
		if targetID == 0 {
			return rootID
		}
		rootID = targetID
		if seen[rootID] {
			return rootID
		}
		parent, ok := postsByID[rootID]
		if !ok {
			return rootID
		}
		seen[rootID] = true
		current = parent
	}
}

func boundedThreadContext(posts []world.Post, limit int) []world.Post {
	if limit <= 0 || len(posts) <= limit {
		return posts
	}
	out := make([]world.Post, 0, limit)
	out = append(out, posts[0])
	start := len(posts) - (limit - 1)
	if start < 1 {
		start = 1
	}
	out = append(out, posts[start:]...)
	return out
}

func demoPostSemanticSummary(post world.Post) string {
	parts := make([]string, 0, 6)
	if post.Intent.Topic != "" {
		parts = append(parts, "topic="+post.Intent.Topic)
	}
	if post.Intent.Goal != "" {
		parts = append(parts, "goal="+post.Intent.Goal)
	}
	if len(post.Intent.Claims) > 0 {
		parts = append(parts, "claims="+strings.Join(post.Intent.Claims, " / "))
	}
	if post.Intent.RespondsToPostID != 0 {
		parts = append(parts, fmt.Sprintf("responds_to_msg=%04d", post.Intent.RespondsToPostID))
	}
	if len(post.Intent.RespondsToClaims) > 0 {
		parts = append(parts, "reacts_to="+strings.Join(post.Intent.RespondsToClaims, " / "))
	}
	if len(parts) == 0 {
		return "(no additional semantic detail)"
	}
	return strings.Join(parts, "; ")
}

func normalizeDemoSubject(subject string) string {
	s := strings.TrimSpace(subject)
	for strings.HasPrefix(strings.ToLower(s), "re:") {
		s = strings.TrimSpace(s[3:])
	}
	return s
}

func truncateDemoContext(value string, maxRunes int) string {
	value = strings.ReplaceAll(value, "\r\n", "\n")
	value = strings.ReplaceAll(value, "\r", "\n")
	runes := []rune(value)
	if len(runes) <= maxRunes {
		return value
	}
	return string(runes[:maxRunes]) + "…"
}
