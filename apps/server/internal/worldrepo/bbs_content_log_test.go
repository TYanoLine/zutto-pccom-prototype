package worldrepo

import (
	"bytes"
	"encoding/json"
	"log"
	"strings"
	"testing"
	"time"

	"zutto-pccom/apps/server/internal/bbsengine"
	"zutto-pccom/apps/server/internal/world"
)

func captureGeneratedContentRecords(t *testing.T, action func()) []generatedContentLog {
	t.Helper()
	var buf bytes.Buffer
	original := log.Writer()
	log.SetOutput(&buf)
	defer log.SetOutput(original)
	action()
	var records []generatedContentLog
	for _, line := range strings.Split(buf.String(), "\n") {
		idx := strings.Index(line, "BBS generated content: ")
		if idx < 0 {
			continue
		}
		var record generatedContentLog
		if err := json.Unmarshal([]byte(line[idx+len("BBS generated content: "):]), &record); err != nil {
			t.Fatalf("parse generated content log: %v", err)
		}
		records = append(records, record)
	}
	return records
}

func TestHAKATAAuditLogsOnlyNewCommittedWorldHeaders(t *testing.T) {
	base := world.NewMemoryStore()
	host, err := base.HostByPhone("0920000196")
	if err != nil {
		t.Fatal(err)
	}
	board := world.Board{ID: "20/1", Name: "GAME"}
	repo := New(base, nil, nil, "1996-08-26")
	repo.SetDebugLogHAKATAGenerated(true)
	at := time.Date(1996, 8, 26, 21, 0, 0, 0, time.Local)
	old := base.AddPost(host.ID, world.Post{BoardID: board.ID, Author: "OLDER",
		Subject: "previous", CreatedAt: at, Intent: world.PostIntent{Action: bbsengine.ActionWorldCatchup}})
	before := base.ListPosts(host.ID)
	newPost := base.AddPost(host.ID, world.Post{BoardID: board.ID, Author: "NPC",
		Subject: "サクラ大戦の戦闘", CreatedAt: at, Intent: world.PostIntent{
			Action: bbsengine.ActionWorldCatchup, SituationKind: "games_opinion",
			SituationSummary: "遊んだ場面の感想", SituationFacts: []string{"world_adopted_summary=遊んだ場面の感想"},
		}})
	_ = base.AddPost(host.ID, world.Post{BoardID: board.ID, Subject: "human private text",
		Body: "do not log", Intent: world.PostIntent{Action: "human"}})
	_ = base.AddPost(host.ID, world.Post{BoardID: "20/2", Subject: "different board",
		Intent: world.PostIntent{Action: bbsengine.ActionWorldCatchup}})

	records := captureGeneratedContentRecords(t, func() {
		repo.logNewBBSHeaders(host, board, before)
		// A repeat read with the same canonical snapshot emits nothing.
		repo.logNewBBSHeaders(host, board, base.ListPosts(host.ID))
	})
	if len(records) != 1 {
		t.Fatalf("got %d records, want 1: %+v", len(records), records)
	}
	got := records[0]
	if got.Stage != "header_committed" || got.PostID != newPost.ID || got.PostID == old.ID ||
		got.Subject != newPost.Subject || got.SituationSummary != newPost.Intent.SituationSummary ||
		len(got.SituationFacts) != 1 || got.SituationFacts[0] != newPost.Intent.SituationFacts[0] {
		t.Fatalf("incomplete/mismatched committed header log: %+v", got)
	}
	if got.Body != "" {
		t.Fatalf("header audit should not include unmaterialized body: %+v", got)
	}

	repo.SetDebugLogHAKATAGenerated(false)
	if got := captureGeneratedContentRecords(t, func() { repo.logBBSGeneratedContent("header_committed", host, board, newPost) }); len(got) != 0 {
		t.Fatalf("disabled audit emitted content: %+v", got)
	}
	repo.SetDebugLogHAKATAGenerated(true)
	other := host
	other.ID = "some-other-host"
	other.Debug.ContentLog = false // a different host that did not opt in
	if got := captureGeneratedContentRecords(t, func() { repo.logBBSGeneratedContent("header_committed", other, board, newPost) }); len(got) != 0 {
		t.Fatalf("audit leaked to another host: %+v", got)
	}
}

func TestHAKATAAuditLogsBodyAfterSaveOnlyOnce(t *testing.T) {
	base := world.NewMemoryStore()
	host, err := base.HostByPhone("0920000196")
	if err != nil {
		t.Fatal(err)
	}
	board := world.Board{ID: "20/1", Name: "GAME"}
	repo := New(base, observationTestEvidence{}, &blockingObservationMaterializer{
		body: "１回目の記録\n２行目も残す",
	}, "1996-08-26")
	repo.SetDebugLogHAKATAGenerated(true)
	p := base.AddPost(host.ID, world.Post{BoardID: board.ID, Author: "NPC", Subject: "固定された件名",
		CreatedAt: time.Date(1996, 8, 26, 21, 0, 0, 0, time.Local),
		Intent: world.PostIntent{Action: bbsengine.ActionWorldCatchup, ArticleDetailsMaterialized: true},
	})
	records := captureGeneratedContentRecords(t, func() {
		first, found, generated, diagnostic := repo.MaterializationArticleWithDebug(host, board, p.ID)
		if !found || !generated || strings.Contains(diagnostic, "error stage=") {
			t.Fatalf("first body failed: found=%t generated=%t diagnostic=%s", found, generated, diagnostic)
		}
		second, found, generated, diagnostic := repo.MaterializationArticleWithDebug(host, board, p.ID)
		if !found || generated || strings.Contains(diagnostic, "error stage=") || first.Body != second.Body {
			t.Fatalf("cached body read failed: found=%t generated=%t diagnostic=%s", found, generated, diagnostic)
		}
		persisted, ok := repo.findMaterializationPost(host.ID, board.ID, p.ID)
		if !ok || persisted.Body != first.Body || persisted.Subject != p.Subject {
			t.Fatalf("logged body is not committed or title changed: %+v", persisted)
		}
	})
	if len(records) != 1 || records[0].Stage != "body_committed" || records[0].PostID != p.ID ||
		records[0].Subject != p.Subject || records[0].Body != "１回目の記録\n２行目も残す" {
		t.Fatalf("body log missing, duplicated or altered: %+v", records)
	}
}

func TestHAKATAAuditNeverLogsHumanBody(t *testing.T) {
	base := world.NewMemoryStore()
	host, err := base.HostByPhone("0920000196")
	if err != nil {
		t.Fatal(err)
	}
	repo := New(base, nil, nil, "1996-08-26")
	repo.SetDebugLogHAKATAGenerated(true)
	board := world.Board{ID: "20/1"}
	human := world.Post{ID: 42, Subject: "human", Body: "private", Intent: world.PostIntent{Action: "user-post"}}
	records := captureGeneratedContentRecords(t, func() {
		repo.logBBSGeneratedContent("body_committed", host, board, human)
		repo.logBBSGeneratedContent("header_committed", host, board, human)
	})
	if len(records) != 0 {
		t.Fatalf("logged human content: %+v", records)
	}
}
