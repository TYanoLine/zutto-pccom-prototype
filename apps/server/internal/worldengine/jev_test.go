package worldengine

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

type jevRoundTripFunc func(*http.Request) (*http.Response, error)

func (f jevRoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

func TestJevAdvisorBatchesPersonaWriteQuestions(t *testing.T) {
	var gotPayload map[string]any
	advisor := JevAdvisor{
		APIKey:   "test-key",
		Model:    "jev-latest",
		Endpoint: "https://example.invalid/v1/systemone",
		Client: &http.Client{Transport: jevRoundTripFunc(func(req *http.Request) (*http.Response, error) {
			if got := req.Header.Get("Authorization"); got != "Bearer test-key" {
				t.Fatalf("authorization=%q", got)
			}
			if err := json.NewDecoder(req.Body).Decode(&gotPayload); err != nil {
				t.Fatalf("decode payload: %v", err)
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Status:     "200 OK",
				Header:     make(http.Header),
				Body: io.NopCloser(strings.NewReader(`{
					"model":"jev-1.13.0",
					"answers":{
						"persona_0":{"type":"noul","noul":0.18},
						"persona_1":{"type":"noul","noul":0.71}
					},
					"usage":{"input_tokens":321,"output_tokens":0}
				}`)),
			}, nil
		})},
	}

	got, err := advisor.AdviseWritePropensities(context.Background(), WritePropensityRequest{
		WorldDate: "1996-08-26",
		HostID:    "host-1",
		HostName:  "TEST NET",
		BoardID:   "2",
		BoardName: "パソコン通信・モデム",
		Personas: []WritePropensityPersona{
			{ID: "p1", Handle: "ROMMER", LurkerTendency: .9, BoardAffinity: .8, VisitCount: 3},
			{ID: "p2", Handle: "TALKER", LurkerTendency: .1, ReplyTendency: .8, BoardAffinity: .9, VisitCount: 4},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Model != "jev-1.13.0" || got.InputTokens != 321 {
		t.Fatalf("unexpected metadata: %+v", got)
	}
	if got.Probabilities["p1"] != .18 || got.Probabilities["p2"] != .71 {
		t.Fatalf("unexpected probabilities: %+v", got.Probabilities)
	}
	questions, ok := gotPayload["questions"].(map[string]any)
	if !ok || len(questions) != 2 {
		t.Fatalf("questions=%#v", gotPayload["questions"])
	}
	state, ok := gotPayload["state"].(map[string]any)
	if !ok || state["world_date"] != "1996-08-26" {
		t.Fatalf("state=%#v", gotPayload["state"])
	}
}

func TestJevAdvisorRequiresAPIKey(t *testing.T) {
	_, err := (JevAdvisor{}).AdviseWritePropensities(context.Background(), WritePropensityRequest{
		Personas: []WritePropensityPersona{{ID: "p1"}},
	})
	if err == nil {
		t.Fatal("expected missing-key error")
	}
}


func TestJevAdvisorBatchesBehaviorQuestions(t *testing.T) {
	var gotPayload map[string]any
	advisor := JevAdvisor{
		APIKey:   "test-key",
		Model:    "jev-latest",
		Endpoint: "https://example.invalid/v1/systemone",
		Client: &http.Client{Transport: jevRoundTripFunc(func(req *http.Request) (*http.Response, error) {
			if err := json.NewDecoder(req.Body).Decode(&gotPayload); err != nil {
				t.Fatalf("decode payload: %v", err)
			}
			questions, ok := gotPayload["questions"].(map[string]any)
			if !ok {
				t.Fatalf("questions=%#v", gotPayload["questions"])
			}
			answers := make(map[string]any, len(questions))
			for key := range questions {
				value := .50
				switch {
				case strings.HasSuffix(key, "_visit"):
					value = .70
				case strings.HasSuffix(key, "_write"):
					value = .40
				case strings.HasSuffix(key, "_reply"):
					value = .60
				}
				answers[key] = map[string]any{"type": "noul", "noul": value}
			}
			body, err := json.Marshal(map[string]any{
				"model":   "jev-1.13.0",
				"answers": answers,
				"usage":   map[string]any{"input_tokens": 987, "output_tokens": 0},
			})
			if err != nil {
				t.Fatalf("marshal response: %v", err)
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Status:     "200 OK",
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(string(body))),
			}, nil
		})},
	}

	got, err := advisor.AdviseBehavior(context.Background(), BehaviorAdviceRequest{
		WorldDate: "1996-08-26",
		HostID:    "host-1",
		HostName:  "TEST NET",
		Personas: []BehaviorPersona{
			{ID: "p1", Handle: "ROMMER", LurkerTendency: .9, Interests: map[string]float64{"games": .8}},
			{ID: "p2", Handle: "TALKER", LurkerTendency: .1, ReplyTendency: .8, Interests: map[string]float64{"games": .9}},
		},
		Boards: []BehaviorBoard{
			{ID: "b1", Name: "フリートーク"},
			{ID: "b2", Name: "ゲーム"},
		},
		Affinities: []BehaviorAffinity{
			{PersonaID: "p1", BoardID: "b1", Value: .2},
			{PersonaID: "p1", BoardID: "b2", Value: .8},
			{PersonaID: "p2", BoardID: "b1", Value: .6},
			{PersonaID: "p2", BoardID: "b2", Value: .9},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	questions, ok := gotPayload["questions"].(map[string]any)
	if !ok || len(questions) != 12 {
		t.Fatalf("question count=%d, want 12", len(questions))
	}
	if got.Model != "jev-1.13.0" || got.InputTokens != 987 {
		t.Fatalf("unexpected metadata: %+v", got)
	}
	pair, ok := got.Probabilities[BehaviorPairKey("p1", "b2")]
	if !ok {
		t.Fatalf("missing p1/b2 pair: %+v", got.Probabilities)
	}
	if pair.Visit != .70 || pair.Write != .40 || pair.Reply != .60 {
		t.Fatalf("unexpected pair probabilities: %+v", pair)
	}
}

func TestJevBehaviorAdvisorRequiresAPIKey(t *testing.T) {
	_, err := (JevAdvisor{}).AdviseBehavior(context.Background(), BehaviorAdviceRequest{
		Personas: []BehaviorPersona{{ID: "p1"}},
		Boards:   []BehaviorBoard{{ID: "b1"}},
	})
	if err == nil {
		t.Fatal("expected missing-key error")
	}
}
