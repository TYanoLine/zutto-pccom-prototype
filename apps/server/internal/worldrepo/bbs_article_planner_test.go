package worldrepo

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"
	"sync/atomic"

	"zutto-pccom/apps/server/internal/bbsengine"
	"zutto-pccom/apps/server/internal/historicalkb"
	"zutto-pccom/apps/server/internal/llm"
	"zutto-pccom/apps/server/internal/world"
	"zutto-pccom/apps/server/internal/worldengine"
)

type fakeSharedTitleRenderer struct {
	contextCalls int
	lastContext  llm.BBSContextualTitleCandidateRequest
	titles       []string
	started      chan string
	release      <-chan struct{}
	active       atomic.Int32
	maxActive    atomic.Int32
}

func (f *fakeSharedTitleRenderer) GenerateBoardPost(context.Context, llm.BoardPostRequest) (llm.BoardPostDraft, error) {
	return llm.BoardPostDraft{Author: "X", Subject: "X", Body: "body"}, nil
}

func (f *fakeSharedTitleRenderer) GenerateBBSTitleCandidates(context.Context, string, string) (llm.BBSTitleCandidates, error) {
	return llm.BBSTitleCandidates{}, fmt.Errorf("legacy non-contextual candidate path should not be used")
}

func (f *fakeSharedTitleRenderer) GenerateContextualBBSTitleCandidates(ctx context.Context, req llm.BBSContextualTitleCandidateRequest) (llm.BBSTitleCandidates, error) {
	f.contextCalls++
	f.lastContext = req
	active := f.active.Add(1)
	defer f.active.Add(-1)
	for {
		max := f.maxActive.Load()
		if active <= max || f.maxActive.CompareAndSwap(max, active) {
			break
		}
	}
	if f.started != nil {
		f.started <- req.BoardName
	}
	if f.release != nil {
		select {
		case <-f.release:
		case <-ctx.Done():
			return llm.BBSTitleCandidates{}, ctx.Err()
		}
	}
	titles := append([]string(nil), f.titles...)
	if len(titles) == 0 {
		titles = []string{
			"バーチャ2のパイ", "ポケモン赤と緑", "サターンのパッド", "メモリーカード不足",
			"通信対戦してみたい", "中古ソフト売り場", "攻略本を買いました", "RPGで徹夜(^^;",
			"格ゲーのコマンド", "セーブデータ消失", "二人用ゲーム探し", "アーケード移植",
			"音ゲーじゃないけど", "SFCまだ現役", "PSのロード時間", "ゲーム雑誌の付録",
			"夏休みの一本", "シューティング苦手", "対戦相手募集", "エンディング後の話",
		}
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

type fallbackTitleTestEngine struct{}

func (fallbackTitleTestEngine) ResolveEvidence(_ context.Context, req worldengine.EvidenceRequest) (worldengine.EvidenceDecision, error) {
	if strings.HasPrefix(req.Subject, "bbs-title-era:") {
		// Simulate a selected proper-name candidate whose synchronous historical
		// verification did not finish. The planner must consume it and try the
		// next candidate from the already-generated pool.
		return worldengine.EvidenceDecision{
			Knowledge: historicalkb.KnowledgeResult{CanUse: false},
		}, nil
	}
	return worldengine.EvidenceDecision{
		Knowledge: historicalkb.KnowledgeResult{CanUse: true},
	}, nil
}

func (fallbackTitleTestEngine) AdviseTitleCandidates(_ context.Context, req worldengine.TitleCandidateAdviceRequest) (worldengine.TitleCandidateAdviceDecision, error) {
	out := worldengine.TitleCandidateAdviceDecision{
		Era: map[int]worldengine.TitleEraProbabilities{},
		Fit: map[string]float64{},
	}
	for i := range req.Titles {
		candidate := i + 1
		if candidate == 1 {
			out.Era[candidate] = worldengine.TitleEraProbabilities{
				SafeWithoutResearch: .10,
				LogicallyImpossible: .00,
			}
		} else {
			out.Era[candidate] = worldengine.TitleEraProbabilities{
				SafeWithoutResearch: .95,
				LogicallyImpossible: .00,
			}
		}
		for _, event := range req.Events {
			score := .10
			switch candidate {
			case 1:
				score = .90
			case 2:
				score = .80
			}
			out.Fit[worldengine.TitleCandidatePairKey(candidate, event.EventID)] = score
		}
	}
	return out, nil
}

func TestSharedBBSPlannerFallsBackWithinPoolWhenSelectedResearchCandidateIsUnverified(t *testing.T) {
	base := world.NewMemoryStore()
	host, err := base.HostByPhone("0920000196")
	if err != nil {
		t.Fatal(err)
	}
	titles := []string{
		"要研究の実在作品A",
		"攻略本なしで進める？",
	}
	for i := 3; i <= 20; i++ {
		titles = append(titles, fmt.Sprintf("低適合候補%02d", i))
	}
	renderer := &fakeSharedTitleRenderer{titles: titles}
	repo := New(base, fallbackTitleTestEngine{}, LLMMaterializer{
		Renderer: renderer,
	}, "1996-08-26")

	req := bbsengine.BatchRequest{
		Host:     host,
		Board:    world.Board{ID: "20/1", Name: "ＧＡＭＥ"},
		WorldNow: time.Date(1996, 8, 26, 23, 30, 0, 0, time.Local),
		Slots: []bbsengine.Slot{
			{Index: 1, Author: "MARI", AuthorPersonaID: "hakata-mari", CreatedAt: time.Date(1996, 8, 26, 22, 0, 0, 0, time.Local)},
		},
	}

	planned, err := (repositoryBBSBatchPlanner{repo: repo}).PlanBBSBatch(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if len(planned) != 1 {
		t.Fatalf("planned=%d, want 1", len(planned))
	}
	if planned[0].Subject != "攻略本なしで進める？" {
		t.Fatalf("subject=%q, want same-pool safe fallback", planned[0].Subject)
	}
	if renderer.contextCalls != 1 {
		t.Fatalf("candidate pools=%d, want reuse of one generated pool", renderer.contextCalls)
	}
}

func TestSharedBBSHeaderMaterializationSerializesBoardsPerHost(t *testing.T) {
	base := world.NewMemoryStore()
	host, err := base.HostByPhone("0920000196")
	if err != nil {
		t.Fatal(err)
	}
	release := make(chan struct{})
	renderer := &fakeSharedTitleRenderer{
		started: make(chan string, 2),
		release: release,
	}
	repo := New(base, nil, LLMMaterializer{Renderer: renderer}, "1996-08-26")
	repo.SetWorldNow(func() time.Time {
		return time.Date(1996, 8, 26, 23, 30, 0, 0, time.Local)
	})

	boardA := world.Board{ID: "4", Name: "ふり～と～く"}
	boardB := world.Board{ID: "20/1", Name: "ＧＡＭＥ"}
	repo.BeginHostObservation(host, []world.Board{boardA})
	select {
	case <-renderer.started:
	case <-time.After(time.Second):
		t.Fatal("first shared BBS board did not start")
	}
	repo.BeginHostObservation(host, []world.Board{boardB})

	select {
	case second := <-renderer.started:
		t.Fatalf("second board entered title generation concurrently: %s", second)
	case <-time.After(80 * time.Millisecond):
	}
	if got := renderer.maxActive.Load(); got != 1 {
		t.Fatalf("max concurrent title generators=%d, want 1", got)
	}

	close(release)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if _, err := repo.WaitForBoardHeaders(ctx, host, boardA); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.WaitForBoardHeaders(ctx, host, boardB); err != nil {
		t.Fatal(err)
	}
	if got := renderer.maxActive.Load(); got != 1 {
		t.Fatalf("max concurrent title generators after completion=%d, want 1", got)
	}
}
