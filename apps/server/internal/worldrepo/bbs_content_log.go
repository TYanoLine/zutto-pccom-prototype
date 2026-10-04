package worldrepo

import (
	"encoding/json"
	"log"
	"time"

	"zutto-pccom/apps/server/internal/bbsengine"
	"zutto-pccom/apps/server/internal/world"
)

// generatedContentLog is operational telemetry, never a world fact.
// Keep one JSON record per committed article so Render search results can
// correlate a header and its lazily materialized body by host/board/post.
type generatedContentLog struct {
	Stage            string   `json:"stage"`
	Host             string   `json:"host"`
	Board            string   `json:"board"`
	PostID           int64    `json:"post_id"`
	ParentID         int64    `json:"parent_id,omitempty"`
	RespondsToPostID int64    `json:"responds_to_post_id,omitempty"`
	Author           string   `json:"author"`
	CreatedAt        string   `json:"created_at"`
	Subject          string   `json:"subject"`
	SituationKind    string   `json:"situation_kind,omitempty"`
	SituationSummary string   `json:"situation_summary,omitempty"`
	SituationFacts   []string `json:"situation_facts,omitempty"`
	Body             string   `json:"body,omitempty"`
}

// SetDebugLogGeneratedContent is the process-wide switch for the generated
// content log. A host is logged only when this is on and the host opted in with
// debug.content_log.
func (r *Repository) SetDebugLogGeneratedContent(enabled bool) {
	if r != nil {
		r.debugLogGeneratedContent = enabled
	}
}

func (r *Repository) shouldLogGeneratedContent(host world.Host) bool {
	return r != nil && r.debugLogGeneratedContent && host.Debug.ContentLog
}

// Called only after an observation has returned. Read the store again instead
// of logging provisional LLM drafts; also capture any posts actually saved
// before a later event in the batch failed.
func (r *Repository) logNewBBSHeaders(host world.Host, board world.Board, before []world.Post) {
	if !r.shouldLogGeneratedContent(host) {
		return
	}
	seen := make(map[int64]bool, len(before))
	for _, post := range before {
		seen[post.ID] = true
	}
	for _, post := range filterBoard(r.Base.ListPosts(host.ID), board.ID) {
		if !seen[post.ID] {
			r.logBBSGeneratedContent("header_committed", host, board, post)
		}
	}
}

func (r *Repository) logBBSGeneratedContent(stage string, host world.Host, board world.Board, post world.Post) {
	if !r.shouldLogGeneratedContent(host) || post.Intent.Action != bbsengine.ActionWorldCatchup {
		return // Never log human posts or content from hosts that did not opt in.
	}
	entry := generatedContentLog{
		Stage: stage, Host: host.ID, Board: board.ID, PostID: post.ID,
		ParentID: post.ParentID, RespondsToPostID: world.ResponseTargetID(post),
		Author: post.Author, CreatedAt: post.CreatedAt.Format(time.RFC3339),
		Subject: post.Subject,
	}
	switch stage {
	case "header_committed":
		entry.SituationKind = post.Intent.SituationKind
		entry.SituationSummary = post.Intent.SituationSummary
		entry.SituationFacts = append([]string(nil), post.Intent.SituationFacts...)
	case "body_committed":
		if post.Body == "" {
			return
		}
		entry.Body = post.Body
	default:
		return
	}
	encoded, err := json.Marshal(entry)
	if err != nil {
		log.Printf("BBS generated content audit encoding failed: host=%s board=%s post=%d err=%v", host.ID, board.ID, post.ID, err)
		return
	}
	log.Printf("BBS generated content: %s", encoded)
}
