package worldrepo

import (
	"context"
	"errors"
	"strings"
	"testing"

	"zutto-pccom/apps/server/internal/historicalkb"
	"zutto-pccom/apps/server/internal/llm"
	"zutto-pccom/apps/server/internal/world"
	"zutto-pccom/apps/server/internal/worldengine"
)

func TestDevelopmentMaterializerDoesNotCommitGenericFallback(t *testing.T) {
	renderer := &fakeBoardRenderer{err: errors.New("model unavailable")}
	fallback := &failingFallback{}
	m := LLMMaterializer{Renderer: renderer, Fallback: fallback}

	posts, err := m.GenerateBoardPosts(context.Background(), BoardMaterializationRequest{
		Host:      world.Host{SoftwareID: "materialization-demo"},
		BoardTopic: "みなさんの98環境",
		WorldDate: "1996-08-29",
	}, worldengine.EvidenceDecision{
		Level:     historicalkb.EvidenceAtmospheric,
		Knowledge: historicalkb.KnowledgeResult{CanUse: true},
	})
	if err == nil || !strings.Contains(err.Error(), "model unavailable") {
		t.Fatalf("development renderer failure was hidden: posts=%#v err=%v", posts, err)
	}
	if fallback.calls != 0 {
		t.Fatalf("development host committed generic fallback: calls=%d", fallback.calls)
	}
	if len(posts) != 0 {
		t.Fatalf("development host returned fallback posts: %#v", posts)
	}
}

type failureEngine struct{}

func (failureEngine) ResolveEvidence(_ context.Context, _ worldengine.EvidenceRequest) (worldengine.EvidenceDecision, error) {
	return worldengine.EvidenceDecision{
		Level:     historicalkb.EvidenceAtmospheric,
		Knowledge: historicalkb.KnowledgeResult{CanUse: true},
	}, nil
}

func TestDevelopmentArticleSurfacesRendererFailureAndKeepsBodyEmpty(t *testing.T) {
	base := world.NewMemoryStore()
	renderer := &fakeBoardRenderer{err: errors.New("context deadline exceeded")}
	repo := New(base, failureEngine{}, LLMMaterializer{Renderer: renderer, Fallback: FallbackMaterializer{}}, "1996-08-29")
	h, err := repo.HostByPhone("0450000196")
	if err != nil {
		t.Fatal(err)
	}
	boards, _ := repo.MaterializationBoards(h)
	posts, _ := repo.MaterializationPersonaArticleHeaders(h, boards[1])
	if len(posts) == 0 {
		t.Fatal("no development posts")
	}

	post, found, created, diagnostic := repo.MaterializationArticleWithDebug(h, boards[1], posts[0].ID)
	if !found || created {
		t.Fatalf("found=%v created=%v", found, created)
	}
	if post.Body != "" {
		t.Fatalf("failed renderer body was committed: %q", post.Body)
	}
	if !strings.Contains(diagnostic, "error stage=renderer") || !strings.Contains(diagnostic, "context deadline exceeded") {
		t.Fatalf("renderer failure not surfaced: %q", diagnostic)
	}
	if stored := base.ListPosts(h.ID); len(stored) == 0 || stored[0].Body != "" {
		t.Fatalf("canonical envelope was not kept retryable: %#v", stored)
	}
}

var _ llm.BoardPostRenderer = (*fakeBoardRenderer)(nil)


type retryOnceBoardRenderer struct {
	calls int
}

func (r *retryOnceBoardRenderer) GenerateBoardPost(_ context.Context, req llm.BoardPostRequest) (llm.BoardPostDraft, error) {
	r.calls++
	if r.calls == 1 {
		return llm.BoardPostDraft{}, errors.New("temporary structured response failure")
	}
	return llm.BoardPostDraft{Author: req.AuthorHandle, Subject: req.CanonicalSubject, Body: "二回目で生成できた本文です。"}, nil
}

func TestDevelopmentArticleRetriesOneTransientRendererFailure(t *testing.T) {
	base := world.NewMemoryStore()
	renderer := &retryOnceBoardRenderer{}
	repo := New(base, failureEngine{}, LLMMaterializer{Renderer: renderer}, "1996-08-29")
	host, err := repo.HostByPhone("0450000196")
	if err != nil {
		t.Fatal(err)
	}
	board := world.Board{ID: "retry-test", Name: "再試行テスト"}
	post := base.AddPost(host.ID, world.Post{
		BoardID: "retry-test",
		Author:  "MARI",
		Subject: "本文生成",
		Intent: world.PostIntent{
			SituationSummary: "本文生成の一時失敗を再試行する",
		},
	})

	got, found, created, diagnostic := repo.MaterializationArticleWithDebug(host, board, post.ID)
	if !found || !created {
		t.Fatalf("found=%v created=%v diagnostic=%s", found, created, diagnostic)
	}
	if renderer.calls != 2 {
		t.Fatalf("renderer calls=%d, want 2", renderer.calls)
	}
	if got.Body != "二回目で生成できた本文です。" {
		t.Fatalf("body=%q", got.Body)
	}
}
