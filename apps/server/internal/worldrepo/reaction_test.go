package worldrepo

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"zutto-pccom/apps/server/internal/world"
	"zutto-pccom/apps/server/internal/worldengine"
)

type panicMaterializer struct{}

func (panicMaterializer) GenerateBoardPosts(context.Context, BoardMaterializationRequest, worldengine.EvidenceDecision) ([]world.Post, error) {
	panic("LLM materializer must not be called during reaction planning or header materialization")
}

func TestHumanPostReactionLifecycleWithoutLLM(t *testing.T) {
	base := world.NewMemoryStore()
	repo := New(base, nil, panicMaterializer{}, "1996-05-10")

	now := time.Date(1996, 5, 10, 23, 10, 0, 0, time.FixedZone("JST", 9*3600))
	repo.SetWorldNow(func() time.Time { return now })

	host, err := repo.HostByPhone("0920000196")
	if err != nil {
		t.Fatalf("HostByPhone failed: %v", err)
	}
	board := repo.resolveBoardByID(host, "60/1") // PC-98 / MODEM board

	// Pre-populate board until the next root ordinal produces N >= 1 replies.
	var humanPost world.Post
	for attempt := 0; attempt < 20; attempt++ {
		humanPost = repo.AddPost(host.ID, world.Post{
			BoardID:        board.ID,
			Author:         "GUEST",
			AuthorMemberID: "GUEST",
			AuthorHumanID:  "human-1",
			Subject:        fmt.Sprintf("PC-9821とモデムの接続設定について (%d)", attempt+1),
			Body:           "28800bpsモデムをRS-232Cに繋いだのですがATコマンドの初期化文字列で迷っています。",
			CreatedAt:      now,
		})
		if plan, ok := base.ReactionPlan(host.ID, humanPost.ID); ok && len(plan.Replies) > 0 {
			break
		}
	}

	plan, ok := base.ReactionPlan(host.ID, humanPost.ID)
	if !ok {
		t.Fatalf("expected ReactionPlan to be saved for human post %d", humanPost.ID)
	}
	if len(plan.Replies) == 0 {
		t.Fatalf("expected at least 1 planned reaction on PC-98 board, got 0")
	}

	firstReply := plan.Replies[0]
	if !firstReply.DueAt.After(now) {
		t.Fatalf("expected first reply DueAt (%v) > now (%v)", firstReply.DueAt, now)
	}

	// 1. Immediately after posting (before DueAt), no reply post exists yet.
	repo.materializeDueReactions(host, board)
	for _, p := range base.ListPosts(host.ID) {
		if p.Intent.RespondsToPostID == humanPost.ID {
			t.Fatalf("expected no reply before DueAt, found %+v", p)
		}
	}

	// 2. Advance world clock past the last reply's DueAt and materialize headers.
	lastDue := plan.Replies[len(plan.Replies)-1].DueAt
	now = lastDue.Add(time.Minute)

	created := repo.materializeDueReactions(host, board)
	if created != len(plan.Replies) {
		t.Fatalf("expected %d due reactions materialized, got %d", len(plan.Replies), created)
	}

	// Verify headers exist with empty Body (lazy body generation) and matching PersonaID / DueAt.
	updatedPlan, ok := base.ReactionPlan(host.ID, humanPost.ID)
	if !ok {
		t.Fatalf("expected updated ReactionPlan")
	}
	postsByID := map[int64]world.Post{}
	for _, p := range base.ListPosts(host.ID) {
		postsByID[p.ID] = p
	}
	for _, rep := range updatedPlan.Replies {
		if rep.MaterializedPostID == 0 {
			t.Fatalf("expected MaterializedPostID != 0 for reply index %d", rep.Index)
		}
		post, exists := postsByID[rep.MaterializedPostID]
		if !exists {
			t.Fatalf("materialized post %d not found in store", rep.MaterializedPostID)
		}
		if post.AuthorPersonaID != rep.PersonaID {
			t.Fatalf("expected AuthorPersonaID %q, got %q", rep.PersonaID, post.AuthorPersonaID)
		}
		if !post.CreatedAt.Equal(rep.DueAt) {
			t.Fatalf("expected CreatedAt %v, got %v", rep.DueAt, post.CreatedAt)
		}
		if post.Body != "" {
			t.Fatalf("expected header-only post (empty Body) before article body observation, got %q", post.Body)
		}
		if post.Intent.RespondsToPostID != humanPost.ID {
			t.Fatalf("expected RespondsToPostID=%d, got %d", humanPost.ID, post.Intent.RespondsToPostID)
		}
	}

	// 3. Idempotency: calling observation 3 times does not duplicate posts.
	beforeCount := len(base.ListPosts(host.ID))
	for obs := 0; obs < 3; obs++ {
		if _, err := repo.WaitForBoardHeaders(context.Background(), host, board); err != nil {
			t.Fatalf("WaitForBoardHeaders (obs %d) failed: %v", obs+1, err)
		}
		_ = repo.ListBoardPosts(host, board.ID, board.Name)
	}
	afterCount := len(base.ListPosts(host.ID))
	if afterCount != beforeCount {
		t.Fatalf("expected post count to remain %d after 3 repeated observations, got %d", beforeCount, afterCount)
	}
}

