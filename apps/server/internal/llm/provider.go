package llm

import "context"

type ReplyRequest struct {
	HostName  string
	Persona   string
	WorldDate string
	Subject   string
	Body      string
	EraRules  string
}

type TokenUsage struct {
	InputTokens       int
	CachedInputTokens int
	OutputTokens      int
	ReasoningTokens   int
	TotalTokens       int
	Model             string
}

type BoardPostRequest struct {
	HostName         string
	HostRegion       string
	HostSoftware     string
	BoardID          string
	BoardTopic       string
	WorldDate        string
	HistoricalFacts  []string
	EraRules         string
	AuthorHandle     string
	PersonaProfile   string
	PostIntent       string
	CanonicalSubject string
}

type BoardPostDraft struct {
	Author  string     `json:"author"`
	Subject string     `json:"subject"`
	Body    string     `json:"body"`
	Usage   TokenUsage `json:"-"`
}

// BBSIntentEvent is a world-selected causal event shell. The semantic planner is
// not allowed to change actor, timestamp, root/reply topology, source event,
// anchor key, or cause kind. It only realizes human-readable semantics and the
// exact subject text for an event that already has a reason to exist.
type BBSIntentEvent struct {
	Index            int      `json:"index"`
	AuthorHandle     string   `json:"author_handle"`
	CreatedAt        string   `json:"created_at"`
	Action           string   `json:"action"`
	ParentEventIndex int      `json:"parent_event_index,omitempty"`
	SourceEventIndex int      `json:"source_event_index,omitempty"`
	AnchorKey        string   `json:"anchor_key"`
	CauseKind        string   `json:"cause_kind"`
	CauseSummary     string   `json:"cause_summary"`
	DiscourseMode    string   `json:"discourse_mode,omitempty"`
	CanonicalSubject string   `json:"canonical_subject,omitempty"`
	PersonaProfile   string   `json:"persona_profile"`
	ExistingFacts    []string `json:"existing_facts,omitempty"`
}

type BBSTimelineIntentRequest struct {
	HostName       string
	HostRegion     string
	HostSoftware   string
	BoardID        string
	BoardName      string
	WorldDate      string
	EraRules       string
	RecentBBSState string
	Events         []BBSIntentEvent
}

type BBSIntentFactDraft struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type BBSIntentDraft struct {
	Index      int                  `json:"index"`
	Subject    string               `json:"subject"`
	Topic      string               `json:"topic"`
	Motivation string               `json:"motivation"`
	Stance     string               `json:"stance"`
	Goal       string               `json:"goal"`
	Facts      []BBSIntentFactDraft `json:"facts"`
}

type BBSTimelineIntentDraft struct {
	Events []BBSIntentDraft `json:"events"`
	Usage  TokenUsage       `json:"-"`
}

// BBSWorldWindowEvent is the producer-facing view of one already-selected world
// action. EventID is unique across boards for the materialized time window. The
// producer may add semantic specificity and editorial instructions, but it may
// not change whether the event exists, its board, actor, time or thread topology.
type BBSWorldWindowEvent struct {
	EventID        string   `json:"event_id"`
	BoardID        string   `json:"board_id"`
	BoardName      string   `json:"board_name"`
	AuthorHandle   string   `json:"author_handle"`
	CreatedAt      string   `json:"created_at"`
	Action         string   `json:"action"`
	ParentEventID  string   `json:"parent_event_id,omitempty"`
	SourceEventID  string   `json:"source_event_id,omitempty"`
	AnchorKey      string   `json:"anchor_key"`
	CauseKind      string   `json:"cause_kind"`
	CauseSummary   string   `json:"cause_summary"`
	DiscourseMode  string   `json:"discourse_mode,omitempty"`
	PersonaProfile string   `json:"persona_profile"`
	ExistingFacts  []string `json:"existing_facts,omitempty"`
}

