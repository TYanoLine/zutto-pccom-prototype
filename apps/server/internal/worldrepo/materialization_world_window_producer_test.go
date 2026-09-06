package worldrepo

import (
	"context"
	"strings"
	"testing"

	"zutto-pccom/apps/server/internal/llm"
	"zutto-pccom/apps/server/internal/world"
	"zutto-pccom/apps/server/internal/worldengine"
)

type fakeWorldWindowMaterializer struct {
	calls  int
	shells []developmentWindowShell
}

func (f *fakeWorldWindowMaterializer) GenerateBoardPosts(context.Context, BoardMaterializationRequest, worldengine.EvidenceDecision) ([]world.Post, error) {
	return nil, nil
}

func (f *fakeWorldWindowMaterializer) PlanDevelopmentWorldWindow(_ context.Context, _ world.Host, _ string, shells []developmentWindowShell, _ map[string][]world.PersonaFact, _ string) (developmentWorldWindowPlan, error) {
	f.calls++
	f.shells = append([]developmentWindowShell(nil), shells...)
	events := make([]developmentWorldWindowPlanEvent, 0, len(shells))
	for _, item := range shells {
		events = append(events, developmentWorldWindowPlanEvent{
			eventID:         item.eventID,
			subject:         "件名 " + item.eventID,
			episode:         "producer fixed episode for " + item.eventID,
			referents:       []string{"shared-ref:" + item.shell.anchorKey},
			actorKnowledge:  []string{"actor knows the selected episode"},
			audienceContext: []string{"only established BBS context may be implicit"},
			contribution:    []string{"make the selected contribution"},
			mustNot:         []string{"do not invent another event"},
			topic:           "producer topic",
			motivation:      "producer motivation",
			stance:          "producer stance",
			goal:            "producer goal",
		})
	}
	return developmentWorldWindowPlan{events: events, usage: GenerationUsage{TotalTokens: 123, Model: "fake-producer"}}, nil
}

func TestWorldWindowProducerSeesMultipleBoardsBeforeCommit(t *testing.T) {
	store := world.NewMemoryStore()
	materializer := &fakeWorldWindowMaterializer{}
	repo := New(store, nil, materializer, "1996-08-26")
	host, err := repo.HostByPhone("0450000196")
	if err != nil {
		t.Fatal(err)
	}
	boards, _ := repo.MaterializationBoards(host)
	if len(boards) != 3 {
		t.Fatalf("boards=%d, want 3", len(boards))
	}

	_, created := repo.MaterializationPersonaArticleHeaders(host, boards[0])
	if !created {
		t.Fatal("first board observation should materialize the host window")
	}
	if materializer.calls != 1 {
		t.Fatalf("producer calls=%d, want 1", materializer.calls)
	}
	seenBoards := map[string]bool{}
	for _, item := range materializer.shells {
		seenBoards[item.board.ID] = true
	}
	if len(seenBoards) < 2 {
		t.Fatalf("producer only saw boards %v; want a host-wide multi-board window", seenBoards)
	}

	posts := store.ListPosts(host.ID)
	if len(posts) == 0 {
		t.Fatal("producer committed no posts")
	}
	for _, post := range posts {
		if post.Intent.ProducerEventID == "" || post.Intent.ProducerEpisode == "" {
			t.Fatalf("post %d missing canonical producer brief: %+v", post.ID, post.Intent)
		}
		if len(post.Intent.ProducerContribution) == 0 || len(post.Intent.ProducerMustNot) == 0 {
			t.Fatalf("post %d missing worker instructions: %+v", post.ID, post.Intent)
		}
	}

	// Reading any other board must expose the same already-produced world instead
	// of invoking another board-local or producer planning pass.
	for _, board := range boards[1:] {
		_, _ = repo.MaterializationPersonaArticleHeaders(host, board)
	}
	if materializer.calls != 1 {
		t.Fatalf("producer calls after other board reads=%d, want 1", materializer.calls)
	}
}

type captureWorkerRenderer struct {
	req llm.BoardPostRequest
}

func (c *captureWorkerRenderer) GenerateBoardPost(_ context.Context, req llm.BoardPostRequest) (llm.BoardPostDraft, error) {
	c.req = req
	return llm.BoardPostDraft{Author: req.AuthorHandle, Subject: req.CanonicalSubject, Body: "本文"}, nil
}

func TestArticleWorkerReceivesCanonicalProducerBrief(t *testing.T) {
	renderer := &captureWorkerRenderer{}
	m := LLMMaterializer{Renderer: renderer}
	persona := world.Persona{Handle: "NEKO", WritingStyle: "短め"}
	_, _, err := m.GenerateBoardPostsWithUsage(context.Background(), BoardMaterializationRequest{
		Host:             world.Host{Name: "TEST", Region: "神奈川県"},
		BoardID:          "1",
		BoardTopic:       "フリートーク",
		WorldDate:        "1996-08-26",
		Persona:          &persona,
		CanonicalSubject: "あの面どうした？",
		Intent: world.PostIntent{
			Action:                  "thread_start",
			AnchorKey:               "games",
			CauseKind:               "recent_salience",
			ProducerEventID:         "board-1:event-0005",
			ProducerEpisode:         "今夜、進行中のゲームの同じ場面で二度試して先へ進めなかった。",
			ProducerReferents:       []string{"現在遊んでいる同一ゲーム", "問題の場面"},
			ProducerActorKnowledge:  []string{"NEKOは二つの進み方を試した"},
			ProducerAudienceContext: []string{"ゲーム名はこのスレッド以前には共有されていない"},
			ProducerContribution:    []string{"詰まっている位置と試したことを伝えて助言を求める"},
			ProducerMustNot:         []string{"ゲーム名を勝手に発明しない"},
		},
	}, worldengine.EvidenceDecision{})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"producer_event_id=board-1:event-0005",
		"producer_episode=今夜、進行中のゲーム",
		"producer_referents=現在遊んでいる同一ゲーム",
		"producer_actor_knowledge=NEKOは二つの進み方を試した",
		"producer_audience_context=ゲーム名はこのスレッド以前には共有されていない",
		"producer_required_contribution=詰まっている位置",
		"producer_must_not=ゲーム名を勝手に発明しない",
	} {
		if !strings.Contains(renderer.req.PostIntent, want) {
			t.Fatalf("worker intent missing %q: %s", want, renderer.req.PostIntent)
		}
	}
}
