package worldrepo

import (
	"context"
	"strings"
	"testing"

	"zutto-pccom/apps/server/internal/historicalkb"
	"zutto-pccom/apps/server/internal/llm"
	"zutto-pccom/apps/server/internal/world"
	"zutto-pccom/apps/server/internal/worldengine"
)

type conversationViewEvidenceEngine struct{}

func (conversationViewEvidenceEngine) ResolveEvidence(context.Context, worldengine.EvidenceRequest) (worldengine.EvidenceDecision, error) {
	return worldengine.EvidenceDecision{
		Level:     historicalkb.EvidenceAtmospheric,
		Knowledge: historicalkb.KnowledgeResult{CanUse: true},
	}, nil
}

func TestConversationViewPoCStoresWorldShellsWithoutProducerBriefs(t *testing.T) {
	base := world.NewMemoryStore()
	repo := New(base, nil, nil, "1996-08-29")
	repo.EnableDevelopmentConversationViewPoC()
	host, err := repo.HostByPhone("0450000196")
	if err != nil {
		t.Fatal(err)
	}
	boards, _ := repo.MaterializationBoards(host)
	posts, created := repo.MaterializationPersonaArticleHeaders(host, boards[0])
	if !created || len(posts) == 0 {
		t.Fatalf("conversation view did not create board shells: created=%v posts=%d", created, len(posts))
	}
	all := base.ListPosts(host.ID)
	if len(all) < len(posts) {
		t.Fatalf("host window was not committed: host=%d board=%d", len(all), len(posts))
	}
	for _, post := range all {
		if post.Intent.ProducerEventID != "" || post.Intent.ProducerEpisode != "" || len(post.Intent.ProducerContribution) != 0 {
			t.Fatalf("producer brief leaked into conversation-view shell: %#v", post.Intent)
		}
		if post.Intent.Action == "" || post.Intent.AnchorKey == "" || post.Intent.CauseKind == "" || post.Intent.Motivation == "" {
			t.Fatalf("world shell missing immutable cause/topology data: %#v", post.Intent)
		}
		if post.Intent.SituationKind == "" || post.Intent.SituationSummary == "" || len(post.Intent.SituationFacts) == 0 {
			t.Fatalf("world shell missing sparse canonical situation: %#v", post.Intent)
		}
		if post.Body != "" {
			t.Fatalf("header phase unexpectedly rendered body: msg=%d body=%q", post.ID, post.Body)
		}
	}
}

func TestConversationViewPoCLetsWorkerChooseRootSubjectFromConversationContext(t *testing.T) {
	base := world.NewMemoryStore()
	renderer := &fakeBoardRenderer{draft: llm.BoardPostDraft{Author: "WRONG", Subject: "自然に決めた件名", Body: "会話の流れで書いた本文です。"}}
	repo := New(base, conversationViewEvidenceEngine{}, LLMMaterializer{Renderer: renderer}, "1996-08-29")
	repo.EnableDevelopmentConversationViewPoC()
	host, err := repo.HostByPhone("0450000196")
	if err != nil {
		t.Fatal(err)
	}
	boards, _ := repo.MaterializationBoards(host)
	posts, _ := repo.MaterializationPersonaArticleHeaders(host, boards[0])
	var root world.Post
	for _, post := range posts {
		if post.ParentID == 0 && post.Intent.SourcePostID == 0 {
			root = post
			break
		}
	}
	if root.ID == 0 {
		t.Fatal("no independent root post selected")
	}
	rendered, found, created, diagnostic := repo.MaterializationArticleWithDebug(host, boards[0], root.ID)
	if !found || !created || rendered.Subject != "自然に決めた件名" {
		t.Fatalf("generated root subject was not committed: found=%v created=%v post=%#v diag=%q", found, created, rendered, diagnostic)
	}
	if renderer.req.CanonicalSubject != "" {
		t.Fatalf("conversation worker was still forced to canonical subject: %q", renderer.req.CanonicalSubject)
	}
	if renderer.req.BoardTopic != boards[0].Name {
		t.Fatalf("worker cue should be board conversation, got %q want %q", renderer.req.BoardTopic, boards[0].Name)
	}
	for _, want := range []string{"canonical_event=", "focus=", "occurrence=", "scope_boundary="} {
		if !strings.Contains(renderer.req.PostIntent, want) {
			t.Fatalf("compact worker content missing %q: %s", want, renderer.req.PostIntent)
		}
	}
	for _, leaked := range []string{"CONVERSATION VIEW POC", "CURRENT WORLD SLOT", "WORLD-LAYER CAUSE BOUNDARY", "CANONICAL BOARD CONVERSATION", "MSG "} {
		if strings.Contains(renderer.req.PostIntent, leaked) {
			t.Fatalf("debug metadata leaked into worker context %q: %s", leaked, renderer.req.PostIntent)
		}
	}
	if strings.Contains(renderer.req.PostIntent, "producer_event_id=") {
		t.Fatalf("producer worker contract remained active: %s", renderer.req.PostIntent)
	}
}

