package worldrepo

import (
	"context"
	"fmt"
	"testing"
	"time"

	"zutto-pccom/apps/server/internal/llm"
	"zutto-pccom/apps/server/internal/world"
	"zutto-pccom/apps/server/internal/worldengine"
)

type fakeTimelineMaterializer struct{}

func (fakeTimelineMaterializer) GenerateBoardPosts(_ context.Context, req BoardMaterializationRequest, _ worldengine.EvidenceDecision) ([]world.Post, error) {
	return []world.Post{{Author: req.Persona.Handle, Subject: req.CanonicalSubject, Body: "body: " + req.Intent.Goal}}, nil
}

func (fakeTimelineMaterializer) PlanDevelopmentTimeline(_ context.Context, _ world.Host, board world.Board, _ string, shells []developmentTimelineShell, _ map[string][]world.PersonaFact, _ string) (developmentTimelinePlan, error) {
	events := make([]developmentTimelinePlanEvent, 0, len(shells))
	for _, shell := range shells {
		subject := fmt.Sprintf("%s / %s / %02d", board.Name, shell.persona.Handle, shell.index)
		events = append(events, developmentTimelinePlanEvent{
			index: shell.index, subject: subject,
			topic: fmt.Sprintf("free semantic topic %02d", shell.index),
			motivation: "test semantic motivation", stance: "test stance", goal: "test free-form goal",
			facts: []llm.BBSIntentFactDraft{{Key: fmt.Sprintf("personal.observation.%02d", shell.index), Value: fmt.Sprintf("%sの投稿時点で必要になった個人的事実%d", shell.persona.Handle, shell.index)}},
		})
	}
	return developmentTimelinePlan{events: events, usage: GenerationUsage{Model: "fake-planner", TotalTokens: 10}}, nil
}

func TestPersonaMaterializationUsesGenericPlannerForContent(t *testing.T) {
	base := world.NewMemoryStore()
	repo := New(base, nil, fakeTimelineMaterializer{}, "1996-08-29")
	h, err := repo.HostByPhone("0450000196")
	if err != nil { t.Fatal(err) }
	boards, _ := repo.MaterializationBoards(h)

	posts, created := repo.MaterializationPersonaArticleHeaders(h, boards[0])
	if !created { t.Fatal("persona-driven envelopes should be created on first board observation") }
	if len(posts) < 15 || len(posts) > 36 { t.Fatalf("envelope count=%d, want bounded sampled history", len(posts)) }

	counts := map[string]int{}
	replies := 0
	for i, p := range posts {
		if p.Body != "" { t.Fatalf("post %d body was eagerly materialized", p.ID) }
		if p.AuthorPersonaID == "" { t.Fatalf("post %d has no persistent persona", p.ID) }
		if p.Intent.Action == "" || p.Intent.Topic == "" || p.Intent.Goal == "" { t.Fatalf("post %d missing free-form semantic intent: %+v", p.ID, p.Intent) }
		if p.ParentID != 0 { replies++ }
		if i > 0 && p.CreatedAt.Before(posts[i-1].CreatedAt) { t.Fatalf("posts not chronological") }
		if p.CreatedAt.After(worldTime("1996-08-29")) { t.Fatalf("post %d generated in future", p.ID) }
		counts[p.Author]++
	}
	if replies < 1 { t.Fatal("expected at least one reply topology") }
	if counts["NEKO"] <= counts["TAKA"] { t.Fatalf("active/lurker bias not visible: NEKO=%d TAKA=%d", counts["NEKO"], counts["TAKA"]) }
	if posts[len(posts)-1].CreatedAt.Sub(posts[0].CreatedAt) < 8*24*time.Hour { t.Fatalf("history span too short") }

	reused, created := repo.MaterializationPersonaArticleHeaders(h, boards[0])
	if created || len(reused) != len(posts) || reused[0].ID != posts[0].ID { t.Fatal("stored semantic history was not reused") }
}

func TestActorTimelineIsDeterministicBeforeSemanticPlanning(t *testing.T) {
	materialize := func() []world.Post {
		base := world.NewMemoryStore()
		repo := New(base, nil, fakeTimelineMaterializer{}, "1996-08-29")
		h, _ := repo.HostByPhone("0450000196")
		boards, _ := repo.MaterializationBoards(h)
		posts, _ := repo.MaterializationPersonaArticleHeaders(h, boards[0])
		return posts
	}
	first, second := materialize(), materialize()
	if len(first) != len(second) { t.Fatalf("history length changed: %d vs %d", len(first), len(second)) }
	for i := range first {
		a, b := first[i], second[i]
		if a.Author != b.Author || a.Subject != b.Subject || a.ParentID != b.ParentID || a.Intent.Action != b.Intent.Action || a.Intent.Topic != b.Intent.Topic || !a.CreatedAt.Equal(b.CreatedAt) {
			t.Fatalf("history differs at %d:\nA=%+v\nB=%+v", i, a, b)
		}
	}
}

func TestBoardAffinityUsesPersonaInterestsWithoutChoosingContent(t *testing.T) {
	base := world.NewMemoryStore()
	repo := New(base, nil, nil, "1996-08-29")
	h, _ := repo.HostByPhone("0450000196")
	personas, _ := repo.MaterializationPersonas(h)
	yuki, _ := personaByHandle(personas, "YUKI")
	nori, _ := personaByHandle(personas, "NORI")
	boards, _ := repo.MaterializationBoards(h)
	if demoBoardAffinity(nori, boards[1]) <= demoBoardAffinity(yuki, boards[1]) { t.Fatal("NORI should have stronger technical-board affinity than YUKI") }
	if demoBoardAffinity(yuki, boards[0]) <= 0 { t.Fatal("YUKI should have nonzero free-talk affinity") }
}
