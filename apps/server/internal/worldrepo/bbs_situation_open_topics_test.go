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

type openTopicSituationProposer struct {
    requests []llm.BBSWorldSituationProposalRequest
}

func (s *openTopicSituationProposer) GenerateBBSWorldSituationProposals(_ context.Context, req llm.BBSWorldSituationProposalRequest) (llm.BBSWorldSituationProposalDraft, error) {
    s.requests = append(s.requests, req)
    out := llm.BBSWorldSituationProposalDraft{}
    for _, event := range req.Events {
        out.Situations = append(out.Situations, llm.BBSWorldSituationDraft{
            EventID:     event.EventID,
            ObjectClass: event.BoardName,
            Occurrence:  event.BoardName + "に書き込む状況",
            PostContent: event.BoardName + "の範囲で投稿する中身",
            NoveltyKey:  fmt.Sprintf("%s-open-%s", event.BoardID, event.EventID),
        })
    }
    return out, nil
}

type openTopicTitlePlanner struct {
    requests []llm.BBSSituationTitleRequest
}

func (s *openTopicTitlePlanner) GenerateBBSSituationTitles(_ context.Context, req llm.BBSSituationTitleRequest) (llm.BBSSituationTitleDraft, error) {
    s.requests = append(s.requests, req)
    out := llm.BBSSituationTitleDraft{}
    for _, article := range req.Articles {
        out.Titles = append(out.Titles, llm.BBSSituationTitle{
            EventID: article.EventID,
            Subject: "最近気づいたこと",
        })
    }
    return out, nil
}

func TestProductionAllBoardMaterialsHaveNoSelectedTopic(t *testing.T) {
    materials := productionOpenSituationMaterials([]string{
        "interest=games",
        "previously_observed=会員が以前の記事を読んだ",
    })
    want := []string{
        "persona_context=interest=games",
        "persona_context=previously_observed=会員が以前の記事を読んだ",
    }
    if !reflect.DeepEqual(materials, want) {
        t.Fatalf("unexpected open-topic material: got=%#v want=%#v", materials, want)
    }
}

