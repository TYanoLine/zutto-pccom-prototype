package worldrepo

import (
	"context"
	"strings"
	"testing"
	"time"

	"zutto-pccom/apps/server/internal/actormemory"
	"zutto-pccom/apps/server/internal/historicalkb"
	"zutto-pccom/apps/server/internal/llm"
	"zutto-pccom/apps/server/internal/world"
	"zutto-pccom/apps/server/internal/worldengine"
)

type mockToolTurnModel struct {
	calls        int
	onComplete   func(req llm.ToolTurnRequest) (llm.ToolTurnResponse, error)
	receivedMsgs []llm.ToolTurnMessage
}

func (m *mockToolTurnModel) Complete(ctx context.Context, req llm.ToolTurnRequest) (llm.ToolTurnResponse, error) {
	m.calls++
	m.receivedMsgs = append(m.receivedMsgs, req.Messages...)
	if m.onComplete != nil {
		return m.onComplete(req)
	}
	return llm.ToolTurnResponse{Text: `{"author":"MARI","subject":"件名","body":"本文です。"}`}, nil
}

func TestMaterializeToolWithToolCall(t *testing.T) {
	store := newTestStore()
	store.AddPost(genericTestHostID, world.Post{
		ID:              100,
		BoardID:         "1",
		Author:          "MARI",
		AuthorPersonaID: "mari",
		Subject:         "グラボの設定",
		Body:            "ディップスイッチSW2の3番をONにしました。",
		CreatedAt:       time.Date(1996, 8, 29, 10, 0, 0, 0, time.UTC),
	})

	toolModelCalls := 0
	toolModel := &mockToolTurnModel{
		onComplete: func(req llm.ToolTurnRequest) (llm.ToolTurnResponse, error) {
			toolModelCalls++
			if toolModelCalls == 1 {
				// 1回目: ツール呼び出しを要求
				return llm.ToolTurnResponse{
					ToolCalls: []llm.ToolCall{
						{
							ID:        "c1",
							Name:      llm.ToolGetMyRecentPosts,
							Arguments: []byte(`{"limit":3}`),
						},
					},
				}, nil
			}
			// 2回目: ツール結果を確認して最終本文を出力
			var foundToolResult bool
			for _, m := range req.Messages {
				if m.Role == "tool" && m.ToolResult != nil && strings.Contains(m.ToolResult.Content, "グラボの設定") {
					foundToolResult = true
				}
			}
			if !foundToolResult {
				t.Errorf("expected tool result containing 'グラボの設定' in messages: %+v", req.Messages)
			}
			return llm.ToolTurnResponse{
				Text: `{"author":"MARI","subject":"件名","body":"あ、さっきのグラボの件ですね！SW2の3番をONにすれば動きますよ (^^)"}`,
			}, nil
		},
	}

	persona := world.Persona{ID: "mari", Handle: "MARI"}
	m := LLMMaterializer{
		ToolModel:        toolModel,
		ActorMemoryStore: store,
	}

	req := BoardMaterializationRequest{
		Host:             world.Host{ID: genericTestHostID, Name: "TEST NET"},
		BoardID:          "1",
		BoardTopic:       "PC技術",
		WorldDate:        "1996-08-29",
		Persona:          &persona,
		CanonicalSubject: "Re: グラボどうでした？",
	}
	decision := worldengine.EvidenceDecision{Level: historicalkb.EvidenceAtmospheric, Knowledge: historicalkb.KnowledgeResult{CanUse: true}}

	posts, usage, err := m.GenerateBoardPostsWithUsage(context.Background(), req, decision)
	if err != nil {
		t.Fatalf("GenerateBoardPostsWithUsage failed: %v", err)
	}
	if len(posts) != 1 {
		t.Fatalf("posts len=%d, want 1", len(posts))
	}
	if !strings.Contains(posts[0].Body, "さっきのグラボの件") {
		t.Errorf("body = %q, want mention of tool result", posts[0].Body)
	}
	if toolModelCalls != 2 {
		t.Errorf("toolModelCalls = %d, want 2 (round 1 + round 2)", toolModelCalls)
	}
	if usage.Model != "tool-turn" {
		t.Errorf("usage.Model = %q, want tool-turn", usage.Model)
	}
}

type staticEmbedder struct{}

func (s staticEmbedder) Embed(_ context.Context, texts []string) ([][]float32, error) {
	out := make([][]float32, len(texts))
	for i := range texts {
		out[i] = []float32{1, 0, 0}
	}
	return out, nil
}

