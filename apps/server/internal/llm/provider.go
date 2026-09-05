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
	Index             int      `json:"index"`
	AuthorHandle      string   `json:"author_handle"`
	CreatedAt         string   `json:"created_at"`
	Action            string   `json:"action"`
	ParentEventIndex  int      `json:"parent_event_index,omitempty"`
	SourceEventIndex  int      `json:"source_event_index,omitempty"`
	AnchorKey         string   `json:"anchor_key"`
	CauseKind         string   `json:"cause_kind"`
	CauseSummary      string   `json:"cause_summary"`
	CanonicalSubject string   `json:"canonical_subject,omitempty"`
	PersonaProfile    string   `json:"persona_profile"`
	ExistingFacts     []string `json:"existing_facts,omitempty"`
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

type Provider interface {
	GenerateReply(context.Context, ReplyRequest) (string, error)
}

type BoardPostRenderer interface {
	GenerateBoardPost(context.Context, BoardPostRequest) (BoardPostDraft, error)
}

// BBSTimelineIntentPlanner realizes free-form semantic wording for causal event
// shells selected by the world layer. It does not select whether anyone writes,
// which board they visit, root-vs-reply, or the posting anchor. There is also no
// fixed prose/subject template bank or response-act menu here.
type BBSTimelineIntentPlanner interface {
	GenerateBBSTimelineIntent(context.Context, BBSTimelineIntentRequest) (BBSTimelineIntentDraft, error)
}
