package worldrepo

import (
	"strings"
	"testing"

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
