package main

import (
	"crypto/subtle"
	"encoding/json"
	"net/http"
	"strings"

	"zutto-pccom/apps/server/internal/world"
)

type bbsResetStore interface {
	HostByPhone(phone string) (world.Host, error)
	MaterializationObservationRunning(hostID string) bool
	PrepareDebugBBSConnection(host world.Host) (removed int, kept int, ok bool)
	ResetBBSGeneratedArticles(host world.Host) (removed int, kept int, ok bool)
}

func newBBSResetHandler(token string, store world.Store, runtimeStore bbsResetStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		if token != "" && subtle.ConstantTimeCompare([]byte(r.Header.Get("X-Zutto-Debug-Token")), []byte(token)) != 1 {
			w.WriteHeader(http.StatusForbidden)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "debug BBS reset requires valid DEBUG_RESET_TOKEN"})
			return
		}
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", "POST")
			w.WriteHeader(http.StatusMethodNotAllowed)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": "POST only"})
			return
		}
		phone := strings.TrimSpace(r.URL.Query().Get("phone"))
		if phone == "" {
			phone = defaultDebugEndpointPhone(store)
		}
		// Public test deployment safety: expose the generic reset machinery only
		// for hosts whose definition opts in with debug.http_endpoints. An unknown
		// number and a host without the flag get the same answer on purpose.
		host, err := runtimeStore.HostByPhone(phone)
		if err != nil || !host.Debug.HTTPEndpoints {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": "debug reset is not enabled for this host"})
			return
		}
		if runtimeStore.MaterializationObservationRunning(host.ID) {
			w.WriteHeader(http.StatusConflict)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": "BBS observation/generation is still running; retry after it completes"})
			return
		}
		mode := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("mode")))
		all := r.URL.Query().Get("all") == "true" || mode == "all"
		var removed, kept int
		var ok bool
		if all {
			removed, kept, ok = runtimeStore.PrepareDebugBBSConnection(host)
		} else {
			removed, kept, ok = runtimeStore.ResetBBSGeneratedArticles(host)
		}
		if !ok {
			w.WriteHeader(http.StatusConflict)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": "shared BBS article reset unavailable"})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"ok":                      true,
			"phone":                   phone,
			"host_id":                 host.ID,
			"removed_generated_posts": removed,
			"removed_posts":           removed,
			"kept_posts":              kept,
			"mode": func() string {
				if all {
					return "all"
				}
				return "generated"
			}(),
			"kept": func() string {
				if all {
					return "boards + personas + host program configuration"
				}
				return "seed/user history + boards + personas + host program configuration"
			}(),
			"next": "visit a board again to run a fresh shared-engine catch-up batch",
		})
	}
}
