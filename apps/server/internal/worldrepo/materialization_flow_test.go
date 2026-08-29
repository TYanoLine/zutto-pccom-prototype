package worldrepo

import (
	"context"
	"testing"

	"zutto-pccom/apps/server/internal/historicalkb"
	"zutto-pccom/apps/server/internal/world"
	"zutto-pccom/apps/server/internal/worldengine"
)

type flowEngine struct{ calls int }
func (f *flowEngine) ResolveEvidence(_ context.Context,_ worldengine.EvidenceRequest)(worldengine.EvidenceDecision,error){f.calls++;return worldengine.EvidenceDecision{Level:historicalkb.EvidenceAtmospheric,ModelFirst:true,Knowledge:historicalkb.KnowledgeResult{CanUse:true}},nil}

type flowMaterializer struct{ calls int }
func (f *flowMaterializer) GenerateBoardPosts(_ context.Context,_ BoardMaterializationRequest,_ worldengine.EvidenceDecision)([]world.Post,error){f.calls++;return []world.Post{{Author:"AI",Subject:"unused",Body:"補完された本文です。"}},nil}

func TestDevelopmentMaterializationFlow(t *testing.T){
	base:=world.NewMemoryStore();engine:=&flowEngine{};mat:=&flowMaterializer{}
	repo:=New(base,engine,mat,"1996-08-29")
	h,err:=repo.HostByPhone("0450000196");if err!=nil{t.Fatal(err)}
	if h.Name==""||h.Lines==0||h.MaxBaud==0{t.Fatalf("host profile was not completed: %+v",h)}
	if !repo.HostWasMaterialized(h.ID){t.Fatal("host materialization not recorded")}
	h2,err:=repo.HostByPhone("0450000196");if err!=nil{t.Fatal(err)}
	if h2.Name!=h.Name{t.Fatal("completed host profile was not stored")}

	boards,created:=repo.MaterializationBoards(h);if !created||len(boards)!=3{t.Fatalf("boards created=%v len=%d",created,len(boards))}
	_,created=repo.MaterializationBoards(h);if created{t.Fatal("board catalog should be reused")}

	headers,created:=repo.MaterializationArticleHeaders(h,boards[0]);if !created||len(headers)!=3{t.Fatalf("headers created=%v len=%d",created,len(headers))}
	if headers[0].Body!=""{t.Fatal("article body must stay unmaterialized until read")}

	p,found,bodyCreated:=repo.MaterializationArticle(h,boards[0],headers[0].ID)
	if !found||!bodyCreated||p.Body==""{t.Fatalf("article found=%v created=%v body=%q",found,bodyCreated,p.Body)}
	if engine.calls!=1||mat.calls!=1{t.Fatalf("engine=%d materializer=%d, want 1 each",engine.calls,mat.calls)}
	p,found,bodyCreated=repo.MaterializationArticle(h,boards[0],headers[0].ID)
	if !found||bodyCreated||p.Body==""{t.Fatalf("stored article should be reused: found=%v created=%v body=%q",found,bodyCreated,p.Body)}
	if engine.calls!=1||mat.calls!=1{t.Fatal("re-read unexpectedly regenerated article")}
}
