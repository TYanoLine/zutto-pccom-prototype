package worldrepo

import (
	"context"
	"fmt"
	"testing"

	"zutto-pccom/apps/server/internal/llm"
	"zutto-pccom/apps/server/internal/world"
)

type titleFirstTestRenderer struct {
	fakeBoardRenderer
	calls  int
	reject bool
}

func (f *titleFirstTestRenderer) GenerateBBSTitleCandidates(context.Context, string, string) (llm.BBSTitleCandidates, error) {
	f.calls++
	titles := []string{}
	for i := 0; i < 20; i++ {
		titles = append(titles, fmt.Sprintf("話題%d", i))
	}
	return llm.BBSTitleCandidates{Titles: titles}, nil
}
func (f *titleFirstTestRenderer) ReviewBBSTitleCandidates(_ context.Context, r llm.BBSTitleReviewRequest) (llm.BBSTitleReview, error) {
	decisions := []llm.BBSTitleDecision{}
	for i := range r.Titles {
		d := llm.BBSTitleDecision{Candidate: i + 1, Reason: "適合枠なし"}
		if i == 0 && len(r.Events) > 0 && !f.reject {
			d.EventID = r.Events[0].EventID
			d.Subject = r.Titles[i]
			d.Summary = "感想を共有"
			d.Reason = "整合"
		}
		decisions = append(decisions, d)
	}
	return llm.BBSTitleReview{Decisions: decisions}, nil
}
func TestTitleFirstPreservesSubjectAndArchivesRejectedCandidates(t *testing.T) {
	base := world.NewMemoryStore()
	renderer := &titleFirstTestRenderer{fakeBoardRenderer: fakeBoardRenderer{draft: llm.BoardPostDraft{Author: "WRONG", Subject: "書き換えられた件名", Body: "感想です。"}}}
	repo := New(base, conversationViewEvidenceEngine{}, LLMMaterializer{Renderer: renderer}, "1996-08-29")
	repo.EnableDevelopmentConversationViewPoC()
	repo.EnableDevelopmentTitleFirstPoC(nil)
	host, err := repo.HostByPhone("0450000196")
	if err != nil {
		t.Fatal(err)
	}
	boards, _ := repo.MaterializationBoards(host)
	repo.MaterializationPersonaArticleHeaders(host, boards[0])
	rows := repo.DevelopmentTitleCandidates()
	if len(rows) == 0 || len(rows)%20 != 0 {
		t.Fatalf("missing candidate pool: %v", rows)
	}
	all := base.ListPosts(host.ID)
	foundRoot := false
	for _, post := range all {
		if post.ParentID != 0 || post.Intent.SourcePostID != 0 {
			if titleFirstSubject(post.Intent.SituationFacts) != "" {
				t.Fatal("reply inherited fixed root subject")
			}
			continue
		}
		foundRoot = true
		if post.Subject != "話題0" {
			t.Fatal(post.Subject)
		}
		rendered, found, created, diag := repo.MaterializationArticleWithDebug(host, world.Board{ID: post.BoardID, Name: "雑談"}, post.ID)
		if !found || !created || rendered.Subject != post.Subject || renderer.req.CanonicalSubject != post.Subject {
			t.Fatalf("subject changed: %+v %s", rendered, diag)
		}
	}
	if !foundRoot {
		t.Fatal("no accepted root")
	}
	calls := renderer.calls
	repo.MaterializationPersonaArticleHeaders(host, boards[0])
	if renderer.calls != calls {
		t.Fatal("regenerated candidate pool")
	}
}
func TestTitleFirstAllRejectedDoesNotRegenerateOrCreateOrphans(t *testing.T) {
	base := world.NewMemoryStore()
	renderer := &titleFirstTestRenderer{reject: true}
	repo := New(base, nil, LLMMaterializer{Renderer: renderer}, "1996-08-29")
	repo.EnableDevelopmentConversationViewPoC()
	repo.EnableDevelopmentTitleFirstPoC(nil)
	host, err := repo.HostByPhone("0450000196")
	if err != nil {
		t.Fatal(err)
	}
	repo.materializeConversationWorldWindow(host)
	calls := renderer.calls
	repo.materializeConversationWorldWindow(host)
	if calls == 0 || renderer.calls != calls || len(base.ListPosts(host.ID)) != 0 {
		t.Fatalf("calls %d -> %d posts %d", calls, renderer.calls, len(base.ListPosts(host.ID)))
	}
}
