package worldrepo

import (
	"context"
	"fmt"
	"testing"

	"zutto-pccom/apps/server/internal/historicalkb"
	"zutto-pccom/apps/server/internal/world"
	"zutto-pccom/apps/server/internal/worldengine"
)

type flowEngine struct{ calls int }

func (f *flowEngine) ResolveEvidence(_ context.Context, _ worldengine.EvidenceRequest) (worldengine.EvidenceDecision, error) {
	f.calls++
	return worldengine.EvidenceDecision{Level: historicalkb.EvidenceAtmospheric, ModelFirst: true, Knowledge: historicalkb.KnowledgeResult{CanUse: true}}, nil
}

type flowMaterializer struct {
	calls int
	last  BoardMaterializationRequest
}

func (f *flowMaterializer) GenerateBoardPosts(_ context.Context, req BoardMaterializationRequest, _ worldengine.EvidenceDecision) ([]world.Post, error) {
	f.calls++
	f.last = req
	return []world.Post{{Author: "AI", Subject: "unused", Body: "補完された本文です。"}}, nil
}

func (f *flowMaterializer) PlanDevelopmentTimeline(_ context.Context, _ world.Host, board world.Board, _ string, shells []developmentTimelineShell, _ map[string][]world.PersonaFact, _ string) (developmentTimelinePlan, error) {
	plan := developmentTimelinePlan{events: make([]developmentTimelinePlanEvent, 0, len(shells))}
	for _, shell := range shells {
		plan.events = append(plan.events, developmentTimelinePlanEvent{
			index:      shell.index,
			subject:    fmt.Sprintf("%s %02d", board.Name, shell.index),
			topic:      fmt.Sprintf("test-topic-%02d", shell.index),
			motivation: "test semantic motivation",
			stance:     "test semantic stance",
			goal:       "test semantic goal",
		})
	}
	return plan, nil
}

func TestDevelopmentMaterializationFlow(t *testing.T) {
	base := world.NewMemoryStore()
	engine := &flowEngine{}
	mat := &flowMaterializer{}
	repo := New(base, engine, mat, "1996-08-29")
	h, err := repo.HostByPhone("0450000196")
	if err != nil {
		t.Fatal(err)
	}
	if h.Name == "" || h.Lines == 0 || h.MaxBaud == 0 {
		t.Fatalf("host profile was not completed: %+v", h)
	}
	if !repo.HostWasMaterialized(h.ID) {
		t.Fatal("host materialization not recorded")
	}
	if !repo.PopulationWasMaterialized(h.ID) {
		t.Fatal("core population should materialize with first host observation")
	}

	personas, created := repo.MaterializationPersonas(h)
	if created {
		t.Fatal("core personas should already be stored after first host observation")
	}
	if len(personas) < 5 {
		t.Fatalf("core persona count=%d, want at least 5", len(personas))
	}
	if personas[0].ID == "" || personas[0].Handle == "" || personas[0].WritingStyle == "" {
		t.Fatalf("persona is missing persistent identity/profile: %+v", personas[0])
	}

	h2, err := repo.HostByPhone("0450000196")
	if err != nil {
		t.Fatal(err)
	}
	if h2.Name != h.Name {
		t.Fatal("completed host profile was not stored")
	}

	boards, created := repo.MaterializationBoards(h)
	if !created || len(boards) != 3 {
		t.Fatalf("boards created=%v len=%d", created, len(boards))
	}
	_, created = repo.MaterializationBoards(h)
	if created {
		t.Fatal("board catalog should be reused")
	}

	headers, created := repo.MaterializationArticleHeaders(h, boards[0])
	if !created || len(headers) == 0 {
		t.Fatalf("headers created=%v len=%d", created, len(headers))
	}
	if headers[0].Body != "" {
		t.Fatal("article body must stay unmaterialized until read")
	}
	if headers[0].AuthorPersonaID == "" {
		t.Fatal("article envelope must reference a persistent persona")
	}
	if headers[0].Intent.Action == "" || headers[0].Intent.Topic == "" {
		t.Fatalf("article envelope missing semantic intent: %+v", headers[0].Intent)
	}

	p, found, bodyCreated := repo.MaterializationArticle(h, boards[0], headers[0].ID)
	if !found || !bodyCreated || p.Body == "" {
		t.Fatalf("article found=%v created=%v body=%q", found, bodyCreated, p.Body)
	}
	if engine.calls != 1 || mat.calls != 1 {
		t.Fatalf("engine=%d materializer=%d, want 1 each", engine.calls, mat.calls)
	}
	if mat.last.Persona == nil || mat.last.Persona.ID != headers[0].AuthorPersonaID {
		t.Fatalf("materializer did not receive canonical persona: %+v", mat.last.Persona)
	}
	if mat.last.CanonicalSubject != headers[0].Subject {
		t.Fatalf("canonical subject=%q, want %q", mat.last.CanonicalSubject, headers[0].Subject)
	}
	if mat.last.Intent.Action != headers[0].Intent.Action || mat.last.Intent.Topic != headers[0].Intent.Topic {
		t.Fatalf("materializer intent=%+v, envelope=%+v", mat.last.Intent, headers[0].Intent)
	}

	p, found, bodyCreated = repo.MaterializationArticle(h, boards[0], headers[0].ID)
	if !found || bodyCreated || p.Body == "" {
		t.Fatalf("stored article should be reused: found=%v created=%v body=%q", found, bodyCreated, p.Body)
	}
	if engine.calls != 1 || mat.calls != 1 {
		t.Fatal("re-read unexpectedly regenerated article")
	}
}