func TestHumanAppendPostAlsoPlansReactionsAndPreservesPlanAfterPopulationChange(t *testing.T) {
	base := world.NewMemoryStore()
	countingMat := &countingMaterializer{}
	repo := New(base, &fakeEngine{}, countingMat, "1996-05-10")

	now := time.Date(1996, 5, 10, 23, 20, 0, 0, time.FixedZone("JST", 9*3600))
	repo.SetWorldNow(func() time.Time { return now })

	host, err := repo.HostByPhone("0920000196")
	if err != nil {
		t.Fatalf("HostByPhone failed: %v", err)
	}
	board := repo.resolveBoardByID(host, "60/1")

	root := repo.AddPost(host.ID, world.Post{
		BoardID:         board.ID,
		Author:          "NPC01",
		AuthorPersonaID: "hakata-canal-net:p001",
		Subject:         "PC-98の音源ボードについて",
		Body:            "86音源とSpeakBoardのどちらかで迷っています。",
		CreatedAt:       now,
	})

	// 4. Human reply (append) post also gets a ReactionPlan.
	var appendPost world.Post
	var appendPlan world.ReactionPlan
	for i := 0; i < 20; i++ {
		appendPost = repo.AddPost(host.ID, world.Post{
			BoardID:        board.ID,
			ParentID:       root.ID,
			Author:         "GUEST",
			AuthorMemberID: "GUEST",
			AuthorHumanID:  "human-1",
			Subject:        fmt.Sprintf("Re: PC-98の音源ボードについて (%d)", i+1),
			Body:           "私も86音源の中古を探しています！相場はいくらくらいでしょうか？",
			CreatedAt:      now.Add(time.Duration(i+1) * time.Minute),
			Intent: world.PostIntent{
				RespondsToPostID: root.ID,
			},
		})
		var ok bool
		if appendPlan, ok = base.ReactionPlan(host.ID, appendPost.ID); ok && len(appendPlan.Replies) > 0 {
			break
		}
	}
	if len(appendPlan.Replies) == 0 {
		t.Fatalf("expected human append post to have planned replies")
	}
	for _, rep := range appendPlan.Replies {
		if strings.EqualFold(rep.PersonaID, appendPost.Author) {
			t.Fatalf("expected reactor (%q) not to be the human author (%q)", rep.PersonaID, appendPost.Author)
		}
	}

	// 6. Adding new residents after the plan was saved must not alter the saved plan's reactors (FR-007).
	expectedReactors := make([]string, len(appendPlan.Replies))
	for i, r := range appendPlan.Replies {
		expectedReactors[i] = r.PersonaID
	}
	for i := 0; i < 10; i++ {
		p := world.Persona{
			ID:               fmt.Sprintf("%s:extra-%02d", host.ID, i+1),
			Handle:           fmt.Sprintf("EXTRA%02d", i+1),
			ReplyTendency:    0.99,
			NewcomerOpenness: 0.99,
			ActivityPattern:  "regular",
			Interests:        map[string]float64{"hardware": 1.0, "computers": 1.0},
		}
		base.SavePersona(p)
		base.AddMembership(host.ID, p.ID)
	}

	// Advance clock and observe headers.
	lastDue := appendPlan.Replies[len(appendPlan.Replies)-1].DueAt
	now = lastDue.Add(time.Minute)
	if _, err := repo.WaitForBoardHeaders(context.Background(), host, board); err != nil {
		t.Fatalf("WaitForBoardHeaders failed: %v", err)
	}

	// 5. Fake LLM provider call count is 0 during planning and header observation (SC-002).
	if countingMat.calls != 0 {
		t.Fatalf("expected 0 LLM calls during planning and header observation, got %d", countingMat.calls)
	}

	afterPlan, ok := base.ReactionPlan(host.ID, appendPost.ID)
	if !ok {
		t.Fatalf("expected ReactionPlan for appendPost")
	}
	for i, r := range afterPlan.Replies {
		if r.PersonaID != expectedReactors[i] {
			t.Fatalf("FR-007 violation: reactor at index %d changed from %q to %q after adding residents", i, expectedReactors[i], r.PersonaID)
		}
		if r.MaterializedPostID == 0 {
			t.Fatalf("expected reply %d to be materialized", i)
		}
	}
}

