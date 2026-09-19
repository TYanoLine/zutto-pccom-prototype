package main

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"zutto-pccom/apps/server/internal/worldengine"
)

type jevProbeResponse struct {
	Configured    bool               `json:"configured"`
	OK            bool               `json:"ok"`
	Model         string             `json:"model,omitempty"`
	InputTokens   int                `json:"input_tokens,omitempty"`
	Probabilities map[string]float64 `json:"probabilities,omitempty"`
	Error         string             `json:"error,omitempty"`
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

		ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
		defer cancel()
		decision, err := engine.AdviseWritePropensities(ctx, worldengine.WritePropensityRequest{
			WorldDate: worldDate,
			HostID:    "jev-probe-host",
			HostName:  "JEV PROBE NET",
			BoardID:   "game",
			BoardName: "ゲーム雑談",
			Personas: []worldengine.WritePropensityPersona{
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
					BoardAffinity:       0.82,
					Interests:           map[string]float64{"games": 0.82},
					VisitCount:          4,
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
					BoardAffinity:       0.78,
					Interests:           map[string]float64{"games": 0.78},
					VisitCount:          4,
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
					BoardAffinity:       0.84,
					Interests:           map[string]float64{"games": 0.84},
					VisitCount:          4,
				},
			},
		})
		if err != nil {
			w.WriteHeader(http.StatusBadGateway)
			_ = json.NewEncoder(w).Encode(jevProbeResponse{Configured: true, Error: err.Error()})
			return
		}
		_ = json.NewEncoder(w).Encode(jevProbeResponse{
			Configured:    true,
			OK:            true,
			Model:         decision.Model,
			InputTokens:   decision.InputTokens,
			Probabilities: decision.Probabilities,
		})
	}
}