func TestConversationViewPoCReplyUsesRenderedParentAsChatHistory(t *testing.T) {
	base := world.NewMemoryStore()
	renderer := &fakeBoardRenderer{draft: llm.BoardPostDraft{Author: "WRONG", Subject: "会話の件", Body: "親から順番に生成された本文です。"}}
	repo := New(base, conversationViewEvidenceEngine{}, LLMMaterializer{Renderer: renderer}, "1996-08-29")
	repo.EnableDevelopmentConversationViewPoC()
	host, err := repo.HostByPhone("0450000196")
	if err != nil {
		t.Fatal(err)
	}
	boards, _ := repo.MaterializationBoards(host)
	_, _ = repo.MaterializationPersonaArticleHeaders(host, boards[0])
	all := base.ListPosts(host.ID)
	var reply world.Post
	var board world.Board
	for _, candidate := range all {
		if candidate.ParentID == 0 {
			continue
		}
		for _, b := range boards {
			if b.ID == candidate.BoardID {
				reply = candidate
				board = b
				break
			}
		}
		if reply.ID != 0 {
			break
		}
	}
	if reply.ID == 0 {
		t.Fatal("no reply selected in conversation window")
	}
	source, ok := developmentConversationFindPost(all, reply.Intent.SourcePostID)
	if !ok {
		t.Fatalf("reply source %d not found", reply.Intent.SourcePostID)
	}
	if reply.Intent.SituationKind != source.Intent.SituationKind {
		t.Fatalf("reply did not inherit source situation: reply=%q source=%q", reply.Intent.SituationKind, source.Intent.SituationKind)
	}
	rendered, found, created, diagnostic := repo.MaterializationArticleWithDebug(host, board, reply.ID)
	if !found || !created {
		t.Fatalf("reply render failed: found=%v created=%v diag=%q", found, created, diagnostic)
	}
	if rendered.Subject != "Re: 会話の件" {
		t.Fatalf("reply subject not canonicalized from rendered root: %q", rendered.Subject)
	}
	if !strings.Contains(renderer.req.PostIntent, "THREAD CONTEXT (canonical article content only)") || !strings.Contains(renderer.req.PostIntent, "親から順番に生成された本文です。") {
		t.Fatalf("reply did not receive compact prior prose context: %s", renderer.req.PostIntent)
	}
	if strings.Contains(renderer.req.PostIntent, "MSG ") || strings.Contains(renderer.req.PostIntent, "board=") {
		t.Fatalf("reply worker context leaked transport metadata: %s", renderer.req.PostIntent)
	}
	if renderer.req.CanonicalSubject != "" {
		t.Fatalf("reply worker should choose prose with subject canonicalized after render: %q", renderer.req.CanonicalSubject)
	}
}

func TestConversationViewDoesNotCommitAnonymizedTopicTarget(t *testing.T) {
	base := world.NewMemoryStore()
	renderer := &fakeBoardRenderer{draft: llm.BoardPostDraft{Author: "NEKO", Subject: "このゲームの話", Body: "架空ゲームAの感想です。"}}
	repo := New(base, conversationViewEvidenceEngine{}, LLMMaterializer{Renderer: renderer}, "1996-08-29")
	repo.EnableDevelopmentConversationViewPoC()
	host, err := repo.HostByPhone("0450000196")
	if err != nil {
		t.Fatal(err)
	}
	boards, _ := repo.MaterializationBoards(host)
	posts, _ := repo.MaterializationPersonaArticleHeaders(host, boards[0])
	var root world.Post
	for _, p := range posts {
		if p.ParentID == 0 && p.Intent.SourcePostID == 0 {
			root = p
			break
		}
	}
	if root.ID == 0 {
		t.Fatal("no root")
	}
	root.Intent.SituationFacts = append(root.Intent.SituationFacts, "topic_target=架空ゲームA")
	base.UpdatePost(host.ID, root)
	_, found, created, diag := repo.MaterializationArticleWithDebug(host, boards[0], root.ID)
	if !found || created || !strings.Contains(diag, "stage=subject") {
		t.Fatalf("found=%v created=%v diag=%s", found, created, diag)
	}
	stored, _ := developmentConversationFindPost(base.ListPosts(host.ID), root.ID)
	if stored.Body != "" || stored.Subject != root.Subject {
		t.Fatal("invalid subject was committed")
	}
	renderer.draft.Subject = "架空ゲームAの感想"
	got, _, created, diag := repo.MaterializationArticleWithDebug(host, boards[0], root.ID)
	if !created || got.Subject != renderer.draft.Subject {
		t.Fatalf("retry failed: %s", diag)
	}
}
