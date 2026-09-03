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

type Provider interface {
	GenerateReply(context.Context, ReplyRequest) (string, error)
}

type BoardPostRenderer interface {
	GenerateBoardPost(context.Context, BoardPostRequest) (BoardPostDraft, error)
}

type TemplateProvider struct{}

func (TemplateProvider) GenerateReply(_ context.Context, req ReplyRequest) (string, error) {
	return "ども、NEKOです(^^)\r\n\r\n読ませてもらいました。\r\nまた何かあったら書いてくださいね。\r\n", nil
}

func (TemplateProvider) GenerateBoardPost(_ context.Context, req BoardPostRequest) (BoardPostDraft, error) {
	author := "NEKO"
	if req.AuthorHandle != "" {
		author = req.AuthorHandle
	}
	subject := req.BoardTopic + "の話"
	if req.CanonicalSubject != "" {
		subject = req.CanonicalSubject
	}
	return BoardPostDraft{Author: author, Subject: subject, Body: "ども、" + author + "です(^^)\r\n\r\nこのへんの話、みなさんどうです？\r\n"}, nil
}
