package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestBuildBoardPostPromptOmitsInfrastructureMetadata(t *testing.T) {
	prompt := BuildBoardPostPrompt(BoardPostRequest{BoardTopic: "音楽", WorldDate: "1996-06-07", AuthorHandle: "MARI", CanonicalSubject: "YMOを聴き直しています", PostIntent: "article_detail=observation:音の重なり方が以前より気になった"})
	for _, bad := range []string{"Host software family", "Board ID", "CURRENT WORLD SLOT", "routing_domain"} {
		if strings.Contains(prompt, bad) {
			t.Fatalf("prompt leaked infrastructure metadata %q", bad)
		}
	}
	if !strings.Contains(prompt, "YMOを聴き直しています") || !strings.Contains(prompt, "article_detail=") {
		t.Fatalf("canonical content missing: %s", prompt)
	}
}

func TestBuildBoardPostPromptAllowsSparseUnfinishedHumanPosts(t *testing.T) {
	prompt := BuildBoardPostPrompt(BoardPostRequest{BoardTopic: "ゲーム", WorldDate: "1996-06-07", AuthorHandle: "YUKI", PersonaProfile: "writing=勢いのある短文が多い。", CanonicalSubject: "最近こればかりやってます"})
	for _, want := range []string{
		"canonical Situation はすでに世界で起きた事実",
		"事実を全部説明する必要はない",
		"本文は用件から自然に始める",
		"ask_peersだけが質問を主目的",
		"文章をFAQ・解説・結論付きの整った記事へ無理に仕上げない",
		"writing=勢いのある短文が多い。",
		"世界日付は 1996-06-07",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("missing concise Situation-rendering guidance %q", want)
		}
	}
	for _, legacy := range []string{"utterance_attention", "10〜20文字程度ごと", "端末側の80桁級表示", "同じ三文構成"} {
		if strings.Contains(prompt, legacy) {
			t.Fatalf("legacy prose micromanagement survived refresh: %q", legacy)
		}
	}
}

func TestBuildBoardPostPromptKeepsCanonicalSubjectInsteadOfRewritingIt(t *testing.T) {
	prompt := BuildBoardPostPrompt(BoardPostRequest{
		BoardTopic:       "パソコン通信・モデム",
		WorldDate:        "1996-08-29",
		AuthorHandle:     "NORI",
		PersonaProfile:   "writing=短く要点を書くこともある",
		CanonicalSubject: "Windows 95でモデムが認識されません",
		PostIntent:       "discourse_mode=ask_peers\noccurrence=Windows 95でモデムが認識されず相談する",
	})
	for _, want := range []string{
		"件名: Windows 95でモデムが認識されません",
		"canonical Situation / thread facts",
		`{"author":"...","subject":"Windows 95でモデムが認識されません","body":"..."}`,
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("canonical subject contract missing %q: %s", want, prompt)
		}
	}
	for _, legacy := range []string{"意味判定用タイトル", "短縮、口語化、省略", "固定パターン化しない"} {
		if strings.Contains(prompt, legacy) {
			t.Fatalf("legacy title-rewrite guidance survived refresh: %q", legacy)
		}
	}
}

func TestValidateBoardPostWorkerDraftRejectsHeaderNarration(t *testing.T) {
	req := BoardPostRequest{}
	for _, body := range []string{"5/22 00:27、新スレです。おすすめありますか。", "音楽板のMSG 1201です。YMOの話です。", "音楽板から失礼します。"} {
		if err := validateBoardPostWorkerDraft(req, BoardPostDraft{Author: "MARI", Subject: "x", Body: body}); err == nil {
			t.Fatalf("accepted metadata narration: %q", body)
		}
	}
}

func TestGeminiProviderUsesInteractionsStructuredOutput(t *testing.T) {
	var got map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("x-goog-api-key") != "secret" {
			t.Errorf("missing gemini key header")
		}
		_ = json.NewDecoder(r.Body).Decode(&got)
		w.Header().Set("Content-Type", "application/json")
		inner, _ := json.Marshal(map[string]any{
			"author": "MARI", "subject": "YMOを聴き直しています",
			"body": "最近またYMOを聴いています。音の重なり方が前より気になります。",
		})
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status": "completed",
			"steps": []any{map[string]any{
				"type":    "model_output",
				"content": []any{map[string]any{"type": "text", "text": string(inner)}},
			}},
			"usage": map[string]any{
				"total_input_tokens": 10, "total_output_tokens": 20,
				"total_thought_tokens": 3, "total_tokens": 33,
			},
		})
	}))
	defer srv.Close()

	p := GeminiProvider{APIKey: "secret", Model: "gemini-3.8-flash", Endpoint: srv.URL, Client: srv.Client()}
	draft, err := p.GenerateBoardPost(context.Background(), BoardPostRequest{
		BoardTopic: "音楽", WorldDate: "1996-06-07", AuthorHandle: "MARI",
		CanonicalSubject: "YMOを聴き直しています",
	})
	if err != nil {
		t.Fatal(err)
	}
	if draft.Usage.Model != "gemini-3.8-flash" || draft.Usage.TotalTokens != 33 {
		t.Fatalf("usage=%+v", draft.Usage)
	}
	if got["model"] != "gemini-3.8-flash" {
		t.Fatalf("model=%v", got["model"])
	}
	cfg := got["generation_config"].(map[string]any)
	if cfg["thinking_level"] != "low" {
		t.Fatalf("thinking=%v", cfg)
	}
	rf := got["response_format"].(map[string]any)
	if rf["mime_type"] != "application/json" {
		t.Fatalf("response_format=%v", rf)
	}
}


func TestBuildBoardPostPromptRequiresRootReferentToSurface(t *testing.T) {
	prompt := BuildBoardPostPrompt(BoardPostRequest{
		BoardTopic:       "ANIME/MANGA",
		WorldDate:        "1996-08-12",
		AuthorHandle:     "KOJI.B",
		CanonicalSubject: "伏線に気づいた？",
		PostIntent: strings.Join([]string{
			"world_adopted_summary=前の回で聞き流した台詞が後の回を見て気になった",
			"article_detail=referent:新世紀エヴァンゲリオン",
			"article_detail=observation:前の回では聞き流した台詞が、後の回を見てから気になった",
			"article_referent_required=新世紀エヴァンゲリオン",
		}, "\n"),
	})
	for _, want := range []string{
		"canonical referent「新世紀エヴァンゲリオン」",
		"表示件名がこの対象名を明示していない場合",
		"本文の自然な位置で対象名を少なくとも一度",
		"対象を別の作品・製品・店等へ置き換えない",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("required referent guidance missing %q:\n%s", want, prompt)
		}
	}
	if strings.Contains(prompt, "article_referent_required=") {
		t.Fatalf("render-control metadata leaked into canonical fact block:\n%s", prompt)
	}
}

func TestExtractArticleReferentControlPreservesOtherFacts(t *testing.T) {
	intent, referent := extractArticleReferentControl(strings.Join([]string{
		"discourse_mode=share_observation",
		"article_referent_required=新世紀エヴァンゲリオン",
		"article_detail=observation:前の回の台詞が気になった",
	}, "\n"))
	if referent != "新世紀エヴァンゲリオン" {
		t.Fatalf("referent=%q", referent)
	}
	if strings.Contains(intent, "article_referent_required=") || !strings.Contains(intent, "article_detail=observation:") {
		t.Fatalf("unexpected cleaned intent: %q", intent)
	}
}
