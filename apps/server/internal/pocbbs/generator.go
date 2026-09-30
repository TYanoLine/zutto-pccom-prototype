package pocbbs

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"zutto-pccom/apps/server/internal/azureopenai"
	"zutto-pccom/apps/server/internal/historicalkb"
)

type Generator struct {
	Endpoint string
	APIKey string
	Model string
	WorldDate string
	History historicalkb.Service
	Client *http.Client
}

type Article struct {
	ID string `json:"id"`
	Title string `json:"title"`
	Handle string `json:"handle"`
	PostedAt string `json:"postedAt"`
	Preview string `json:"preview"`
	Evidence string `json:"evidence"`
	Body string `json:"body,omitempty"`
	ResearchCaseID string `json:"researchCaseId,omitempty"`
	ResearchStatus string `json:"researchStatus,omitempty"`
	Confidence float64 `json:"confidence,omitempty"`
	Sources []historicalkb.SourceEvidence `json:"sources,omitempty"`
}

type Board struct {
	ID string `json:"id"`
	Title string `json:"title"`
	Description string `json:"description"`
	Articles []Article `json:"articles"`
}

type Snapshot struct {
	HostName string `json:"hostName"`
	Welcome string `json:"welcome"`
	Notice string `json:"notice"`
	WorldDate string `json:"worldDate"`
	Boards []Board `json:"boards"`
}

func (g Generator) Snapshot(ctx context.Context) (Snapshot, error) {
	if g.APIKey == "" { return fallbackSnapshot(g.WorldDate), nil }
	prompt := fmt.Sprintf(`You are producing atmospheric content for a Japanese grassroots PC-communication BBS in %s.
This is NOT a research task. Do not use web search. Use broad model knowledge only and avoid brittle exact claims such as exact release dates, prices, market shares, or product availability.
Create a small, natural mid-1990s Japanese BBS snapshot. Keep the tone ordinary rather than nostalgic parody.

Return ONLY JSON with this exact shape:
{"hostName":"...","welcome":"...","notice":"...","boards":[
 {"id":"general","title":"...","description":"...","articles":[
   {"id":"general-1","title":"...","handle":"...","postedAt":"08/25 22:14","preview":"...","evidence":"atmospheric"},
   {"id":"general-2","title":"...","handle":"...","postedAt":"08/26 00:41","preview":"...","evidence":"atmospheric"}
 ]},
 {"id":"pc","title":"...","description":"...","articles":[
   {"id":"modem-288","title":"モデム、みんなどのくらい？","handle":"...","postedAt":"08/26 01:08","preview":"28.8Kとか33.6Kとか最近よく聞くけど…","evidence":"verified"},
   {"id":"pc-2","title":"...","handle":"...","postedAt":"08/26 02:03","preview":"...","evidence":"atmospheric"}
 ]},
 {"id":"chat","title":"...","description":"...","articles":[
   {"id":"chat-1","title":"...","handle":"...","postedAt":"08/26 00:12","preview":"...","evidence":"atmospheric"}
 ]}
]}
Do not change IDs or evidence values.`, g.WorldDate)
	var snap Snapshot
	if err := g.respondJSON(ctx,prompt,&snap); err != nil { return fallbackSnapshot(g.WorldDate), nil }
	snap.WorldDate=g.WorldDate
	return snap,nil
}

func (g Generator) ArticleDetail(ctx context.Context, articleID string) (Article, error) {
	if articleID == "modem-288" {
		if g.History.Store == nil { return Article{}, historicalkb.ErrNotConfigured }
		question := "1996年8月時点の日本の個人向けパソコン通信利用者にとって、14.4kbps・28.8kbps・33.6kbps級モデムはそれぞれどのような位置づけだったか。発売済みか、一般的か、先端的かを区別し、断定できない点も示す。"
		caseItem, err := g.findOrResearch(ctx,"PoC: 1996年8月のモデム速度事情",question)
		if err != nil { return Article{}, err }
		body := "ども、TAKAです。\n\n最近28.8Kとか33.6Kとか聞くようになったけど、実際みんな何使ってます？ うちはまだ14.4K。夜中につないでると、これでもまあ困らないような、でも速いのを見ると欲しくなるような(^^;\n\n---- 考証メモを反映した背景 ----\n"+caseItem.ProvisionalAnswer+"\n\nというわけで、実際の使用感とか相性の話あったら教えてください。"
		return Article{ID:articleID,Title:"モデム、みんなどのくらい？",Handle:"TAKA",PostedAt:"08/26 01:08",Preview:"28.8Kとか33.6Kとか最近よく聞くけど…",Evidence:"verified",Body:body,ResearchCaseID:caseItem.ID,ResearchStatus:string(caseItem.Status),Confidence:caseItem.Confidence,Sources:caseItem.Sources},nil
	}

	if g.APIKey == "" { return fallbackArticle(articleID), nil }
	prompt := fmt.Sprintf(`Write one short Japanese grassroots BBS post dated around %s. This is atmospheric generation only: do NOT web-search and do NOT make exact claims about release dates, prices, market shares, or historically sensitive facts. Article id: %s. Return ONLY JSON: {"title":"...","handle":"...","body":"..."}. Keep it ordinary and concise, like a 1996 Japanese PC-communication user.`,g.WorldDate,articleID)
	var out struct{Title,Handle,Body string}
	if err:=g.respondJSON(ctx,prompt,&out); err!=nil { return fallbackArticle(articleID),nil }
	return Article{ID:articleID,Title:out.Title,Handle:out.Handle,PostedAt:"08/26 00:30",Evidence:"atmospheric",Body:out.Body},nil
}