type BBSWorldWindowProductionRequest struct {
	HostName       string
	HostRegion     string
	HostSoftware   string
	WorldDate      string
	WindowStart    string
	WindowEnd      string
	EraRules       string
	RecentBBSState string
	Events         []BBSWorldWindowEvent
}

// BBSArticleBriefDraft is the producer's canonical semantic/editorial brief for
// one article. It is intentionally richer than prose intent: the later article
// renderer is a worker that must follow this brief rather than inventing a new
// event or backstory.
type BBSArticleBriefDraft struct {
	EventID         string               `json:"event_id"`
	Subject         string               `json:"subject"`
	Episode         string               `json:"episode"`
	Referents       []string             `json:"referents"`
	ActorKnowledge  []string             `json:"actor_knowledge"`
	AudienceContext []string             `json:"audience_context"`
	Contribution    []string             `json:"contribution"`
	MustNot         []string             `json:"must_not"`
	Topic           string               `json:"topic"`
	Motivation      string               `json:"motivation"`
	Stance          string               `json:"stance"`
	Goal            string               `json:"goal"`
	Facts           []BBSIntentFactDraft `json:"facts"`
}

type BBSWorldWindowProductionDraft struct {
	Briefs []BBSArticleBriefDraft `json:"briefs"`
	Usage  TokenUsage             `json:"-"`
}

// BBSWorldSituationProposalRequest asks for small canonical world-situation
// proposals for a bounded set of already-selected standalone root slots. Unlike
// BBSWorldWindowProduction, this pass does not plan article prose/editorial
// briefs; it only proposes what concretely happened before prose is rendered.
type BBSWorldSituationProposalRequest struct {
	HostName                   string
	HostRegion                 string
	HostSoftware               string
	WorldDate                  string
	WindowStart                string
	WindowEnd                  string
	EraRules                   string
	HistoricalFacts            []string
	AllowModelHistoricalMemory bool
	RecentBBSState             string
	Events                     []BBSWorldWindowEvent
	AvoidSituations            []string
}

type BBSWorldSituationDraft struct {
	EventID          string   `json:"event_id"`
	ObjectClass      string   `json:"object_class"`
	ChangeClass      string   `json:"change_class"`
	Occurrence       string   `json:"occurrence"`
	ActorObservation string   `json:"actor_observation"`
	Impact           string   `json:"impact"`
	Uncertainty      string   `json:"uncertainty"`
	NoveltyKey       string   `json:"novelty_key"`
	MustNot          []string `json:"must_not"`
}

type BBSWorldSituationProposalDraft struct {
	Situations []BBSWorldSituationDraft `json:"situations"`
	Usage      TokenUsage               `json:"-"`
}

// BBSWorldSituationProposer sees multiple independent roots at once so it can
// propose diverse concrete world facts in one call. The world layer validates
// and commits accepted proposals before any article worker writes prose.
type BBSWorldSituationProposer interface {
	GenerateBBSWorldSituationProposals(context.Context, BBSWorldSituationProposalRequest) (BBSWorldSituationProposalDraft, error)
}

type Provider interface {
	GenerateReply(context.Context, ReplyRequest) (string, error)
}

type BoardPostRenderer interface {
	GenerateBoardPost(context.Context, BoardPostRequest) (BoardPostDraft, error)
}

// BBSTimelineIntentPlanner is retained as a compatibility path for tests and
// older development callers. Production materialization prefers the world-window
// producer below so cross-board/persona consistency is planned before articles
// are rendered.
type BBSTimelineIntentPlanner interface {
	GenerateBBSTimelineIntent(context.Context, BBSTimelineIntentRequest) (BBSTimelineIntentDraft, error)
}

// BBSWorldWindowProducer receives the whole bounded materialization window and
// issues detailed briefs to downstream article workers. World-selected action
// topology remains immutable; the producer coordinates semantic consistency.
type BBSWorldWindowProducer interface {
	GenerateBBSWorldWindowProduction(context.Context, BBSWorldWindowProductionRequest) (BBSWorldWindowProductionDraft, error)
}
