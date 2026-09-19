package worldengine

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestJevAdvisorTitleCandidatesParsesEraAndFitProbabilities(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload struct {
			Model string `json:"model"`
			Questions map[string]any `json:"questions"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		if payload.Model != "jev-test" {
			t.Fatalf("unexpected model %q", payload.Model)
		}
		answers := map[string]any{}
		for key := range payload.Questions {
			value := 0.1
			switch {
			case strings.Contains(key, "era_safe"):
				value = 0.9
			case strings.Contains(key, "era_impossible"):
				value = 0.02
			case strings.Contains(key, "_fit"):
				value = 0.82
			}
			answers[key] = map[string]any{"type": "noul", "noul": value}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"model": "jev-test-resolved",
			"answers": answers,
			"usage": map[string]any{"input_tokens": 321},
		})
	}))
	defer server.Close()

	advisor := JevAdvisor{
		APIKey: "test-key",
		Model: "jev-test",
		Endpoint: server.URL,
		Client: server.Client(),
	}
	got, err := advisor.AdviseTitleCandidates(context.Background(), TitleCandidateAdviceRequest{
		WorldDate: "1996-08-26",
		HostID: "h",
		HostName: "host",
		BoardID: "b",
		BoardName: "ゲーム",
		Titles: []string{"セガサターンについて"},
		Events: []TitleEvaluationEvent{{EventID: "event:1", AuthorHandle: "NORI", DiscourseMode: "share"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Model != "jev-test-resolved" || got.InputTokens != 321 {
		t.Fatalf("unexpected telemetry: %+v", got)
	}
	era := got.Era[1]
	if era.SafeWithoutResearch != 0.9 || era.LogicallyImpossible != 0.02 {
		t.Fatalf("unexpected era probabilities: %+v", era)
	}
	if got.Fit[TitleCandidatePairKey(1, "event:1")] != 0.82 {
		t.Fatalf("unexpected fit: %+v", got.Fit)
	}
}
