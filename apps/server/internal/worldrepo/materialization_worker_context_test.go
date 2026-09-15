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
