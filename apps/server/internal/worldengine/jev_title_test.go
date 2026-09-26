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
			State map[string]any `json:"state"`
			Questions map[string]any `json:"questions"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		if payload.Model != "jev-test" {
			t.Fatalf("unexpected model %q", payload.Model)
		}
		facts, ok := payload.State["historical_facts"].([]any)
		if !ok || len(facts) != 1 || facts[0] != "セガサターンは1994-11-22までに日本で発売済み。" {
			t.Fatalf("historical facts not forwarded to Jev state: %#v", payload.State["historical_facts"])
		}
		policy, ok := payload.State["policy"].(map[string]any)
		if !ok || !strings.Contains(policy["era"].(string), "explicitly supported by state.historical_facts") {
			t.Fatalf("era policy does not permit supplied evidence reuse: %#v", payload.State["policy"])
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
		HistoricalFacts: []string{"セガサターンは1994-11-22までに日本で発売済み。"},
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


func TestJevAdvisorTitleCandidatesFitOnlyOmitsEraQuestions(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload struct {
			Questions map[string]any `json:"questions"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		answers := map[string]any{}
		for key := range payload.Questions {
			if strings.Contains(key, "era_") {
				t.Fatalf("fit-only request unexpectedly contained era question %q", key)
			}
			if !strings.Contains(key, "_fit") {
				t.Fatalf("fit-only request contained non-fit question %q", key)
			}
			answers[key] = map[string]any{"type": "noul", "noul": 0.77}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"model": "jev-fit-only",
			"answers": answers,
		})
	}))
	defer server.Close()

	advisor := JevAdvisor{APIKey: "test-key", Endpoint: server.URL, Client: server.Client()}
	got, err := advisor.AdviseTitleCandidates(context.Background(), TitleCandidateAdviceRequest{
		WorldDate: "1996-08-26",
		HostID: "h",
		HostName: "host",
		BoardID: "b",
		BoardName: "ゲーム",
		Titles: []string{"候補A", "候補B"},
		Events: []TitleEvaluationEvent{{EventID: "e1"}, {EventID: "e2"}},
		FitOnly: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Era) != 0 {
		t.Fatalf("fit-only era results=%d, want 0", len(got.Era))
	}
	if len(got.Fit) != 4 {
		t.Fatalf("fit-only pairs=%d, want 4", len(got.Fit))
	}
}
