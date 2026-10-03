package worldrepo
import (
 "testing"
 "zutto-pccom/apps/server/internal/world"
)

func TestRepositoryPreservesErikaAppendRepresentation(t *testing.T) {
    base:=world.NewMemoryStore()
    host,err:=base.HostByPhone("0920000196")
    if err!=nil {t.Fatal(err)}
    repo:=New(base,nil,LLMMaterializer{},"1996-08-26")
    if repo.bbsArticles==nil || repo.bbsArticles.ReplyProjector==nil {t.Fatal("missing host-specific projector")}
    source:=world.Post{ID:77,BoardID:"20/1",Subject:"元記事"}
    got,err:=repo.bbsArticles.ReplyProjector.ProjectReply(host,source,"提案件名")
    if err!=nil {t.Fatal(err)}
    if got.ParentID!=source.ID || got.Subject!="" { t.Fatalf("Erika append unexpectedly gained independent title: %+v",got) }
}
