package worldengine

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestJevAdvisorAuditsPersonaProfiles(t *testing.T) {
	var payload map[string]any
	advisor := JevAdvisor{
		APIKey: "test-key", Model: "jev-latest", Endpoint: "https://example.invalid/v1/systemone",
		Client: &http.Client{Transport: jevRoundTripFunc(func(req *http.Request) (*http.Response, error) {
			if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
				t.Fatalf("decode payload: %v", err)
			}
			return &http.Response{
				StatusCode: http.StatusOK, Status: "200 OK", Header: make(http.Header),
				Body: io.NopCloser(strings.NewReader(`{
					"model":"jev-test",
					"answers":{
						"p_P00001_future":{"type":"noul","noul":0.03},
						"p_P00001_review":{"type":"noul","noul":0.08}
					},
					"usage":{"input_tokens":144}
				}`)),
			}, nil
		})},
	}
	got, err := advisor.AdvisePersonaProfiles(context.Background(), PersonaProfileAdviceRequest{
		WorldDate: "1996-08-26",
		Profiles: []PersonaProfileAdviceItem{{ID:"P00001", Profile:"通信の話では具体的に返すことが多い。"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	p := got.Profiles["P00001"]
	if p.FutureInformation != 0.03 || p.ExternalReview != 0.08 {
		t.Fatalf("unexpected probabilities: %+v", p)
	}
	if got.Model != "jev-test" || got.InputTokens != 144 {
		t.Fatalf("unexpected metadata: %+v", got)
	}
	state, _ := payload["state"].(map[string]any)
	if state["world_date"] != "1996-08-26" {
		t.Fatalf("world date=%v", state["world_date"])
	}
	questions, _ := payload["questions"].(map[string]any)
	if len(questions) != 2 {
		t.Fatalf("questions=%v", questions)
	}
}
