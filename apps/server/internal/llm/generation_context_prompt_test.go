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

func TestSituationPromptExplainsServiceAndLetsTheBoardDecideThePostForm(t *testing.T) {
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
				"output":[{"content":[{"type":"output_text","text":"{\"situations\":{\"slot-1\":{\"object_class\":\"game\",\"occurrence\":\"サクラ大戦を進めていて先へ進む手掛かりに迷った\",\"post_content\":\"詰まった場面を示し、他の会員がどう進めたか尋ねる\",\"novelty_key\":\"sakura-progress-question\",\"must_not\":[]}}}"}]}],
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

	draft, err := provider.GenerateBBSWorldSituationProposals(context.Background(), BBSWorldSituationProposalRequest{
		HostName:   "TEST BBS",
		HostRegion: "福岡県",
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
	if len(draft.Situations) != 1 || draft.Situations[0].PostContent == "" || draft.Situations[0].Occurrence == "" {
		t.Fatalf("situation was not decoded into the generic fields: %+v", draft.Situations)
	}

	for _, want := range []string{
		"「ずっとパソコン通信」の内部生成",
		"利用者が見ていない間も続いている永続世界",
		"後段のBBS件名と記事本文を生成する材料",
		"post_content",
	} {
		if !strings.Contains(capturedPrompt, want) {
			t.Fatalf("Situation prompt missing %q:\n%s", want, capturedPrompt)
		}
	}
	for _, old := range []string{"固定事項:", "discourse_modeの意味:", `"post_purpose"`, `"discourse_mode"`} {
		if strings.Contains(capturedPrompt, old) {
			t.Fatalf("Situation prompt retained %q:\n%s", old, capturedPrompt)
		}
	}
}

func TestSituationTitlePromptExplainsDownstreamDisplayPurpose(t *testing.T) {
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
				"output":[{"content":[{"type":"output_text","text":"{\"titles\":{\"slot-1\":\"サクラ大戦、ここからどう進めた？\"}}"}]}],
				"usage":{"input_tokens":20,"input_tokens_details":{"cached_tokens":0},"output_tokens":10,"output_tokens_details":{"reasoning_tokens":0},"total_tokens":30}
			}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Status:     "200 OK",
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(response)),
			}, nil
		})},
	}}

	_, err := provider.GenerateBBSSituationTitles(context.Background(), BBSSituationTitleRequest{
		HostName:   "TEST BBS",
		HostRegion: "福岡県",
		BoardID:    "4",
		BoardName:  "GAME",
		WorldDate:  "1996-08-29",
		Articles: []BBSSituationTitleSeed{{
			EventID:          "slot-1",
			AuthorHandle:     "YUKI",
			CreatedAt:        "1996-08-28T23:15:00+09:00",
			SituationKind:    "recent_salience",
			SituationSummary: "サクラ大戦を進めていて先へ進む手掛かりに迷った",
			SituationFacts:   []string{"post_content=他の会員がどう進めたか聞きたい"},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}

	for _, want := range []string{
		"「ずっとパソコン通信」の内部生成",
		"すでに正本化されたSituationからBBSの記事一覧に表示するroot件名",
		"確定済みSituationの話題をBBS一覧で伝える短い件名",
	} {
		if !strings.Contains(capturedPrompt, want) {
			t.Fatalf("title prompt missing %q:\n%s", want, capturedPrompt)
		}
	}
	if strings.Contains(capturedPrompt, `"discourse_mode"`) {
		t.Fatalf("title prompt still carries a discourse mode:\n%s", capturedPrompt)
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
