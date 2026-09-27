package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"zutto-pccom/apps/server/internal/historicalkb"
	"zutto-pccom/apps/server/internal/llm"
	"zutto-pccom/apps/server/internal/world"
	"zutto-pccom/apps/server/internal/worldengine"
	"zutto-pccom/apps/server/internal/worldrepo"
)

type sharedFreshArticleProvider struct{ detailCalls, bodyCalls int }

func (p *sharedFreshArticleProvider) MaterializeBBSTitleArticleDetails(_ context.Context, req llm.BBSTitleArticleDetailRequest) (llm.BBSTitleArticleDetailDraft, error) {
	p.detailCalls++
	articles := make([]llm.BBSTitleArticleDetailSet, 0, len(req.Articles))
	for _, article := range req.Articles {
		articles = append(articles, llm.BBSTitleArticleDetailSet{EventID: article.EventID, Details: []llm.BBSArticleDetail{{Kind: "observation", Fact: "画面の右端に短い表示が残った"}}})
	}
	return llm.BBSTitleArticleDetailDraft{Articles: articles}, nil
}

func (p *sharedFreshArticleProvider) GenerateBoardPost(_ context.Context, req llm.BoardPostRequest) (llm.BoardPostDraft, error) {
	p.bodyCalls++
	return llm.BoardPostDraft{Author: req.AuthorHandle, Subject: req.CanonicalSubject, Body: "本文を確認しました。"}, nil
}

type sharedFreshArticleEvidence struct{}

func (sharedFreshArticleEvidence) ResolveEvidence(context.Context, worldengine.EvidenceRequest) (worldengine.EvidenceDecision, error) {
	return worldengine.EvidenceDecision{Level: historicalkb.EvidenceAtmospheric, Knowledge: historicalkb.KnowledgeResult{CanUse: true}}, nil
}

func TestNormalReadAndFreshLabUseSameArticleDetailPipeline(t *testing.T) {
	provider := &sharedFreshArticleProvider{}
	materializer := worldrepo.LLMMaterializer{Renderer: provider}
	lab := newMaterializationLab(nil, sharedFreshArticleEvidence{}, materializer, "1996-08-29", "")
	if lab.detailPlanner == nil {
		t.Fatal("Lab did not retain the shared Article Detail planner capability")
	}

	type readPath struct {
		name string
		read func(*worldrepo.Repository, world.Host, world.Board, int64) (world.Post, bool, bool, string)
	}
	paths := []readPath{
		{name: "normal", read: func(repo *worldrepo.Repository, host world.Host, board world.Board, id int64) (world.Post, bool, bool, string) {
			return repo.MaterializationArticleWithDebug(host, board, id)
		}},
		{name: "fresh-lab-isolated-copy", read: func(repo *worldrepo.Repository, host world.Host, board world.Board, id int64) (world.Post, bool, bool, string) {
			return repo.MaterializationArticleWithDebugTimeout(host, board, id, time.Second)
		}},
	}
	for _, path := range paths {
		t.Run(path.name, func(t *testing.T) {
			store := world.NewMemoryStore()
			host, err := store.HostByPhone("0450000196")
			if err != nil {
				t.Fatal(err)
			}
			board := world.Board{ID: "main", Name: "雑談"}
			post := store.AddPost(host.ID, world.Post{BoardID: board.ID, Author: "MARI", Subject: "画面の表示", Intent: world.PostIntent{SituationSummary: "表示を見た"}})
			repo := worldrepo.New(store, sharedFreshArticleEvidence{}, materializer, "1996-08-29")
			repo.SetArticleDetailPlanner(lab.detailPlanner)
			got, found, created, diagnostic := path.read(repo, host, board, post.ID)
			if !found || !created || got.Body == "" || !got.Intent.ArticleDetailsMaterialized || strings.Contains(diagnostic, "error stage=") {
				t.Fatalf("shared article read failed: post=%+v diagnostic=%s", got, diagnostic)
			}
			detailCount := 0
			for _, fact := range got.Intent.SituationFacts {
				if strings.HasPrefix(fact, "article_detail=") {
					detailCount++
				}
			}
			if detailCount != 1 {
				t.Fatalf("detail was not persisted through shared pipeline: %+v", got.Intent.SituationFacts)
			}
		})
	}
	if provider.detailCalls != 2 || provider.bodyCalls != 2 {
		t.Fatalf("normal/Lab provider calls detail=%d body=%d; want 2 each", provider.detailCalls, provider.bodyCalls)
	}
}

