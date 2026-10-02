package llm

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

// The Situation prompt must bind topics to the board name and scope. Neither a
// routing/domain key nor the author's interests may decide what is posted.
func TestSituationPromptBindsTopicToBoardScope(t *testing.T) {
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
				"output":[{"content":[{"type":"output_text","text":"{\"situations\":{\"slot-1\":{\"object_class\":\"intro\",\"change_class\":\"new\",\"occurrence\":\"初めて書き込む前に呼び名を考えた\",\"attempted_actions\":\"ハンドルを決めて読み返した\",\"question\":\"署名は付けるものか聞きたい\",\"novelty_key\":\"intro-handle\",\"must_not\":[]}}}"}]}],
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

	_, err := provider.GenerateBBSWorldSituationProposals(context.Background(), BBSWorldSituationProposalRequest{
		HostName:   "TEST BBS",
		HostRegion: "福岡県",
		WorldDate:  "1996-08-29",
		Events: []BBSWorldWindowEvent{{
			EventID:        "slot-1",
			BoardID:        "2",
			BoardName:      "自己紹介・新人歓迎",
			BoardScope:     "新規会員の自己紹介、常連からの歓迎。",
			AuthorHandle:   "YUKI",
			CreatedAt:      "1996-08-28T23:15:00+09:00",
			Action:         "thread_start",
			AnchorKey:      "2",
			CauseKind:      "board_activity_window",
			DiscourseMode:  "ask_peers",
			PersonaProfile: "games=0.8",
		}},
	})
	if err != nil {
		t.Fatal(err)
	}

	for _, want := range []string{
		"board_name と board_scope は、その掲示板に投稿してよい話題の範囲を表します",
		"必ずこの板の範囲の内側で",
		"話題を決める根拠にはなりません",
		"範囲内の別の切り口を選んでください",
		`"board_scope":"新規会員の自己紹介、常連からの歓迎。"`,
	} {
		if !strings.Contains(capturedPrompt, want) {
			t.Fatalf("Situation prompt missing %q:\n%s", want, capturedPrompt)
		}
	}
	for _, old := range []string{"話題のプリセットではなく", `"anchor_key"`} {
		if strings.Contains(capturedPrompt, old) {
			t.Fatalf("Situation prompt retained %q:\n%s", old, capturedPrompt)
		}
	}
}
