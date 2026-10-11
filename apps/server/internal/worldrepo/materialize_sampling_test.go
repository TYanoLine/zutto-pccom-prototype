package worldrepo

import (
	"context"
	"strings"
	"testing"
	"time"

	"zutto-pccom/apps/server/internal/historicalkb"
	"zutto-pccom/apps/server/internal/llm"
	"zutto-pccom/apps/server/internal/world"
	"zutto-pccom/apps/server/internal/worldengine"
)

// TestMaterializeSampling_RestrainedMemoryVerification verifies that:
// 1. Casual introductory/general threads do not trigger tool calls (restrained).
// 2. Direct inquiries about past settings trigger exactly one targeted tool call and produce a helpful 1996-appropriate response.
// 3. New unrelated topics from known peers focus on the current topic without unsolicited digression.
func TestMaterializeSampling_RestrainedMemoryVerification(t *testing.T) {
	ctx := context.Background()
	store := newTestStore()

	// Seed historical data: MARI previously posted about modem settings.
	store.AddPost(genericTestHostID, world.Post{
		ID:              201,
		BoardID:         "tech",
		Author:          "MARI",
		AuthorPersonaID: "mari",
		Subject:         "モデムのATコマンド覚書",
		Body:            "AT&F&C1&D2 で安定しました。フロー制御は RTS/CTS です。",
		CreatedAt:       time.Date(1996, 5, 10, 14, 0, 0, 0, time.UTC),
	})

	persona := world.Persona{
		ID:           "mari",
		Handle:       "MARI",
		WritingStyle: "明るく親しみやすい。時々 (^^) や (笑) を使う。",
	}

	decision := worldengine.EvidenceDecision{
		Level:     historicalkb.EvidenceAtmospheric,
		Knowledge: historicalkb.KnowledgeResult{CanUse: true},
	}

	t.Run("Scenario A: Casual introduction - no tool call needed", func(t *testing.T) {
		toolCallsCount := 0
		model := &mockToolTurnModel{
			onComplete: func(req llm.ToolTurnRequest) (llm.ToolTurnResponse, error) {
				toolCallsCount++
				// First turn: model recognizes no past reference needed and directly answers.
				return llm.ToolTurnResponse{
					Text: `{"author":"MARI","subject":"はじめまして！","body":"はじめまして、MARIと申します (^^)\r\n最近こちらのネットに参加しました。普段はパソコン通信とレトロPCいじりをしています。どうぞよろしくお願いします！"}`,
				}, nil
			},
		}

		m := LLMMaterializer{
			ToolModel:        model,
			ActorMemoryStore: store,
		}

		req := BoardMaterializationRequest{
			Host:             world.Host{ID: genericTestHostID, Name: "CYBER NET"},
			BoardID:          "general",
			BoardTopic:       "自己紹介・雑談",
			WorldDate:        "1996-05-15",
			Persona:          &persona,
			CanonicalSubject: "はじめまして！",
			Intent: world.PostIntent{
				DiscourseMode: "greet",
				Topic:         "自己紹介",
			},
		}

		posts, _, err := m.GenerateBoardPostsWithUsage(ctx, req, decision)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(posts) != 1 {
			t.Fatalf("expected 1 post, got %d", len(posts))
		}
		if toolCallsCount != 1 {
			t.Errorf("expected exactly 1 model call (no tool turn needed), got %d", toolCallsCount)
		}
		t.Logf("[Scenario A Generated Body]:\n%s", posts[0].Body)

		if strings.Contains(posts[0].Body, "AT&F") {
			t.Errorf("unsolicited memory leaked into casual intro: %s", posts[0].Body)
		}
	})

	t.Run("Scenario B: Direct inquiry about past settings - targeted tool call", func(t *testing.T) {
		toolCallsCount := 0
		model := &mockToolTurnModel{
			onComplete: func(req llm.ToolTurnRequest) (llm.ToolTurnResponse, error) {
				toolCallsCount++
				if toolCallsCount == 1 {
					// Round 1: Model sees inquiry about past modem command, issues ToolGetMyRecentPosts.
					return llm.ToolTurnResponse{
						ToolCalls: []llm.ToolCall{
							{
								ID:        "call_1",
								Name:      llm.ToolGetMyRecentPosts,
								Arguments: []byte(`{"limit":3}`),
							},
						},
					}, nil
				}
				// Round 2: Model receives tool result containing AT command and replies naturally.
				var toolResultReceived bool
				for _, msg := range req.Messages {
					if msg.Role == "tool" && msg.ToolResult != nil && strings.Contains(msg.ToolResult.Content, "AT&F&C1&D2") {
						toolResultReceived = true
						break
					}
				}
				if !toolResultReceived {
					t.Errorf("expected tool result with AT command in round 2 messages")
				}
				return llm.ToolTurnResponse{
					Text: `{"author":"MARI","subject":"Re: モデムの設定教えてください","body":"> モデムの初期化コマンド、何にしてましたっけ？\r\n\r\n以前試したときは「AT&F&C1&D2」で安定して接続できましたよ。\r\nフロー制御をRTS/CTSにするのがコツみたいです (^^)"}`,
				}, nil
			},
		}

		m := LLMMaterializer{
			ToolModel:        model,
			ActorMemoryStore: store,
		}

		req := BoardMaterializationRequest{
			Host:             world.Host{ID: genericTestHostID, Name: "CYBER NET"},
			BoardID:          "tech",
			BoardTopic:       "モデム技術相談",
			WorldDate:        "1996-05-15",
			Persona:          &persona,
			CanonicalSubject: "Re: モデムの設定教えてください",
			ParentPost: &world.Post{
				Subject: "モデムの設定教えてください",
				Body:    "MARIさん、以前書かれていたモデムの初期化コマンド、何にしてましたっけ？",
			},
			Intent: world.PostIntent{
				DiscourseMode: "reply",
				Topic:         "モデム初期化コマンドの確認",
			},
		}

		posts, _, err := m.GenerateBoardPostsWithUsage(ctx, req, decision)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(posts) != 1 {
			t.Fatalf("expected 1 post, got %d", len(posts))
		}
		if toolCallsCount != 2 {
			t.Errorf("expected 2 model calls (1 tool call + 1 answer), got %d", toolCallsCount)
		}
		t.Logf("[Scenario B Generated Body]:\n%s", posts[0].Body)

		if !strings.Contains(posts[0].Body, "AT&F&C1&D2") {
			t.Errorf("expected verified command in answer: %s", posts[0].Body)
		}
	})

	t.Run("Scenario C: Unrelated new topic from peer - restraint without digression", func(t *testing.T) {
		toolCallsCount := 0
		model := &mockToolTurnModel{
			onComplete: func(req llm.ToolTurnRequest) (llm.ToolTurnResponse, error) {
				toolCallsCount++
				// Model does not call tool or dig up modem info because topic is game music CD.
				return llm.ToolTurnResponse{
					Text: `{"author":"MARI","subject":"Re: 新しいCD買いました","body":"> クロノ・トリガーのサントラを買いました！\r\n\r\n光田さんの曲、本当に素晴らしいですよね！\r\n私も『風の憧憬』が特にお気に入りで、作業中に何度もリピートして聴いています (笑)"}`,
				}, nil
			},
		}

		m := LLMMaterializer{
			ToolModel:        model,
			ActorMemoryStore: store,
		}

		req := BoardMaterializationRequest{
			Host:             world.Host{ID: genericTestHostID, Name: "CYBER NET"},
			BoardID:          "music",
			BoardTopic:       "音楽・CD",
			WorldDate:        "1996-05-15",
			Persona:          &persona,
			CanonicalSubject: "Re: 新しいCD買いました",
			ParentPost: &world.Post{
				Subject: "新しいCD買いました",
				Body:    "クロノ・トリガーのサントラを買いました！どの曲も名曲揃いですね。",
			},
			Intent: world.PostIntent{
				DiscourseMode: "reply",
				Topic:         "ゲーム音楽の感想",
			},
		}

		posts, _, err := m.GenerateBoardPostsWithUsage(ctx, req, decision)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(posts) != 1 {
			t.Fatalf("expected 1 post, got %d", len(posts))
		}
		if toolCallsCount != 1 {
			t.Errorf("expected 1 call (no tool needed for unrelated music discussion), got %d", toolCallsCount)
		}
		t.Logf("[Scenario C Generated Body]:\n%s", posts[0].Body)

		if strings.Contains(posts[0].Body, "モデム") || strings.Contains(posts[0].Body, "AT&F") {
			t.Errorf("unsolicited modem discussion leaked into music topic: %s", posts[0].Body)
		}
	})
}
