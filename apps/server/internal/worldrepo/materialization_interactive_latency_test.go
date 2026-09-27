package worldrepo

import (
	"context"
	"strings"
	"testing"
	"time"

	"zutto-pccom/apps/server/internal/llm"
	"zutto-pccom/apps/server/internal/world"
)

type interactiveTitleFirstTestRenderer struct {
	titleFirstTestRenderer
	detailCalls int
	detailReq   llm.BBSTitleArticleDetailRequest
}

func (f *interactiveTitleFirstTestRenderer) MaterializeBBSTitleArticleDetails(ctx context.Context, req llm.BBSTitleArticleDetailRequest) (llm.BBSTitleArticleDetailDraft, error) {
	f.detailCalls++
	f.detailReq = req
	return f.titleFirstTestRenderer.MaterializeBBSTitleArticleDetails(ctx, req)
}

func TestInteractiveTitleFirstPlansOnlySelectedBoardAndDefersArticleDetails(t *testing.T) {
	base := world.NewMemoryStore()
	renderer := &interactiveTitleFirstTestRenderer{titleFirstTestRenderer: titleFirstTestRenderer{
		fakeBoardRenderer: fakeBoardRenderer{draft: llm.BoardPostDraft{Author: "WRONG", Subject: "WRONG", Body: "本文です。"}},
	}}
	repo := New(base, conversationViewEvidenceEngine{}, LLMMaterializer{Renderer: renderer}, "1996-08-29")
	repo.EnableDevelopmentInteractiveTitleFirstPoC()
	host, err := repo.HostByPhone("0450000196")
	if err != nil {
		t.Fatal(err)
	}
	boards, _ := repo.MaterializationBoards(host)
	posts, created := repo.MaterializationPersonaArticleHeaders(host, boards[0])
	if !created || len(posts) == 0 {
		t.Fatalf("selected board was not materialized: created=%v posts=%d", created, len(posts))
	}
	for _, post := range base.ListPosts(host.ID) {
		if post.BoardID != boards[0].ID {
			t.Fatalf("interactive index materialized unrelated board %s", post.BoardID)
		}
	}
	if renderer.detailCalls != 0 {
		t.Fatalf("article details ran while only the index was requested: %d", renderer.detailCalls)
	}

	var root world.Post
	for _, post := range posts {
		if post.ParentID == 0 && titleFirstSubject(post.Intent.SituationFacts) != "" {
			root = post
			break
		}
	}
	if root.ID == 0 {
		t.Fatal("no accepted title-first root")
	}
	if hasInteractiveArticleDetails(root.Intent.SituationFacts) {
		t.Fatalf("article detail leaked into index materialization: %+v", root.Intent.SituationFacts)
	}

	rendered, found, bodyCreated, diagnostic := repo.MaterializationArticleWithDebug(host, boards[0], root.ID)
	if !found || !bodyCreated || strings.TrimSpace(rendered.Body) == "" {
		t.Fatalf("article open did not materialize body: found=%v created=%v diagnostic=%s", found, bodyCreated, diagnostic)
	}
	if renderer.detailCalls != 1 {
		t.Fatalf("article open should materialize details exactly once, got %d", renderer.detailCalls)
	}
	if len(renderer.detailReq.Articles) != 1 || !strings.Contains(renderer.detailReq.Articles[0].PersonaProfile, "everyday_baseline=") {
		t.Fatalf("article detail materializer did not receive persona baseline: %+v", renderer.detailReq.Articles)
	}
	if !hasInteractiveArticleDetails(rendered.Intent.SituationFacts) {
		t.Fatalf("article details were not persisted before prose: %+v", rendered.Intent.SituationFacts)
	}
	if !strings.Contains(renderer.req.PostIntent, "article_detail=") {
		t.Fatalf("body renderer did not receive deferred article details: %s", renderer.req.PostIntent)
	}
}

func TestInteractiveTitleIndexSkipsSynchronousEraWebResearch(t *testing.T) {
	base := world.NewMemoryStore()
	evidence := &titleEraEvidenceResolver{claim: "ERA_OK: test"}
	renderer := &interactiveTitleFirstTestRenderer{titleFirstTestRenderer: titleFirstTestRenderer{
		eraStatuses: map[int]string{1: llm.BBSTitleEraResearch},
	}}
	repo := New(base, evidence, LLMMaterializer{Renderer: renderer}, "1996-08-29")
	repo.EnableDevelopmentInteractiveTitleFirstPoC()
	host, err := repo.HostByPhone("0450000196")
	if err != nil {
		t.Fatal(err)
	}
	boards, _ := repo.MaterializationBoards(host)
	posts, _ := repo.MaterializationPersonaArticleHeaders(host, boards[0])
	if evidence.calls != 0 {
		t.Fatalf("interactive article index performed synchronous historical Web research: calls=%d", evidence.calls)
	}
	if len(posts) == 0 {
		t.Fatal("safe candidates should still be usable when a research candidate is skipped")
	}
	rows := repo.DevelopmentTitleCandidates()
	if len(rows) < 1 || rows[0].EraStatus != "research" || rows[0].Status != "era_rejected" {
		t.Fatalf("research candidate was not conservatively excluded from interactive index: %+v", rows)
	}
}

