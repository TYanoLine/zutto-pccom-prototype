package worldrepo

import (
	"context"
	"strings"
	"testing"
	"time"

	"zutto-pccom/apps/server/internal/llm"
	"zutto-pccom/apps/server/internal/world"
	"zutto-pccom/apps/server/internal/worldengine"
)

type periodCaptureRenderer struct {
	discourseCaptureRenderer
	situation llm.BBSWorldSituationProposalRequest
	article llm.BoardPostRequest
}

func (r *periodCaptureRenderer) GenerateBoardPost(_ context.Context, req llm.BoardPostRequest) (llm.BoardPostDraft, error) {
	r.article = req
	return llm.BoardPostDraft{Subject: req.CanonicalSubject, Body: "本文"}, nil
}

func (r *periodCaptureRenderer) GenerateBBSWorldSituationProposals(_ context.Context, req llm.BBSWorldSituationProposalRequest) (llm.BBSWorldSituationProposalDraft, error) {
	r.situation = req
	var out llm.BBSWorldSituationProposalDraft
	for _, event := range req.Events { out.Situations = append(out.Situations, llm.BBSWorldSituationDraft{EventID: event.EventID}) }
	return out, nil
}

func TestPeriodSupportReachesSituationProducerAndArticle(t *testing.T) {
	renderer := &periodCaptureRenderer{}
	m := LLMMaterializer{Renderer: renderer, CuratedHistoricalReferences: true}
	host := world.Host{ID: "h", Name: "test"}
	date := time.Date(1996, 2, 26, 12, 0, 0, 0, time.FixedZone("JST", 9*3600))
	shell := developmentWindowShell{eventID: "e", board: world.Board{ID: "g", Name: "ゲーム"}, shell: developmentTimelineShell{createdAt: date, action: "thread_start", anchorKey: "games"}}
	if _, err := m.PlanDevelopmentWorldSituations(context.Background(), host, "1996-03-01", []developmentWindowShell{shell}, nil, "", nil); err != nil { t.Fatal(err) }
	if _, err := m.PlanDevelopmentWorldWindow(context.Background(), host, "1996-03-01", []developmentWindowShell{shell}, nil, ""); err != nil { t.Fatal(err) }
	if _, err := m.GenerateBoardPosts(context.Background(), BoardMaterializationRequest{Host: host, WorldDate: "1996-02-26", CanonicalSubject: "PlayStation"}, worldengine.EvidenceDecision{}); err != nil { t.Fatal(err) }
	for stage, value := range map[string]string{
		"situation": strings.Join(renderer.situation.HistoricalFacts, "\n"),
		"producer": renderer.req.EraRules,
		"article": strings.Join(renderer.article.HistoricalFacts, "\n"),
	} {
		if !strings.Contains(value, "PlayStation") { t.Fatalf("%s lost named support", stage) }
		if strings.Contains(value, "ポケットモンスター") { t.Fatalf("%s leaked a later release into an earlier event", stage) }
	}
	if renderer.article.CanonicalSubject != "PlayStation" { t.Fatal("canonical name lost") }
	if len(m.HistoricalTexture) != 0 { t.Fatal("request mutated shared materializer") }
}

func TestPeriodSupportPreservesOffAndExplicitTexture(t *testing.T) {
	off := (LLMMaterializer{}).withPeriodReferents("1996-08-29")
	if len(off.HistoricalTexture) != 0 || !strings.Contains(off.planningEraRules(), "HISTORICAL_REFERENCES=OFF") { t.Fatal("off changed") }
	m := LLMMaterializer{CuratedHistoricalReferences: true, HistoricalTexture: []string{"fixture"}}
	copy := m.withPeriodReferents("1996-08-29")
	copy.HistoricalTexture[0] = "changed"
	if m.HistoricalTexture[0] != "fixture" { t.Fatal("shared explicit texture mutated") }
	if got := m.withPeriodReferents("invalid"); len(got.HistoricalTexture) != 1 { t.Fatal("invalid date added evidence") }
}
