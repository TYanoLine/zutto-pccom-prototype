package worldrepo

import (
    "context"
    "errors"
    "strings"
    "testing"
    "time"

    "zutto-pccom/apps/server/internal/llm"
    "zutto-pccom/apps/server/internal/world"
    "zutto-pccom/apps/server/internal/worldengine"
)

type freeformResearchSpy struct { calls int }

func (s *freeformResearchSpy) ResolveEvidence(_ context.Context, _ worldengine.EvidenceRequest) (worldengine.EvidenceDecision, error) {
    s.calls++
    return worldengine.EvidenceDecision{}, errors.New("freeform mode must not call research")
}

type freeformBodyRenderer struct {
    detailCalls int
    bodyCalls int
    request llm.BoardPostRequest
}
func (r *freeformBodyRenderer) GenerateBoardPost(_ context.Context, req llm.BoardPostRequest) (llm.BoardPostDraft, error) {
    r.bodyCalls++
    r.request = req
    return llm.BoardPostDraft{Author: req.AuthorHandle, Subject: req.CanonicalSubject, Body: "月末の結果について思ったことを書きました。"}, nil
}
func (r *freeformBodyRenderer) MaterializeBBSTitleArticleDetails(_ context.Context, _ llm.BBSTitleArticleDetailRequest) (llm.BBSTitleArticleDetailDraft, error) {
    r.detailCalls++
    return llm.BBSTitleArticleDetailDraft{}, errors.New("freeform mode must not call Article Detail")
}

func TestFreeformReadDoesNotCallOptionalLLMOrChangeAcceptedFacts(t *testing.T) {
    base := world.NewMemoryStore()
    host, err := base.HostByPhone("0920000196")
    if err != nil { t.Fatal(err) }
    board := world.Board{ID:"20/2", Name:"アニメ・漫画"}
    renderer := &freeformBodyRenderer{}
    research := &freeformResearchSpy{}
    repo := New(base, research, LLMMaterializer{Renderer:renderer, ProductionMinimalHistoricalPrompt:true, ModelHistoricalMemory:true}, "1996-02-17")
    repo.SetFreeformBody(true)
    originalFacts := []string{"occurrence=プリンセスメーカー2の月末結果を見比べた", "scope_boundary=今回の結果の比較"}
    post := base.AddPost(host.ID, world.Post{
        BoardID:board.ID,
        Author:"KOJI.I",
        Subject:"プリンセスメーカー2、月末結果の違い",
        CreatedAt:time.Date(1996,2,17,21,0,0,0,time.FixedZone("JST",9*3600)),
        Intent:world.PostIntent{
            Action:"thread_start",
            SituationSummary:"プリンセスメーカー2の月末結果を比べて気づいたことを話す",
            SituationFacts:append([]string(nil),originalFacts...),
        },
    })
    result, found, generated, diagnostic := repo.MaterializationArticleWithDebug(host, board, post.ID)
    if !found || !generated || strings.TrimSpace(result.Body)=="" || strings.Contains(diagnostic,"error stage=") {
        t.Fatalf("freeform article failed: found=%t generated=%t diagnostic=%s",found,generated,diagnostic)
    }
    if research.calls!=0 || renderer.detailCalls!=0 || renderer.bodyCalls!=1 {
        t.Fatalf("unexpected model calls: research=%d detail=%d body=%d",research.calls,renderer.detailCalls,renderer.bodyCalls)
    }
    if !renderer.request.FreeformFromSubject || renderer.request.BoardTopic!=board.Name ||
        renderer.request.CanonicalSubject!=post.Subject {
        t.Fatalf("wrong board / article title wiring: %+v",renderer.request)
    }
    if !strings.Contains(renderer.request.PostIntent,"確定した状況:") ||
        strings.Contains(renderer.request.PostIntent,"scope_boundary=") ||
        strings.Contains(renderer.request.PostIntent,"occurrence=") {
        t.Fatalf("body packet was not reduced to essential Situation: %s",renderer.request.PostIntent)
    }
    saved, ok := repo.findMaterializationPost(host.ID,board.ID,post.ID)
    if !ok || saved.Subject!=post.Subject || saved.Intent.SituationSummary!=post.Intent.SituationSummary ||
        strings.Join(saved.Intent.SituationFacts,"|")!=strings.Join(originalFacts,"|") ||
        saved.Intent.ArticleDetailsMaterialized {
        t.Fatalf("changed canonical state while relaxing prose: %+v",saved)
    }
    _, _, secondGenerated, _ := repo.MaterializationArticleWithDebug(host,board,post.ID)
    if secondGenerated || renderer.bodyCalls!=1 { t.Fatal("cached body generated twice") }
}

func TestFreeformDoesNotApplyToOtherHosts(t *testing.T) {
    repo := New(world.NewMemoryStore(),nil,nil,"1996-02-17")
    repo.SetFreeformBody(true)
    if !repo.useFreeformBody(experimentTestHost()) ||
        repo.useFreeformBody(world.Host{ID:"another-host"}) {
        t.Fatal("freeform mode leaked to other hosts")
    }
    repo.SetFreeformBody(false)
    if repo.useFreeformBody(experimentTestHost()) {
        t.Fatal("freeform mode not reversible")
    }
}

func TestFreeformIntentKeepsRequiredReferentAndReplyCausality(t *testing.T) {
    summary := freeformIntentSummary(world.PostIntent{
        Action:"reply",
        SituationSummary:"今回の気づきを返信する",
        Goal:"先にあった比較へ反応する",
        RespondsToPostID:123,
        SituationFacts:[]string{"occurrence=internal noise","article_referent_required=プリンセスメーカー2","article_detail=referent:プリンセスメーカー2"},
        RenderContext:"[MARI] 結果が変わった\n私はこう見た",
    })
    for _, part := range []string{"今回の気づき","返信の目的:","プリンセスメーカー2","これまでの会話:"} {
        if !strings.Contains(summary,part) { t.Fatalf("missing %s: %s",part,summary) }
    }
    if strings.Contains(summary,"occurrence=") { t.Fatalf("planning internals leaked: %s",summary) }
}