func TestRepairInteractiveArticleDetailFactsDropsEntireMetadataTaintedSet(t *testing.T) {
	in := []string{
		"title_first_subject=YMOを聴き直しています",
		"article_detail=locator:音楽板のMSG 1201として掲示されている",
		"article_detail=timing:1996年6月7日21時36分に投稿された",
		"article_detail_contract=old",
		"world_adopted_summary=YMOを聴き直している",
	}
	out, changed := repairInteractiveArticleDetailFacts(in)
	if !changed {
		t.Fatal("metadata-tainted detail set should be repaired")
	}
	joined := strings.Join(out, "\n")
	if strings.Contains(joined, "article_detail=") || strings.Contains(joined, "article_detail_contract=") {
		t.Fatalf("old detail set survived repair: %s", joined)
	}
	if !strings.Contains(joined, "title_first_subject=") || !strings.Contains(joined, "world_adopted_summary=") {
		t.Fatalf("unrelated canonical facts were removed: %s", joined)
	}
}

func TestInteractiveTitleFirstDoesNotInventCannedSubjectsWhenPoolsRejectEverything(t *testing.T) {
	base := world.NewMemoryStore()
	// Force every generated candidate to be rejected. The diagnostic path must
	// report the failed realization rather than converting selected roots into
	// synthetic "<board>について" subjects.
	renderer := &interactiveTitleFirstTestRenderer{titleFirstTestRenderer: titleFirstTestRenderer{
		reject:            true,
		fakeBoardRenderer: fakeBoardRenderer{draft: llm.BoardPostDraft{Author: "WRONG", Subject: "WRONG", Body: "本文です。"}},
	}}
	repo := New(base, conversationViewEvidenceEngine{}, LLMMaterializer{Renderer: renderer}, "1996-08-29")
	repo.EnableDevelopmentInteractiveTitleFirstPoC()
	host, err := repo.HostByPhone("0450000196")
	if err != nil {
		t.Fatal(err)
	}
	boards, _ := repo.MaterializationBoards(host)
	board := boards[0]
	posts, _ := repo.MaterializationPersonaArticleHeaders(host, board)

	value, ok := developmentSelectionTelemetry.Load(developmentPlanningKey{repo: repo, hostID: host.ID, boardID: board.ID})
	if !ok {
		t.Fatal("missing selection telemetry")
	}
	stats := value.(developmentSelectionStats)
	if stats.Roots == 0 {
		t.Fatalf("test world selected no roots: %+v", stats)
	}
	if len(posts) != 0 {
		t.Fatalf("failed title realization committed %d posts instead of failing atomically", len(posts))
	}
	diagnostic := repo.MaterializationPlanningDiagnostic(host.ID, board.ID)
	if !strings.Contains(diagnostic, "planning_error=") || !strings.Contains(diagnostic, "canned title fallback is disabled") {
		t.Fatalf("missing explicit no-canned-fallback planning error: %s", diagnostic)
	}
	for _, post := range base.ListPosts(host.ID) {
		if strings.Contains(post.Subject, board.Name+"について") || strings.Contains(post.Subject, board.Name+"の情報交換") {
			t.Fatalf("canned board-name fallback leaked into canonical history: %+v", post)
		}
	}
}

