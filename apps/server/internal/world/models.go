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
	FoundedOn      string  `json:"founded_on,omitempty"`
	ANSI           bool    `json:"ansi"`
	GuestAllowed   bool    `json:"guest_allowed"`
	TelehoFriendly bool    `json:"teleho_friendly"`
}

type Board struct {
	ID              string  `json:"id"`
	Name            string  `json:"name"`
	SemanticScope   string  `json:"semantic_scope,omitempty"`
	ActivityWeight  float64 `json:"activity_weight,omitempty"`
	ReplyRate       float64 `json:"reply_rate,omitempty"`
	RetainedRootCap int     `json:"retained_root_cap,omitempty"`
	OpenedOn        string  `json:"opened_on,omitempty"`
	// VerifiedReferentRate is world/station content tuning, not a historical
	// host-program default. When positive, title planning keeps this approximate
	// share of roots available for historically verified named referents while
	// retaining enough claim-free candidates to complete the board safely.
	VerifiedReferentRate float64 `json:"verified_referent_rate,omitempty"`
}

// BoardActivityState is the cheap world-layer existence plan for one board.
// It is deliberately prose-free: counts/timestamps may exist before any title
// or article body has been materialized by an LLM.
type BoardActivityState struct {
	BoardID         string    `json:"board_id"`
	AsOfDate        string    `json:"as_of_date"`
	OpenedOn        string    `json:"opened_on"`
	CurrentMembers  int       `json:"current_members"`
	InitialMembers  int       `json:"initial_members"`
	AverageMembers  float64   `json:"average_members"`
	GrowthExponent  float64   `json:"growth_exponent"`
	TotalRoots      int       `json:"total_roots"`
	TotalReplies    int       `json:"total_replies"`
	RetainedRoots   int       `json:"retained_roots"`
	RetainedReplies int       `json:"retained_replies"`
	RetainedSince   time.Time `json:"retained_since"`
	LastPostAt      time.Time `json:"last_post_at"`
	ActivityWeight  float64   `json:"activity_weight"`
	ReplyRate       float64   `json:"reply_rate"`
	SimulationBasis string    `json:"simulation_basis"`
}

// PostIntent stores canonical semantic state for an actual post. Action,
// AnchorKey, CauseKind and DiscourseMode are selected by the world layer before LLM semantic
// realization. Topic/Motivation/Stance/Goal are human-readable realization of
// that fixed cause, not an invitation for the LLM to choose what happens.
//
// Situation* fields are a sparse world-owned micro-situation selected only after
// a write action exists. They fix enough of the occurrence to keep independent
// roots distinct and answerable without simulating every person's life at full
// resolution. SituationFacts are open boundary/fact strings rather than a global
// fixed event schema, so future host/domain logic can extend them compositionally.
//
// Producer* fields are the canonical article brief created by the host-window
// semantic producer after world action selection and before article prose is
// rendered. They let the later per-article worker write freely at the wording
// level without inventing a new event, referent, owner, backstory or purpose.
//
// PersonaFact is deliberately separate: a persistent fact is background for
// consistency and never becomes a posting trigger merely because it exists.
type PostIntent struct {
	Action        string `json:"action,omitempty"`
	AnchorKey     string `json:"anchor_key,omitempty"`
	CauseKind     string `json:"cause_kind,omitempty"`
	DiscourseMode string `json:"discourse_mode,omitempty"`
	SourcePostID  int64  `json:"source_post_id,omitempty"`

	SituationKind    string   `json:"situation_kind,omitempty"`
	SituationSummary string   `json:"situation_summary,omitempty"`
	SituationFacts   []string `json:"situation_facts,omitempty"`
	// ArticleDetailsMaterialized distinguishes a successful empty detail set from
	// an article whose detail pass has not completed yet.
	ArticleDetailsMaterialized bool `json:"article_details_materialized,omitempty"`

	Topic            string   `json:"topic,omitempty"`
	Motivation       string   `json:"motivation,omitempty"`
	Stance           string   `json:"stance,omitempty"`
	Goal             string   `json:"goal,omitempty"`
	Claims           []string `json:"claims,omitempty"`
	RespondsToClaims []string `json:"responds_to_claims,omitempty"`
	RespondsToPostID int64    `json:"responds_to_post_id,omitempty"`

	ProducerEventID         string   `json:"producer_event_id,omitempty"`
	ProducerEpisode         string   `json:"producer_episode,omitempty"`
	ProducerReferents       []string `json:"producer_referents,omitempty"`
	ProducerActorKnowledge  []string `json:"producer_actor_knowledge,omitempty"`
	ProducerAudienceContext []string `json:"producer_audience_context,omitempty"`
	ProducerContribution    []string `json:"producer_contribution,omitempty"`
	ProducerMustNot         []string `json:"producer_must_not,omitempty"`

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

	// EverydayContext describes ordinary, already-established baseline conditions
	// in this person's life. These facts are primarily contradiction/interpretation
	// context and are normally left unspoken. They are deliberately separate from
	// Interests so an ordinary machine, service, membership, commute or habit does
	// not become a posting topic merely because it is part of the person's life.
	EverydayContext []string

	Interests map[string]float64
	Opinions  map[string]float64
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
