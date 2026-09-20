package main

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"zutto-pccom/apps/server/internal/buildinfo"
	"zutto-pccom/apps/server/internal/personapoc"
)

func newPersonaLabHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": "GET only"})
			return
		}

		count := 100
		if raw := r.URL.Query().Get("count"); raw != "" {
			if n, err := strconv.Atoi(raw); err == nil {
				count = n
			}
		}
		if count < 1 {
			count = 1
		}
		if count > 2000 {
			count = 2000
		}

		seed := int64(19960826)
		if raw := r.URL.Query().Get("seed"); raw != "" {
			if n, err := strconv.ParseInt(raw, 10, 64); err == nil {
				seed = n
			}
		}
		profile := r.URL.Query().Get("profile")
		if profile == "" {
			profile = "general"
		}

		run := personapoc.Generate(seed, count, profile, true)
		benchmarks := personapoc.Benchmark(seed+1000003, profile, []int{50, 100, 500, 1000})
		build := buildinfo.Current()

		_ = json.NewEncoder(w).Encode(map[string]any{
			"generated_at": time.Now().UTC(),
			"build_commit": build.Commit,
			"build_branch": build.Branch,
			"profiles": personapoc.Profiles(),
			"run": run,
			"benchmarks": benchmarks,
			"semantics": map[string]any{
				"api_calls": 0,
				"llm_calls": 0,
				"persona_bank": false,
				"host_profile_affects": "membership selection only",
				"canonical": false,
				"note": "development PoC only; generated people are not inserted into the persistent world",
			},
		})
	}
}
