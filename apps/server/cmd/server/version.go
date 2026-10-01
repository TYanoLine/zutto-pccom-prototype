package main

import (
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"time"
)

type serverBuildInfo struct {
	Commit    string    `json:"commit"`
	Branch    string    `json:"branch"`
	StartedAt time.Time `json:"started_at"`
}

// serverBuildInfoFromEnvironment reports the running binary's deployment,
// not GitHub's latest main. Render injects these variables at runtime.
func serverBuildInfoFromEnvironment(startedAt time.Time) serverBuildInfo {
	commit := strings.TrimSpace(os.Getenv("RENDER_GIT_COMMIT"))
	if commit == "" {
		commit = strings.TrimSpace(os.Getenv("GIT_COMMIT"))
	}
	branch := strings.TrimSpace(os.Getenv("RENDER_GIT_BRANCH"))
	if branch == "" {
		branch = strings.TrimSpace(os.Getenv("GIT_BRANCH"))
	}
	if commit == "" {
		commit = "unknown"
	}
	if branch == "" {
		branch = "unknown"
	}
	return serverBuildInfo{Commit: commit, Branch: branch, StartedAt: startedAt.UTC()}
}

func newServerVersionHandler(info serverBuildInfo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", "GET")
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		_ = json.NewEncoder(w).Encode(info)
	}
}
