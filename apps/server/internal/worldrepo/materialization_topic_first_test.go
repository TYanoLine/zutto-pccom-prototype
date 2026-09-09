package worldrepo

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"zutto-pccom/apps/server/internal/historicalkb"
	"zutto-pccom/apps/server/internal/world"
	"zutto-pccom/apps/server/internal/worldengine"
)

type topicEvidence struct {
	mu       sync.Mutex
	requests []worldengine.EvidenceRequest
	status   historicalkb.FactStatus
}

func (e *topicEvidence) ResolveEvidence(_ context.Context, req worldengine.EvidenceRequest) (worldengine.EvidenceDecision, error) {
	e.mu.Lock()
	e.requests = append(e.requests, req)
	e.mu.Unlock()
	return worldengine.EvidenceDecision{Knowledge: historicalkb.KnowledgeResult{CanUse: true, Facts: []historicalkb.HistoricalFact{{Status: e.status, Claim: "CANDIDATE: 架空ゲームA || fixture evidence\nCANDIDATE: 架空ゲームB || fixture evidence"}}}}, nil
}

func TestTopicFirstSelectsBeforeOccurrenceAndKeepsEarliestDate(t *testing.T) {
	e := &topicEvidence{status: historicalkb.FactVerified}
	r := New(world.NewMemoryStore(), e, nil, "1996-08-29")
	date := time.Date(1996, 8, 20, 0, 0, 0, 0, time.UTC)
	roots := []developmentWindowShell{
		{eventID: "a", board: world.Board{ID: "4", Name: "ゲーム"}, shell: developmentTimelineShell{anchorKey: "games", createdAt: date.AddDate(0, 0, 1)}},
		{eventID: "b", board: world.Board{ID: "4", Name: "ゲーム"}, shell: developmentTimelineShell{anchorKey: "games", createdAt: date}},
		{eventID: "c", board: world.Board{ID: "3"}, shell: developmentTimelineShell{anchorKey: "local", createdAt: date}},
	}
	got, err := r.developmentSelectTopicTargets(world.Host{ID: "h"}, roots)
	if err != nil {
		t.Fatal(err)
	}
	if len(e.requests) != 1 || e.requests[0].WorldDate != "1996-08-20" {
		t.Fatalf("requests=%+v", e.requests)
	}
	if !strings.Contains(e.requests[0].Need, "出来事を作る前") {
		t.Fatal("target must precede occurrence")
	}
	if got[0].topicTarget == nil || got[1].topicTarget == nil || got[0].topicTarget.Name == got[1].topicTarget.Name {
		t.Fatalf("targets=%+v", got)
	}
	if got[2].topicTarget != nil || got[2].topicTargetStatus != "general_topic" {
		t.Fatal("general root was forced to use a name")
	}
	if roots[0].topicTarget != nil || got[0].shell.createdAt != roots[0].shell.createdAt {
		t.Fatal("mutated input/topology")
	}
}

func TestTopicFirstDoesNotUseUnverifiedCandidates(t *testing.T) {
	e := &topicEvidence{status: historicalkb.FactProvisional}
	r := New(world.NewMemoryStore(), e, nil, "1996-08-29")
	got, err := r.developmentSelectTopicTargets(world.Host{}, []developmentWindowShell{{eventID: "a", shell: developmentTimelineShell{anchorKey: "games", createdAt: time.Now()}}})
	if err != nil {
		t.Fatal(err)
	}
	if got[0].topicTarget != nil || got[0].topicTargetStatus != "no_verified_candidate" {
		t.Fatalf("got=%+v", got)
	}
}

func TestTopicTargetSubjectMatching(t *testing.T) {
	for _, tt := range []struct {
		subject, target string
		want            bool
	}{
		{"ＶＦ２の話", "VF2", true}, {"バーチャファイター２の感想", "バーチャファイター2", true},
		{"このゲームの話", "VF2", false}, {"最近なに遊んでます？", "", false},
	} {
		if got := TopicTargetInSubject(tt.subject, tt.target); got != tt.want {
			t.Fatalf("%+v got %v", tt, got)
		}
	}
}

func TestTopicTargetMustSurviveSituationProposal(t *testing.T) {
	root := developmentWindowShell{eventID: "a", topicTarget: &developmentGroundingCandidate{Name: "架空ゲームA"}}
	p := developmentSituationProposal{objectClass: "ゲーム", changeClass: "感想", occurrence: "面白かった", actorObservation: "遊んだ", noveltyKey: "impression"}
	if reason := developmentValidateOneSituationProposal(root, p, map[string]string{}, nil, nil); !strings.Contains(reason, "target missing") {
		t.Fatalf("reason=%s", reason)
	}
	p.objectClass = "架空ゲームA"
	if reason := developmentValidateOneSituationProposal(root, p, map[string]string{}, nil, nil); reason != "" {
		t.Fatal(reason)
	}
	p.topicTarget = "架空ゲームA"
	p.topicTargetStatus = "selected"
	s := developmentSparseSituationFromProposal(root.shell, p)
	if topicTargetFact(s.facts) != "架空ゲームA" || !strings.Contains(strings.Join(s.facts, "\n"), "subject_contract=") {
		t.Fatalf("facts=%v", s.facts)
	}
}

func TestSameTargetDoesNotAllowDuplicateOccurrence(t *testing.T) {
	roots := []developmentWindowShell{{eventID: "a", board: world.Board{ID: "4"}}, {eventID: "b", board: world.Board{ID: "4"}}}
	a := developmentSituationProposal{objectClass: "架空ゲームA", changeClass: "感想", occurrence: "友人と対戦して負けた", actorObservation: "負けた", noveltyKey: "a"}
	b := a
	b.noveltyKey = "b"
	_, rejected := developmentValidateSituationProposalBatch(roots, map[string]developmentSituationProposal{"a": a, "b": b}, nil)
	if len(rejected) != 1 {
		t.Fatalf("rejected=%v", rejected)
	}
}
