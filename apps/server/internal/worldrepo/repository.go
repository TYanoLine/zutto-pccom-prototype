package worldrepo

import (
	"context"
	"fmt"
	"sync"
	"time"

	"zutto-pccom/apps/server/internal/historicalkb"
	"zutto-pccom/apps/server/internal/world"
	"zutto-pccom/apps/server/internal/worldengine"
)

type EvidenceResolver interface {
	ResolveEvidence(context.Context, worldengine.EvidenceRequest) (worldengine.EvidenceDecision, error)
}

type Materializer interface {
	GenerateBoardPosts(context.Context, BoardMaterializationRequest, worldengine.EvidenceDecision) ([]world.Post, error)
}

type BoardMaterializationRequest struct {
	Host       world.Host
	BoardID    string
	BoardTopic string
	WorldDate  string
}

type Repository struct {
	Base         world.Store
	Engine       EvidenceResolver
	Materializer Materializer
	WorldDate    string

	mu                 sync.Mutex
	materialized       map[string]bool
	hosts              map[string]world.Host
	hostMaterialized    map[string]bool
}

func New(base world.Store, engine EvidenceResolver, materializer Materializer, worldDate string) *Repository {
	return &Repository{Base:base,Engine:engine,Materializer:materializer,WorldDate:worldDate,materialized:map[string]bool{},hosts:map[string]world.Host{},hostMaterialized:map[string]bool{}}
}

func (r *Repository) HostByPhone(phone string) (world.Host,error) {
	h,err:=r.Base.HostByPhone(phone)
	if err!=nil{return h,err}
	if incompleteHost(h) {
		h = completeDevelopmentHost(h)
		if w,ok:=r.Base.(world.HostWriter);ok{w.SaveHost(h)}
		r.mu.Lock();r.hostMaterialized[h.ID]=true;r.mu.Unlock()
	}
	r.mu.Lock();r.hosts[h.ID]=h;r.mu.Unlock()
	return h,nil
}

// HostWasMaterialized is exposed for the development-only materialization host.
// Historical host runtimes do not need to render this internal state.
func (r *Repository) HostWasMaterialized(hostID string) bool { r.mu.Lock();defer r.mu.Unlock();return r.hostMaterialized[hostID] }

// ListPosts is the legacy Store boundary. When a known host has no posts at all,
// materialize a minimal default board once. More capable host runtimes can use
// BoardPostStore below to request a specific, already-known board lazily.
func (r *Repository) ListPosts(hostID string) []world.Post {
	if existing:=r.Base.ListPosts(hostID);len(existing)>0{return existing}
	r.mu.Lock();h,known:=r.hosts[hostID];r.mu.Unlock()
	if known && h.SoftwareID!="materialization-demo" { _=r.ensureBoard(h,"main","フリートーク") }
	return r.Base.ListPosts(hostID)
}

func (r *Repository) AddPost(hostID string,p world.Post) world.Post { return r.Base.AddPost(hostID,p) }

// ListBoardPosts is invoked only after a host-program runtime has established
// that the board path is real. Empty storage therefore means an unmaterialized
// known board, not permission to invent a nonexistent board.
func (r *Repository) ListBoardPosts(host world.Host, boardID, boardTopic string) []world.Post {
	if existing:=filterBoard(r.Base.ListPosts(host.ID),boardID);len(existing)>0{return existing}
	_ = r.ensureBoard(host,boardID,boardTopic)
	return filterBoard(r.Base.ListPosts(host.ID),boardID)
}

// MaterializationBoards creates and stores only the board catalog. No article
// prose is generated here; this mirrors the intended lazy hierarchy.
func (r *Repository) MaterializationBoards(host world.Host)([]world.Board,bool){
	bs,ok:=r.Base.(world.BoardStore)
	if !ok{return nil,false}
	if existing:=bs.ListBoards(host.ID);len(existing)>0{return existing,false}
	boards:=[]world.Board{{ID:"1",Name:"フリートーク"},{ID:"2",Name:"パソコン通信・モデム"},{ID:"3",Name:"地域の話題"}}
	bs.SaveBoards(host.ID,boards)
	return boards,true
}

// MaterializationArticleHeaders creates lightweight headers when a known board
// is first entered. Bodies remain empty until the user selects an article.
func (r *Repository) MaterializationArticleHeaders(host world.Host,board world.Board)([]world.Post,bool){
	if existing:=filterBoard(r.Base.ListPosts(host.ID),board.ID);len(existing)>0{return existing,false}
	stamp:=worldTime(r.WorldDate)
	templates:=[]struct{author,subject string}{
		{"NEKO",board.Name+"、どうです？"},
		{"MARI","はじめまして"},
		{"SYSOP","このボードについて"},
	}
	out:=make([]world.Post,0,len(templates))
	for i,t:=range templates{
		p:=r.Base.AddPost(host.ID,world.Post{BoardID:board.ID,Author:t.author,Subject:t.subject,CreatedAt:stamp.Add(time.Duration(i)*17*time.Minute)})
		out=append(out,p)
	}
	return out,true
}

