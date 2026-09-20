package llm

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestGeneratePersonaProfilesKeepsPresentationBoundary(t *testing.T) {
	response := `{
		"model":"gpt-profile-test",
		"output":[{"content":[{"type":"output_text","text":"{\"profiles\":[{\"id\":\"P00001\",\"profile\":\"普段は短めに返すことが多く、興味のある話題でも毎回は書き込まない。質問には比較的まめに答える。引用は少なめで、夜の接続が中心。\"}]}"}]}],
		"usage":{"input_tokens":321,"output_tokens":88,"total_tokens":409}
	}`
	var prompt string
	provider := StructuredOpenAIProvider{OpenAIProvider: OpenAIProvider{
		APIKey: "test-key",
		Model:  "gpt-test",
		Client: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			var payload map[string]any
			if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
				t.Fatalf("decode request: %v", err)
			}
			prompt, _ = payload["input"].(string)
			return &http.Response{
				StatusCode: http.StatusOK,
				Status:     "200 OK",
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(response)),
			}, nil
		})},
	}}
	draft, err := provider.GeneratePersonaProfiles(context.Background(), "1996-08-26", []PersonaProfileSeed{{
		ID: "P00001", Handle: "NORI", Age: 27, Occupation: "会社員", ActivityClass: "active",
		TopInterests: []string{"communications", "games"}, StyleTags: []string{"短文", "引用少なめ"},
		ConnectWindow: "23:00-01:00", Quirk: "質問には答えるが雑談には毎回は入らない。",
	}})
	if err != nil {
		t.Fatal(err)
	}
	if len(draft.Profiles) != 1 || draft.Profiles[0].ID != "P00001" || draft.Profiles[0].Profile == "" {
		t.Fatalf("unexpected profiles: %+v", draft.Profiles)
	}
	if draft.Usage.Model != "gpt-profile-test" || draft.Usage.TotalTokens != 409 {
		t.Fatalf("unexpected usage: %+v", draft.Usage)
	}
	for _, want := range []string{"WORLD DATE: 1996-08-26", "Do NOT create new world facts", "Never use knowledge, terminology, products, services, culture, or retrospective viewpoints from after that date"} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("prompt missing %q", want)
		}
	}
}
