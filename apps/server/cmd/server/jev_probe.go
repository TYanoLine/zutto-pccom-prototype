package main

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"zutto-pccom/apps/server/internal/worldengine"
)

type jevProbeResponse struct {
	Configured    bool                                         `json:"configured"`
	OK            bool                                         `json:"ok"`
	Model         string                                       `json:"model,omitempty"`
	InputTokens   int                                          `json:"input_tokens,omitempty"`
	Probabilities map[string]worldengine.BehaviorProbabilities `json:"probabilities,omitempty"`
	Error         string                                       `json:"error,omitempty"`
}

func newJevProbeHandler(engine worldengine.Engine, configured bool, worldDate string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			_ = json.NewEncoder(w).Encode(jevProbeResponse{Configured: configured, Error: "GET only"})
			return
		}
		if !configured {
			w.WriteHeader(http.StatusServiceUnavailable)
			_ = json.NewEncoder(w).Encode(jevProbeResponse{Configured: false, Error: "JEV_APIKEY is not configured"})
			return
		}

		personas := []worldengine.BehaviorPersona{
			{
				ID:                  "lurker",
				Handle:              "ROMMER",
				Age:                 29,
				Occupation:          "会社員",
				ActivityPattern:     "late_evening",
				ReplyTendency:       0.08,
				ThreadStartTendency: 0.04,
				LurkerTendency:      0.94,
				NewcomerOpenness:    0.25,
				Interests:           map[string]float64{"games": 0.82},
			},
			{
				ID:                  "balanced",
				Handle:              "KAZU",
				Age:                 24,
				Occupation:          "学生",
				ActivityPattern:     "evening",
				ReplyTendency:       0.42,
				ThreadStartTendency: 0.25,
				LurkerTendency:      0.48,
				NewcomerOpenness:    0.55,
				Interests:           map[string]float64{"games": 0.78},
			},
			{
				ID:                  "talkative",
				Handle:              "NORI",
				Age:                 31,
				Occupation:          "会社員",
				ActivityPattern:     "evening",
				ReplyTendency:       0.82,
				ThreadStartTendency: 0.62,
				LurkerTendency:      0.10,
				NewcomerOpenness:    0.72,
				Interests:           map[string]float64{"games": 0.84},
			},
		}
		board := worldengine.BehaviorBoard{ID: "game", Name: "ゲーム雑談"}
		affinities := []worldengine.BehaviorAffinity{
			{PersonaID: "lurker", BoardID: board.ID, Value: 0.82},
			{PersonaID: "balanced", BoardID: board.ID, Value: 0.78},
			{PersonaID: "talkative", BoardID: board.ID, Value: 0.84},
		}

		ctx, cancel := context.WithTimeout(r.Context(), 4*time.Second)
		defer cancel()
		decision, err := engine.AdviseBehavior(ctx, worldengine.BehaviorAdviceRequest{
			WorldDate:  worldDate,
			HostID:     "jev-probe-host",
			HostName:   "JEV PROBE NET",
			Personas:   personas,
			Boards:     []worldengine.BehaviorBoard{board},
			Affinities: affinities,
		})
		if err != nil {
			w.WriteHeader(http.StatusBadGateway)
			_ = json.NewEncoder(w).Encode(jevProbeResponse{Configured: true, Error: err.Error()})
			return
		}

		probabilities := make(map[string]worldengine.BehaviorProbabilities, len(personas))
		for _, persona := range personas {
			probabilities[persona.ID] = decision.Probabilities[worldengine.BehaviorPairKey(persona.ID, board.ID)]
		}
		_ = json.NewEncoder(w).Encode(jevProbeResponse{
			Configured:    true,
			OK:            true,
			Model:         decision.Model,
			InputTokens:   decision.InputTokens,
			Probabilities: probabilities,
		})
	}
}
