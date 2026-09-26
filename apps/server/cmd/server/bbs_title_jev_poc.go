package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"zutto-pccom/apps/server/internal/hostprogram/erikak"
	"zutto-pccom/apps/server/internal/llm"
	"zutto-pccom/apps/server/internal/world"
	"zutto-pccom/apps/server/internal/worldengine"
	"zutto-pccom/apps/server/internal/worldrepo"
)

const (
	titleJevPoCCandidateCount = 100
	titleJevPoCSafeThreshold = 0.80
	titleJevPoCImpossibleThreshold = 0.80
)

func newBBSTitleJevPoCHandler(
	store *worldrepo.Repository,
	renderer llm.StructuredOpenAIProvider,
	jevKey string,
	jevModel string,
	worldDate string,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": "GET only"})
			return
		}
		if store == nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": "world repository is not configured"})
			return
		}
		if strings.TrimSpace(jevKey) == "" {
			w.WriteHeader(http.StatusServiceUnavailable)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": "JEV_APIKEY is not configured"})
			return
		}

		host, err := store.HostByPhone(erikaKExperimentPhone)
		if err != nil {
			w.WriteHeader(http.StatusNotFound)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": "HAKATA experiment host not found"})
			return
		}
		boardID := strings.TrimSpace(r.URL.Query().Get("board"))
		if boardID == "" {
			boardID = "6"
		}
		var board world.Board
		if host.SoftwareID == "erika-k" {
			if resolved, ok := erikak.BoardByPath(boardID); ok {
				board = resolved
			}
		}
		if board.ID == "" {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": "unknown Erika-K board path"})
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 90*time.Second)
		defer cancel()

		generationStarted := time.Now()
		pool, err := renderer.GenerateContextualBBSTitleCandidates(ctx, llm.BBSContextualTitleCandidateRequest{
			WorldDate: worldDate,
			BoardName: board.Name,
			BoardScope: board.SemanticScope,
			RemainingNeeded: 48,
			CandidateCount: titleJevPoCCandidateCount,
		})
		generationDuration := time.Since(generationStarted)
		if err != nil {
			w.WriteHeader(http.StatusBadGateway)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"world_date": worldDate,
				"host_id": host.ID,
				"board_id": board.ID,
				"candidate_count": titleJevPoCCandidateCount,
				"generation_duration_ms": generationDuration.Milliseconds(),
				"error": err.Error(),
			})
			return
		}

		advisor := worldengine.JevAdvisor{
			APIKey: jevKey,
			Model: jevModel,
			Client: &http.Client{Timeout: 15 * time.Second},
		}
		jevStarted := time.Now()
		decision, err := advisor.AdviseTitleCandidates(ctx, worldengine.TitleCandidateAdviceRequest{
			WorldDate: worldDate,
			HostID: host.ID,
			HostName: host.Name,
			BoardID: board.ID,
			BoardName: board.Name,
			BoardScope: board.SemanticScope,
			Titles: append([]string(nil), pool.Titles...),
		})
		jevDuration := time.Since(jevStarted)
		if err != nil {
			w.WriteHeader(http.StatusBadGateway)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"world_date": worldDate,
				"host_id": host.ID,
				"board_id": board.ID,
				"candidate_count": len(pool.Titles),
				"openai_model": pool.Usage.Model,
				"generation_duration_ms": generationDuration.Milliseconds(),
				"generation_input_tokens": pool.Usage.InputTokens,
				"generation_output_tokens": pool.Usage.OutputTokens,
				"jev_duration_ms": jevDuration.Milliseconds(),
				"error": err.Error(),
			})
			return
		}

		claimsByCandidate := make(map[int][]llm.BBSTitleHistoricalClaim)
		for _, claim := range pool.HistoricalClaims {
			claimsByCandidate[claim.Candidate] = append(claimsByCandidate[claim.Candidate], claim)
		}
		summary := map[string]int{
			"total": len(pool.Titles),
			"claim_free": 0,
			"with_claims": 0,
			"jev_safe": 0,
			"jev_research": 0,
			"jev_impossible": 0,
			"claim_free_research": 0,
			"claimed_safe": 0,
		}
		candidates := make([]map[string]any, 0, len(pool.Titles))
		for i, title := range pool.Titles {
			candidate := i + 1
			prob := decision.Era[candidate]
			claims := claimsByCandidate[candidate]
			route := "research"
			switch {
			case prob.LogicallyImpossible >= titleJevPoCImpossibleThreshold:
				route = "impossible"
				summary["jev_impossible"]++
			case prob.SafeWithoutResearch >= titleJevPoCSafeThreshold:
				route = "safe"
				summary["jev_safe"]++
			default:
				summary["jev_research"]++
			}
			if len(claims) == 0 {
				summary["claim_free"]++
				if route == "research" {
					summary["claim_free_research"]++
				}
			} else {
				summary["with_claims"]++
				if route == "safe" {
					summary["claimed_safe"]++
				}
			}
			candidates = append(candidates, map[string]any{
				"candidate": candidate,
				"title": title,
				"historical_claims": claims,
				"jev_safe_without_research": prob.SafeWithoutResearch,
				"jev_logically_impossible": prob.LogicallyImpossible,
				"jev_route": route,
			})
			log.Printf("BBS title Jev PoC candidate: board=%s candidate=%d route=%s safe=%.3f impossible=%.3f claims=%d title=%q", board.ID, candidate, route, prob.SafeWithoutResearch, prob.LogicallyImpossible, len(claims), title)
		}
		log.Printf("BBS title Jev PoC summary: board=%s generation_ms=%d jev_ms=%d summary=%v", board.ID, generationDuration.Milliseconds(), jevDuration.Milliseconds(), summary)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"ok": true,
			"world_date": worldDate,
			"host_id": host.ID,
			"host_name": host.Name,
			"board_id": board.ID,
			"board_name": board.Name,
			"board_scope": board.SemanticScope,
			"candidate_count": len(pool.Titles),
			"openai_model": pool.Usage.Model,
			"jev_model": decision.Model,
			"generation_duration_ms": generationDuration.Milliseconds(),
			"jev_duration_ms": jevDuration.Milliseconds(),
			"generation_input_tokens": pool.Usage.InputTokens,
			"generation_output_tokens": pool.Usage.OutputTokens,
			"jev_input_tokens": decision.InputTokens,
			"safe_threshold": titleJevPoCSafeThreshold,
			"impossible_threshold": titleJevPoCImpossibleThreshold,
			"summary": summary,
			"candidates": candidates,
		})
	}
}
