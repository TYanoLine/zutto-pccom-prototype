package worldrepo

import (
	"context"
	"strings"
	"testing"

	"zutto-pccom/apps/server/internal/llm"
	"zutto-pccom/apps/server/internal/world"
)

type interactiveTitleFirstTestRenderer struct {
	titleFirstTestRenderer
	detailCalls int
}

func (f *interactiveTitleFirstTestRenderer) MaterializeBBSTitleArticleDetails(ctx context.Context, req llm.BBSTitleArticleDetailRequest) (llm.BBSTitleArticleDetailDraft, error) {
	f.detailCalls++
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
