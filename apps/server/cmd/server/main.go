package main

import (
	"context"
	"crypto/subtle"
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

	generateNames := func(ctx context.Context, count int) ([]string, error) {
		generated, err := catalogGenerator.Generate(ctx, count, cfg.WorldDate)
		if err != nil { return nil, err }
		names := make([]string, len(generated))
		for i := range generated { names[i] = generated[i].Name }
		return names, nil
	}

	bootstrapWorld := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if catalogStore == nil { http.Error(w, `{"error":"persistent world database is not configured"}`, http.StatusServiceUnavailable); return }
		worldKey := r.URL.Query().Get("key")
		if !worldcatalog.ValidWorldKey(worldKey) { http.Error(w, `{"error":"invalid world key"}`, http.StatusBadRequest); return }
		ctx, cancel := context.WithTimeout(r.Context(), 75*time.Second); defer cancel()
		catalog, err := catalogStore.GetOrCreate(ctx, worldKey, generatedCenterCount, generateNames)
		if err != nil { log.Printf("world bootstrap failed: %v", err); w.WriteHeader(http.StatusBadGateway); _ = json.NewEncoder(w).Encode(map[string]any{"error": err.Error()}); return }
		_ = json.NewEncoder(w).Encode(map[string]any{"worldId": catalog.WorldID, "centers": catalog.Centers, "created": catalog.Created, "source": func() string { if catalog.Created { return "openai" }; return "postgres" }(), "model": cfg.OpenAIModel})
	}

	debugAuthorized := func(r *http.Request) bool {
		if cfg.DebugResetToken == "" { return false }
		got := r.Header.Get("X-Zutto-Debug-Token")
		return len(got) == len(cfg.DebugResetToken) && subtle.ConstantTimeCompare([]byte(got), []byte(cfg.DebugResetToken)) == 1
	}
	debugGuard := func(w http.ResponseWriter, r *http.Request) bool {
		w.Header().Set("Content-Type", "application/json")
		if r.Method != http.MethodPost { w.WriteHeader(http.StatusMethodNotAllowed); return false }
		if catalogStore == nil { http.Error(w, `{"error":"persistent world database is not configured"}`, http.StatusServiceUnavailable); return false }
		if !debugAuthorized(r) { http.Error(w, `{"error":"debug reset is disabled or unauthorized"}`, http.StatusForbidden); return false }
		return true
	}

	resetWorld := func(w http.ResponseWriter, r *http.Request) {
		if !debugGuard(w, r) { return }
		worldKey := r.URL.Query().Get("key")
		if !worldcatalog.ValidWorldKey(worldKey) { http.Error(w, `{"error":"invalid world key"}`, http.StatusBadRequest); return }
		ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second); defer cancel()
		if err := catalogStore.ResetWorld(ctx, worldKey); err != nil { w.WriteHeader(http.StatusInternalServerError); _ = json.NewEncoder(w).Encode(map[string]any{"error":err.Error()}); return }
		_ = json.NewEncoder(w).Encode(map[string]any{"ok":true,"reset":"world","next":"call /api/world/bootstrap with the same key to generate a new canonical world"})
	}

	resetHost := func(w http.ResponseWriter, r *http.Request) {
		if !debugGuard(w, r) { return }
		worldKey, hostID := r.URL.Query().Get("key"), r.URL.Query().Get("host")
		if !worldcatalog.ValidWorldKey(worldKey) || hostID == "" { http.Error(w, `{"error":"invalid world key or host"}`, http.StatusBadRequest); return }
		ctx, cancel := context.WithTimeout(r.Context(), 75*time.Second); defer cancel()
		center, err := catalogStore.ResetHost(ctx, worldKey, hostID, func(ctx context.Context) (string,error) { names,err:=generateNames(ctx,1); if err!=nil{return "",err}; return names[0],nil })
		if err != nil { w.WriteHeader(http.StatusBadGateway); _ = json.NewEncoder(w).Encode(map[string]any{"error":err.Error()}); return }
		_ = json.NewEncoder(w).Encode(map[string]any{"ok":true,"reset":"host","center":center})
	}

	mux := http.NewServeMux()
	mux.Handle("/ws", wsserver.Handler{Network: network, Store: store, Sessions: sessions})
	mux.HandleFunc("/api/world/bootstrap", bootstrapWorld)
	mux.HandleFunc("/api/centers", bootstrapWorld)
	mux.HandleFunc("/api/debug/world/reset", resetWorld)
	mux.HandleFunc("/api/debug/host/reset", resetHost)
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) { w.Header().Set("Content-Type", "application/json"); _ = json.NewEncoder(w).Encode(map[string]any{"ok":true,"world_date":cfg.WorldDate,"time":clock.Now(),"persistent_worlds":catalogStore!=nil,"debug_reset":cfg.DebugResetToken!=""}) })

	srv := &http.Server{Addr: cfg.Addr, Handler: cors(mux), ReadHeaderTimeout: 5*time.Second}
	log.Printf("zutto server listening on %s", cfg.Addr)
	log.Fatal(srv.ListenAndServe())
}

func cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-Zutto-Debug-Token")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		if r.Method == http.MethodOptions { w.WriteHeader(http.StatusNoContent); return }
		next.ServeHTTP(w,r)
	})
}
