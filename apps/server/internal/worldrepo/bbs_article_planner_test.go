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
	refillTitles     []string
	historicalClaims []llm.BBSTitleHistoricalClaim
	started          chan string
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
	if req.PreferEraSafe && len(f.refillTitles) > 0 {
		titles = append([]string(nil), f.refillTitles...)
	}
	if len(titles) == 0 {
		titles = []string{
			"バーチャ2のパイ", "ポケモン赤と緑", "サターンのパッド", "メモリーカード不足",
			"通信対戦してみたい", "中古ソフト売り場", "攻略本を買いました", "RPGで徹夜(^^;",
			"格ゲーのコマンド", "セーブデータ消失", "二人用ゲーム探し", "アーケード移植",
			"音ゲーじゃないけど", "SFCまだ現役", "PSのロード時間", "ゲーム雑誌の付録",
			"夏休みの一本", "シューティング苦手", "対戦相手募集", "エンディング後の話",
		}
	}
	if f.started != nil {
		// Concurrency tests can now exercise activity plans larger than one
		// 20-title pool. A real provider is asked for fresh candidates on refill;
		// make this blocking fake behave the same way instead of repeating the
		// identical pool forever.
		offset := len(req.AvoidSubjects)
		titles = make([]string, 0, 20)
		for i := 0; i < 20; i++ {
			titles = append(titles, fmt.Sprintf("候補%03d", offset+i+1))
		}
	}
	return llm.BBSTitleCandidates{
		Titles:           titles,
		HistoricalClaims: append([]llm.BBSTitleHistoricalClaim(nil), f.historicalClaims...),
	}, nil
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