type countingMaterializer struct {
	calls int
}

func (m *countingMaterializer) GenerateBoardPosts(_ context.Context, req BoardMaterializationRequest, _ worldengine.EvidenceDecision) ([]world.Post, error) {
	m.calls++
	return []world.Post{
		{
			BoardID: req.BoardID,
			Author:  req.AuthorHandle,
			Subject: req.CanonicalSubject,
			Body:    "こちらはテスト用の本文です(^^)",
		},
	}, nil
}

func TestHumanPostZeroReplyOrdinalStaysUnanswered(t *testing.T) {
	base := world.NewMemoryStore()
	repo := New(base, nil, panicMaterializer{}, "1996-05-10")

	now := time.Date(1996, 5, 10, 21, 0, 0, 0, time.FixedZone("JST", 9*3600))
	repo.SetWorldNow(func() time.Time { return now })

	host, err := repo.HostByPhone("0920000196")
	if err != nil {
		t.Fatalf("HostByPhone failed: %v", err)
	}
	// Use a quiet custom board with reply_rate 0 so N == 0 is guaranteed.
	quietBoard := world.Board{
		ID:        "quiet-board",
		Name:      "静かな板",
		ReplyRate: 0.0,
	}
	base.SaveBoards(host.ID, []world.Board{quietBoard})

	humanPost := repo.AddPost(host.ID, world.Post{
		BoardID:        quietBoard.ID,
		Author:         "GUEST",
		AuthorMemberID: "GUEST",
		AuthorHumanID:  "human-1",
		Subject:        "独り言です",
		Body:           "特に返信不要のメモです。",
		CreatedAt:      now,
	})

	plan, ok := base.ReactionPlan(host.ID, humanPost.ID)
	if !ok {
		t.Fatalf("expected ReactionPlan to be recorded even when N == 0")
	}
	if len(plan.Replies) != 0 {
		t.Fatalf("expected 0 replies for zero reply rate, got %d", len(plan.Replies))
	}

	// Advance clock by 7 days; still 0 replies.
	now = now.Add(7 * 24 * time.Hour)
	if created := repo.materializeDueReactions(host, quietBoard); created != 0 {
		t.Fatalf("expected 0 materialized reactions after 7 days, got %d", created)
	}
}

