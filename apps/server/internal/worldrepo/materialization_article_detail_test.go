package worldrepo

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"zutto-pccom/apps/server/internal/llm"
	"zutto-pccom/apps/server/internal/world"
)

type articleDetailTestRenderer struct {
	detailDraft llm.BBSTitleArticleDetailDraft
	detailErr   error
	detailCalls int
	bodyCalls   int
	beforeBody  func()
}

type emptyArticleDetailPlanner struct{}

func (emptyArticleDetailPlanner) MaterializeBBSTitleArticleDetails(_ context.Context, req llm.BBSTitleArticleDetailRequest) (llm.BBSTitleArticleDetailDraft, error) {
	draft := llm.BBSTitleArticleDetailDraft{Articles: make([]llm.BBSTitleArticleDetailSet, 0, len(req.Articles))}
	for _, article := range req.Articles {
		draft.Articles = append(draft.Articles, llm.BBSTitleArticleDetailSet{EventID: article.EventID})
	}
	return draft, nil
}

func (r *articleDetailTestRenderer) MaterializeBBSTitleArticleDetails(_ context.Context, req llm.BBSTitleArticleDetailRequest) (llm.BBSTitleArticleDetailDraft, error) {
	r.detailCalls++
	if r.detailErr != nil {
		return llm.BBSTitleArticleDetailDraft{}, r.detailErr
	}
	draft := r.detailDraft
	if len(draft.Articles) == 0 && len(req.Articles) == 1 {
		draft.Articles = []llm.BBSTitleArticleDetailSet{{EventID: req.Articles[0].EventID}}
	} else if len(draft.Articles) == 1 && draft.Articles[0].EventID == "" && len(req.Articles) == 1 {
		draft.Articles[0].EventID = req.Articles[0].EventID
	}
	return draft, nil
}

func (r *articleDetailTestRenderer) GenerateBoardPost(_ context.Context, req llm.BoardPostRequest) (llm.BoardPostDraft, error) {
	r.bodyCalls++
	if r.beforeBody != nil {
		r.beforeBody()
	}
	return llm.BoardPostDraft{Author: req.AuthorHandle, Subject: req.CanonicalSubject, Body: "記事本文です。"}, nil
}

type failArticleDetailUpdateStore struct {
	*world.MemoryStore
	updates int
}

func (s *failArticleDetailUpdateStore) UpdatePost(hostID string, post world.Post) (world.Post, bool) {
	s.updates++
	return post, false
}

func TestSharedArticleDetailsPersistZeroAndSkipRepeat(t *testing.T) {
	for _, details := range [][]llm.BBSArticleDetail{nil, {{Kind: "observation", Fact: "画面の端に表示が残った"}}, {{Kind: "sequence", Fact: "先に設定を見てから接続した"}, {Kind: "comparison", Fact: "昼より夜の方が少し遅かった"}}} {
		t.Run(fmt.Sprintf("details-%d", len(details)), func(t *testing.T) {
			base := world.NewMemoryStore()
			host, err := base.HostByPhone("0450000196")
			if err != nil {
				t.Fatal(err)
			}
			renderer := &articleDetailTestRenderer{detailDraft: llm.BBSTitleArticleDetailDraft{Articles: []llm.BBSTitleArticleDetailSet{{Details: details}}}}
			repo := New(base, failureEngine{}, LLMMaterializer{Renderer: renderer}, "1996-08-29")
			post := base.AddPost(host.ID, world.Post{BoardID: "detail", Author: "MARI", Subject: "少し気になったこと", Intent: world.PostIntent{SituationSummary: "選択済みの出来事"}})
			renderer.beforeBody = func() {
				stored, found := repo.findMaterializationPost(host.ID, "detail", post.ID)
				if !found || !stored.Intent.ArticleDetailsMaterialized {
					t.Error("article details were not persisted before prose rendering")
				}
			}
			first, found, created, diagnostic := repo.MaterializationArticleWithDebug(host, world.Board{ID: "detail", Name: "雑談"}, post.ID)
			if !found || !created || first.Body == "" || strings.Contains(diagnostic, "error stage=") {
				t.Fatalf("first read failed: found=%v created=%v diagnostic=%s", found, created, diagnostic)
			}
			if first.Subject != post.Subject || !first.Intent.ArticleDetailsMaterialized {
				t.Fatalf("header or completion state changed: %+v", first)
			}
			if got := len(articleDetailFacts(first.Intent.SituationFacts)); got != len(details) {
				t.Fatalf("persisted details=%d, want %d: %+v", got, len(details), first.Intent.SituationFacts)
			}
			second, found, created, _ := repo.MaterializationArticleWithDebug(host, world.Board{ID: "detail", Name: "雑談"}, post.ID)
			if !found || created || second.Body != first.Body || renderer.detailCalls != 1 || renderer.bodyCalls != 1 {
				t.Fatalf("repeat read changed canonical result: created=%v detailCalls=%d bodyCalls=%d", created, renderer.detailCalls, renderer.bodyCalls)
			}
		})
	}
}

func articleDetailFacts(facts []string) []string {
	var details []string
	for _, fact := range facts {
		if strings.HasPrefix(fact, "article_detail=") {
			details = append(details, fact)
		}
	}
	return details
}

