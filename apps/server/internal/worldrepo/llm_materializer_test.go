package worldrepo

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"zutto-pccom/apps/server/internal/historicalkb"
	"zutto-pccom/apps/server/internal/llm"
	"zutto-pccom/apps/server/internal/world"
	"zutto-pccom/apps/server/internal/worldengine"
)

type fakeBoardRenderer struct {
	req         llm.BoardPostRequest
	draft       llm.BoardPostDraft
	err         error
	intentReq   llm.BBSTimelineIntentRequest
	intentDraft llm.BBSTimelineIntentDraft
	plannerErr  error
}

func (f *fakeBoardRenderer) GenerateBoardPost(_ context.Context, r llm.BoardPostRequest) (llm.BoardPostDraft, error) {
	f.req = r
	return f.draft, f.err
}

func (f *fakeBoardRenderer) GenerateBBSTimelineIntent(_ context.Context, r llm.BBSTimelineIntentRequest) (llm.BBSTimelineIntentDraft, error) {
	f.intentReq = r
	if f.plannerErr != nil {
		return llm.BBSTimelineIntentDraft{}, f.plannerErr
	}
	if len(f.intentDraft.Events) > 0 {
		return f.intentDraft, nil
	}
	out := llm.BBSTimelineIntentDraft{Events: make([]llm.BBSIntentDraft, 0, len(r.Events))}
	for _, event := range r.Events {
		out.Events = append(out.Events, llm.BBSIntentDraft{
			Index:      event.Index,
			Subject:    fmt.Sprintf("%s %02d", r.BoardName, event.Index),
			Topic:      fmt.Sprintf("test-topic-%02d", event.Index),
			Motivation: "test semantic motivation",
			Stance:     "test semantic stance",
			Goal:       "test semantic goal",
		})
	}
	return out, nil
}

type failingFallback struct{ calls int }

func (f *failingFallback) GenerateBoardPosts(_ context.Context, _ BoardMaterializationRequest, _ worldengine.EvidenceDecision) ([]world.Post, error) {
	f.calls++
	return []world.Post{{Author: "FALLBACK", Subject: "fallback", Body: "fallback"}}, nil
}

func TestLLMMaterializerPassesOnlyUsableHistoricalFacts(t *testing.T) {
	renderer := &fakeBoardRenderer{draft: llm.BoardPostDraft{Author: "NORI", Subject: "モデムの話", Body: "最近ちょっと気になります。"}}
	m := LLMMaterializer{Renderer: renderer, Fallback: FallbackMaterializer{}}
	decision := worldengine.EvidenceDecision{Level: historicalkb.EvidencePlausible, Knowledge: historicalkb.KnowledgeResult{CanUse: true, Facts: []historicalkb.HistoricalFact{
		{Claim: "28.8kbps V.34 modem was available", Status: historicalkb.FactVerified},
		{Claim: "rejected claim", Status: historicalkb.FactRejected},
	}}}
	posts, err := m.GenerateBoardPosts(context.Background(), BoardMaterializationRequest{Host: world.Host{Name: "TEST NET", Region: "東京", Software: "KTBBS", SoftwareID: "ktbbs"}, BoardID: "60/1", BoardTopic: "PC-98/モデム", WorldDate: "1996-08-29"}, decision)
	if err != nil {
		t.Fatal(err)
	}
	if len(posts) != 1 || posts[0].Author != "NORI" {
		t.Fatalf("unexpected posts: %#v", posts)
	}
	if len(renderer.req.HistoricalFacts) != 1 || renderer.req.HistoricalFacts[0] != "28.8kbps V.34 modem was available" {
		t.Fatalf("unexpected facts: %#v", renderer.req.HistoricalFacts)
	}
}

func TestLLMMaterializerBindsCanonicalPersonaAndEnvelope(t *testing.T) {
	renderer := &fakeBoardRenderer{draft: llm.BoardPostDraft{Author: "WRONG", Subject: "違う件名", Body: "本文だけ生成しました。"}}
	m := LLMMaterializer{Renderer: renderer, Fallback: FallbackMaterializer{}}
	persona := world.Persona{ID: "p1", Handle: "MARI", Age: 21, Occupation: "短大生", ActivityPattern: "夜中心", ReplyTendency: .62, ThreadStartTendency: .30, LurkerTendency: .24, NewcomerOpenness: .88, Argumentativeness: .08, WritingStyle: "柔らかい口調"}
	intent := world.PostIntent{Action: "thread_start", Topic: "フリートーク", Motivation: "挨拶したい", Stance: "友好的"}
	posts, err := m.GenerateBoardPosts(context.Background(), BoardMaterializationRequest{Host: world.Host{Name: "TEST NET"}, BoardID: "1", BoardTopic: "はじめまして", WorldDate: "1996-08-29", Persona: &persona, Intent: intent, CanonicalSubject: "はじめまして"}, worldengine.EvidenceDecision{Level: historicalkb.EvidenceAtmospheric, Knowledge: historicalkb.KnowledgeResult{CanUse: true}})
	if err != nil {
		t.Fatal(err)
	}
	if len(posts) != 1 || posts[0].Author != "MARI" || posts[0].Subject != "はじめまして" {
		t.Fatalf("canonical actor/header were replaced: %#v", posts)
	}
	if renderer.req.AuthorHandle != "MARI" || renderer.req.CanonicalSubject != "はじめまして" {
		t.Fatalf("renderer did not receive canonical actor/header: %#v", renderer.req)
	}
	if renderer.req.PersonaProfile == "" || renderer.req.PostIntent == "" {
		t.Fatalf("renderer missing persona/intent context: %#v", renderer.req)
	}
}

func TestLLMMaterializerDoesNotFallbackForAtmosphericRenderFailure(t *testing.T) {
	renderer := &fakeBoardRenderer{err: errors.New("model unavailable")}
	fallback := &failingFallback{}
	m := LLMMaterializer{Renderer: renderer, Fallback: fallback}
	posts, err := m.GenerateBoardPosts(context.Background(), BoardMaterializationRequest{BoardTopic: "雑談", WorldDate: "1996-08-29"}, worldengine.EvidenceDecision{Level: historicalkb.EvidenceAtmospheric, Knowledge: historicalkb.KnowledgeResult{CanUse: true}})
	if err == nil || !errors.Is(err, renderer.err) {
		t.Fatalf("renderer failure must remain visible: posts=%#v err=%v", posts, err)
	}
	if fallback.calls != 0 {
		t.Fatalf("atmospheric failure invoked canned fallback: %d", fallback.calls)
	}
	if len(posts) != 0 {
		t.Fatalf("render failure returned synthetic posts: %#v", posts)
	}
}

func TestLLMMaterializerDoesNotFallbackForVerifiedFailure(t *testing.T) {
	renderer := &fakeBoardRenderer{err: errors.New("model unavailable")}
	fallback := &failingFallback{}
	m := LLMMaterializer{Renderer: renderer, Fallback: fallback}
	_, err := m.GenerateBoardPosts(context.Background(), BoardMaterializationRequest{BoardTopic: "製品仕様", WorldDate: "1996-08-29"}, worldengine.EvidenceDecision{Level: historicalkb.EvidenceVerified, Knowledge: historicalkb.KnowledgeResult{CanUse: true}})
	if err == nil {
		t.Fatal("verified render failure must propagate")
	}
	if fallback.calls != 0 {
		t.Fatalf("verified failure silently fell back: %d", fallback.calls)
	}
}

var _ llm.BoardPostRenderer = (*fakeBoardRenderer)(nil)
var _ llm.BBSTimelineIntentPlanner = (*fakeBoardRenderer)(nil)