func TestHumanPostTopicSelectionContrastAcrossBoards(t *testing.T) {
	base := world.NewMemoryStore()
	repo := New(base, nil, panicMaterializer{}, "1996-05-10")

	now := time.Date(1996, 5, 10, 23, 15, 0, 0, time.FixedZone("JST", 9*3600))
	repo.SetWorldNow(func() time.Time { return now })

	host, err := repo.HostByPhone("0920000196")
	if err != nil {
		t.Fatalf("HostByPhone failed: %v", err)
	}

	personaByID := map[string]world.Persona{}
	for _, p := range base.ListHostPersonas(host.ID) {
		personaByID[p.ID] = p
	}

	pcBoard := repo.resolveBoardByID(host, "60/1")    // PC-98 / MODEM
	localBoard := repo.resolveBoardByID(host, "10/1") // 博多・天神ローカル

	var pcReactors, localReactors []world.Persona
	for i := 0; i < 40; i++ {
		p1 := repo.AddPost(host.ID, world.Post{
			BoardID:        pcBoard.ID,
			Author:         "GUEST",
			AuthorMemberID: "GUEST",
			Subject:        fmt.Sprintf("PC-98とモデム設定の質問 %d", i),
			Body:           "PC-9821で28800bpsモデムとテレホーダイを使う設定について教えてください。",
			CreatedAt:      now,
		})
		if plan, ok := base.ReactionPlan(host.ID, p1.ID); ok {
			for _, r := range plan.Replies {
				if p, exists := personaByID[r.PersonaID]; exists {
					pcReactors = append(pcReactors, p)
				}
			}
		}

		p2 := repo.AddPost(host.ID, world.Post{
			BoardID:        localBoard.ID,
			Author:         "GUEST",
			AuthorMemberID: "GUEST",
			Subject:        fmt.Sprintf("天神のラーメン屋と商店街 %d", i),
			Body:           "週末に地元で美味しいラーメンと定食を探しています。",
			CreatedAt:      now,
		})
		if plan, ok := base.ReactionPlan(host.ID, p2.ID); ok {
			for _, r := range plan.Replies {
				if p, exists := personaByID[r.PersonaID]; exists {
					localReactors = append(localReactors, p)
				}
			}
		}
	}

	if len(pcReactors) == 0 || len(localReactors) == 0 {
		t.Fatalf("expected non-empty reactors on both boards (pc=%d, local=%d)", len(pcReactors), len(localReactors))
	}

	avgInterest := func(list []world.Persona, keys ...string) float64 {
		sum := 0.0
		for _, p := range list {
			best := 0.0
			for _, k := range keys {
				if v := p.Interests[k]; v > best {
					best = v
				}
			}
			sum += best
		}
		return sum / float64(len(list))
	}

	pcOnTech := avgInterest(pcReactors, "hardware", "software", "communications", "computers", "telecom", "retro_tech")
	localOnTech := avgInterest(localReactors, "hardware", "software", "communications", "computers", "telecom", "retro_tech")
	if !(pcOnTech > localOnTech) {
		t.Fatalf("expected PC board reactors to have higher tech interest (%f) than local board reactors (%f)", pcOnTech, localOnTech)
	}

	localOnLife := avgInterest(localReactors, "local", "daily_life", "local_life", "food")
	pcOnLife := avgInterest(pcReactors, "local", "daily_life", "local_life", "food")
	if !(localOnLife > pcOnLife) {
		t.Fatalf("expected local board reactors to have higher local_life/food interest (%f) than PC board reactors (%f)", localOnLife, pcOnLife)
	}
}

func TestMaterializedReactionBodyGeneratesOnDemand(t *testing.T) {
	base := world.NewMemoryStore()
	fakeMat := &countingMaterializer{}
	repo := New(base, &fakeEngine{}, fakeMat, "1996-05-10")

	now := time.Date(1996, 5, 10, 23, 0, 0, 0, time.FixedZone("JST", 9*3600))
	repo.SetWorldNow(func() time.Time { return now })

	host, _ := repo.HostByPhone("0920000196")
	board := repo.resolveBoardByID(host, "60/1")

	var humanPost world.Post
	var plan world.ReactionPlan
	for i := 0; i < 20; i++ {
		humanPost = repo.AddPost(host.ID, world.Post{
			BoardID:        board.ID,
			Author:         "GUEST",
			AuthorMemberID: "GUEST",
			Subject:        fmt.Sprintf("モデムの初期化コマンド %d", i),
			Body:           "PC-98でモデムを使うときの設定を教えてください。",
			CreatedAt:      now,
		})
		var ok bool
		if plan, ok = base.ReactionPlan(host.ID, humanPost.ID); ok && len(plan.Replies) > 0 {
			break
		}
	}
	if len(plan.Replies) == 0 {
		t.Fatalf("expected at least 1 planned reply")
	}

	now = plan.Replies[0].DueAt.Add(time.Minute)
	repo.materializeDueReactions(host, board)

	updatedPlan, _ := base.ReactionPlan(host.ID, humanPost.ID)
	replyPostID := updatedPlan.Replies[0].MaterializedPostID
	if replyPostID == 0 {
		t.Fatalf("expected materialized reply post ID")
	}

	// Reading the article body via WaitForArticleBody triggers on-demand body generation.
	got, found, err := repo.WaitForArticleBody(context.Background(), host, board, replyPostID)
	if err != nil || !found {
		t.Fatalf("WaitForArticleBody failed: found=%v err=%v", found, err)
	}
	if strings.TrimSpace(got.Body) == "" {
		t.Fatalf("expected non-empty body after WaitForArticleBody")
	}
}