// MaterializationArticle completes article prose only when the article is read.
// It reuses the normal evidence policy and LLM materializer; the header selected
// by WorldRepository stays canonical and only the missing body is filled.
func (r *Repository) MaterializationArticle(host world.Host,board world.Board,postID int64)(world.Post,bool,bool){
	var selected world.Post
	found:=false
	for _,p:=range r.Base.ListPosts(host.ID){if p.ID==postID&&p.BoardID==board.ID{selected=p;found=true;break}}
	if !found{return world.Post{},false,false}
	if selected.Body!=""{return selected,true,false}
	if r.Engine==nil||r.Materializer==nil{return selected,true,false}
	ctx,cancel:=context.WithTimeout(context.Background(),20*time.Second);defer cancel()
	decision,err:=r.Engine.ResolveEvidence(ctx,worldengine.EvidenceRequest{
		Kind:historicalkb.KnowledgeCulturalSignal,Subject:board.Name,WorldDate:r.WorldDate,Region:host.Region,Audience:[]string{host.SoftwareID},
		Need:fmt.Sprintf("%s の %s ボード、件名『%s』の記事本文を1996年の自然なパソコン通信文体で補完する",host.Name,board.Name,selected.Subject),
		Persistence:true,Importance:.30,Specificity:.30,
	})
	if err!=nil{return selected,true,false}
	// The canonical article subject is the actual content cue. Board placement,
	// host region and other world context remain constraints in the renderer and
	// must not be treated as a checklist of things to mention in the prose.
	posts,err:=r.Materializer.GenerateBoardPosts(ctx,BoardMaterializationRequest{Host:host,BoardID:board.ID,BoardTopic:selected.Subject,WorldDate:r.WorldDate},decision)
	if err!=nil||len(posts)==0{return selected,true,false}
	selected.Body=posts[0].Body
	if selected.Body==""{return selected,true,false}
	if u,ok:=r.Base.(world.PostUpdater);ok{updated,ok:=u.UpdatePost(host.ID,selected);if ok{return updated,true,true}}
	return selected,true,true
}

func (r *Repository) ensureBoard(host world.Host,boardID,boardTopic string) error {
	if r.Engine==nil||r.Materializer==nil{return nil}
	key:=host.ID+"|"+boardID
	r.mu.Lock()
	if r.materialized[key]{r.mu.Unlock();return nil}
	r.materialized[key]=true
	r.mu.Unlock()

	ctx,cancel:=context.WithTimeout(context.Background(),20*time.Second);defer cancel()
	decision,err:=r.Engine.ResolveEvidence(ctx,worldengine.EvidenceRequest{
		Kind:historicalkb.KnowledgeCulturalSignal,
		Subject:boardTopic,
		WorldDate:r.WorldDate,
		Region:host.Region,
		Audience:[]string{host.SoftwareID},
		Need:fmt.Sprintf("%s の %s ボードに自然な投稿を生成するために必要な時代背景",host.Name,boardTopic),
		Persistence:true,
		Importance:.25,
		Specificity:.25,
	})
	if err!=nil{return err}
	posts,err:=r.Materializer.GenerateBoardPosts(ctx,BoardMaterializationRequest{Host:host,BoardID:boardID,BoardTopic:boardTopic,WorldDate:r.WorldDate},decision)
	if err!=nil{return err}
	for _,p:=range posts{p.BoardID=boardID;r.Base.AddPost(host.ID,p)}
	return nil
}

func filterBoard(all []world.Post,boardID string)[]world.Post{out:=make([]world.Post,0);for _,p:=range all{if p.BoardID==boardID{out=append(out,p)}};return out}

func incompleteHost(h world.Host)bool{return h.Name==""||h.Software==""||h.Lines<=0||h.MaxBaud<=0}
func completeDevelopmentHost(h world.Host)world.Host{
	if h.Name==""{h.Name="LAZY MATERIALIZE BBS"}
	if h.Region==""{h.Region="神奈川県"}
	if h.Software==""{h.Software="局固有の架空ホスト (development)"}
	if h.Lines<=0{h.Lines=2};if h.MaxBaud<=0{h.MaxBaud=14400};if h.Members<=0{h.Members=48};if h.Popularity<=0{h.Popularity=.22}
	h.GuestAllowed=true;h.TelehoFriendly=true
	return h
}

// FallbackMaterializer is deterministic/non-AI. It keeps AI optional and gives
// the materialization pipeline a safe degradation path. A prose renderer can be
// swapped in without changing host-program runtimes or repository semantics.
type FallbackMaterializer struct{}
func (FallbackMaterializer) GenerateBoardPosts(_ context.Context,r BoardMaterializationRequest,d worldengine.EvidenceDecision)([]world.Post,error){
	if d.Level==historicalkb.EvidenceVerified&&!d.Knowledge.CanUse{return nil,fmt.Errorf("verified historical knowledge unavailable")}
	return []world.Post{{Author:"GUEST",Subject:r.BoardTopic+"の話",Body:"このボード、まだ書き込み少ないですね(^^;\r\nとりあえず足あとだけ残しておきます。",CreatedAt:worldTime(r.WorldDate)}},nil
}
func worldTime(v string)time.Time{t,err:=time.ParseInLocation("2006-01-02",v,time.Local);if err!=nil{return time.Now()};return t.Add(22*time.Hour+15*time.Minute)}
