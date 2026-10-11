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

func TestMaterializeSampling_PersonaToneAndRelationship(t *testing.T) {
	ctx := context.Background()
	store := newTestStore()
	host := experimentTestHost()
	store.SaveHost(host)

	// Seed actors: KAZU.O (author of root post) and JOKER.N (regular peer)
	kazuActor := world.ActorRef{Kind: "persona", ID: "kazu"}
	jokerActor := world.ActorRef{Kind: "persona", ID: "joker"}

	// Seed relationship: KAZU.O and JOKER.N have interacted 4 times (regulars)
	_ = store.UpdateRelationship(world.RelationshipEdge{
		HostID:          host.ID,
		FromActor:       jokerActor,
		ToActor:         kazuActor,
		BoardReplyCount: 4,
	})

	// Parent post: KAZU.O offers Tactics Ogre lending
	parentPost := world.Post{
		BoardID:         "game",
		Author:          "KAZU.O",
		AuthorPersonaID: "kazu",
		Subject:         "タクティクスオウガ、クリアしたので貸せます",
		Body:            "SFC版の『タクティクスオウガ』、ひとまずクリアしました。しばらく遊ぶ予定もないので、やってみたい方がいたらお貸しします。希望の方はこの板かメールで連絡ください。",
		CreatedAt:       time.Date(1996, 2, 11, 16, 0, 0, 0, time.UTC),
	}
	savedParent := store.AddPost(host.ID, parentPost)

	decision := worldengine.EvidenceDecision{
		Level:     historicalkb.EvidenceAtmospheric,
		Knowledge: historicalkb.KnowledgeResult{CanUse: true},
	}

	t.Run("Scenario 1: Regular peer (JOKER.N) - casual clubroom tone without stiff business mail", func(t *testing.T) {
		jokerPersona := world.Persona{
			ID:           "joker",
			Handle:       "JOKER.N",
			Age:          24,
			Gender:       "male",
			Occupation:   "会社員",
			WritingStyle: "気さくでテンポの良い口調。(笑) や (^^) を好む。",
		}
		store.SavePersona(jokerPersona)

		var capturedPrompt string
		model := &mockToolTurnModel{
			onComplete: func(req llm.ToolTurnRequest) (llm.ToolTurnResponse, error) {
				if len(req.Messages) > 0 {
					capturedPrompt = req.Messages[0].Content
				}
				return llm.ToolTurnResponse{
					Text: `{"author":"JOKER.N","subject":"Re: タクティクスオウガ、クリアしたので貸せます","body":"うわ、もう希望者が(笑) じゃあ自分は2番手でお願いします！\r\nオウガ面白いって評判ですよね〜。順番回ってきたら声かけてください (^^)"}`,
				}, nil
			},
		}

		repo := New(store, nil, LLMMaterializer{ToolModel: model, ActorMemoryStore: store}, "1996-02-12")
		repo.SetFreeformBody(true)

		replyPost := world.Post{
			BoardID:         "game",
			Author:          "JOKER.N",
			AuthorPersonaID: "joker",
			ParentID:        savedParent.ID,
			Subject:         "Re: タクティクスオウガ、クリアしたので貸せます",
			CreatedAt:       time.Date(1996, 2, 12, 10, 0, 0, 0, time.UTC),
			Intent: world.PostIntent{
				DiscourseMode:    "reply",
				Topic:            "貸出希望の表明",
				RespondsToPostID: savedParent.ID,
			},
		}
		savedReply := store.AddPost(host.ID, replyPost)

		result, found, generated, diag := repo.materializeArticleBodyOnce(
			host,
			world.Board{ID: "game", Name: "ゲーム"},
			savedReply,
		)
		if !found || !generated {
			t.Fatalf("materialize failed: found=%v generated=%v diag=%s", found, generated, diag)
		}

		// Verify that relationship context is injected into prompt
		if !strings.Contains(capturedPrompt, "常連仲間") {
			t.Errorf("expected regular peer relationship context in prompt, got:\n%s", capturedPrompt)
		}
		// Verify that persona traits are injected into prompt
		if !strings.Contains(capturedPrompt, "24歳・male・会社員") || !strings.Contains(capturedPrompt, "気さくでテンポの良い口調") {
			t.Errorf("expected persona traits in prompt, got:\n%s", capturedPrompt)
		}

		t.Logf("[Scenario 1 Regular Peer Output]:\n%s", result.Body)
	})

	t.Run("Scenario 2: First-time poster (JUN) - polite yet approachable tone", func(t *testing.T) {
		junPersona := world.Persona{
			ID:               "jun",
			Handle:           "JUN",
			Age:              19,
			Gender:           "female",
			Occupation:       "学生",
			WritingStyle:     "丁寧だが堅苦しくない親しみやすい口調。",
			NewcomerOpenness: 0.8,
		}
		store.SavePersona(junPersona)

		var capturedPrompt string
		model := &mockToolTurnModel{
			onComplete: func(req llm.ToolTurnRequest) (llm.ToolTurnResponse, error) {
				if len(req.Messages) > 0 {
					capturedPrompt = req.Messages[0].Content
				}
				return llm.ToolTurnResponse{
					Text: `{"author":"JUN","subject":"Re: タクティクスオウガ、クリアしたので貸せます","body":"はじめまして！ 私もオウガずっと気になっていたので、もし順番が回ってきたらぜひ遊ばせてください。急ぎませんのでゆっくりで大丈夫です (^_^)"}`,
				}, nil
			},
		}

		repo := New(store, nil, LLMMaterializer{ToolModel: model, ActorMemoryStore: store}, "1996-02-12")
		repo.SetFreeformBody(true)

		replyPost := world.Post{
			BoardID:         "game",
			Author:          "JUN",
			AuthorPersonaID: "jun",
			ParentID:        savedParent.ID,
			Subject:         "Re: タクティクスオウガ、クリアしたので貸せます",
			CreatedAt:       time.Date(1996, 2, 12, 11, 0, 0, 0, time.UTC),
			Intent: world.PostIntent{
				DiscourseMode:    "reply",
				Topic:            "初対面での貸出希望",
				RespondsToPostID: savedParent.ID,
			},
		}
		savedReply := store.AddPost(host.ID, replyPost)

		result, found, generated, diag := repo.materializeArticleBodyOnce(
			host,
			world.Board{ID: "game", Name: "ゲーム"},
			savedReply,
		)
		if !found || !generated {
			t.Fatalf("materialize failed: found=%v generated=%v diag=%s", found, generated, diag)
		}

		// Verify that first-time relationship context is injected into prompt
		if !strings.Contains(capturedPrompt, "初めてのやり取り（初対面）") {
			t.Errorf("expected first-time relationship context in prompt, got:\n%s", capturedPrompt)
		}
		if !strings.Contains(capturedPrompt, "19歳・female・学生") {
			t.Errorf("expected student persona traits in prompt, got:\n%s", capturedPrompt)
		}

		t.Logf("[Scenario 2 First-Time Poster Output]:\n%s", result.Body)
	})

	t.Run("Scenario 3: Argumentative tech persona (X68.G) - opinionated & logical tone", func(t *testing.T) {
		x68Persona := world.Persona{
			ID:                "x68g",
			Handle:            "X68.G",
			Age:               32,
			Gender:            "male",
			Occupation:        "エンジニア",
			WritingStyle:      "要点を明確に書く。技術的な理由を添える。",
			Argumentativeness: 0.85,
		}

		m := LLMMaterializer{
			Renderer: &fakeSamplingRenderer{
				onGenerate: func(req llm.BoardPostRequest) (llm.BoardPostDraft, error) {
					// Check prompt contains personality traits
					if !strings.Contains(req.PersonaProfile, "自分のこだわりや論理をはっきり述べる傾向がある") {
						t.Errorf("expected argumentative trait in PersonaProfile: %s", req.PersonaProfile)
					}
					return llm.BoardPostDraft{
						Author:  req.AuthorHandle,
						Subject: req.CanonicalSubject,
						Body:    "グラフィック処理の負荷を考えると、SFCのカスタム音源チップの使い方が秀逸ですね。特に多重スクロールの制御は見事だと思います。\r\n",
					}, nil
				},
			},
		}

		req := BoardMaterializationRequest{
			Host:             world.Host{ID: genericTestHostID, Name: "TEST NET"},
			BoardID:          "game",
			BoardTopic:       "ゲーム技術",
			WorldDate:        "1996-02-12",
			Persona:          &x68Persona,
			CanonicalSubject: "Re: オウガの描画エンジンについて",
		}

		posts, _, err := m.GenerateBoardPostsWithUsage(ctx, req, decision)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(posts) != 1 {
			t.Fatalf("expected 1 post, got %d", len(posts))
		}
		t.Logf("[Scenario 3 Argumentative Engineer Output]:\n%s", posts[0].Body)
	})
}

type fakeSamplingRenderer struct {
	onGenerate func(req llm.BoardPostRequest) (llm.BoardPostDraft, error)
}

func (f *fakeSamplingRenderer) GenerateBoardPost(ctx context.Context, req llm.BoardPostRequest) (llm.BoardPostDraft, error) {
	if f.onGenerate != nil {
		return f.onGenerate(req)
	}
	return llm.BoardPostDraft{Author: req.AuthorHandle, Subject: req.CanonicalSubject, Body: "ok"}, nil
}

