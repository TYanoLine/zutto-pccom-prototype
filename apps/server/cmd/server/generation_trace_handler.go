package main

import (
	"encoding/json"
	"net/http"

	"zutto-pccom/apps/server/internal/worldrepo"
)

type generationTraceSource interface {
	GenerationTraceSnapshot() worldrepo.GenerationTraceSnapshot
}

// This deliberately unauthenticated, read-only endpoint is for an evaluation
// deployment where the host opts in with debug.generation_trace. Prompts may
// contain thread context; disable capture before any real user access.
func newGenerationTraceHandler(enabled bool, source generationTraceSource) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Robots-Tag", "noindex, nofollow, noarchive")
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", "GET")
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if !enabled {
			w.WriteHeader(http.StatusForbidden)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "generation trace is disabled"})
			return
		}
		_ = json.NewEncoder(w).Encode(source.GenerationTraceSnapshot())
	}
}
