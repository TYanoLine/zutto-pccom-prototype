package worldrepo

import (
	"strings"
	"testing"
	"time"

	"zutto-pccom/apps/server/internal/world"
)

func TestBBSRenderContextUsesBodiesAndLazyEnvelopes(t *testing.T) {
	base := world.NewMemoryStore()
	repo := New(base, nil, nil, "1996-08-29")
	h, _ := repo.HostByPhone("0450000196")
	boards, _ := repo.MaterializationBoards(h)
	board := boards[1]
	at := worldTime("1996-08-29").Add(-72 * time.Hour)

	base.AddPost(h.ID, world.Post{
		BoardID:   board.ID,
		Author:    "NORI",
		Subject:   "前にも98環境の話",
		Intent:    world.PostIntent{Action: "thread_start", Topic: "98を通信に使う環境", Goal: "自分の使い方を話す", Claims: []string{"以前もPC-98の使い分けについて話題になった"}},
		CreatedAt: at.Add(-24 * time.Hour),
	})

	root := base.AddPost(h.ID, world.Post{
		BoardID: board.ID,
		Author:  "TAKA",
		Subject: "みなさんの98環境",
		Intent:  world.PostIntent{Action: "thread_start", Topic: "98を通信に使う環境", Goal: "自分の兼用状況を話して他の人の環境も知りたい", Claims: []string{"通信とゲームで兼用している"}},
		Body:    "うちは通信とゲームで同じ98を使ってます。\r\n", CreatedAt: at,
	})
	base.AddPost(h.ID, world.Post{
		BoardID: board.ID, ParentID: root.ID, Author: "NEKO", Subject: "Re: みなさんの98環境",
		Intent:    world.PostIntent{Action: "reply", Topic: "98を通信に使う環境", Goal: "自分の例を短く返す", Claims: []string{"外付けモデムを使っている"}, RespondsToClaims: []string{"通信とゲームで兼用している"}},
		CreatedAt: at.Add(2 * time.Hour),
	})
	selected := base.AddPost(h.ID, world.Post{
		BoardID: board.ID, ParentID: root.ID, Author: "MARI", Subject: "Re: みなさんの98環境",
		Intent: world.PostIntent{Action: "reply", Topic: "98を通信に使う環境", Goal: "ここまでの話を読んで自分の事情を返す"}, CreatedAt: at.Add(4 * time.Hour),
	})

	context, stats := repo.materializationBBSRenderContext(h, board, selected)
	for _, want := range []string{"TAKA] みなさんの98環境", "うちは通信とゲームで同じ98を使ってます。", "NEKO] Re: みなさんの98環境", "semantic envelope", "goal=自分の例を短く返す", "外付けモデムを使っている", "RELATED EARLIER POSTS", "前にも98環境の話"} {
		if !strings.Contains(context, want) {
			t.Fatalf("context missing %q:\n%s", want, context)
		}
	}
	if stats.threadPosts != 2 || stats.threadBodies != 1 || stats.threadEnvelopes != 1 || stats.relatedPosts != 1 {
		t.Fatalf("unexpected context stats: %+v", stats)
	}
}

func TestIntentSummaryCarriesFreeFormGoalAndContentOnlyBBSContext(t *testing.T) {
	intent := world.PostIntent{Action: "reply", Topic: "98を通信に使う環境", Goal: "短く自分の例を返す", RenderContext: "THREAD CONTEXT\n[TAKA] 件名\n本文"}
	summary := intentSummary(intent)
	if !strings.Contains(summary, "goal=短く自分の例を返す") || !strings.Contains(summary, "conversation_context:") || !strings.Contains(summary, "[TAKA] 件名") {
		t.Fatalf("free-form intent/render context missing from prompt summary: %q", summary)
	}
}
