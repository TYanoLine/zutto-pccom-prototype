package worldrepo

import (
	"context"
	"fmt"
	"strings"
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
			topic: fmt.Sprintf("realized %s event %02d", shell.anchorKey, shell.index),
			motivation: "test semantic realization of a fixed world cause", stance: "test stance", goal: "test free-form goal",
			facts: []llm.BBSIntentFactDraft{{Key: fmt.Sprintf("personal.observation.%02d", shell.index), Value: fmt.Sprintf("%sの投稿時点で必要になった個人的事実%d", shell.persona.Handle, shell.index)}},
		})
	}
	return developmentTimelinePlan{events: events, usage: GenerationUsage{Model: "fake-planner", TotalTokens: 10}}, nil
}

func TestPersonaMaterializationUsesSparseCausalPlanner(t *testing.T) {
	base := world.NewMemoryStore()
	repo := New(base, nil, fakeTimelineMaterializer{}, "1996-08-29")
	h, err := repo.HostByPhone("0450000196")
	if err != nil { t.Fatal(err) }
	boards, _ := repo.MaterializationBoards(h)

	posts, created := repo.MaterializationPersonaArticleHeaders(h, boards[0])
	if !created { t.Fatal("causal envelopes should be created on first board observation") }
	if len(posts) < 5 || len(posts) > 24 { t.Fatalf("envelope count=%d, want sparse bounded sampled history", len(posts)) }

	value, ok := developmentSelectionTelemetry.Load(developmentPlanningKey{repo: repo, hostID: h.ID, boardID: boards[0].ID})
	if !ok { t.Fatal("missing sparse selection telemetry") }
	stats := value.(developmentSelectionStats)
	if stats.Visits <= stats.Posts || stats.ROM <= 0 { t.Fatalf("activity must not imply a post: %+v", stats) }
	if stats.Posts != len(posts) { t.Fatalf("selection posts=%d committed=%d", stats.Posts, len(posts)) }

	counts := map[string]int{}
	replies := 0
	for i, p := range posts {
		if p.Body != "" { t.Fatalf("post %d body was eagerly materialized", p.ID) }
		if p.AuthorPersonaID == "" { t.Fatalf("post %d has no persistent persona", p.ID) }
		if p.Intent.Action == "" || p.Intent.AnchorKey == "" || p.Intent.CauseKind == "" || p.Intent.Topic == "" || p.Intent.Goal == "" {
			t.Fatalf("post %d missing causal intent: %+v", p.ID, p.Intent)
		}
		if p.ParentID != 0 {
			replies++
			if p.Intent.CauseKind != "observed_thread" || p.Intent.SourcePostID == 0 || p.Intent.RespondsToPostID == 0 {
				t.Fatalf("reply %d missing observed-thread cause: %+v", p.ID, p.Intent)
			}
		}
		if i > 0 && p.CreatedAt.Before(posts[i-1].CreatedAt) { t.Fatalf("posts not chronological") }
		if p.CreatedAt.After(worldTime("1996-08-29")) { t.Fatalf("post %d generated in future", p.ID) }
		counts[p.Author]++
	}
	if replies < 1 { t.Fatal("expected at least one reply topology") }
	if counts["NEKO"] <= counts["TAKA"] { t.Fatalf("active/lurker bias not visible: NEKO=%d TAKA=%d", counts["NEKO"], counts["TAKA"]) }
	if posts[len(posts)-1].CreatedAt.Sub(posts[0].CreatedAt) < 5*24*time.Hour { t.Fatalf("history span too short") }

	reused, created := repo.MaterializationPersonaArticleHeaders(h, boards[0])
	if created || len(reused) != len(posts) || reused[0].ID != posts[0].ID { t.Fatal("stored causal history was not reused") }
}

