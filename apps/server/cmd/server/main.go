package main

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"zutto-pccom/apps/server/internal/config"
	"zutto-pccom/apps/server/internal/telephone"
	"zutto-pccom/apps/server/internal/world"
	"zutto-pccom/apps/server/internal/worldclock"
	wsserver "zutto-pccom/apps/server/internal/ws"
)

func main() {
	cfg := config.Load()
	store := world.NewMemoryStore()
	jst, err := time.LoadLocation("Asia/Tokyo")
	if err != nil {
		log.Fatalf("load Japan timezone: %v", err)
	}
	clock, err := worldclock.New(cfg.WorldDate, jst)
	if err != nil {
		log.Fatalf("create world clock: %v", err)
	}
	network := telephone.New(store, clock)

	mux := http.NewServeMux()
	mux.Handle("/ws", wsserver.Handler{Network: network, Store: store})
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "world_date": cfg.WorldDate, "time": clock.Now()})
	})

	srv := &http.Server{Addr: cfg.Addr, Handler: cors(mux), ReadHeaderTimeout: 5 * time.Second}
	log.Printf("zutto server listening on %s", cfg.Addr)
	log.Fatal(srv.ListenAndServe())
}

func cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
