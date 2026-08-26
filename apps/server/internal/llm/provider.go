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

type Provider interface {
	GenerateReply(context.Context, ReplyRequest) (string, error)
}

type TemplateProvider struct{}

func (TemplateProvider) GenerateReply(_ context.Context, req ReplyRequest) (string, error) {
	return "ども、NEKOです(^^)\r\n\r\n読ませてもらいました。\r\nまた何かあったら書いてくださいね。\r\n", nil
}
