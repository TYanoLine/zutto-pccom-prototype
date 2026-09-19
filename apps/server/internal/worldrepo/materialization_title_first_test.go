package worldrepo

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"zutto-pccom/apps/server/internal/historicalkb"
	"zutto-pccom/apps/server/internal/llm"
	"zutto-pccom/apps/server/internal/world"
	"zutto-pccom/apps/server/internal/worldengine"
)

type titleFirstTestRenderer struct {
	fakeBoardRenderer
	calls          int
	reviewCalls    int
	reject         bool
	malformedFirst bool
	eraStatuses    map[int]string
	reviewedTitles []string
	detailErr      bool
}

func (f *titleFirstTestRenderer) GenerateBBSTitleCandidates(context.Context, string, string) (llm.BBSTitleCandidates, error) {
	f.calls++
	titles := []string{}
	for i := 0; i < 20; i++ {
		titles = append(titles, fmt.Sprintf("話題%d", i))
	}
	return llm.BBSTitleCandidates{Titles: titles}, nil
}
func (f *titleFirstTestRenderer) ValidateBBSTitleEra(_ context.Context, r llm.BBSTitleEraRequest) (llm.BBSTitleEraReview, error) {
	decisions := make([]llm.BBSTitleEraDecision, 0, len(r.Titles))
	for i := range r.Titles {
		status := llm.BBSTitleEraOK
		if f.eraStatuses != nil && f.eraStatuses[i+1] != "" {
			status = f.eraStatuses[i+1]
		}
		decisions = append(decisions, llm.BBSTitleEraDecision{Candidate: i + 1, Status: status, Reason: "era-test"})
	}
	return llm.BBSTitleEraReview{Decisions: decisions}, nil
}
func (f *titleFirstTestRenderer) ReviewBBSTitleCandidates(_ context.Context, r llm.BBSTitleReviewRequest) (llm.BBSTitleReview, error) {
	f.reviewCalls++
	f.reviewedTitles = append(f.reviewedTitles, r.Titles...)
	decisions := []llm.BBSTitleDecision{}
	for i := range r.Titles {
		d := llm.BBSTitleDecision{Candidate: i + 1, Reason: "適合枠なし"}
		if i == 0 && len(r.Events) > 0 && !f.reject {
			if f.malformedFirst && f.reviewCalls == 1 {
				d.Reason = "検査結果不備：モデルが理由を返さなかったため不採用"
			} else {
				d.EventID = r.Events[0].EventID
				d.Subject = r.Titles[i]
				d.Summary = "感想を共有"
				d.Reason = "整合"
			}
		}
		decisions = append(decisions, d)
	}
	return llm.BBSTitleReview{Decisions: decisions}, nil
}

func (f *titleFirstTestRenderer) MaterializeBBSTitleArticleDetails(_ context.Context, r llm.BBSTitleArticleDetailRequest) (llm.BBSTitleArticleDetailDraft, error) {
	if f.detailErr {
		return llm.BBSTitleArticleDetailDraft{}, fmt.Errorf("detail planner unavailable")
	}
	articles := make([]llm.BBSTitleArticleDetailSet, 0, len(r.Articles))
	for _, seed := range r.Articles {
		articles = append(articles, llm.BBSTitleArticleDetailSet{EventID: seed.EventID, Details: []llm.BBSArticleDetail{
			{Kind: "locator", Fact: "手元の資料の142ページ、一覧表の3行目だった"},
			{Kind: "comparison", Fact: "資料の表記と画面で確認した表記が食い違っていた"},
		}})
	}
	return llm.BBSTitleArticleDetailDraft{Articles: articles}, nil
}

