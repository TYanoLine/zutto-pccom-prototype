package main

import (
	"crypto/subtle"
	"encoding/json"
	"net/http"

	"zutto-pccom/apps/server/internal/worldrepo"
)

type generationTraceSource interface {
	GenerationTraceSnapshot() worldrepo.GenerationTraceSnapshot
}

// Trace inspection is strictly read-only and independent of world generation.
func newHakataTraceHandler(token string, enabled bool, source generationTraceSource) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", "GET")
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if token == "" || !enabled || subtle.ConstantTimeCompare([]byte(r.Header.Get("X-Zutto-Debug-Token")), []byte(token)) != 1 {
			w.WriteHeader(http.StatusForbidden)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "HAKATA trace requires the configured debug token"})
			return
		}
		_ = json.NewEncoder(w).Encode(source.GenerationTraceSnapshot())
	}
}
