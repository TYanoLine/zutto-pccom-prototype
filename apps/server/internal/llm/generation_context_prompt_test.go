package llm

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

type generationContextRoundTripFunc func(*http.Request) (*http.Response, error)

func (f generationContextRoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestRootDraftsPromptExplainsServiceAndLetsTheBoardDecideThePostForm(t *testing.T) {
	var capturedPrompt string
	provider := StructuredOpenAIProvider{OpenAIProvider: OpenAIProvider{
		Endpoint: "https://test.openai.azure.com",
		APIKey:   "test-key",
		Model:    "gpt-test",
		Client: &http.Client{Transport: generationContextRoundTripFunc(func(req *http.Request) (*http.Response, error) {
			var payload map[string]any
			if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
				t.Fatalf("decode request: %v", err)
			}
			capturedPrompt, _ = payload["input"].(string)
			response := `{
				"model":"gpt-test",
				"output":[{"content":[{"type":"output_text","text":"{\"roots\":{\"slot-1\":{\"object_class\":\"game\",\"occurrence\":\"サクラ大戦を進めていて先へ進む手掛かりに迷った\",\"post_content\":\"詰まった場面を示し、他の会員がどう進めたか尋ねる\",\"novelty_key\":\"sakura-progress-question\",\"must_not\":[],\"titles\":[\"サクラ大戦の進め方\"]}}}"}]}],
				"usage":{"input_tokens":30,"input_tokens_details":{"cached_tokens":0},"output_tokens":20,"output_tokens_details":{"reasoning_tokens":0},"total_tokens":50}
			}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Status:     "200 OK",
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(response)),
			}, nil
		})},
	}}

	result, err := provider.GenerateBBSRootDrafts(context.Background(), BBSRootDraftRequest{
		HostName:   "TEST BBS",
		HostRegion: "福岡県",
		BoardID:    "4",
		BoardName:  "GAME",
		WorldDate:  "1996-08-29",
		Events: []BBSWorldWindowEvent{{
			EventID:        "slot-1",
			BoardID:        "4",
			BoardName:      "GAME",
			AuthorHandle:   "YUKI",
			CreatedAt:      "1996-08-28T23:15:00+09:00",
			Action:         "thread_start",
			AnchorKey:      "4",
			CauseKind:      "board_activity_window",
			PersonaProfile: "games=0.8",
			ExistingFacts:  []string{"ALLOWED HISTORICAL REFERENT FOR THIS EVENT DATE: サクラ大戦"},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Roots) != 1 || result.Roots["slot-1"].PostContent == "" || result.Roots["slot-1"].Occurrence == "" || len(result.Roots["slot-1"].Titles) != 1 {
		t.Fatalf("root draft was not decoded into the generic fields: %+v", result.Roots)
	}

	for _, want := range []string{
		"「ずっとパソコン通信」の内部生成",
		"利用者が見ていない間も続いている永続世界",
		"確定済みSituationの話題をBBS一覧で伝える短い件名",
		"post_content",
	} {
		if !strings.Contains(capturedPrompt, want) {
			t.Fatalf("Root draft prompt missing %q:\n%s", want, capturedPrompt)
		}
	}
	for _, old := range []string{"固定事項:", "discourse_modeの意味:", `"post_purpose"`, `"discourse_mode"`} {
		if strings.Contains(capturedPrompt, old) {
			t.Fatalf("Root draft prompt retained %q:\n%s", old, capturedPrompt)
		}
	}
}

func TestBoardPostPromptExplainsSavedArticlePurpose(t *testing.T) {
	prompt := BuildBoardPostPrompt(BoardPostRequest{
		HostName:         "TEST BBS",
		BoardTopic:       "GAME",
		WorldDate:        "1996-08-29",
		AuthorHandle:     "YUKI",
		CanonicalSubject: "サクラ大戦、ここからどう進めた？",
		PostIntent:       "occurrence=サクラ大戦を進めていて先へ進む手掛かりに迷った",
	})
	for _, want := range []string{
		"「ずっとパソコン通信」の内部生成",
		"会員が読む記事本文として文章化し、保存・表示",
		"canonical Situation / thread facts",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("body prompt missing %q:\n%s", want, prompt)
		}
	}
}
