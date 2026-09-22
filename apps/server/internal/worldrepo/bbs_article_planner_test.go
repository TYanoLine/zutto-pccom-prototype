package worldrepo

import (
	"context"
	"fmt"
	"testing"
	"time"

	"zutto-pccom/apps/server/internal/bbsengine"
	"zutto-pccom/apps/server/internal/llm"
	"zutto-pccom/apps/server/internal/world"
)

type fakeSharedTitleRenderer struct {
	contextCalls int
	lastContext  llm.BBSContextualTitleCandidateRequest
}

func (f *fakeSharedTitleRenderer) GenerateBoardPost(context.Context, llm.BoardPostRequest) (llm.BoardPostDraft, error) {
	return llm.BoardPostDraft{Author: "X", Subject: "X", Body: "body"}, nil
}

func (f *fakeSharedTitleRenderer) GenerateBBSTitleCandidates(context.Context, string, string) (llm.BBSTitleCandidates, error) {
	return llm.BBSTitleCandidates{}, fmt.Errorf("legacy non-contextual candidate path should not be used")
}

func (f *fakeSharedTitleRenderer) GenerateContextualBBSTitleCandidates(_ context.Context, req llm.BBSContextualTitleCandidateRequest) (llm.BBSTitleCandidates, error) {
	f.contextCalls++
	f.lastContext = req
	titles := []string{
		"バーチャ2のパイ", "ポケモン赤と緑", "サターンのパッド", "メモリーカード不足",
		"通信対戦してみたい", "中古ソフト売り場", "攻略本を買いました", "RPGで徹夜(^^;",
		"格ゲーのコマンド", "セーブデータ消失", "二人用ゲーム探し", "アーケード移植",
		"音ゲーじゃないけど", "SFCまだ現役", "PSのロード時間", "ゲーム雑誌の付録",
		"夏休みの一本", "シューティング苦手", "対戦相手募集", "エンディング後の話",
	}
	return llm.BBSTitleCandidates{Titles: titles}, nil
}

func (f *fakeSharedTitleRenderer) ReviewBBSTitleCandidates(_ context.Context, req llm.BBSTitleReviewRequest) (llm.BBSTitleReview, error) {
	decisions := make([]llm.BBSTitleDecision, 0, len(req.Titles))
	eventIndex := 0
	for i, title := range req.Titles {
		d := llm.BBSTitleDecision{Candidate: i + 1, Reason: "not selected"}
		if eventIndex < len(req.Events) {
			d.EventID = req.Events[eventIndex].EventID
			d.Subject = title
			d.Summary = "「" + title + "」を話題にする"
			d.Details = []string{}
			d.Reason = "fits selected world slot"
			eventIndex++
		}
		decisions = append(decisions, d)
	}
	return llm.BBSTitleReview{Decisions: decisions}, nil
}

func (f *fakeSharedTitleRenderer) ValidateBBSTitleEra(_ context.Context, req llm.BBSTitleEraRequest) (llm.BBSTitleEraReview, error) {
	out := make([]llm.BBSTitleEraDecision, 0, len(req.Titles))
	for i := range req.Titles {
		out = append(out, llm.BBSTitleEraDecision{Candidate: i + 1, Status: llm.BBSTitleEraOK, Reason: "test-safe"})
	}
	return llm.BBSTitleEraReview{Decisions: out}, nil
}

func TestSharedBBSPlannerUsesContextualTitleFirstPool(t *testing.T) {
	base := world.NewMemoryStore()
	host, err := base.HostByPhone("0920000196")
	if err != nil {
		t.Fatal(err)
	}
	renderer := &fakeSharedTitleRenderer{}
	repo := New(base, nil, LLMMaterializer{
		Renderer:                    renderer,
		CuratedHistoricalReferences: true,
	}, "1996-08-26")

	recent := world.Post{
		ID:        10,
		BoardID:   "20/1",
		Author:    "MARI",
		Subject:   "前に出たゲーム話",
		CreatedAt: time.Date(1996, 8, 26, 20, 0, 0, 0, time.Local),
	}
	req := bbsengine.BatchRequest{
		Host:       host,
		Board:      world.Board{ID: "20/1", Name: "ＧＡＭＥ"},
		WorldNow:   time.Date(1996, 8, 26, 23, 30, 0, 0, time.Local),
		RecentPosts: []world.Post{recent},
		Slots: []bbsengine.Slot{
			{Index: 1, Author: "MARI", AuthorPersonaID: "hakata-mari", CreatedAt: time.Date(1996, 8, 26, 21, 0, 0, 0, time.Local)},
			{Index: 2, Author: "KAZU", AuthorPersonaID: "hakata-kazu", CreatedAt: time.Date(1996, 8, 26, 22, 0, 0, 0, time.Local)},
			{Index: 3, Author: "NORI", AuthorPersonaID: "hakata-nori", CreatedAt: time.Date(1996, 8, 26, 23, 0, 0, 0, time.Local)},
		},
	}

	planned, err := (repositoryBBSBatchPlanner{repo: repo}).PlanBBSBatch(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if renderer.contextCalls != 1 {
		t.Fatalf("contextual title pool calls=%d, want 1", renderer.contextCalls)
	}
	if renderer.lastContext.RecentBBSState == "" {
		t.Fatal("recent BBS state was not supplied to candidate generation")
	}
	foundRecent := false
	for _, subject := range renderer.lastContext.RecentSubjects {
		if subject == recent.Subject {
			foundRecent = true
		}
	}
	if !foundRecent {
		t.Fatalf("recent subject missing from candidate context: %+v", renderer.lastContext.RecentSubjects)
	}
	if len(planned) != 3 {
		t.Fatalf("planned=%d, want 3", len(planned))
	}
	for _, post := range planned {
		if post.Subject == "" || post.SituationSummary == "" {
			t.Fatalf("adopted root is incomplete: %+v", post)
		}
	}
}