func TestTitleFirstPreservesSemanticSubjectAndArchivesRejectedCandidates(t *testing.T) {
	base := world.NewMemoryStore()
	renderer := &titleFirstTestRenderer{fakeBoardRenderer: fakeBoardRenderer{draft: llm.BoardPostDraft{Author: "WRONG", Subject: "書き換えられた件名", Body: "感想です。"}}}
	repo := New(base, conversationViewEvidenceEngine{}, LLMMaterializer{Renderer: renderer}, "1996-08-29")
	repo.EnableDevelopmentConversationViewPoC()
	repo.EnableDevelopmentTitleFirstPoC(nil)
	host, err := repo.HostByPhone("0450000196")
	if err != nil {
		t.Fatal(err)
	}
	boards, _ := repo.MaterializationBoards(host)
	repo.MaterializationPersonaArticleHeaders(host, boards[0])
	rows := repo.DevelopmentTitleCandidates()
	if len(rows) == 0 || len(rows)%20 != 0 {
		t.Fatalf("missing candidate pool: %v", rows)
	}
	if rows[0].EraStatus != "ok" {
		t.Fatalf("era status not recorded: %+v", rows[0])
	}
	all := base.ListPosts(host.ID)
	foundRoot := false
	for _, post := range all {
		if post.ParentID != 0 || post.Intent.SourcePostID != 0 {
			if titleFirstSubject(post.Intent.SituationFacts) != "" {
				t.Fatal("reply inherited fixed root subject")
			}
			continue
		}
		foundRoot = true
		if post.Subject != "話題0" {
			t.Fatal(post.Subject)
		}
		if post.Intent.SituationKind != "title_first" || post.Intent.SituationSummary != "感想を共有" {
			t.Fatalf("accepted title was not promoted to canonical title-first event: %+v", post.Intent)
		}
		hasWorldAdoption := false
		hasAdoptedSummary := false
		detailCount := 0
		for _, fact := range post.Intent.SituationFacts {
			if fact == "world_adoption=title_candidate" {
				hasWorldAdoption = true
			}
			if fact == "world_adopted_summary=感想を共有" {
				hasAdoptedSummary = true
			}
			if strings.HasPrefix(fact, "article_detail=") {
				detailCount++
			}
		}
		if !hasWorldAdoption || !hasAdoptedSummary || detailCount < 2 {
			t.Fatalf("missing world adoption/detail facts: %+v", post.Intent.SituationFacts)
		}
		semanticSubject := post.Subject
		rendered, found, created, diag := repo.MaterializationArticleWithDebug(host, world.Board{ID: post.BoardID, Name: "雑談"}, post.ID)
		if !found || !created || renderer.req.CanonicalSubject != semanticSubject {
			t.Fatalf("semantic subject was not preserved for rendering: %+v %s", rendered, diag)
		}
		if rendered.Subject != semanticSubject {
			t.Fatalf("accepted title was renamed by the prose renderer: got %q want %q; %+v %s", rendered.Subject, semanticSubject, rendered, diag)
		}
		if titleFirstSubject(rendered.Intent.SituationFacts) != semanticSubject {
			t.Fatalf("semantic title fact changed after surface realization: %+v", rendered.Intent.SituationFacts)
		}
		if !strings.Contains(renderer.req.PostIntent, "article_detail=") {
			t.Fatalf("article details were not supplied to body worker: %s", renderer.req.PostIntent)
		}
	}
	if !foundRoot {
		t.Fatal("no accepted root")
	}
	calls := renderer.calls
	repo.MaterializationPersonaArticleHeaders(host, boards[0])
	if renderer.calls != calls {
		t.Fatal("regenerated candidate pool")
	}
}

