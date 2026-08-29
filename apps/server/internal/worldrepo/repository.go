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

	mu           sync.Mutex
	materialized map[string]bool
	hosts        map[string]world.Host
}

func New(base world.Store, engine EvidenceResolver, materializer Materializer, worldDate string) *Repository {
	return &Repository{Base:base,Engine:engine,Materializer:materializer,WorldDate:worldDate,materialized:map[string]bool{},hosts:map[string]world.Host{}}
}

func (r *Repository) HostByPhone(phone string) (world.Host,error) {
	h,err:=r.Base.HostByPhone(phone)
	if err==nil { r.mu.Lock();r.hosts[h.ID]=h;r.mu.Unlock() }
	return h,err
}

// ListPosts is the legacy Store boundary. When a known host has no posts at all,
// materialize a minimal default board once. More capable host runtimes can use
// BoardPostStore below to request a specific, already-known board lazily.
func (r *Repository) ListPosts(hostID string) []world.Post {
	if existing:=r.Base.ListPosts(hostID);len(existing)>0{return existing}
	r.mu.Lock();h,known:=r.hosts[hostID];r.mu.Unlock()
	if known { _=r.ensureBoard(h,"main","フリートーク") }
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

// FallbackMaterializer is deterministic/non-AI. It keeps AI optional and gives
// the materialization pipeline a safe degradation path. A prose renderer can be
// swapped in without changing host-program runtimes or repository semantics.
type FallbackMaterializer struct{}
func (FallbackMaterializer) GenerateBoardPosts(_ context.Context,r BoardMaterializationRequest,d worldengine.EvidenceDecision)([]world.Post,error){
	if d.Level==historicalkb.EvidenceVerified&&!d.Knowledge.CanUse{return nil,fmt.Errorf("verified historical knowledge unavailable")}
	return []world.Post{{Author:"GUEST",Subject:r.BoardTopic+"の話",Body:"このボード、まだ書き込み少ないですね(^^;\r\nとりあえず足あとだけ残しておきます。",CreatedAt:worldTime(r.WorldDate)}},nil
}
func worldTime(v string)time.Time{t,err:=time.ParseInLocation("2006-01-02",v,time.Local);if err!=nil{return time.Now()};return t.Add(22*time.Hour+15*time.Minute)}
