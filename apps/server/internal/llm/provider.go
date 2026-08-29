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

type BoardPostRequest struct {
	HostName       string
	HostRegion     string
	HostSoftware   string
	BoardID        string
	BoardTopic     string
	WorldDate      string
	HistoricalFacts []string
	EraRules       string
}

type BoardPostDraft struct {
	Author  string `json:"author"`
	Subject string `json:"subject"`
	Body    string `json:"body"`
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
	return BoardPostDraft{Author:"NEKO",Subject:req.BoardTopic+"の話",Body:"ども、NEKOです(^^)\r\n\r\nこのへんの話、みなさんどうです？\r\n"},nil
}
