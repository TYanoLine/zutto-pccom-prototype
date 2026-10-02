package worldrepo

import (
    "context"
    "errors"
    "fmt"
    "reflect"
    "strings"
    "testing"
    "time"

    "zutto-pccom/apps/server/internal/bbsengine"
    "zutto-pccom/apps/server/internal/llm"
    "zutto-pccom/apps/server/internal/world"
    "zutto-pccom/apps/server/internal/worldengine"
)

type chunkBudgetProposer struct {
    sizes []int
    alwaysFail bool
    delegate unpresetGameProposer
}

func (p *chunkBudgetProposer) GenerateBBSWorldSituationProposals(ctx context.Context, req llm.BBSWorldSituationProposalRequest) (llm.BBSWorldSituationProposalDraft, error) {
    p.sizes=append(p.sizes,len(req.Events))
    if p.alwaysFail {
        return llm.BBSWorldSituationProposalDraft{},errors.New("ordinary provider error")
    }
    if len(req.Events)>2 {
        return llm.BBSWorldSituationProposalDraft{},fmt.Errorf("provider reached max_output_tokens: %w",llm.ErrBBSWorldSituationOutputTruncated)
    }
    return p.delegate.GenerateBBSWorldSituationProposals(ctx,req)
}

func animeBudgetBatch(count int) bbsengine.BatchRequest {
    start:=time.Date(1995,5,1,19,0,0,0,time.FixedZone("JST",9*3600))
    req:=bbsengine.BatchRequest{
        Host:world.Host{ID:"hakata-canal-net",Name:"HAKATA CANAL NET"},
        Board:world.Board{ID:"20/2",Name:"ＡＮＩＭＥ／ＭＡＮＧＡ",SemanticScope:"アニメ・漫画の作品についての感想や相談"},
    }
    for i:=0;i<count;i++ {
        req.Slots=append(req.Slots,bbsengine.Slot{
            Index:i+1, Author:fmt.Sprintf("MEMBER-%02d",i), CreatedAt:start.Add(time.Duration(i)*time.Hour),
        })
    }
    return req
}

func TestSituationPlannerSplitsTruncatedAnimeChunkAndAcceptsAllHeaders(t *testing.T) {
    base:=world.NewMemoryStore()
    planner:=repositoryBBSBatchPlanner{repo:New(base,nil,nil,"1996-02-17")}
    proposer:=&chunkBudgetProposer{}
    titles:=&unpresetGameTitles{}
    req:=animeBudgetBatch(10)
    planned,err:=planner.planSituationFirstRoots(context.Background(),LLMMaterializer{ModelHistoricalMemory:true},worldengine.EvidenceDecision{},proposer,titles,req,req.Slots,"1996-02-17")
    if err!=nil {t.Fatal(err)}
    wantCalls:=[]int{6,3,2,2,2,2,2}
    if !reflect.DeepEqual(proposer.sizes,wantCalls) {
        t.Fatalf("budget split and retained chunk size: got %v want %v",proposer.sizes,wantCalls)
    }
    if len(planned)!=10 || len(titles.requests)!=1 || len(titles.requests[0].Articles)!=10 {
        t.Fatalf("Situation/title materialization was incomplete: planned=%d titles=%d",len(planned),len(titles.requests))
    }
    seen:=map[int]bool{}
    for _,post:=range planned {
        if seen[post.SlotIndex] || post.Subject=="" || post.SituationSummary=="" {
            t.Fatalf("duplicate or empty accepted article header: %+v",post)
        }
        seen[post.SlotIndex]=true
    }
    if len(seen)!=10 {
        t.Fatalf("lost World-selected slots after fallback: %v",seen)
    }
}

func TestSituationPlannerOnlySplitsOnProvenOutputBudgetTruncation(t *testing.T) {
    planner:=repositoryBBSBatchPlanner{repo:New(world.NewMemoryStore(),nil,nil,"1996-02-17")}
    proposer:=&chunkBudgetProposer{alwaysFail:true}
    titles:=&unpresetGameTitles{}
    req:=animeBudgetBatch(4)
    _,err:=planner.planSituationFirstRoots(context.Background(),LLMMaterializer{},worldengine.EvidenceDecision{},proposer,titles,req,req.Slots,"1996-02-17")
    if err==nil || !strings.Contains(err.Error(),"ordinary provider error") {
        t.Fatalf("ordinary error was suppressed: %v",err)
    }
    if want:=[]int{4,4,4};!reflect.DeepEqual(proposer.sizes,want) {
        t.Fatalf("unexpected split or retry for ordinary error: got %v want %v",proposer.sizes,want)
    }
    if len(titles.requests)!=0 {t.Fatal("title generation ran without accepted Situations")}
}