func TestTopicFirstRejectsIncompatibleTextureBeforeAdmission(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/?action=start&situation_mode=topic-first&historical_texture=off", nil)
	w := httptest.NewRecorder()
	(&materializationLab{}).handleFreshStart(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status=%d", w.Code)
	}
}

func TestFreshArticlesExposeTopicTargetAndMissingSubject(t *testing.T) {
	posts := []world.Post{{Subject: "このゲーム", Intent: world.PostIntent{SituationFacts: []string{"topic_target=架空ゲームA", "topic_target_status=selected"}}}}
	a := collectMaterializationFreshArticles(posts)[0]
	if a.TopicTarget != "架空ゲームA" || a.TopicTargetStatus != "selected" || a.SubjectTargetPresent {
		t.Fatalf("article=%+v", a)
	}
	posts[0].Subject = "架空ゲームAの話"
	if !collectMaterializationFreshArticles(posts)[0].SubjectTargetPresent {
		t.Fatal("named subject should match")
	}
}

func TestCollectMaterializationFreshArticlesPreservesProducerBrief(t *testing.T) {
	at := time.Date(1996, 8, 24, 23, 12, 0, 0, time.Local)
	posts := []world.Post{{
		ID:        1234,
		BoardID:   "2",
		ParentID:  1200,
		Author:    "NORI",
		CreatedAt: at,
		Subject:   "Re: 接続の件",
		Body:      "本文",
		Intent: world.PostIntent{
			Action:                  "reply",
			AnchorKey:               "communications",
			CauseKind:               "observed_thread",
			SourcePostID:            1220,
			RespondsToPostID:        1220,
			ProducerEventID:         "board-2:event-0003",
			ProducerEpisode:         "同じ症状について別の条件を確認した。",
			ProducerReferents:       []string{"同じ接続症状"},
			ProducerActorKnowledge:  []string{"NORIは別条件で再現した"},
			ProducerAudienceContext: []string{"直前の記事で症状が共有済み"},
			ProducerContribution:    []string{"別条件でも起きたことを追加する"},
			ProducerMustNot:         []string{"他人の体験として語らない"},
		},
	}}

	got := collectMaterializationFreshArticles(posts)
	if len(got) != 1 {
		t.Fatalf("articles=%d want 1", len(got))
	}
	a := got[0]
	if a.ID != 1234 || a.BoardID != "2" || a.ParentID != 1200 || a.Author != "NORI" || !a.CreatedAt.Equal(at) {
		t.Fatalf("identity fields not preserved: %+v", a)
	}
	if a.Subject != "Re: 接続の件" || a.Body != "本文" || a.Action != "reply" || a.SourcePostID != 1220 || a.RespondsToPostID != 1220 {
		t.Fatalf("article fields not preserved: %+v", a)
	}
	if a.ProducerEventID != "board-2:event-0003" || a.ProducerEpisode == "" || len(a.ProducerReferents) != 1 || len(a.ProducerActorKnowledge) != 1 || len(a.ProducerAudienceContext) != 1 || len(a.ProducerContribution) != 1 || len(a.ProducerMustNot) != 1 {
		t.Fatalf("producer brief not preserved: %+v", a)
	}

	posts[0].Intent.ProducerReferents[0] = "mutated"
	if got[0].ProducerReferents[0] != "同じ接続症状" {
		t.Fatalf("captured producer lists must be detached copies: %+v", got[0])
	}
}
