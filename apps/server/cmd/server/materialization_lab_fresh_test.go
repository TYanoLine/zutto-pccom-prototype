package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"zutto-pccom/apps/server/internal/world"
)

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
