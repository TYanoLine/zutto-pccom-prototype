package worldrepo

import (
	"strings"
	"testing"
	"time"

	"zutto-pccom/apps/server/internal/world"
)

func TestIntentSummaryForWorkerDropsPlanningMetadataButKeepsArticleFacts(t *testing.T) {
	i := world.PostIntent{Action: "thread_start", AnchorKey: "music", CauseKind: "recent_salience", Topic: "music", Motivation: "internal cause", DiscourseMode: "share_experience", SituationSummary: "YMOを最近また聴いている", SituationFacts: []string{"article_detail=observation:音の重なり方が以前より気になった", "title_first_subject=YMOを聴き直しています"}}
	got := intentSummary(i)
	for _, bad := range []string{"action=", "internal_routing_domain", "world_cause=", "motivation="} {
		if strings.Contains(got, bad) {
			t.Fatalf("worker packet leaked %q: %s", bad, got)
		}
	}
	if !strings.Contains(got, "canonical_event=") || !strings.Contains(got, "article_detail=") {
		t.Fatalf("worker facts missing: %s", got)
	}
	if strings.Contains(got, "title_first_subject=") {
		t.Fatalf("subject metadata leaked: %s", got)
	}
}

func TestArticleWorkerContextHasNoMSGIdsOrTimestamps(t *testing.T) {
	base := world.NewMemoryStore()
	repo := New(base, nil, nil, "1996-08-29")
	host := world.Host{ID: "h"}
	root := base.AddPost(host.ID, world.Post{BoardID: "5", Author: "MARI", Subject: "YMOを聴き直しています", Body: "最近また聴いています。"})
	reply := world.Post{ID: root.ID + 1, BoardID: "5", ParentID: root.ID, Author: "YUKI", Subject: "Re: YMOを聴き直しています", Intent: world.PostIntent{SourcePostID: root.ID}}
	got := repo.materializationArticleWorkerContext(host, world.Board{ID: "5", Name: "音楽"}, reply)
	if strings.Contains(got, "MSG") || strings.Contains(got, "board=") || strings.Contains(got, "01/02") {
		t.Fatalf("metadata leaked: %s", got)
	}
	if !strings.Contains(got, "[MARI] YMOを聴き直しています") {
		t.Fatalf("source content missing: %s", got)
	}
}

func TestMaterializationThreadPredecessorsIncludeEarlierNestedAndSiblingReplies(t *testing.T) {
	base := world.NewMemoryStore()
	repo := New(base, nil, nil, "1996-08-29")
	h, _ := repo.HostByPhone("0450000196")
	boards, _ := repo.MaterializationBoards(h)
	board := boards[1]
	at := worldTime("1996-08-29")

	root := base.AddPost(h.ID, world.Post{BoardID: board.ID, Author: "TAKA", Subject: "親", CreatedAt: at})
	reply := base.AddPost(h.ID, world.Post{BoardID: board.ID, ParentID: root.ID, Author: "NEKO", Subject: "Re: 親", CreatedAt: at.Add(time.Minute)})
	sibling := base.AddPost(h.ID, world.Post{BoardID: board.ID, ParentID: root.ID, Author: "MARI", Subject: "Re: 親", CreatedAt: at.Add(2 * time.Minute)})
	nested := base.AddPost(h.ID, world.Post{BoardID: board.ID, ParentID: reply.ID, Author: "YUKI", Subject: "Re^2: 親", CreatedAt: at.Add(3 * time.Minute)})
	selected := base.AddPost(h.ID, world.Post{BoardID: board.ID, ParentID: nested.ID, Author: "SORA", Subject: "Re^3: 親", CreatedAt: at.Add(4 * time.Minute)})

	predecessors := repo.materializationThreadPredecessors(h.ID, board.ID, selected)
	if len(predecessors) != 4 {
		t.Fatalf("predecessors=%+v", predecessors)
	}
	if predecessors[0].ID != root.ID || predecessors[1].ID != reply.ID || predecessors[2].ID != sibling.ID || predecessors[3].ID != nested.ID {
		t.Fatalf("unexpected predecessor order: %+v", predecessors)
	}
}


