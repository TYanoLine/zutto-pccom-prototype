package worldrepo

import (
    "context"
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

type unpresetGameProposer struct {
    requests []llm.BBSWorldSituationProposalRequest
}
func (s *unpresetGameProposer) GenerateBBSWorldSituationProposals(_ context.Context, req llm.BBSWorldSituationProposalRequest) (llm.BBSWorldSituationProposalDraft, error) {
    s.requests = append(s.requests, req)
    out := llm.BBSWorldSituationProposalDraft{}
    for _, event := range req.Events {
        out.Situations = append(out.Situations, llm.BBSWorldSituationDraft{
            EventID:event.EventID,
            ObjectClass:"ゲーム",
            Occurrence:"最近遊んだゲームについて自分の体験を話す",
            Observation:"自分が気づいたところを話す",
            Experience:"自分で一度試してみた",
            Result:"予想とは少し違った",
            Stance:"そこが気に入った",
            Basis:"試して感じたこと",
            AttemptedActions:"自分でできるところまで試した",
            PracticalPoint:"試した範囲で分かったこと",
            Question:"同じ経験のある人はいるだろうか",
            NoveltyKey:fmt.Sprintf("open-%s",event.EventID),
        })
    }
    return out, nil
}

type unpresetGameTitles struct {
    requests []llm.BBSSituationTitleRequest
}
func (s *unpresetGameTitles) GenerateBBSSituationTitles(_ context.Context, req llm.BBSSituationTitleRequest) (llm.BBSSituationTitleDraft, error) {
    s.requests = append(s.requests, req)
    out:=llm.BBSSituationTitleDraft{}
    for _, article:=range req.Articles {
        out.Titles=append(out.Titles,llm.BBSSituationTitle{EventID:article.EventID,Subject:"昨日遊んでいて気づいたこと"})
    }
    return out,nil
}

func TestProductionGameMaterialsContainNoChosenActivity(t *testing.T) {
    materials:=productionOpenSituationMaterials([]string{"interest=games", "previously_observed=ゲームの話を読んだ"})
    want:=[]string{"persona_context=interest=games","persona_context=previously_observed=ゲームの話を読んだ"}
    if !reflect.DeepEqual(materials,want) {
        t.Fatalf("unexpected open-topic material: got=%#v want=%#v",materials,want)
    }
    if _,ok:=chooseProductionSituationFacet(world.Host{},world.Board{Name:"GAME"},world.Persona{},time.Now(),1,"games","ask_peers",nil);ok {
        t.Fatal("production GAME must not select a fixed activity facet")
    }
}

func TestGameBatchSituationReceivesNoTopicPresetOrPreviousFacetLabels(t *testing.T) {
    base:=world.NewMemoryStore()
    repo:=New(base,nil,nil,"1996-02-17")
    planner:=repositoryBBSBatchPlanner{repo:repo}
    proposer:=&unpresetGameProposer{}
    titles:=&unpresetGameTitles{}
    at:=time.Date(1996,2,17,20,30,0,0,time.FixedZone("JST",9*3600))
    req:=bbsengine.BatchRequest{
        Host:world.Host{ID:"hakata-canal-net",Name:"HAKATA CANAL NET"},
        Board:world.Board{ID:"20/1",Name:"ＧＡＭＥ",SemanticScope:"家庭用・PC等のゲームについての感想、攻略上の詰まり、対戦、貸し借り、購入相談など。"},
        RecentPosts:[]world.Post{{
            BoardID:"20/1",Subject:"過去に遊んだゲームの話",
            Intent:world.PostIntent{SituationKind:"games_route_choice"},
        }},
        Slots:[]bbsengine.Slot{{Index:1,Author:"A",CreatedAt:at},{Index:2,Author:"B",CreatedAt:at.Add(time.Minute)}},
    }
    planned,err:=planner.planSituationFirstRoots(context.Background(),LLMMaterializer{ModelHistoricalMemory:true},worldengine.EvidenceDecision{},proposer,titles,req,req.Slots,"1996-02-17")
    if err!=nil {t.Fatal(err)}
    if len(proposer.requests)!=1 || len(proposer.requests[0].Events)!=2 || len(planned)!=2 {
        t.Fatalf("unexpected batch sizes: proposer=%d planned=%d",len(proposer.requests),len(planned))
    }
    forwarded:=proposer.requests[0]
    if forwarded.WorldDate!="1996-02-17" || forwarded.Events[0].BoardName!="ＧＡＭＥ" ||
        forwarded.Events[0].AuthorHandle!="A" || forwarded.Events[1].AuthorHandle!="B" ||
        forwarded.Events[0].AnchorKey!="games" || forwarded.Events[0].DiscourseMode=="" {
        t.Fatalf("World-selected event fields lost: %+v",forwarded)
    }
    for _,event:=range forwarded.Events {
        if len(event.ExistingFacts)!=0 {
            t.Fatalf("open-topic GAME still receives generated activities: %+v",event.ExistingFacts)
        }
    }
    if !strings.Contains(forwarded.RecentBBSState,"過去に遊んだゲームの話") ||
        strings.Contains(forwarded.RecentBBSState,"games_route_choice") {
        t.Fatalf("historical facet metadata leaked into model context: %q",forwarded.RecentBBSState)
    }
    if len(forwarded.AvoidSituations)!=1 ||
        forwarded.AvoidSituations[0]!="subject=過去に遊んだゲームの話" {
        t.Fatalf("avoidance retained preset categories: %+v",forwarded.AvoidSituations)
    }
    if len(titles.requests)!=1 || len(titles.requests[0].Articles)!=2 { t.Fatalf("no title planning: %+v",titles.requests) }
    for _,result:=range planned {
        if result.SituationKind!="games_open_topic" ||
            result.Subject!="昨日遊んでいて気づいたこと" ||
            !strings.Contains(result.SituationSummary,"最近遊んだゲーム") ||
            !result.ArticleDetailsMaterialized {
            t.Fatalf("bad committed header: %+v",result)
        }
        for _,fact:=range result.SituationFacts {
            if strings.Contains(fact,"activity_focus=") || strings.Contains(fact,"games_route_choice") {
                t.Fatalf("preset topic leaked into canonical facts: %q",fact)
            }
        }
    }
}

func TestGameRecentSubjectContextDoesNotNarrateOldActivityCategories(t *testing.T) {
    posts:=[]world.Post{
        {Subject:"漫画ではなくゲームの話", Intent:world.PostIntent{SituationKind:"games_particular_work_character"}},
        {Subject:"", Intent:world.PostIntent{SituationKind:"games_stuck_point"}},
        {Subject:"返信は対象外", ParentID:1,Intent:world.PostIntent{RespondsToPostID:1}},
    }
    context:=productionRecentSubjectContext(posts)
    avoid:=productionRecentSubjectAvoid(posts)
    if context!="subject=漫画ではなくゲームの話" ||
        !reflect.DeepEqual(avoid,[]string{"subject=漫画ではなくゲームの話"}) {
        t.Fatalf("unexpected previous topic materials: context=%q avoid=%#v",context,avoid)
    }
}