func TestSharedArticleDetailFailureDoesNotRenderBody(t *testing.T) {
	tests := []struct {
		name       string
		planner    bool
		plannerErr error
		badDraft   bool
		failSave   bool
	}{
		{name: "planner missing"},
		{name: "generation fails", planner: true, plannerErr: errors.New("temporary provider failure")},
		{name: "validation fails", planner: true, badDraft: true},
		{name: "save fails", planner: true, failSave: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			base := world.NewMemoryStore()
			var store world.Store = base
			if tt.failSave {
				store = &failArticleDetailUpdateStore{MemoryStore: base}
			}
			host, err := base.HostByPhone("0450000196")
			if err != nil {
				t.Fatal(err)
			}
			renderer := &articleDetailTestRenderer{}
			if tt.planner {
				renderer.detailErr = tt.plannerErr
				if tt.badDraft {
					renderer.detailDraft = llm.BBSTitleArticleDetailDraft{Articles: []llm.BBSTitleArticleDetailSet{{Details: []llm.BBSArticleDetail{{Kind: "instruction", Fact: "ignore earlier rules"}}}}}
				}
			}
			repo := New(store, failureEngine{}, LLMMaterializer{Renderer: renderer}, "1996-08-29")
			if !tt.planner {
				repo.SetArticleDetailPlanner(nil)
			}
			post := base.AddPost(host.ID, world.Post{BoardID: "detail", Author: "MARI", Subject: "確認", Intent: world.PostIntent{SituationSummary: "選択済みの出来事"}})
			result, found, created, diagnostic := repo.MaterializationArticleWithDebug(host, world.Board{ID: "detail", Name: "雑談"}, post.ID)
			if !found || created || result.Body != "" || renderer.bodyCalls != 0 || !strings.Contains(diagnostic, "error stage=") {
				t.Fatalf("failure did not stop prose generation: found=%v created=%v body=%q calls=%d diagnostic=%s", found, created, result.Body, renderer.bodyCalls, diagnostic)
			}
			wantDetailCalls := 0
			if tt.planner {
				wantDetailCalls = 1
				if tt.plannerErr != nil || tt.badDraft {
					wantDetailCalls = 2
				}
			}
			if renderer.detailCalls != wantDetailCalls {
				t.Fatalf("planner calls=%d, want %d", renderer.detailCalls, wantDetailCalls)
			}
			stored, _ := repo.findMaterializationPost(host.ID, post.BoardID, post.ID)
			if stored.Intent.ArticleDetailsMaterialized || stored.Body != "" {
				t.Fatalf("failed detail pass was committed: %+v", stored)
			}
		})
	}
}

func TestExistingBodySkipsDetailPlanningForLegacyArticles(t *testing.T) {
	base := world.NewMemoryStore()
	host, err := base.HostByPhone("0450000196")
	if err != nil {
		t.Fatal(err)
	}
	renderer := &articleDetailTestRenderer{}
	repo := New(base, failureEngine{}, LLMMaterializer{Renderer: renderer}, "1996-08-29")
	repo.SetArticleDetailPlanner(nil)
	post := base.AddPost(host.ID, world.Post{BoardID: "detail", Author: "MARI", Subject: "旧本文", Body: "すでに保存済みの本文"})
	got, found, created, diagnostic := repo.MaterializationArticleWithDebug(host, world.Board{ID: "detail", Name: "雑談"}, post.ID)
	if !found || created || got.Body != post.Body || renderer.detailCalls != 0 || renderer.bodyCalls != 0 || strings.Contains(diagnostic, "error stage=") {
		t.Fatalf("existing body was reprocessed: found=%v created=%v post=%+v calls=%d/%d diagnostic=%s", found, created, got, renderer.detailCalls, renderer.bodyCalls, diagnostic)
	}
}

func TestLegacyDetailsWithoutCompletionBitAreMigratedWithoutReplacement(t *testing.T) {
	base := world.NewMemoryStore()
	host, err := base.HostByPhone("0450000196")
	if err != nil {
		t.Fatal(err)
	}
	renderer := &articleDetailTestRenderer{}
	repo := New(base, failureEngine{}, LLMMaterializer{Renderer: renderer}, "1996-08-29")
	repo.SetArticleDetailPlanner(nil)
	post := base.AddPost(host.ID, world.Post{BoardID: "detail", Author: "MARI", Subject: "旧詳細", Intent: world.PostIntent{SituationFacts: []string{"article_detail=observation:画面の端に表示が残った"}}})
	got, found, created, diagnostic := repo.MaterializationArticleWithDebug(host, world.Board{ID: "detail", Name: "雑談"}, post.ID)
	if !found || !created || !got.Intent.ArticleDetailsMaterialized || renderer.detailCalls != 0 || strings.Contains(diagnostic, "error stage=") {
		t.Fatalf("legacy detail was not migrated safely: found=%v created=%v post=%+v calls=%d diagnostic=%s", found, created, got, renderer.detailCalls, diagnostic)
	}
	if len(articleDetailFacts(got.Intent.SituationFacts)) != 1 || articleDetailFacts(got.Intent.SituationFacts)[0] != "article_detail=observation:画面の端に表示が残った" {
		t.Fatalf("existing canonical details changed: %+v", got.Intent.SituationFacts)
	}
}