func TestTitleFirstDetailFailureKeepsAdoptedArticle(t *testing.T) {
	base := world.NewMemoryStore()
	renderer := &titleFirstTestRenderer{
		detailErr: true,
		fakeBoardRenderer: fakeBoardRenderer{draft: llm.BoardPostDraft{Author: "WRONG", Subject: "WRONG", Body: "本文です。"}},
	}
	repo := New(base, conversationViewEvidenceEngine{}, LLMMaterializer{Renderer: renderer}, "1996-08-29")
	repo.EnableDevelopmentConversationViewPoC()
	repo.EnableDevelopmentTitleFirstPoC(nil)
	host, err := repo.HostByPhone("0450000196")
	if err != nil {
		t.Fatal(err)
	}
	boards, _ := repo.MaterializationBoards(host)
	repo.MaterializationPersonaArticleHeaders(host, boards[0])

	rows := repo.DevelopmentTitleCandidates()
	foundAccepted := false
	for _, row := range rows {
		if row.Status == "accepted" || row.Status == "corrected" {
			foundAccepted = true
			if !strings.Contains(row.Reason, "採用記事は保持") {
				t.Fatalf("detail failure was not recorded without rejection: %+v", row)
			}
		}
	}
	if !foundAccepted {
		t.Fatal("detail failure removed every adopted title")
	}

	for _, post := range base.ListPosts(host.ID) {
		if post.ParentID != 0 || post.Intent.SourcePostID != 0 {
			continue
		}
		semanticSubject := titleFirstSubject(post.Intent.SituationFacts)
		if semanticSubject == "" || post.Subject != semanticSubject {
			t.Fatalf("adopted root lost canonical title: %+v", post)
		}
		rendered, found, _, diag := repo.MaterializationArticleWithDebug(host, world.Board{ID: post.BoardID, Name: "雑談"}, post.ID)
		if !found || strings.TrimSpace(rendered.Body) == "" {
			t.Fatalf("adopted root did not materialize prose: %+v %s", rendered, diag)
		}
	}
}

func TestTitleFirstAllRejectedDoesNotRegenerateOrCreateOrphans(t *testing.T) {
	base := world.NewMemoryStore()
	renderer := &titleFirstTestRenderer{reject: true}
	repo := New(base, nil, LLMMaterializer{Renderer: renderer}, "1996-08-29")
	repo.EnableDevelopmentConversationViewPoC()
	repo.EnableDevelopmentTitleFirstPoC(nil)
	host, err := repo.HostByPhone("0450000196")
	if err != nil {
		t.Fatal(err)
	}
	repo.materializeConversationWorldWindow(host)
	calls := renderer.calls
	repo.materializeConversationWorldWindow(host)
	if calls == 0 || renderer.calls != calls || len(base.ListPosts(host.ID)) != 0 {
		t.Fatalf("calls %d -> %d posts %d", calls, renderer.calls, len(base.ListPosts(host.ID)))
	}
}

type titleEraEvidenceResolver struct {
	claim string
	calls int
}

func (f *titleEraEvidenceResolver) ResolveEvidence(context.Context, worldengine.EvidenceRequest) (worldengine.EvidenceDecision, error) {
	f.calls++
	return worldengine.EvidenceDecision{
		Level: historicalkb.EvidenceVerified,
		Knowledge: historicalkb.KnowledgeResult{
			CanUse: true,
			Facts:  []historicalkb.HistoricalFact{{Status: historicalkb.FactVerified, Claim: f.claim}},
		},
	}, nil
}

func TestTitleFirstDoesNotResearchUnselectedEraCandidates(t *testing.T) {
	base := world.NewMemoryStore()
	// Candidate 1 is selected by the fake matcher; candidate 2 requires era
	// research but is never selected and therefore must not spend a search.
	renderer := &titleFirstTestRenderer{eraStatuses: map[int]string{2: llm.BBSTitleEraResearch}}
	resolver := &titleEraEvidenceResolver{claim: "ERA_OK: 基準日までに成立"}
	repo := New(base, resolver, LLMMaterializer{Renderer: renderer}, "1996-08-29")
	repo.EnableDevelopmentConversationViewPoC()
	repo.EnableDevelopmentTitleFirstPoC(nil)
	host, err := repo.HostByPhone("0450000196")
	if err != nil {
		t.Fatal(err)
	}
	repo.materializeConversationWorldWindow(host)
	if resolver.calls != 0 {
		t.Fatalf("unselected era candidates spent research calls: %d", resolver.calls)
	}
	rows := repo.DevelopmentTitleCandidates()
	foundNotNeeded := false
	for _, row := range rows {
		if row.Candidate == 2 && row.EraStatus == "not_needed" {
			foundNotNeeded = true
			break
		}
	}
	if !foundNotNeeded {
		t.Fatalf("unselected research candidate was not archived as not_needed: %+v", rows)
	}
}