type fakeBehaviorAdvisor struct {
	calls int
	last  worldengine.BehaviorAdviceRequest
	err   error
	score float64
}

func (f *fakeBehaviorAdvisor) AdviseBehavior(_ context.Context, req worldengine.BehaviorAdviceRequest) (worldengine.BehaviorAdviceDecision, error) {
	f.calls++
	f.last = req
	if f.err != nil {
		return worldengine.BehaviorAdviceDecision{}, f.err
	}
	probs := make(map[string]worldengine.BehaviorProbabilities, len(req.Personas))
	for _, p := range req.Personas {
		for _, b := range req.Boards {
			probs[worldengine.BehaviorPairKey(p.ID, b.ID)] = worldengine.BehaviorProbabilities{
				Reply: f.score,
			}
		}
	}
	return worldengine.BehaviorAdviceDecision{Probabilities: probs}, nil
}

func TestReactionAdvisorEnabledDisabledAndFailedModes(t *testing.T) {
	now := time.Date(1996, 5, 10, 23, 30, 0, 0, time.FixedZone("JST", 9*3600))

	// 1. Disabled (default nil) vs Failing advisor produce identical deterministic plans.
	baseDisabled := world.NewMemoryStore()
	repoDisabled := New(baseDisabled, nil, panicMaterializer{}, "1996-05-10")
	repoDisabled.SetWorldNow(func() time.Time { return now })

	baseFailed := world.NewMemoryStore()
	repoFailed := New(baseFailed, nil, panicMaterializer{}, "1996-05-10")
	repoFailed.SetWorldNow(func() time.Time { return now })
	failingBeh := &fakeBehaviorAdvisor{err: fmt.Errorf("jev timeout")}
	repoFailed.SetReactionAdvisor(BehaviorReactionAdvisor{
		Behavior: failingBeh,
		TopicTagger: func(context.Context, world.Post) ([]string, error) {
			return nil, fmt.Errorf("tagger timeout")
		},
	})

	host, _ := repoDisabled.HostByPhone("0920000196")
	_, _ = repoFailed.HostByPhone("0920000196")
	board := repoDisabled.resolveBoardByID(host, "60/1")

	var planDisabled, planFailed world.ReactionPlan
	for i := 0; i < 20; i++ {
		pD := repoDisabled.AddPost(host.ID, world.Post{
			BoardID:   board.ID,
			Author:    "GUEST",
			Subject:   fmt.Sprintf("昨日の出来事 %d", i),
			Body:      "ちょっと珍しいことがありました。",
			CreatedAt: now,
		})
		pF := repoFailed.AddPost(host.ID, world.Post{
			BoardID:   board.ID,
			Author:    "GUEST",
			Subject:   fmt.Sprintf("昨日の出来事 %d", i),
			Body:      "ちょっと珍しいことがありました。",
			CreatedAt: now,
		})
		planDisabled, _ = baseDisabled.ReactionPlan(host.ID, pD.ID)
		planFailed, _ = baseFailed.ReactionPlan(host.ID, pF.ID)
		if len(planDisabled.Replies) > 0 {
			break
		}
	}
	if len(planDisabled.Replies) == 0 || len(planDisabled.Replies) != len(planFailed.Replies) {
		t.Fatalf("expected matching non-empty plans for disabled (%d) and failed (%d) advisor", len(planDisabled.Replies), len(planFailed.Replies))
	}
	for i := range planDisabled.Replies {
		if planDisabled.Replies[i].PersonaID != planFailed.Replies[i].PersonaID ||
			!planDisabled.Replies[i].DueAt.Equal(planFailed.Replies[i].DueAt) {
			t.Fatalf("expected failed advisor to fall back identically to disabled advisor at reply %d", i)
		}
	}

	// 2. Enabled advisor: tags untagged posts only, queries top-K candidates in 1 call, and quantizes advice to 0.05.
	baseEnabled := world.NewMemoryStore()
	repoEnabled := New(baseEnabled, nil, panicMaterializer{}, "1996-05-10")
	repoEnabled.SetWorldNow(func() time.Time { return now })
	_, _ = repoEnabled.HostByPhone("0920000196")
	tagCalls := 0
	beh := &fakeBehaviorAdvisor{score: 0.774} // quantizes to 0.75
	repoEnabled.SetReactionAdvisor(BehaviorReactionAdvisor{
		Behavior: beh,
		TopicTagger: func(_ context.Context, post world.Post) ([]string, error) {
			tagCalls++
			return []string{"Custom_Topic_96", "PC98"}, nil
		},
	})

	// Tagged post (lexicon matches "PC-98") -> TopicTagger must NOT be called.
	taggedPost := repoEnabled.AddPost(host.ID, world.Post{
		BoardID:   board.ID,
		Author:    "GUEST",
		Subject:   "PC-98のメモリ増設",
		Body:      "PC-9821にメモリを増設しました。",
		CreatedAt: now,
	})
	if tagCalls != 0 {
		t.Fatalf("expected TopicTagger not to be called when lexicon matched, got %d calls", tagCalls)
	}
	if beh.calls != 1 {
		t.Fatalf("expected 1 batch AdviseBehavior call for post %d, got %d", taggedPost.ID, beh.calls)
	}
	if len(beh.last.Personas) > world.ReactionCandidateCap {
		t.Fatalf("expected candidate count <= %d, got %d", world.ReactionCandidateCap, len(beh.last.Personas))
	}

	// Untagged post (no lexicon match) -> TopicTagger IS called once and saved to Post.Intent.Topics.
	var untaggedPost world.Post
	var planEnabled world.ReactionPlan
	for i := 0; i < 20; i++ {
		beforeTag := tagCalls
		untaggedPost = repoEnabled.AddPost(host.ID, world.Post{
			BoardID:   board.ID,
			Author:    "GUEST",
			Subject:   fmt.Sprintf("テスト記録 (%d)", i),
			Body:      "これは検証用の無関係な文章です。",
			CreatedAt: now,
		})
		if tagCalls != beforeTag+1 {
			t.Fatalf("expected TopicTagger to be called once for untagged post, got delta %d", tagCalls-beforeTag)
		}
		var ok bool
		if planEnabled, ok = baseEnabled.ReactionPlan(host.ID, untaggedPost.ID); ok && len(planEnabled.Replies) > 0 {
			break
		}
	}
	for _, p := range baseEnabled.ListPosts(host.ID) {
		if p.ID == untaggedPost.ID {
			if len(p.Intent.Topics) == 0 || p.Intent.Topics[0] != "custom_topic_96" {
				t.Fatalf("expected tagged open-vocabulary topic saved on post, got %+v", p.Intent.Topics)
			}
		}
	}
	for _, rep := range planEnabled.Replies {
		if rep.Advice != 0.75 {
			t.Fatalf("expected quantized advice 0.75 saved in plan, got %v", rep.Advice)
		}
	}

	// 3. Bounded influence (SC-008): advice cannot overturn a strong deterministic weight gap (> (1+beta)/(1-beta) = 2.33x).
	pStrong := world.Persona{
		ID:              "strong",
		ActivityPattern: "regular",
		ReplyTendency:   0.90,
	}
	pWeak := world.Persona{
		ID:              "weak",
		ActivityPattern: "lurker",
		LurkerTendency:  0.80,
		ReplyTendency:   0.10,
	}
	wStrongWithMinAdvice := world.ReactionWeight(pStrong, 0.95, 1.0, world.AdviceMultiplier(0.0), world.ThreadState{}, 1)
	wWeakWithMaxAdvice := world.ReactionWeight(pWeak, 0.05, 1.0, world.AdviceMultiplier(1.0), world.ThreadState{}, 1)
	if !(wStrongWithMinAdvice > wWeakWithMaxAdvice) {
		t.Fatalf("SC-008 violation: max advice overturned strong deterministic weight (%f <= %f)", wStrongWithMinAdvice, wWeakWithMaxAdvice)
	}
}

