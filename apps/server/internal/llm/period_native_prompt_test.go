package llm

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestDiegeticWorldFrameIncludesPeriodNativeConversationConstraints(t *testing.T) {
	for _, want := range []string{
		"Use shared-context economy",
		"Do not expand an ordinary exchange into an explanatory mini-essay",
		"prefer the concrete period-native term",
		"do not repair missing canonical specificity",
		"Board placement is part of the in-world meaning",
		"Avoid assistant/FAQ voice",
		"Do not paraphrase an agreement, anecdote, or explanation",
		"If the actor has already replied, write as someone returning to the thread",
		"CONTRASTIVE INTERPRETATION EXAMPLES",
		"they are NOT reusable content templates",
	} {
		if !strings.Contains(DiegeticWorldFrame, want) {
			t.Fatalf("DiegeticWorldFrame missing %q", want)
		}
	}
}

func TestGenerateBoardPostCarriesPeriodNativeConversationFrame(t *testing.T) {
	var capturedPrompt string
	provider := OpenAIProvider{
		Endpoint: "https://test.openai.azure.com",
		APIKey: "test-key",
		Model:  "gpt-test",
		Client: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			var payload map[string]any
			if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
				t.Fatalf("decode request: %v", err)
			}
			capturedPrompt, _ = payload["input"].(string)
			response := `{"model":"gpt-test","output":[{"content":[{"type":"output_text","text":"{\"author\":\"NEKO\",\"subject\":\"Re: ベンチ\",\"body\":\"了解。\"}"}]}],"usage":{"input_tokens":20,"input_tokens_details":{"cached_tokens":0},"output_tokens":10,"output_tokens_details":{"reasoning_tokens":0},"total_tokens":30}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Status:     "200 OK",
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(response)),
			}, nil
		})},
	}

	_, err := provider.GenerateBoardPost(context.Background(), BoardPostRequest{
		HostName:         "TEST BBS",
		HostRegion:       "神奈川県",
		HostSoftware:     "development",
		BoardID:          "3",
		BoardTopic:       "Re: ベンチ",
		WorldDate:        "1996-08-29",
		EraRules:         "世界時刻より未来の知識を使わない。",
		AuthorHandle:     "NEKO",
		PersonaProfile:   "砕けた口調",
		PostIntent:       "goal=既存スレッドの新しい点に短く反応する; bbs_context=THREAD SO FAR: ...",
		CanonicalSubject: "Re: ベンチ",
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"1996年前後の日本の草の根パソコン通信世界",
		"世界日付: 1996-08-29",
		"確定済み件名: Re: ベンチ",
		"handle=NEKO",
		"砕けた口調",
		"canonical Situation / thread facts",
		"時代背景: 世界時刻より未来の知識を使わない。",
	} {
		if !strings.Contains(capturedPrompt, want) {
			t.Fatalf("body prompt missing period-native context or material %q:\n%s", want, capturedPrompt)
		}
	}
}