func TestMaterializeToolWithSearchIndex(t *testing.T) {
	store := newTestStore()
	store.AddPost(genericTestHostID, world.Post{
		ID:              100,
		BoardID:         "1",
		Author:          "MARI",
		AuthorPersonaID: "mari",
		Subject:         "グラボの設定",
		Body:            "ディップスイッチSW2の3番をONにしました。",
		CreatedAt:       time.Date(1996, 8, 29, 10, 0, 0, 0, time.UTC),
	})

	idx := actormemory.NewIndex(staticEmbedder{}, actormemory.IndexOptions{BatchSize: 1, FlushInterval: time.Millisecond})
	defer idx.Close()
	idx.Enqueue(actormemory.Item{
		EventID: "post:" + genericTestHostID + ":100",
		Scope:   "public_board:" + genericTestHostID + ":1",
		Text:    "グラボの設定 SW2の3番をON",
	})
	deadline := time.Now().Add(2 * time.Second)
	for idx.Len() != 1 {
		if time.Now().After(deadline) {
			t.Fatal("timed out waiting for index item")
		}
		time.Sleep(5 * time.Millisecond)
	}

	toolModel := &mockToolTurnModel{
		onComplete: func(req llm.ToolTurnRequest) (llm.ToolTurnResponse, error) {
			if len(req.Messages) == 1 {
				return llm.ToolTurnResponse{
					ToolCalls: []llm.ToolCall{
						{
							ID:        "c1",
							Name:      llm.ToolSearchMyObservedEvents,
							Arguments: []byte(`{"query":"グラボ"}`),
						},
					},
				}, nil
			}
			return llm.ToolTurnResponse{
				Text: `{"author":"MARI","subject":"件名","body":"検索で見つかりました！"}`,
			}, nil
		},
	}

	persona := world.Persona{ID: "mari", Handle: "MARI"}
	m := LLMMaterializer{
		ToolModel:        toolModel,
		ActorMemoryStore: store,
		ActorMemoryIndex: idx,
	}

	req := BoardMaterializationRequest{
		Host:             world.Host{ID: genericTestHostID, Name: "TEST NET"},
		BoardID:          "1",
		BoardTopic:       "PC技術",
		WorldDate:        "1996-08-29",
		Persona:          &persona,
		CanonicalSubject: "Re: グラボ設定",
	}
	decision := worldengine.EvidenceDecision{Level: historicalkb.EvidenceAtmospheric, Knowledge: historicalkb.KnowledgeResult{CanUse: true}}

	posts, _, err := m.GenerateBoardPostsWithUsage(context.Background(), req, decision)
	if err != nil {
		t.Fatalf("GenerateBoardPostsWithUsage failed: %v", err)
	}
	if len(posts) != 1 || !strings.Contains(posts[0].Body, "検索で見つかりました") {
		t.Errorf("unexpected posts: %+v", posts)
	}
}

func TestMaterializeToolWithoutToolCall(t *testing.T) {
	store := newTestStore()
	toolModelCalls := 0
	toolModel := &mockToolTurnModel{
		onComplete: func(req llm.ToolTurnRequest) (llm.ToolTurnResponse, error) {
			toolModelCalls++
			return llm.ToolTurnResponse{
				Text: `{"author":"MARI","subject":"こんにちは","body":"はじめまして、よろしくお願いします！"}`,
			}, nil
		},
	}

	persona := world.Persona{ID: "mari", Handle: "MARI"}
	m := LLMMaterializer{
		ToolModel:        toolModel,
		ActorMemoryStore: store,
	}

	req := BoardMaterializationRequest{
		Host:             world.Host{ID: genericTestHostID, Name: "TEST NET"},
		BoardID:          "1",
		BoardTopic:       "挨拶",
		WorldDate:        "1996-08-29",
		Persona:          &persona,
		CanonicalSubject: "はじめまして",
	}
	decision := worldengine.EvidenceDecision{Level: historicalkb.EvidenceAtmospheric, Knowledge: historicalkb.KnowledgeResult{CanUse: true}}

	posts, _, err := m.GenerateBoardPostsWithUsage(context.Background(), req, decision)
	if err != nil {
		t.Fatalf("GenerateBoardPostsWithUsage failed: %v", err)
	}
	if len(posts) != 1 {
		t.Fatalf("posts len=%d, want 1", len(posts))
	}
	if toolModelCalls != 1 {
		t.Errorf("toolModelCalls = %d, want 1 (no tool requested)", toolModelCalls)
	}
	if !strings.Contains(posts[0].Body, "はじめまして") {
		t.Errorf("body = %q", posts[0].Body)
	}
}

type fakeLegacyRenderer struct {
	called bool
}

func (f *fakeLegacyRenderer) GenerateBoardPost(ctx context.Context, req llm.BoardPostRequest) (llm.BoardPostDraft, error) {
	f.called = true
	return llm.BoardPostDraft{
		Author:  req.AuthorHandle,
		Subject: req.CanonicalSubject,
		Body:    "従来のレンダラーによる本文です。\r\n",
	}, nil
}

func TestMaterializeToolFallbackWhenToolModelNil(t *testing.T) {
	legacy := &fakeLegacyRenderer{}
	m := LLMMaterializer{
		Renderer: legacy,
	}

	persona := world.Persona{ID: "mari", Handle: "MARI"}
	req := BoardMaterializationRequest{
		Host:             world.Host{ID: genericTestHostID, Name: "TEST NET"},
		BoardID:          "1",
		BoardTopic:       "雑談",
		WorldDate:        "1996-08-29",
		Persona:          &persona,
		CanonicalSubject: "雑談スレ",
	}
	decision := worldengine.EvidenceDecision{Level: historicalkb.EvidenceAtmospheric, Knowledge: historicalkb.KnowledgeResult{CanUse: true}}

	posts, _, err := m.GenerateBoardPostsWithUsage(context.Background(), req, decision)
	if err != nil {
		t.Fatalf("GenerateBoardPostsWithUsage failed: %v", err)
	}
	if !legacy.called {
		t.Errorf("expected legacy renderer to be called as fallback")
	}
	if len(posts) != 1 || !strings.Contains(posts[0].Body, "従来のレンダラー") {
		t.Errorf("unexpected posts: %+v", posts)
	}
}
