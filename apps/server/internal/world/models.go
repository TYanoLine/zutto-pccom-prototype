package world

import "time"

type Host struct {
	ID             string  `json:"id"`
	Phone          string  `json:"phone"`
	Name           string  `json:"name"`
	Region         string  `json:"region"`
	Software       string  `json:"software"`
	SoftwareID     string  `json:"software_id,omitempty"`
	Lines          int     `json:"lines"`
	Popularity     float64 `json:"popularity"`
	MaxBaud        int     `json:"max_baud"`
	Members        int     `json:"members"`
	ANSI           bool    `json:"ansi"`
	GuestAllowed   bool    `json:"guest_allowed"`
	TelehoFriendly bool    `json:"teleho_friendly"`
}

type Board struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// PostIntent stores only semantic world state that is specific to the actual
// post. It deliberately has no topic-slot catalog, response-act enum, follow-up
// checklist, or other fixed conversation template. Topic and Goal are free-form
// semantic summaries produced for this concrete event and then committed.
type PostIntent struct {
	Action           string   `json:"action,omitempty"`
	Topic            string   `json:"topic,omitempty"`
	Motivation       string   `json:"motivation,omitempty"`
	Stance           string   `json:"stance,omitempty"`
	Goal             string   `json:"goal,omitempty"`
	Claims           []string `json:"claims,omitempty"`
	RespondsToClaims []string `json:"responds_to_claims,omitempty"`
	RespondsToPostID int64    `json:"responds_to_post_id,omitempty"`

	// RenderContext is transient input assembled from canonical BBS data immediately
	// before prose rendering. It is never canonical world state and must not be
	// persisted/serialized. The PoC uses it for thread history + small related-post
	// retrieval while keeping the database as the source of truth.
	RenderContext string `json:"-"`
}

type Post struct {
	ID              int64      `json:"id"`
	BoardID         string     `json:"board_id,omitempty"`
	ParentID        int64      `json:"parent_id,omitempty"`
	Author          string     `json:"author"`
	AuthorPersonaID string     `json:"author_persona_id,omitempty"`
	Subject         string     `json:"subject"`
	Intent          PostIntent `json:"intent,omitempty"`
	Body            string     `json:"body"`
	CreatedAt       time.Time  `json:"created_at"`
}

type Persona struct {
	ID                  string
	Handle              string
	Age                 int
	Gender              string
	Occupation          string
	ActivityPattern     string
	ReplyTendency       float64
	ThreadStartTendency float64
	LurkerTendency      float64
	NewcomerOpenness    float64
	Argumentativeness   float64
	WritingStyle        string
	Interests           map[string]float64
	Opinions            map[string]float64
}

// PersonaFact is a concrete fictional-world fact that did not need to exist in
// detail when the persona skeleton was first created. Key is an open semantic
// key, not a member of a fixed slot catalog. Once stored, later planning must
// reuse the same key/value instead of improvising a contradiction.
type PersonaFact struct {
	PersonaID      string    `json:"persona_id"`
	Key            string    `json:"key"`
	Topic          string    `json:"topic,omitempty"`
	Value          string    `json:"value"`
	MaterializedAt time.Time `json:"materialized_at"`
	SourceKind     string    `json:"source_kind,omitempty"`
}