func TestSharedBBSPlannerUsesEarliestSlotDateForHistoricalTitlePool(t *testing.T) {
	base := world.NewMemoryStore()
	host, err := base.HostByPhone("0920000196")
	if err != nil {
		t.Fatal(err)
	}
	renderer := &fakeSharedTitleRenderer{}
	repo := New(base, nil, LLMMaterializer{Renderer: renderer, CuratedHistoricalReferences: true}, "1996-08-26")
	req := bbsengine.BatchRequest{
		Host:     host,
		Board:    world.Board{ID: "20/1", Name: "ＧＡＭＥ"},
		WorldNow: time.Date(1996, 8, 26, 23, 30, 0, 0, time.Local),
		Slots: []bbsengine.Slot{
			{Index: 1, Author: "MARI", CreatedAt: time.Date(1996, 7, 30, 21, 0, 0, 0, time.Local)},
			{Index: 2, Author: "KAZU", CreatedAt: time.Date(1996, 8, 20, 22, 0, 0, 0, time.Local)},
		},
	}
	if _, err := (repositoryBBSBatchPlanner{repo: repo}).PlanBBSBatch(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	if renderer.lastContext.WorldDate != "1996-07-30" {
		t.Fatalf("title as-of=%q, want earliest event date", renderer.lastContext.WorldDate)
	}
}

func TestSharedBBSPlannerPlansReplyToRootFromSameWindow(t *testing.T) {
	base := world.NewMemoryStore()
	host, err := base.HostByPhone("0920000196")
	if err != nil {
		t.Fatal(err)
	}
	renderer := &fakeSharedTitleRenderer{}
	repo := New(base, nil, LLMMaterializer{Renderer: renderer}, "1996-08-26")
	req := bbsengine.BatchRequest{
		Host:     host,
		Board:    world.Board{ID: "20/1", Name: "ＧＡＭＥ"},
		WorldNow: time.Date(1996, 8, 26, 23, 30, 0, 0, time.Local),
		Slots: []bbsengine.Slot{
			{Index: 1, Author: "MARI", AuthorPersonaID: "hakata-mari", CreatedAt: time.Date(1996, 8, 20, 21, 0, 0, 0, time.Local)},
			{Index: 2, Author: "KAZU", AuthorPersonaID: "hakata-kazu", CreatedAt: time.Date(1996, 8, 20, 22, 0, 0, 0, time.Local), ReplyToSlotIndex: 1, ReplyToAuthor: "MARI"},
		},
	}
	planned, err := (repositoryBBSBatchPlanner{repo: repo}).PlanBBSBatch(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if len(planned) != 2 {
		t.Fatalf("planned=%d, want root + reply", len(planned))
	}
	root := planned[0]
	reply := planned[1]
	if strings.TrimSpace(root.Subject) == "" {
		t.Fatal("same-window root has no generated subject")
	}
	if reply.Subject != root.Subject {
		t.Fatalf("reply planner proposed subject=%q, want semantic root topic %q before host projection", reply.Subject, root.Subject)
	}
	if reply.Topic != root.Subject {
		t.Fatalf("reply topic=%q, want adopted root subject %q", reply.Topic, root.Subject)
	}
	if !strings.Contains(reply.SituationSummary, root.Subject) || !strings.Contains(reply.SituationSummary, "MARI") {
		t.Fatalf("reply summary does not reference same-window root: %q", reply.SituationSummary)
	}
}

func TestRepositoryWiresErikaReplyProjectionWithoutSyntheticSubject(t *testing.T) {
	base := world.NewMemoryStore()
	host, err := base.HostByPhone("0920000196")
	if err != nil {
		t.Fatal(err)
	}
	repo := New(base, nil, LLMMaterializer{}, "1996-08-26")
	if repo.bbsArticles == nil || repo.bbsArticles.ReplyProjector == nil {
		t.Fatal("repository did not install host-program reply projector")
	}
	source := world.Post{ID: 77, BoardID: "20/1", Subject: "元記事"}
	projected, err := repo.bbsArticles.ReplyProjector.ProjectReply(host, source, "提案件名")
	if err != nil {
		t.Fatal(err)
	}
	if projected.ParentID != source.ID || projected.Subject != "" {
		t.Fatalf("Erika projection=%+v, want append child with no independent subject", projected)
	}
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
		Board:      world.Board{ID: "20/1", Name: "ＧＡＭＥ", SemanticScope: "ゲームについての板"},
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
	if renderer.lastContext.BoardScope != "ゲームについての板" {
		t.Fatalf("board scope=%q, want hidden semantic scope", renderer.lastContext.BoardScope)
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

type noSafeTitleTestEngine struct{}

func (noSafeTitleTestEngine) ResolveEvidence(_ context.Context, req worldengine.EvidenceRequest) (worldengine.EvidenceDecision, error) {
	if strings.HasPrefix(req.Subject, "bbs-title-era:") {
		return worldengine.EvidenceDecision{Knowledge: historicalkb.KnowledgeResult{CanUse: false}}, nil
	}
	return worldengine.EvidenceDecision{}, nil
}

func (noSafeTitleTestEngine) AdviseTitleCandidates(_ context.Context, req worldengine.TitleCandidateAdviceRequest) (worldengine.TitleCandidateAdviceDecision, error) {
	out := worldengine.TitleCandidateAdviceDecision{
		Era: map[int]worldengine.TitleEraProbabilities{},
		Fit: map[string]float64{},
	}
	for i := range req.Titles {
		candidate := i + 1
		out.Era[candidate] = worldengine.TitleEraProbabilities{
			SafeWithoutResearch: .05,
			LogicallyImpossible: .00,
		}
		for _, event := range req.Events {
			out.Fit[worldengine.TitleCandidatePairKey(candidate, event.EventID)] = .95
		}
	}
	return out, nil
}

type refillTitleTestEngine struct{ noSafeTitleTestEngine }

func (refillTitleTestEngine) AdviseTitleCandidates(_ context.Context, req worldengine.TitleCandidateAdviceRequest) (worldengine.TitleCandidateAdviceDecision, error) {
	out := worldengine.TitleCandidateAdviceDecision{
		Era: map[int]worldengine.TitleEraProbabilities{},
		Fit: map[string]float64{},
	}
	for i, title := range req.Titles {
		candidate := i + 1
		safe := .05
		if strings.HasPrefix(title, "補充:") {
			safe = .95
		}
		out.Era[candidate] = worldengine.TitleEraProbabilities{
			SafeWithoutResearch: safe,
			LogicallyImpossible: .00,
		}
		for _, event := range req.Events {
			out.Fit[worldengine.TitleCandidatePairKey(candidate, event.EventID)] = .95
		}
	}
	return out, nil
}

func TestSharedBBSPlannerGeneratesEraSafeRefillInsteadOfCannedSubjects(t *testing.T) {
	base := world.NewMemoryStore()
	host, err := base.HostByPhone("0920000196")
	if err != nil {
		t.Fatal(err)
	}
	researchTitles := make([]string, 0, 20)
	for i := 1; i <= 20; i++ {
		researchTitles = append(researchTitles, fmt.Sprintf("要調査候補%02d", i))
	}
	refillTitles := []string{
		"補充:パッドの調子", "補充:セーブが消えた", "補充:昨日の続き", "補充:説明書なくした",
		"補充:二人で遊ぶなら", "補充:夜中までやってた", "補充:対戦ありがとう", "補充:貸したソフト",
		"補充:このボス強い", "補充:名前入力で悩む", "補充:攻略本なしで", "補充:中古で見かけた",
		"補充:ロード待ちの間", "補充:弟と対戦中", "補充:クリアしたので", "補充:雑誌の付録",
		"補充:コントローラ故障", "補充:週末の一本", "補充:また最初から", "補充:対戦相手募集",
	}
	renderer := &fakeSharedTitleRenderer{titles: researchTitles, refillTitles: refillTitles}
	repo := New(base, noSafeTitleTestEngine{}, LLMMaterializer{Renderer: renderer}, "1996-08-26")

	slots := make([]bbsengine.Slot, 0, 7)
	for i := 0; i < 7; i++ {
		slots = append(slots, bbsengine.Slot{
			Index:     i + 1,
			Author:    fmt.Sprintf("USER%02d", i+1),
			CreatedAt: time.Date(1996, 8, 26, 20, i, 0, 0, time.Local),
		})
	}
	planned, err := (repositoryBBSBatchPlanner{repo: repo}).PlanBBSBatch(context.Background(), bbsengine.BatchRequest{
		Host:     host,
		Board:    world.Board{ID: "20/1", Name: "ＧＡＭＥ"},
		WorldNow: time.Date(1996, 8, 26, 23, 30, 0, 0, time.Local),
		Slots:    slots,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(planned) != 7 {
		t.Fatalf("planned=%d, want 7 generated roots", len(planned))
	}
	if !renderer.lastContext.PreferEraSafe {
		t.Fatal("final candidate request did not enter era-safe refill mode")
	}
	for _, post := range planned {
		if !strings.HasPrefix(post.Subject, "補充:") {
			t.Fatalf("subject=%q, want generated refill candidate rather than canned board-name fallback", post.Subject)
		}
	}
	if renderer.contextCalls != sharedTitlePoolAttemptLimit(len(slots))+1 {
		t.Fatalf("candidate pools=%d, want normal pools plus one generated refill", renderer.contextCalls)
	}
}

func TestSharedBBSPlannerDoesNotBypassResearchForClaimedEraSafeRefill(t *testing.T) {
	base := world.NewMemoryStore()
	host, err := base.HostByPhone("0920000196")
	if err != nil {
		t.Fatal(err)
	}
	researchTitles := make([]string, 0, 20)
	refillTitles := make([]string, 0, 20)
	claims := make([]llm.BBSTitleHistoricalClaim, 0, 20)
	for i := 1; i <= 20; i++ {
		researchTitles = append(researchTitles, fmt.Sprintf("要調査候補%02d", i))
		refillTitles = append(refillTitles, fmt.Sprintf("補充:実在対象%02d", i))
		claims = append(claims, llm.BBSTitleHistoricalClaim{
			Candidate: i,
			Subject:   fmt.Sprintf("実在対象%02d", i),
			Kind:      "product_availability",
			Need:      "world dateまでの存在確認",
		})
	}
	renderer := &fakeSharedTitleRenderer{
		titles:           researchTitles,
		refillTitles:     refillTitles,
		historicalClaims: claims,
	}
	repo := New(base, noSafeTitleTestEngine{}, LLMMaterializer{Renderer: renderer}, "1996-08-26")

	slots := make([]bbsengine.Slot, 0, 3)
	for i := 0; i < 3; i++ {
		slots = append(slots, bbsengine.Slot{
			Index:     i + 1,
			Author:    fmt.Sprintf("USER%02d", i+1),
			CreatedAt: time.Date(1996, 8, 26, 20, i, 0, 0, time.Local),
		})
	}
	_, err = (repositoryBBSBatchPlanner{repo: repo}).PlanBBSBatch(context.Background(), bbsengine.BatchRequest{
		Host:     host,
		Board:    world.Board{ID: "70/1", Name: "ＰＣ－９８"},
		WorldNow: time.Date(1996, 8, 26, 23, 30, 0, 0, time.Local),
		Slots:    slots,
	})
	if err == nil {
		t.Fatal("claim-bearing refill candidates must still require Historical KB verification")
	}
}

func TestSharedBBSPlannerFailsCleanlyWhenGeneratedRefillsAreExhausted(t *testing.T) {
	base := world.NewMemoryStore()
	host, err := base.HostByPhone("0920000196")
	if err != nil {
		t.Fatal(err)
	}
	titles := make([]string, 0, 20)
	for i := 1; i <= 20; i++ {
		titles = append(titles, fmt.Sprintf("要調査候補%02d", i))
	}
	renderer := &fakeSharedTitleRenderer{titles: titles}
	repo := New(base, noSafeTitleTestEngine{}, LLMMaterializer{Renderer: renderer}, "1996-08-26")

	slots := make([]bbsengine.Slot, 0, 7)
	for i := 0; i < 7; i++ {
		slots = append(slots, bbsengine.Slot{
			Index:     i + 1,
			Author:    fmt.Sprintf("USER%02d", i+1),
			CreatedAt: time.Date(1996, 8, 26, 20, i, 0, 0, time.Local),
		})
	}
	_, err = (repositoryBBSBatchPlanner{repo: repo}).PlanBBSBatch(context.Background(), bbsengine.BatchRequest{
		Host:     host,
		Board:    world.Board{ID: "70/1", Name: "ＰＣ－９８"},
		WorldNow: time.Date(1996, 8, 26, 23, 30, 0, 0, time.Local),
		Slots:    slots,
	})
	if err == nil {
		t.Fatal("expected generation failure after all generated refill pools are exhausted")
	}
	if !strings.Contains(err.Error(), "canned title fallback is disabled") {
		t.Fatalf("unexpected error: %v", err)
	}
	if renderer.contextCalls != sharedTitlePoolAttemptLimit(len(slots))+sharedTitleEraSafeRefillAttempts {
		t.Fatalf("candidate pools=%d, want all normal + refill pools", renderer.contextCalls)
	}
}

type suppliedFactTitleTestEngine struct {
	evidenceCalls atomic.Int32
	sawSaturnFact atomic.Bool
}

func (e *suppliedFactTitleTestEngine) ResolveEvidence(_ context.Context, req worldengine.EvidenceRequest) (worldengine.EvidenceDecision, error) {
	if strings.HasPrefix(req.Subject, "bbs-title-era:") {
		e.evidenceCalls.Add(1)
	}
	return worldengine.EvidenceDecision{Knowledge: historicalkb.KnowledgeResult{CanUse: true}}, nil
}

func (e *suppliedFactTitleTestEngine) AdviseTitleCandidates(_ context.Context, req worldengine.TitleCandidateAdviceRequest) (worldengine.TitleCandidateAdviceDecision, error) {
	for _, fact := range req.HistoricalFacts {
		if strings.Contains(fact, "セガサターン") {
			e.sawSaturnFact.Store(true)
			break
		}
	}
	out := worldengine.TitleCandidateAdviceDecision{
		Era: map[int]worldengine.TitleEraProbabilities{},
		Fit: map[string]float64{},
	}
	for i := range req.Titles {
		candidate := i + 1
		safe := .95
		fit := .20
		if candidate == 1 {
			fit = .90
		}
		out.Era[candidate] = worldengine.TitleEraProbabilities{
			SafeWithoutResearch: safe,
			LogicallyImpossible: .00,
		}
		for _, event := range req.Events {
			out.Fit[worldengine.TitleCandidatePairKey(candidate, event.EventID)] = fit
		}
	}
	return out, nil
}

func TestSharedBBSPlannerDoesNotUseGlobalPeriodCatalogAsTopicInput(t *testing.T) {
	base := world.NewMemoryStore()
	host, err := base.HostByPhone("0920000196")
	if err != nil {
		t.Fatal(err)
	}
	titles := []string{"セガサターンについて", "ゲームの話"}
	for i := 3; i <= 20; i++ {
		titles = append(titles, fmt.Sprintf("低適合候補%02d", i))
	}
	renderer := &fakeSharedTitleRenderer{titles: titles}
	engine := &suppliedFactTitleTestEngine{}
	repo := New(base, engine, LLMMaterializer{
		Renderer:                    renderer,
		CuratedHistoricalReferences: true,
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
	if engine.sawSaturnFact.Load() {
		t.Fatal("global PeriodReferents leaked into candidate/Jev topic input")
	}
	for _, fact := range renderer.lastContext.HistoricalFacts {
		if strings.Contains(fact, "セガサターン") {
			t.Fatalf("period seed leaked into title candidate context: %q", fact)
		}
	}
	if got := engine.evidenceCalls.Load(); got != 0 {
		t.Fatalf("historical Web research calls=%d, want 0 for Jev-safe supplied fact", got)
	}
	if len(planned) != 1 || planned[0].Subject != "セガサターンについて" {
		t.Fatalf("planned=%+v, want sourced concrete title", planned)
	}
}

func TestSharedBBSHeaderMaterializationAllowsDemandAlongsideBackgroundPrefetch(t *testing.T) {
	base := world.NewMemoryStore()
	host, err := base.HostByPhone("0920000196")
	if err != nil {
		t.Fatal(err)
	}
	release := make(chan struct{})
	renderer := &fakeSharedTitleRenderer{
		// Initial board histories may need several 20-title pools. Keep the
		// synchronization channel roomy so post-release refill calls do not block
		// a concurrency test that only cares about the first call per board.
		started: make(chan string, 16),
		release: release,
	}
	repo := New(base, nil, LLMMaterializer{Renderer: renderer}, "1996-08-26")
	repo.SetWorldNow(func() time.Time {
		return time.Date(1996, 8, 26, 23, 30, 0, 0, time.Local)
	})

	boardA := world.Board{ID: "4", Name: "ふり～と～く"}
	boardB := world.Board{ID: "20/1", Name: "ＧＡＭＥ"}
	repo.BeginHostPrefetch(host, []world.Board{boardA})
	select {
	case got := <-renderer.started:
		if got != boardA.Name {
			t.Fatalf("prefetch started %q, want %q", got, boardA.Name)
		}
	case <-time.After(time.Second):
		t.Fatal("background board did not start")
	}

	repo.BeginHostObservation(host, []world.Board{boardB})
	select {
	case got := <-renderer.started:
		if got != boardB.Name {
			t.Fatalf("demand started %q, want %q", got, boardB.Name)
		}
	case <-time.After(time.Second):
		t.Fatal("demanded board did not start alongside background prefetch")
	}
	if got := renderer.maxActive.Load(); got != 2 {
		t.Fatalf("max concurrent title generators=%d, want 2 (background + demand)", got)
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
}


func TestSharedTitleResearchBudgetIsBoardWide(t *testing.T) {
	var deadline time.Time
	start := time.Date(2026, 9, 24, 12, 0, 0, 0, time.UTC)
	if got := sharedTitleResearchRemaining(&deadline, start); got != 6*time.Second {
		t.Fatalf("initial remaining=%s, want 6s", got)
	}
	if got := sharedTitleResearchRemaining(&deadline, start.Add(2*time.Second)); got != 4*time.Second {
		t.Fatalf("second round remaining=%s, want 4s from original board budget", got)
	}
	if got := sharedTitleResearchRemaining(&deadline, start.Add(7*time.Second)); got != 0 {
		t.Fatalf("expired remaining=%s, want 0", got)
	}
}


type fortyTitleRenderer struct {
	calls int
}

func (f *fortyTitleRenderer) GenerateBoardPost(context.Context, llm.BoardPostRequest) (llm.BoardPostDraft, error) {
	return llm.BoardPostDraft{Author: "X", Subject: "X", Body: "body"}, nil
}

func (f *fortyTitleRenderer) GenerateBBSTitleCandidates(context.Context, string, string) (llm.BBSTitleCandidates, error) {
	return llm.BBSTitleCandidates{}, fmt.Errorf("legacy title path should not be used")
}

func (f *fortyTitleRenderer) GenerateContextualBBSTitleCandidates(_ context.Context, _ llm.BBSContextualTitleCandidateRequest) (llm.BBSTitleCandidates, error) {
	f.calls++
	start := (f.calls-1)*20 + 1
	titles := make([]string, 0, 20)
	for i := 0; i < 20; i++ {
		titles = append(titles, fmt.Sprintf("評価用タイトル%02d", start+i))
	}
	return llm.BBSTitleCandidates{Titles: titles}, nil
}

func (f *fortyTitleRenderer) ReviewBBSTitleCandidates(_ context.Context, req llm.BBSTitleReviewRequest) (llm.BBSTitleReview, error) {
	decisions := make([]llm.BBSTitleDecision, 0, len(req.Titles))
	for i, title := range req.Titles {
		d := llm.BBSTitleDecision{Candidate: i + 1, Reason: "not selected"}
		if i < len(req.Events) {
			d.EventID = req.Events[i].EventID
			d.Subject = title
			d.Summary = "「" + title + "」を話題にする"
			d.Details = []string{}
			d.Reason = "fits selected world slot"
		}
		decisions = append(decisions, d)
	}
	return llm.BBSTitleReview{Decisions: decisions}, nil
}

func (f *fortyTitleRenderer) ValidateBBSTitleEra(_ context.Context, req llm.BBSTitleEraRequest) (llm.BBSTitleEraReview, error) {
	decisions := make([]llm.BBSTitleEraDecision, 0, len(req.Titles))
	for i := range req.Titles {
		decisions = append(decisions, llm.BBSTitleEraDecision{Candidate: i + 1, Status: llm.BBSTitleEraOK, Reason: "test-safe"})
	}
	return llm.BBSTitleEraReview{Decisions: decisions}, nil
}

func TestSharedBBSPlannerFillsFortyDebugHeadersAcrossTwoPools(t *testing.T) {
	base := world.NewMemoryStore()
	host, err := base.HostByPhone("0920000196")
	if err != nil {
		t.Fatal(err)
	}
	renderer := &fortyTitleRenderer{}
	repo := New(base, nil, LLMMaterializer{Renderer: renderer}, "1996-08-26")
	slots := make([]bbsengine.Slot, 0, 40)
	for i := 0; i < 40; i++ {
		slots = append(slots, bbsengine.Slot{
			Index: i + 1,
			Author: fmt.Sprintf("USER%02d", i+1),
			CreatedAt: time.Date(1996, 8, 26, 20, i, 0, 0, time.Local),
		})
	}
	planned, err := (repositoryBBSBatchPlanner{repo: repo}).PlanBBSBatch(context.Background(), bbsengine.BatchRequest{
		Host: host,
		Board: world.Board{ID: "20/1", Name: "ＧＡＭＥ"},
		WorldNow: time.Date(1996, 8, 26, 23, 30, 0, 0, time.Local),
		Slots: slots,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(planned) != 40 {
		t.Fatalf("planned=%d, want 40", len(planned))
	}
	if renderer.calls != 2 {
		t.Fatalf("candidate pools=%d, want 2 for 40 headers", renderer.calls)
	}
	seen := map[string]bool{}
	for _, post := range planned {
		if post.Subject == "" || seen[post.Subject] {
			t.Fatalf("missing/duplicate subject in 40-header plan: %+v", post)
		}
		seen[post.Subject] = true
	}
}


func TestSharedTitlePoolAttemptLimitScalesForLargeDebugBatch(t *testing.T) {
	tests := []struct {
		roots int
		want  int
	}{
		{roots: 1, want: 3},
		{roots: 5, want: 3},
		{roots: 20, want: 3},
		{roots: 40, want: 4},
		{roots: 60, want: 5},
		{roots: 100, want: 6},
	}
	for _, tc := range tests {
		if got := sharedTitlePoolAttemptLimit(tc.roots); got != tc.want {
			t.Fatalf("roots=%d attempts=%d, want %d", tc.roots, got, tc.want)
		}
	}
}

type attritionTitleTestEngine struct{}

func (attritionTitleTestEngine) ResolveEvidence(_ context.Context, req worldengine.EvidenceRequest) (worldengine.EvidenceDecision, error) {
	if strings.HasPrefix(req.Subject, "bbs-title-era:") {
		return worldengine.EvidenceDecision{Knowledge: historicalkb.KnowledgeResult{CanUse: false}}, nil
	}
	return worldengine.EvidenceDecision{}, nil
}

func (attritionTitleTestEngine) AdviseTitleCandidates(_ context.Context, req worldengine.TitleCandidateAdviceRequest) (worldengine.TitleCandidateAdviceDecision, error) {
	out := worldengine.TitleCandidateAdviceDecision{
		Era: map[int]worldengine.TitleEraProbabilities{},
		Fit: map[string]float64{},
	}
	for i := range req.Titles {
		candidate := i + 1
		safe := .10
		if candidate <= 12 {
			safe = .95
		}
		out.Era[candidate] = worldengine.TitleEraProbabilities{
			SafeWithoutResearch: safe,
			LogicallyImpossible: .00,
		}
		for _, event := range req.Events {
			out.Fit[worldengine.TitleCandidatePairKey(candidate, event.EventID)] = .90 - float64(candidate)*.001
		}
	}
	return out, nil
}

func TestSharedBBSPlannerUsesFourthPoolAfterEraAttritionForFortyRoots(t *testing.T) {
	base := world.NewMemoryStore()
	host, err := base.HostByPhone("0920000196")
	if err != nil {
		t.Fatal(err)
	}
	renderer := &fortyTitleRenderer{}
	repo := New(base, attritionTitleTestEngine{}, LLMMaterializer{Renderer: renderer}, "1996-08-26")

	slots := make([]bbsengine.Slot, 0, 40)
	for i := 0; i < 40; i++ {
		slots = append(slots, bbsengine.Slot{
			Index:     i + 1,
			Author:    fmt.Sprintf("USER%02d", i+1),
			CreatedAt: time.Date(1996, 8, 26, 20, i, 0, 0, time.Local),
		})
	}
	planned, err := (repositoryBBSBatchPlanner{repo: repo}).PlanBBSBatch(context.Background(), bbsengine.BatchRequest{
		Host:     host,
		Board:    world.Board{ID: "20/1", Name: "ＧＡＭＥ"},
		WorldNow: time.Date(1996, 8, 26, 23, 30, 0, 0, time.Local),
		Slots:    slots,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(planned) != 40 {
		t.Fatalf("planned=%d, want 40", len(planned))
	}
	if renderer.calls != 4 {
		t.Fatalf("candidate pools=%d, want 4 after 12-safe-per-pool attrition", renderer.calls)
	}
}
