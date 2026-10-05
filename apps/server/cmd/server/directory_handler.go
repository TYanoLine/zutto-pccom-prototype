package main

import (
	"encoding/json"
	"net/http"

	"zutto-pccom/apps/server/internal/world"
)

// directoryLister is the part of the store the directory handler needs.
type directoryLister interface {
	ListedHosts() []world.DirectoryEntry
}

// newDirectoryHandler serves the dialing directory: the hosts whose preset says
// listed: true. The directory is part of the immutable host definitions, so it
// needs no authentication and nothing about the caller.
func newDirectoryHandler(dir directoryLister) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		centers := dir.ListedHosts()
		if centers == nil {
			centers = []world.DirectoryEntry{}
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-cache")
		if r.Method == http.MethodHead {
			return
		}
		_ = json.NewEncoder(w).Encode(map[string][]world.DirectoryEntry{"centers": centers})
	}
}