func TestActorTimelineIsDeterministicBeforeSemanticRealization(t *testing.T) {
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
		if a.Author != b.Author || a.Subject != b.Subject || a.ParentID != b.ParentID || a.Intent.Action != b.Intent.Action || a.Intent.AnchorKey != b.Intent.AnchorKey || a.Intent.CauseKind != b.Intent.CauseKind || a.Intent.SourcePostID != b.Intent.SourcePostID || a.Intent.Topic != b.Intent.Topic || !a.CreatedAt.Equal(b.CreatedAt) {
			t.Fatalf("history differs at %d:\nA=%+v\nB=%+v", i, a, b)
		}
	}
}

func TestTechnicalBoardRootsUseWorldSelectedRelevantAnchors(t *testing.T) {
	base := world.NewMemoryStore()
	repo := New(base, nil, fakeTimelineMaterializer{}, "1996-08-29")
	h, _ := repo.HostByPhone("0450000196")
	boards, _ := repo.MaterializationBoards(h)
	posts, created := repo.MaterializationPersonaArticleHeaders(h, boards[1])
	if !created || len(posts) == 0 { t.Fatal("technical board should materialize some causal posts") }

	allowed := map[string]bool{"communications": true, "modem": true, "software": true, "bbs": true}
	for _, post := range posts {
		if !allowed[post.Intent.AnchorKey] {
			t.Fatalf("technical board post escaped world-selected board anchors: msg=%d anchor=%q intent=%+v", post.ID, post.Intent.AnchorKey, post.Intent)
		}
		if post.Intent.AnchorKey == "pc98" {
			t.Fatalf("ordinary machine family leaked back into topic routing: msg=%d", post.ID)
		}
	}
}

func TestPersonaSeparatesEverydayBaselineFromInterests(t *testing.T) {
	base := world.NewMemoryStore()
	repo := New(base, nil, nil, "1996-08-29")
	h, _ := repo.HostByPhone("0450000196")
	personas, _ := repo.MaterializationPersonas(h)
	nori, _ := personaByHandle(personas, "NORI")
	taka, _ := personaByHandle(personas, "TAKA")
	if _, found := nori.Interests["pc98"]; found { t.Fatal("PC-98 family must not be modeled as NORI's conversational interest") }
	if _, found := taka.Interests["pc98"]; found { t.Fatal("PC-98 family must not be modeled as TAKA's conversational interest") }
	if len(nori.EverydayContext) == 0 || len(taka.EverydayContext) == 0 { t.Fatal("ordinary environment baseline should be explicit") }
	summary := personaSummary(nori)
	if !strings.Contains(summary, "everyday_baseline=") || !strings.Contains(summary, "normally unspoken") {
		t.Fatalf("persona summary does not distinguish ordinary baseline: %s", summary)
	}
}

func TestBoardAffinityUsesPersonaInterestsWithoutChoosingProse(t *testing.T) {
	base := world.NewMemoryStore()
	repo := New(base, nil, nil, "1996-08-29")
	h, _ := repo.HostByPhone("0450000196")
	personas, _ := repo.MaterializationPersonas(h)
	yuki, _ := personaByHandle(personas, "YUKI")
	nori, _ := personaByHandle(personas, "NORI")
	boards, _ := repo.MaterializationBoards(h)
	if demoBoardAffinity(nori, boards[1]) <= demoBoardAffinity(yuki, boards[1]) { t.Fatal("NORI should have stronger technical-board affinity than YUKI") }
	if demoBoardAffinity(yuki, boards[0]) <= 0 { t.Fatal("YUKI should have nonzero free-talk affinity") }
	if demoInterestBoardRelevance(boards[1], "games") != 0 { t.Fatal("games should not seed a technical-board root in this development fixture") }
	if demoInterestBoardRelevance(boards[1], "communications") <= 0 { t.Fatal("communications should be a relevant technical-board routing domain") }
	if demoInterestBoardRelevance(boards[1], "pc98") != 0 { t.Fatal("ordinary machine family must not seed a root topic") }
}