// Every board, whatever its wording, goes through the same generic path. The
// anchor key is only the board identity, and no post type is selected: no
// topic domain or discourse mode is derived from the board name, its scope or
// the author's interests.
func TestAllBoardsShareOpenTopicSituationGeneration(t *testing.T) {
    boards := []struct {
        name  string
        board world.Board
    }{
        {"GAME", world.Board{ID: "20/1", Name: "ＧＡＭＥ", SemanticScope: "家庭用・PC等のゲームについての感想、相談など。"}},
        {"ANIME", world.Board{ID: "20/2", Name: "ＡＮＩＭＥ／ＭＡＮＧＡ", SemanticScope: "アニメ、漫画の感想など。"}},
        {"PC98", world.Board{ID: "70/1", Name: "ＰＣ－９８", SemanticScope: "PC-98系機種の利用、設定、周辺機器。"}},
        {"MODEM", world.Board{ID: "60/1", Name: "ＰＣ－９８／ＭＯＤＥＭ", SemanticScope: "PC-98系やモデム、通信環境について。"}},
        {"SOFTWARE", world.Board{ID: "60/3", Name: "ＳＯＦＴＷＡＲＥ", SemanticScope: "ソフトウェア、データ、ファイルについて。"}},
        {"CHAT", world.Board{ID: "68/1", Name: "深夜雑談", SemanticScope: "深夜に接続している会員の雑談。"}},
        {"LOCAL", world.Board{ID: "90/1", Name: "博多ご近所情報", SemanticScope: "地域での出来事や情報交換。"}},
        {"INTRO", world.Board{ID: "2", Name: "自己紹介・新人歓迎", SemanticScope: "新規会員の自己紹介、常連からの歓迎。"}},
        {"QA", world.Board{ID: "3", Name: "Ｑ＆Ａ（質問ボード）", SemanticScope: "日常の具体的な疑問や困りごとを尋ねる一般質問板。"}},
        {"DYNAMIC", world.Board{ID: "dyn-7", Name: "動的に作られた板", SemanticScope: "その板の方向性。"}},
    }
    at := time.Date(1996, 2, 17, 20, 30, 0, 0, time.FixedZone("JST", 9*3600))

    for _, tc := range boards {
        t.Run(tc.name, func(t *testing.T) {
            base := world.NewMemoryStore()
            planner := repositoryBBSBatchPlanner{repo: New(base, nil, nil, "1996-02-17")}
            proposer := &openTopicSituationProposer{}
            titles := &openTopicTitlePlanner{}
            req := bbsengine.BatchRequest{
                Host: world.Host{ID: "hakata-canal-net", Name: "HAKATA CANAL NET"},
                Board: tc.board,
                RecentPosts: []world.Post{{
                    BoardID: tc.board.ID,
                    Subject: "以前に投稿された話",
                    Intent: world.PostIntent{SituationKind: "legacy_fixed_activity"},
                }},
                Slots: []bbsengine.Slot{
                    {Index: 1, Author: "A", CreatedAt: at},
                    {Index: 2, Author: "B", CreatedAt: at.Add(time.Minute)},
                },
            }
            planned, err := planner.planSituationFirstRoots(
                context.Background(), LLMMaterializer{ModelHistoricalMemory: true},
                worldengine.EvidenceDecision{}, proposer, titles,
                req, req.Slots, "1996-02-17",
            )
            if err != nil { t.Fatal(err) }
            if len(proposer.requests) != 1 || len(proposer.requests[0].Events) != 2 || len(planned) != 2 {
                t.Fatalf("unexpected batch sizes: proposer=%d planned=%d", len(proposer.requests), len(planned))
            }
            forwarded := proposer.requests[0]
            if forwarded.WorldDate != "1996-02-17" ||
                forwarded.Events[0].BoardName != tc.board.Name ||
                forwarded.Events[0].BoardID != tc.board.ID ||
                forwarded.Events[0].BoardScope != tc.board.SemanticScope ||
                forwarded.Events[0].AuthorHandle != "A" ||
                forwarded.Events[1].AuthorHandle != "B" ||
                forwarded.Events[0].AnchorKey != tc.board.ID ||
                forwarded.Events[1].AnchorKey != tc.board.ID ||
                forwarded.Events[0].CreatedAt != at.Format(time.RFC3339) {
                t.Fatalf("World-selected event shell changed: %+v", forwarded)
            }
            for _, event := range forwarded.Events {
                if event.DiscourseMode != "" {
                    t.Fatalf("board %s was given a World-selected post type %q", tc.name, event.DiscourseMode)
                }
                if len(event.ExistingFacts) != 0 {
                    t.Fatalf("board %s still receives preset activities: %#v", tc.name, event.ExistingFacts)
                }
            }
            if forwarded.RecentBBSState != "subject=以前に投稿された話" ||
                !reflect.DeepEqual(forwarded.AvoidSituations, []string{"subject=以前に投稿された話"}) {
                t.Fatalf("previous facet metadata leaked: recent=%q avoid=%#v", forwarded.RecentBBSState, forwarded.AvoidSituations)
            }
            if len(titles.requests) != 1 || len(titles.requests[0].Articles) != 2 ||
                titles.requests[0].BoardName != tc.board.Name ||
                titles.requests[0].BoardScope != tc.board.SemanticScope {
                t.Fatalf("title pipeline changed: %+v", titles.requests)
            }
            for _, seed := range titles.requests[0].Articles {
                if seed.SituationKind != "open_topic" ||
                    !strings.Contains(seed.SituationSummary, tc.board.Name) {
                    t.Fatalf("title worker received old facet or lost accepted Situation: %+v", seed)
                }
            }
            for _, post := range planned {
                if post.SituationKind != "open_topic" ||
                    post.AnchorKey != tc.board.ID ||
                    post.DiscourseMode != "" ||
                    post.Subject != "最近気づいたこと" ||
                    !strings.Contains(post.SituationSummary, tc.board.Name) ||
                    !post.ArticleDetailsMaterialized {
                    t.Fatalf("invalid committed world header: %+v", post)
                }
                hasContent := false
                for _, fact := range post.SituationFacts {
                    if strings.HasPrefix(fact, "post_content=") {
                        hasContent = true
                    }
                    for _, typed := range []string{"observation=", "experience=", "result=", "stance=", "basis=", "attempted_actions=", "practical_point=", "question="} {
                        if strings.HasPrefix(fact, typed) {
                            t.Fatalf("post-type field leaked into canonical state: %q", fact)
                        }
                    }
                    if strings.Contains(fact, "activity_focus=") ||
                        strings.Contains(fact, "situation_kind=") ||
                        strings.Contains(fact, "legacy_fixed_activity") {
                        t.Fatalf("old preset leaked into canonical state: %q", fact)
                    }
                }
                if !hasContent {
                    t.Fatalf("canonical state lost post_content: %+v", post.SituationFacts)
                }
            }
        })
    }
}

