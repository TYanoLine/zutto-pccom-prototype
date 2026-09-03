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

	related := base.AddPost(h.ID, world.Post{
		BoardID: board.ID,
		Author:  "NORI",
		Subject: "前にも98環境の話",
		Intent: world.PostIntent{
			Action: "thread_start",
			Topic:  "pc98_environment",
			Claims: []string{"以前もPC-98の使い分けについて話題になった"},
		},
		CreatedAt: at.Add(-24 * time.Hour),
	})
	_ = related

	root := base.AddPost(h.ID, world.Post{
		BoardID: board.ID,
		Author:  "TAKA",
		Subject: "みなさんの98環境",
		Intent: world.PostIntent{
			Action: "thread_start",
			Topic:  "pc98_environment",
			Claims: []string{"通信とゲームで兼用している"},
		},
		Body:      "うちは通信とゲームで同じ98を使ってます。\r\n",
		CreatedAt: at,
	})
	reply := base.AddPost(h.ID, world.Post{
		BoardID:  board.ID,
		ParentID: root.ID,
		Author:   "NEKO",
		Subject:  "Re: みなさんの98環境",
		Intent: world.PostIntent{
			Action:           "reply",
			Topic:            "pc98_environment",
			ResponseAct:      "share_experience",
			Claims:           []string{"外付けモデムを使っている"},
			RespondsToClaims: []string{"通信とゲームで兼用している"},
		},
		CreatedAt: at.Add(2 * time.Hour),
	})
	selected := base.AddPost(h.ID, world.Post{
		BoardID:  board.ID,
		ParentID: root.ID,
		Author:   "MARI",
		Subject:  "Re: みなさんの98環境",
		Intent: world.PostIntent{
			Action: "reply",
			Topic:  "pc98_environment",
		},
		CreatedAt: at.Add(4 * time.Hour),
	})

	context, stats := repo.materializationBBSRenderContext(h, board, selected)
	for _, want := range []string{"MSG 0002", "うちは通信とゲームで同じ98を使ってます。", "MSG 0003", "semantic envelope", "外付けモデムを使っている", "RELATED EARLIER POSTS", "前にも98環境の話"} {
		if !strings.Contains(context, want) {
			t.Fatalf("context missing %q:\n%s", want, context)
		}
	}
	if stats.threadPosts != 2 || stats.threadBodies != 1 || stats.threadEnvelopes != 1 || stats.relatedPosts != 1 {
		t.Fatalf("unexpected context stats: %+v", stats)
	}
	_ = reply
}

func TestIntentSummaryCarriesTransientBBSContextWithoutPersistingIt(t *testing.T) {
	intent := world.PostIntent{
		Action:        "reply",
		Topic:         "pc98_environment",
		ResponseAct:   "brief_reaction",
		RenderContext: "THREAD SO FAR\n[MSG 1002 TAKA] ...",
	}
	summary := intentSummary(intent)
	if !strings.Contains(summary, "bbs_context:") || !strings.Contains(summary, "MSG 1002 TAKA") {
		t.Fatalf("render context missing from prompt summary: %q", summary)
	}
}
