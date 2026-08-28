package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"zutto-pccom/apps/server/internal/config"
	"zutto-pccom/apps/server/internal/llm"
	"zutto-pccom/apps/server/internal/telephone"
	"zutto-pccom/apps/server/internal/world"
	"zutto-pccom/apps/server/internal/worldcatalog"
	"zutto-pccom/apps/server/internal/worldclock"
	wsserver "zutto-pccom/apps/server/internal/ws"
)

const generatedCenterCount = 100

func main() {
	cfg := config.Load()
	store := world.NewMemoryStore()
	jst, err := time.LoadLocation("Asia/Tokyo")
	if err != nil { log.Fatalf("load Japan timezone: %v", err) }
	clock, err := worldclock.New(cfg.WorldDate, jst)
	if err != nil { log.Fatalf("create world clock: %v", err) }
	network := telephone.New(store, clock)
	sessions := wsserver.NewSessionManager(wsserver.DefaultReconnectGrace)
	catalogGenerator := llm.CenterCatalogGenerator{APIKey: cfg.OpenAIKey, Model: cfg.OpenAIModel}

	var catalogStore *worldcatalog.Store
	if cfg.DatabaseURL == "" {
		log.Printf("DATABASE_URL is not set; persistent generated worlds are disabled")
	} else {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		catalogStore, err = worldcatalog.Open(ctx, cfg.DatabaseURL)
		if err == nil { err = catalogStore.EnsureSchema(ctx) }
		cancel()
		if err != nil { log.Fatalf("initialize persistent world catalog: %v", err) }
		defer catalogStore.Close()
	}

	bootstrapWorld := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if catalogStore == nil {
			http.Error(w, `{"error":"persistent world database is not configured"}`, http.StatusServiceUnavailable)
			return
		}
		worldKey := r.URL.Query().Get("key")
		if !worldcatalog.ValidWorldKey(worldKey) {
			http.Error(w, `{"error":"invalid world key"}`, http.StatusBadRequest)
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 75*time.Second)
		defer cancel()
		catalog, err := catalogStore.GetOrCreate(ctx, worldKey, generatedCenterCount, func(ctx context.Context, count int) ([]string, error) {
			generated, err := catalogGenerator.Generate(ctx, count, cfg.WorldDate)
			if err != nil { return nil, err }
			names := make([]string, len(generated))
			for i := range generated { names[i] = generated[i].Name }
			return names, nil
		})
		if err != nil {
			log.Printf("world bootstrap failed: %v", err)
			w.WriteHeader(http.StatusBadGateway)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": err.Error()})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"worldId": catalog.WorldID,
			"centers": catalog.Centers,
			"created": catalog.Created,
			"source": func() string { if catalog.Created { return "openai" }; return "postgres" }(),
			"model": cfg.OpenAIModel,
		})
	}

	mux := http.NewServeMux()
	mux.Handle("/ws", wsserver.Handler{Network: network, Store: store, Sessions: sessions})
	mux.HandleFunc("/api/world/bootstrap", bootstrapWorld)
	// Temporary compatibility route while the communications-software UI moves
	// from an ephemeral catalog endpoint to world bootstrap.
	mux.HandleFunc("/api/centers", bootstrapWorld)
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "world_date": cfg.WorldDate, "time": clock.Now(), "persistent_worlds": catalogStore != nil})
	})

	srv := &http.Server{Addr: cfg.Addr, Handler: cors(mux), ReadHeaderTimeout: 5 * time.Second}
	log.Printf("zutto server listening on %s", cfg.Addr)
	log.Fatal(srv.ListenAndServe())
}

func cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions { w.WriteHeader(http.StatusNoContent); return }
		next.ServeHTTP(w, r)
	})
}