func TestInteractiveTitleFirstReplyGetsConcreteThreadAwareDetails(t *testing.T) {
	base := world.NewMemoryStore()
	renderer := &interactiveTitleFirstTestRenderer{titleFirstTestRenderer: titleFirstTestRenderer{
		fakeBoardRenderer: fakeBoardRenderer{draft: llm.BoardPostDraft{Author: "WRONG", Subject: "WRONG", Body: "自分側の具体的な経験を足した返信です。"}},
	}}
	repo := New(base, conversationViewEvidenceEngine{}, LLMMaterializer{Renderer: renderer}, "1996-08-29")
	repo.EnableDevelopmentInteractiveTitleFirstPoC()
	host, err := repo.HostByPhone("0450000196")
	if err != nil {
		t.Fatal(err)
	}
	persona := world.Persona{
		ID:              "p-mint",
		Handle:          "MINT-Y",
		Age:             25,
		Occupation:      "会社員",
		WritingStyle:    "短め。自分の経験がある時だけ少し具体的に書く。",
		EverydayContext: []string{"平日の夜に接続することが多い"},
		Interests:       map[string]float64{"games": .8},
		Opinions:        map[string]float64{},
	}
	base.SavePersona(persona)
	base.AddMembership(host.ID, persona.ID)
	base.SavePersonaFact(world.PersonaFact{
		PersonaID:      persona.ID,
		Key:            "habit.save_slots",
		Value:          "セーブ枠が複数ある時は用途を分けることがある",
		MaterializedAt: time.Date(1996, time.July, 10, 20, 0, 0, 0, time.FixedZone("JST", 9*60*60)),
	})
	base.SavePersonaFact(world.PersonaFact{
		PersonaID:      persona.ID,
		Key:            "future.fact",
		Value:          "この投稿時点ではまだ存在しない",
		MaterializedAt: time.Date(1996, time.August, 5, 20, 0, 0, 0, time.FixedZone("JST", 9*60*60)),
	})
	base.AddPost(host.ID, world.Post{
		BoardID:         "older",
		Author:          persona.Handle,
		AuthorPersonaID: persona.ID,
		Subject:         "前にもセーブで迷った",
		Body:            "三つある時は一つを戻りたい所用に残していました。",
		CreatedAt:       time.Date(1996, time.July, 12, 22, 0, 0, 0, time.FixedZone("JST", 9*60*60)),
		Intent:          world.PostIntent{Topic: "セーブ"},
	})
	board := world.Board{ID: "reply-detail", Name: "GAME"}
	root := base.AddPost(host.ID, world.Post{
		BoardID:   "reply-detail",
		Author:    "MARU",
		Subject:   "セーブの場所を決めてます",
		Body:      "進めてから残しておけばと思うことがあるので、場所を決めています。",
		CreatedAt: time.Date(1996, time.July, 19, 13, 47, 0, 0, time.FixedZone("JST", 9*60*60)),
		Intent: world.PostIntent{
			SituationKind:    "title_first",
			SituationSummary: "セーブする場所を先に決めている",
			SituationFacts:   []string{"title_first_subject=セーブの場所を決めてます"},
		},
	})
	reply := base.AddPost(host.ID, world.Post{
		BoardID:         "reply-detail",
		ParentID:        root.ID,
		Author:          "MINT-Y",
		AuthorPersonaID: persona.ID,
		CreatedAt:       time.Date(1996, time.July, 31, 9, 59, 0, 0, time.FixedZone("JST", 9*60*60)),
		Intent: world.PostIntent{
			DiscourseMode:    "reply",
			SituationKind:    "title_first",
			SituationSummary: "MINT-YがMARUの記事へ返信する",
			RespondsToPostID: root.ID,
			SourcePostID:     root.ID,
		},
	})

	rendered, found, bodyCreated, diagnostic := repo.MaterializationArticleWithDebug(host, board, reply.ID)
	if !found || !bodyCreated || strings.TrimSpace(rendered.Body) == "" {
		t.Fatalf("reply open did not materialize body: found=%v created=%v diagnostic=%s", found, bodyCreated, diagnostic)
	}
	if renderer.detailCalls != 1 {
		t.Fatalf("reply article detail calls=%d, want 1", renderer.detailCalls)
	}
	if len(renderer.detailReq.Articles) != 1 {
		t.Fatalf("reply detail request=%+v", renderer.detailReq)
	}
	seed := renderer.detailReq.Articles[0]
	if seed.Subject != root.Subject {
		t.Fatalf("reply semantic subject=%q, want parent subject %q", seed.Subject, root.Subject)
	}
	if !strings.Contains(seed.ThreadContext, "MARU") || !strings.Contains(seed.ThreadContext, root.Body) {
		t.Fatalf("reply detail planner lacks canonical thread context: %q", seed.ThreadContext)
	}
	if !strings.Contains(seed.AuthorHistory, "前にもセーブで迷った") || !strings.Contains(seed.AuthorHistory, "三つある時は一つを戻りたい所用") {
		t.Fatalf("reply detail planner lacks bounded self-history: %q", seed.AuthorHistory)
	}
	if strings.Contains(seed.AuthorHistory, root.Body) || strings.Contains(seed.AuthorHistory, "MARU") {
		t.Fatalf("another writer's thread contribution leaked into the reply author's history: %q", seed.AuthorHistory)
	}
	if !strings.Contains(strings.Join(seed.ExistingFacts, "\n"), "habit.save_slots=") {
		t.Fatalf("reply detail planner lacks time-valid persona fact: %+v", seed.ExistingFacts)
	}
	if strings.Contains(strings.Join(seed.ExistingFacts, "\n"), "future.fact=") {
		t.Fatalf("future persona fact leaked backward: %+v", seed.ExistingFacts)
	}
	if !strings.Contains(seed.PersonaProfile, "writing=") || !strings.Contains(seed.PersonaProfile, "everyday_baseline=") {
		t.Fatalf("reply detail planner lacks persona style/baseline: %s", seed.PersonaProfile)
	}
	if !hasInteractiveArticleDetails(rendered.Intent.SituationFacts) {
		t.Fatalf("reply details were not persisted before prose: %+v", rendered.Intent.SituationFacts)
	}
}