func TestTitleFirstResearchNGRematchesSamePoolAndNeverBecomesPost(t *testing.T) {
	base := world.NewMemoryStore()
	renderer := &titleFirstTestRenderer{eraStatuses: map[int]string{1: llm.BBSTitleEraResearch}}
	resolver := &titleEraEvidenceResolver{claim: "ERA_NG: サービス開始は基準日より後"}
	repo := New(base, resolver, LLMMaterializer{Renderer: renderer}, "1996-08-29")
	repo.EnableDevelopmentConversationViewPoC()
	repo.EnableDevelopmentTitleFirstPoC(nil)
	host, err := repo.HostByPhone("0450000196")
	if err != nil {
		t.Fatal(err)
	}
	repo.materializeConversationWorldWindow(host)
	if resolver.calls == 0 {
		t.Fatal("selected era research was not invoked")
	}
	rows := repo.DevelopmentTitleCandidates()
	if len(rows) == 0 || rows[0].EraStatus != "ng" || rows[0].Status != "era_rejected" || rows[0].EraEvidence == "" {
		t.Fatalf("research rejection not archived: %+v", rows)
	}
	if renderer.reviewCalls < 2 {
		t.Fatalf("era rejection did not trigger rematch: %d", renderer.reviewCalls)
	}
	foundFallback := false
	for _, post := range base.ListPosts(host.ID) {
		if strings.Contains(post.Subject, "話題0") {
			t.Fatal("era-rejected title became a post")
		}
		if post.Subject == "話題1" {
			foundFallback = true
		}
	}
	if !foundFallback {
		t.Fatal("safe fallback title was not materialized")
	}
}

func TestTitleFirstMalformedDecisionRematchesWithoutRegeneratingPool(t *testing.T) {
	base := world.NewMemoryStore()
	renderer := &titleFirstTestRenderer{malformedFirst: true}
	repo := New(base, nil, LLMMaterializer{Renderer: renderer}, "1996-08-29")
	repo.EnableDevelopmentConversationViewPoC()
	repo.EnableDevelopmentTitleFirstPoC(nil)
	host, err := repo.HostByPhone("0450000196")
	if err != nil {
		t.Fatal(err)
	}
	repo.materializeConversationWorldWindow(host)
	if renderer.reviewCalls < 2 {
		t.Fatalf("malformed decision did not trigger rematch: %d", renderer.reviewCalls)
	}
	if renderer.calls == 0 {
		t.Fatal("candidate pool was not generated")
	}
	foundFallback := false
	for _, post := range base.ListPosts(host.ID) {
		if post.Subject == "話題1" {
			foundFallback = true
			break
		}
	}
	if !foundFallback {
		t.Fatal("malformed first choice was not replaced by next candidate")
	}
}

func TestTitleEraOutcomeRequiresVerifiedMarker(t *testing.T) {
	decision := worldengine.EvidenceDecision{Knowledge: historicalkb.KnowledgeResult{CanUse: true, Facts: []historicalkb.HistoricalFact{{Status: historicalkb.FactVerified, Claim: "ERA_OK: 1996年までに利用可能"}}}}
	got := developmentTitleEraOutcomeFromEvidence(decision)
	if got.status != "verified" || got.evidence == "" {
		t.Fatalf("unexpected outcome: %+v", got)
	}
	decision.Knowledge.Facts[0].Claim = "確認できたがマーカーなし"
	if got := developmentTitleEraOutcomeFromEvidence(decision); got.status != "unverified" {
		t.Fatalf("accepted unmarked evidence: %+v", got)
	}
}