func TestMaterializationAuthorHistoryContextUsesOnlyPriorOwnPostsAndExcludesCurrentThread(t *testing.T) {
	base := world.NewMemoryStore()
	repo := New(base, nil, nil, "1996-08-29")
	host := world.Host{ID: "h"}
	at := time.Date(1996, time.July, 1, 12, 0, 0, 0, time.FixedZone("JST", 9*60*60))

	prior := base.AddPost(host.ID, world.Post{
		BoardID:         "2",
		Author:          "MINT-Y",
		AuthorPersonaID: "p-mint",
		Subject:         "前にもセーブで迷った",
		Body:            "前は三つの枠を分けて使っていました。",
		CreatedAt:       at,
		Intent:          world.PostIntent{Topic: "save"},
	})
	semanticOnly := base.AddPost(host.ID, world.Post{
		BoardID:         "3",
		Author:          "MINT-Y",
		AuthorPersonaID: "p-mint",
		Subject:         "設定を戻した話",
		CreatedAt:       at.Add(time.Hour),
		Intent: world.PostIntent{
			Topic:            "save",
			SituationSummary: "設定を一度戻してやり直した",
		},
	})
	root := base.AddPost(host.ID, world.Post{
		BoardID:         "1",
		Author:          "MARU",
		AuthorPersonaID: "p-maru",
		Subject:         "セーブの場所を決めてます",
		Body:            "場所を決めてます。",
		CreatedAt:       at.Add(2 * time.Hour),
	})
	sameThread := base.AddPost(host.ID, world.Post{
		BoardID:         "1",
		ParentID:        root.ID,
		Author:          "MINT-Y",
		AuthorPersonaID: "p-mint",
		Body:            "同じスレッドで先に書いた自分のアペです。",
		CreatedAt:       at.Add(3 * time.Hour),
		Intent: world.PostIntent{
			Topic:            "save",
			RespondsToPostID: root.ID,
			SourcePostID:     root.ID,
		},
	})
	base.AddPost(host.ID, world.Post{
		BoardID:         "1",
		Author:          "OTHER",
		AuthorPersonaID: "p-other",
		Body:            "別人の過去です。",
		CreatedAt:       at.Add(4 * time.Hour),
	})
	future := base.AddPost(host.ID, world.Post{
		BoardID:         "1",
		Author:          "MINT-Y",
		AuthorPersonaID: "p-mint",
		Body:            "未来の自分の投稿です。",
		CreatedAt:       at.Add(8 * time.Hour),
	})

	selected := world.Post{
		ID:              future.ID + 100,
		BoardID:         "1",
		ParentID:        root.ID,
		Author:          "MINT-Y",
		AuthorPersonaID: "p-mint",
		CreatedAt:       at.Add(6 * time.Hour),
		Intent: world.PostIntent{
			Topic:            "save",
			RespondsToPostID: root.ID,
			SourcePostID:     root.ID,
		},
	}
	got := repo.materializationAuthorHistoryContext(host, world.Board{ID: "1", Name: "GAME"}, selected, 6)

	for _, want := range []string{prior.Subject, prior.Body, semanticOnly.Subject, "設定を一度戻してやり直した"} {
		if !strings.Contains(got, want) {
			t.Fatalf("author history missing %q: %s", want, got)
		}
	}
	for _, bad := range []string{sameThread.Body, "別人の過去です。", future.Body} {
		if strings.Contains(got, bad) {
			t.Fatalf("author history leaked %q: %s", bad, got)
		}
	}
	for _, post := range base.ListPosts(host.ID) {
		if post.ID == semanticOnly.ID && post.Body != "" {
			t.Fatalf("author-history lookup materialized an unrelated old body: %q", post.Body)
		}
	}
}

func TestMaterializationAuthorHistoryContextIsBoundedAndRecencyDeterministic(t *testing.T) {
	base := world.NewMemoryStore()
	repo := New(base, nil, nil, "1996-08-29")
	host := world.Host{ID: "h"}
	at := time.Date(1996, time.July, 1, 12, 0, 0, 0, time.FixedZone("JST", 9*60*60))
	for i := 0; i < 8; i++ {
		base.AddPost(host.ID, world.Post{
			BoardID:         "2",
			Author:          "MINT-Y",
			AuthorPersonaID: "p-mint",
			Subject:         fmt.Sprintf("過去%02d", i),
			Body:            fmt.Sprintf("本文%02d", i),
			CreatedAt:       at.Add(time.Duration(i) * time.Hour),
		})
	}
	selected := world.Post{
		ID:              999,
		BoardID:         "1",
		Author:          "MINT-Y",
		AuthorPersonaID: "p-mint",
		CreatedAt:       at.Add(12 * time.Hour),
	}
	got := repo.materializationAuthorHistoryContext(host, world.Board{ID: "1", Name: "GAME"}, selected, 3)
	for _, want := range []string{"過去07", "過去06", "過去05"} {
		if !strings.Contains(got, want) {
			t.Fatalf("bounded history missing recent %q: %s", want, got)
		}
	}
	if strings.Contains(got, "過去04") || strings.Count(got, "\n- ") != 2 {
		t.Fatalf("bounded history returned more than three posts: %s", got)
	}
}