func TestOpenTopicRecentSubjectsIgnoreHistoricFacetLabelsAndReplies(t *testing.T) {
    posts := []world.Post{
        {Subject: "実際にあった話", Intent: world.PostIntent{SituationKind: "old_specific_activity"}},
        {Subject: "", Intent: world.PostIntent{SituationKind: "old_generic_activity"}},
        {Subject: "返信は対象外", ParentID: 1, Intent: world.PostIntent{RespondsToPostID: 1}},
    }
    context := productionRecentSubjectContext(posts)
    avoid := productionRecentSubjectAvoid(posts)
    if context != "subject=実際にあった話" ||
        !reflect.DeepEqual(avoid, []string{"subject=実際にあった話"}) {
        t.Fatalf("unexpected previous subject materials: context=%q avoid=%#v", context, avoid)
    }
}

func TestIncompleteSituationIsRejected(t *testing.T) {
    complete := llm.BBSWorldSituationDraft{ObjectClass: "x", Occurrence: "y", PostContent: "z", NoveltyKey: "k"}
    if err := validateProductionSituation(complete); err != nil {
        t.Fatalf("complete situation rejected: %v", err)
    }
    for name, broken := range map[string]llm.BBSWorldSituationDraft{
        "no object":     {Occurrence: "y", PostContent: "z", NoveltyKey: "k"},
        "no occurrence": {ObjectClass: "x", PostContent: "z", NoveltyKey: "k"},
        "no content":    {ObjectClass: "x", Occurrence: "y", NoveltyKey: "k"},
        "no novelty":    {ObjectClass: "x", Occurrence: "y", PostContent: "z"},
    } {
        if err := validateProductionSituation(broken); err == nil {
            t.Fatalf("%s: incomplete situation was accepted", name)
        }
    }
}

func TestStationPostingRulesApplyBeforeSituationProposal(t *testing.T) {
    at := time.Date(1996, 2, 17, 20, 30, 0, 0, time.FixedZone("JST", 9*3600))
    host := world.Host{ID: "hakata-canal-net", Name: "HAKATA CANAL NET"}

    t.Run("staff author cannot be invented by model", func(t *testing.T) {
        planner := repositoryBBSBatchPlanner{repo: New(world.NewMemoryStore(), nil, nil, "1996-02-17")}
        proposer := &openTopicSituationProposer{}
        titles := &openTopicTitlePlanner{}
        req := bbsengine.BatchRequest{
            Host: host,
            Board: world.Board{ID: "1", Name: "事務局からのお知らせ",
                SemanticScope: "HAKATA局のSYSOPによる運営案内。", RootAuthorPolicy: "sysop_only"},
            Slots: []bbsengine.Slot{{Index: 1, Author: "ANOTHER", CreatedAt: at}},
        }
        _, err := planner.planSituationFirstRoots(context.Background(), LLMMaterializer{},
            worldengine.EvidenceDecision{}, proposer, titles, req, req.Slots, "1996-02-17")
        if err == nil || !strings.Contains(err.Error(), "only permits SYSOP roots") ||
            len(proposer.requests) != 0 {
            t.Fatalf("unapproved author passed World boundary: err=%v proposer=%+v", err, proposer.requests)
        }
        req.Slots[0].Author = "SYSOP"
        out, err := planner.planSituationFirstRoots(context.Background(), LLMMaterializer{},
            worldengine.EvidenceDecision{}, proposer, titles, req, req.Slots, "1996-02-17")
        if err != nil || len(out) != 1 ||
            len(proposer.requests) != 1 ||
            proposer.requests[0].Events[0].AuthorHandle != "SYSOP" ||
            proposer.requests[0].Events[0].BoardScope != req.Board.SemanticScope {
            t.Fatalf("SYSOP board invalid: out=%+v err=%v requests=%+v", out, err, proposer.requests)
        }
    })
}
