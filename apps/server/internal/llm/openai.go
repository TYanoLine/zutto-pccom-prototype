package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// OpenAIProvider intentionally uses net/http so the starter remains decoupled
// from SDK release cadence. Codex can replace this with openai-go/v3 later.
type OpenAIProvider struct {
	APIKey string
	Model  string
	Client *http.Client
}

func (p OpenAIProvider) GenerateReply(ctx context.Context, req ReplyRequest) (string, error) {
	prompt := fmt.Sprintf(`You are writing one Japanese grass-roots BBS post as the specified persona.
World date: %s. Never use knowledge, products, slang, or events after this date.
The human-controlled member is not special. Do not flatter them. It is acceptable to disagree, be terse, or have little to say.
Do not mention AI, simulation, prompts, or modern social media.
Use a plausible 1996 Japanese BBS writing style, but do not overuse emoticons.
Host: %s
Persona: %s
Era rules: %s
Incoming subject: %s
Incoming body:
%s

Return only the post body.`, req.WorldDate, req.HostName, req.Persona, req.EraRules, req.Subject, req.Body)
	text, err := p.responseText(ctx,prompt,"low")
	if err != nil { return "",err }
	return normalizeCRLF(text),nil
}

func (p OpenAIProvider) GenerateBoardPost(ctx context.Context, req BoardPostRequest) (BoardPostDraft, error) {
	facts := "(none supplied; keep concrete historical claims generic)"
	if len(req.HistoricalFacts)>0 { facts = "- "+strings.Join(req.HistoricalFacts,"\n- ") }
	prompt := fmt.Sprintf(`Write exactly one natural message for a Japanese grass-roots personal-computer BBS.
The message becomes persistent world state, so obey every constraint below.

PRIMARY CONTENT CUE:
- Current board/header cue: %s
- Write about what an ordinary member would naturally mean by this cue.
- Do not pad the message with unrelated setting details merely because they are listed below.

BACKGROUND CONSTRAINTS — THESE ARE GUARDRAILS, NOT TOPICS TO MENTION:
- World date: %s
- Host: %s
- Region: %s
- Host software family: %s
- Board ID: %s
- Era rules: %s

HISTORICAL FACTS ALLOWED AS CONCRETE FACTUAL SUPPORT:
%s

Important interpretation rules:
- Background constraints exist to prevent contradictions. They are NOT a checklist of details to mention.
- Do not mention the region, date, season, host name, host software, or period technology unless the actual message content naturally requires it.
- Historical facts are permission/constraints for concrete claims, not suggested talking points. Omit them entirely when irrelevant.
- Never add period props such as floppy disks, magazines, modems, heat/weather, or place names just to make the prose feel "1990s".
- Natural topic focus is more important than demonstrating that you understood the supplied context.

Rules:
- Write as an ordinary independent BBS member, not as an assistant or narrator.
- The human-controlled user is not the center of the world and need not be mentioned.
- Never mention AI, simulation, prompts, web searches, databases, social media, smartphones, or anything from after the world date.
- Do not invent exact release dates, prices, model-specific availability, technical specifications, historical events, or other concrete factual claims unless they are supported by the supplied historical facts.
- When no historical facts are supplied, ordinary personal chatter, impressions, questions, habits, and mundane details are fine.
- Do not imply that all members share the same opinion or equipment.
- Use plausible mid-1990s Japanese BBS prose, but avoid conspicuous era cosplay. Emoticons are optional and should not be overused.
- Author handle: 2-12 ASCII letters/digits only.
- Subject: Japanese, at most 36 characters.
- Body: Japanese, 1-5 short paragraphs, at most about 500 Japanese characters.

Return ONLY JSON with exactly these keys:
{"author":"...","subject":"...","body":"..."}`,req.BoardTopic,req.WorldDate,req.HostName,req.HostRegion,req.HostSoftware,req.BoardID,req.EraRules,facts)
	text,err:=p.responseText(ctx,prompt,"low")
	if err!=nil{return BoardPostDraft{},err}
	var draft BoardPostDraft
	if err:=json.Unmarshal([]byte(strings.TrimSpace(text)),&draft);err!=nil{return BoardPostDraft{},fmt.Errorf("decode board post JSON: %w",err)}
	if err:=validateBoardPostDraft(draft);err!=nil{return BoardPostDraft{},err}
	draft.Author=strings.ToUpper(strings.TrimSpace(draft.Author))
	draft.Subject=strings.TrimSpace(draft.Subject)
	draft.Body=normalizeCRLF(draft.Body)
	return draft,nil
}

func (p OpenAIProvider) responseText(ctx context.Context,prompt,verbosity string)(string,error){
	if p.APIKey==""{return "",errors.New("OPENAI_API_KEY is not set")}
	client:=p.Client;if client==nil{client=&http.Client{Timeout:30*time.Second}}
	payload:=map[string]any{"model":p.Model,"input":prompt,"text":map[string]any{"verbosity":verbosity},"max_output_tokens":1200}
	body,_:=json.Marshal(payload)
	httpReq,err:=http.NewRequestWithContext(ctx,http.MethodPost,"https://api.openai.com/v1/responses",bytes.NewReader(body));if err!=nil{return "",err}
	httpReq.Header.Set("Authorization","Bearer "+p.APIKey);httpReq.Header.Set("Content-Type","application/json")
	resp,err:=client.Do(httpReq);if err!=nil{return "",err};defer resp.Body.Close()
	if resp.StatusCode<200||resp.StatusCode>=300{return "",fmt.Errorf("openai responses API returned %s",resp.Status)}
	var decoded struct{Output []struct{Content []struct{Type string `json:"type"`;Text string `json:"text"`} `json:"content"`} `json:"output"`}
	if err:=json.NewDecoder(resp.Body).Decode(&decoded);err!=nil{return "",err}
	for _,out:=range decoded.Output{for _,c:=range out.Content{if c.Type=="output_text"&&strings.TrimSpace(c.Text)!=""{return c.Text,nil}}}
	return "",errors.New("no output_text in OpenAI response")
}

func validateBoardPostDraft(d BoardPostDraft)error{
	a:=strings.TrimSpace(d.Author);s:=strings.TrimSpace(d.Subject);b:=strings.TrimSpace(d.Body)
	if len(a)<2||len(a)>12{return errors.New("board post author length is invalid")}
	for _,r:=range a{if !(r>='A'&&r<='Z'||r>='a'&&r<='z'||r>='0'&&r<='9'){return errors.New("board post author must be ASCII alphanumeric")}}
	if s==""||len([]rune(s))>36{return errors.New("board post subject is empty or too long")}
	if b==""||len([]rune(b))>700{return errors.New("board post body is empty or too long")}
	lower:=strings.ToLower(b+" "+s)
	for _,forbidden:=range []string{"chatgpt","openai","twitter","facebook","instagram","スマホ","生成ai","生成ＡＩ"}{if strings.Contains(lower,strings.ToLower(forbidden)){return fmt.Errorf("board post contains forbidden future/meta term %q",forbidden)}}
	return nil
}

func normalizeCRLF(s string) string {
	s=strings.ReplaceAll(s,"\r\n","\n");s=strings.ReplaceAll(s,"\r","\n")
	return strings.ReplaceAll(strings.TrimSpace(s),"\n","\r\n")+"\r\n"
}
