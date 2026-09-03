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

// BBSIntentEvent is a WorldEngine-decided event shell. The planner is not allowed
// to change actor, timestamp, or root/reply topology; it only proposes semantic
// content for that already-selected event.
type BBSIntentEvent struct {
	Index            int    `json:"index"`
	AuthorHandle     string `json:"author_handle"`
	CreatedAt        string `json:"created_at"`
	Action           string `json:"action"`
	ParentEventIndex int    `json:"parent_event_index,omitempty"`
	CanonicalSubject string `json:"canonical_subject,omitempty"`
	PersonaProfile   string `json:"persona_profile"`
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

type Provider interface {
	GenerateReply(context.Context, ReplyRequest) (string, error)
}

type BoardPostRenderer interface {
	GenerateBoardPost(context.Context, BoardPostRequest) (BoardPostDraft, error)
}

// BBSTimelineIntentPlanner proposes free-form semantic content for event shells
// selected by the world layer. There is intentionally no fixed topic catalog,
// information-slot enum, subject bank, or canned response-act list here.
type BBSTimelineIntentPlanner interface {
	GenerateBBSTimelineIntent(context.Context, BBSTimelineIntentRequest) (BBSTimelineIntentDraft, error)
}