func (g Generator) findOrResearch(ctx context.Context, topic, question string) (historicalkb.ResearchCase,error) {
	cases,err:=g.History.Store.List(ctx,100)
	if err==nil {
		for _,c:=range cases { if c.Topic==topic && c.Question==question { return c,nil } }
	}
	return g.History.Create(ctx,topic,question)
}

func (g Generator) respondJSON(ctx context.Context,prompt string,out any) error {
	client:=g.Client; if client==nil {client=&http.Client{Timeout:45*time.Second}}
	payload:=map[string]any{"model":g.Model,"input":prompt,"text":map[string]any{"verbosity":"low"}}
	body,_:=json.Marshal(payload)
	endpoint,err:=azureopenai.URL(g.Endpoint,"responses"); if err!=nil{return err}
	req,err:=http.NewRequestWithContext(ctx,http.MethodPost,endpoint,bytes.NewReader(body)); if err!=nil{return err}
	if err:=azureopenai.ApplyAPIKey(req,g.APIKey); err!=nil{return err}
	resp,err:=client.Do(req); if err!=nil{return err}; defer resp.Body.Close()
	if resp.StatusCode<200||resp.StatusCode>=300{return fmt.Errorf("Azure Azure OpenAI responses API returned %s",resp.Status)}
	var decoded struct{Output []struct{Content []struct{Type string `json:"type"`; Text string `json:"text"`} `json:"content"`} `json:"output"`}
	if err:=json.NewDecoder(resp.Body).Decode(&decoded);err!=nil{return err}
	var text string
	for _,o:=range decoded.Output {for _,c:=range o.Content {if c.Type=="output_text" {text+=c.Text}}}
	text=strings.TrimSpace(text); text=strings.TrimPrefix(text,"```json"); text=strings.TrimPrefix(text,"```"); text=strings.TrimSuffix(text,"```")
	if text=="" {return errors.New("no output_text in Azure OpenAI response")}
	return json.Unmarshal([]byte(strings.TrimSpace(text)),out)
}

func fallbackSnapshot(worldDate string) Snapshot {
	return Snapshot{HostName:"MIDI NIGHT NET",Welcome:"*** MIDI NIGHT NET ***  いらっしゃいませ",Notice:"夜間は混み合うことがあります。長時間接続はほどほどに(^^;",WorldDate:worldDate,Boards:[]Board{
		{ID:"general",Title:"フリートーク",Description:"とりあえず何でもどうぞ",Articles:[]Article{{ID:"general-1",Title:"暑いですねえ",Handle:"MARI",PostedAt:"08/25 22:14",Preview:"昼間は暑くてPCの前に座ってるのもつらい…",Evidence:"atmospheric"},{ID:"general-2",Title:"夜更かし組(^^;",Handle:"KEN",PostedAt:"08/26 00:41",Preview:"こんな時間でもけっこう人いますね",Evidence:"atmospheric"}}},
		{ID:"pc",Title:"PC・通信",Description:"本体、モデム、通信ソフトなど",Articles:[]Article{{ID:"modem-288",Title:"モデム、みんなどのくらい？",Handle:"TAKA",PostedAt:"08/26 01:08",Preview:"28.8Kとか33.6Kとか最近よく聞くけど…",Evidence:"verified"},{ID:"pc-2",Title:"通信ソフト何使ってる？",Handle:"NEKO",PostedAt:"08/26 02:03",Preview:"最近ちょっと乗り換えようか迷ってます",Evidence:"atmospheric"}}},
		{ID:"chat",Title:"趣味・雑談",Description:"ゲーム、音楽、そのほか",Articles:[]Article{{ID:"chat-1",Title:"週末なにしてた？",Handle:"JUN",PostedAt:"08/26 00:12",Preview:"こっちはずっと家でごろごろしてました(笑)",Evidence:"atmospheric"}}},
	}}
}

func fallbackArticle(id string) Article {
	return Article{ID:id,Title:"ちょっと雑談",Handle:"MARI",PostedAt:"08/26 00:30",Evidence:"atmospheric",Body:"ども、MARIです。\n\n特に用事もないんだけど、なんとなく書き込み(^^;\n夜になるとここも人が増えますね。\n\nではでは。"}
}
