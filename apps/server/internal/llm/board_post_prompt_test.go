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
		"本文に全部書くチェックリストではありません",
		"多少雑でも構いません",
		"この1件だけを切り出して完全に理解できる文章にする必要はありません",
		"毎回「みなさんはどうですか？」型で締めない",
		"本文で全detailを列挙する義務はありません",
		"writing= を最優先",
		"同じ三文構成に揃えない",
		"一文だけでも、多段落でも",
		"具体的な操作手順",
		"将来の予定を「自然な補足」として作らない",
		"utterance_attention",
		"元記事を要約してから返事を始めない",
		"完全な解説記事やチュートリアルへ仕上げない",
		"10〜20文字程度ごと",
		"端末側の80桁級表示",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("missing conversational guidance %q", want)
		}
	}
	if strings.Contains(prompt, "1〜3文でもよく") {
		t.Fatalf("fixed sentence-count hint should not survive: %s", prompt)
	}
}

func TestBuildBoardPostPromptSeparatesSemanticAndSurfaceSubjectForTitleFirstRoot(t *testing.T) {
	prompt := BuildBoardPostPrompt(BoardPostRequest{
		BoardTopic:       "パソコン通信・モデム",
		WorldDate:        "1996-08-29",
		AuthorHandle:     "NORI",
		PersonaProfile:   "writing=短く要点を書くこともある",
		CanonicalSubject: "Windows 95でモデムが認識されません",
		PostIntent:       "surface_subject_mode=title_first_root\ndiscourse_mode=ask_peers\ncanonical_event=Windows 95でモデムが認識されず相談する",
	})
	for _, want := range []string{
		"意味判定用タイトル",
		"表示件名",
		"短縮、口語化、省略",
		"モデムが見えない…",
		"固定パターン化しない",
		"36文字以内・1行",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("surface-subject guidance missing %q: %s", want, prompt)
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
