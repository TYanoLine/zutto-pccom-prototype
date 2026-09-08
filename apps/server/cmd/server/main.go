package main

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"zutto-pccom/apps/server/internal/config"
	"zutto-pccom/apps/server/internal/historicalkb"
	"zutto-pccom/apps/server/internal/llm"
	"zutto-pccom/apps/server/internal/telephone"
	"zutto-pccom/apps/server/internal/worldcatalog"
	"zutto-pccom/apps/server/internal/worldclock"
	"zutto-pccom/apps/server/internal/worldengine"
	"zutto-pccom/apps/server/internal/worldrepo"
	wsserver "zutto-pccom/apps/server/internal/ws"
)

const generatedCenterCount = 100

func main() {
	cfg := config.Load()
	store := newRuntimeStore(cfg.DatabaseURL)
	jst, err := time.LoadLocation("Asia/Tokyo")
	if err != nil { log.Fatalf("load Japan timezone: %v", err) }
	clock, err := worldclock.New(cfg.WorldDate, jst)
	if err != nil { log.Fatalf("create world clock: %v", err) }
	sessions := wsserver.NewSessionManager(wsserver.DefaultReconnectGrace)
	catalogGenerator := llm.CenterCatalogGenerator{APIKey: cfg.OpenAIKey, Model: cfg.OpenAIModel}

	var catalogStore *worldcatalog.Store
	var historyStore *historicalkb.Store
	var freshArchive *postgresMaterializationFreshArchive
	if cfg.DatabaseURL == "" {
		log.Printf("DATABASE_URL is not set; persistent generated worlds and historical research are disabled")
	} else {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		catalogStore, err = worldcatalog.Open(ctx, cfg.DatabaseURL)
		if err == nil { err = catalogStore.EnsureSchema(ctx) }
		if err == nil { historyStore, err = historicalkb.Open(ctx, cfg.DatabaseURL) }
		if err == nil { err = historyStore.EnsureSchema(ctx) }
		if err == nil { freshArchive, err = openPostgresMaterializationFreshArchive(ctx, cfg.DatabaseURL) }
		cancel()
		if err != nil { log.Fatalf("initialize persistent stores: %v", err) }
		defer catalogStore.Close()
		defer historyStore.Close()
		defer freshArchive.Close()
	}

	researcher := historicalkb.Researcher{APIKey: cfg.OpenAIKey, Model: cfg.OpenAIModel}
	historyService := historicalkb.Service{Store: historyStore, Researcher: researcher, WorldDate: cfg.WorldDate}
	knowledgeService := historicalkb.KnowledgeService{Store: historyStore, Researcher: researcher}
	worldEngine := worldengine.Engine{Knowledge: knowledgeService}
	// Timeline planning asks for several structured events at once. A shared 90s
	// client keeps transient provider latency from tripping the provider's 30s
	// default while preserving request cancellation through the caller context.
	postRenderer := llm.StructuredOpenAIProvider{OpenAIProvider: llm.OpenAIProvider{APIKey: cfg.OpenAIKey, Model: cfg.OpenAIModel, Client: &http.Client{Timeout: 90 * time.Second}}}
	postMaterializer := worldrepo.LLMMaterializer{Renderer: postRenderer, Fallback: worldrepo.FallbackMaterializer{}, HistoricalReferencesEnabled: cfg.HistoricalReferencesEnabled, CuratedHistoricalReferences: true}
	runtimeStore := worldrepo.New(store, worldEngine, postMaterializer, cfg.WorldDate)
	materializationLab := newMaterializationLab(store, worldEngine, postMaterializer, cfg.WorldDate, cfg.MaterializationLabToken)
	materializationLab.freshArchive = freshArchive
	network := telephone.New(runtimeStore, clock)

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
	adminGuard := func(w http.ResponseWriter, _ *http.Request) bool {
		w.Header().Set("Content-Type", "application/json")
		if historyStore == nil { http.Error(w, `{"error":"historical research database is not configured"}`, http.StatusServiceUnavailable); return false }
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

	listResearch := func(w http.ResponseWriter, r *http.Request) { if !adminGuard(w,r){return}; cases,err:=historyStore.List(r.Context(),100);if err!=nil{w.WriteHeader(http.StatusInternalServerError);_=json.NewEncoder(w).Encode(map[string]any{"error":err.Error()});return};_=json.NewEncoder(w).Encode(map[string]any{"cases":cases}) }
	getResearch := func(w http.ResponseWriter, r *http.Request) { if !adminGuard(w,r){return};c,err:=historyStore.Get(r.Context(),r.URL.Query().Get("id"));if err!=nil{w.WriteHeader(http.StatusNotFound);_=json.NewEncoder(w).Encode(map[string]any{"error":err.Error()});return};_=json.NewEncoder(w).Encode(c) }
	createResearch := func(w http.ResponseWriter, r *http.Request) { if !adminGuard(w,r){return};if r.Method!=http.MethodPost{w.WriteHeader(http.StatusMethodNotAllowed);return};var in struct{Topic string `json:"topic"`;Question string `json:"question"`};if err:=json.NewDecoder(r.Body).Decode(&in);err!=nil{w.WriteHeader(http.StatusBadRequest);_=json.NewEncoder(w).Encode(map[string]any{"error":err.Error()});return};ctx,cancel:=context.WithTimeout(r.Context(),90*time.Second);defer cancel();c,err:=historyService.Create(ctx,in.Topic,in.Question);if err!=nil{w.WriteHeader(http.StatusBadGateway);_=json.NewEncoder(w).Encode(map[string]any{"error":err.Error()});return};_=json.NewEncoder(w).Encode(c) }
	chatResearch := func(w http.ResponseWriter, r *http.Request) { if !adminGuard(w,r){return};if r.Method!=http.MethodPost{w.WriteHeader(http.StatusMethodNotAllowed);return};var in struct{Message string `json:"message"`};if err:=json.NewDecoder(r.Body).Decode(&in);err!=nil{w.WriteHeader(http.StatusBadRequest);_=json.NewEncoder(w).Encode(map[string]any{"error":err.Error()});return};ctx,cancel:=context.WithTimeout(r.Context(),90*time.Second);defer cancel();c,err:=historyService.Chat(ctx,r.URL.Query().Get("id"),in.Message);if err!=nil{w.WriteHeader(http.StatusBadGateway);_=json.NewEncoder(w).Encode(map[string]any{"error":err.Error()});return};_=json.NewEncoder(w).Encode(c) }
	supplementResearch := func(w http.ResponseWriter, r *http.Request) { if !adminGuard(w,r){return};if r.Method!=http.MethodPost{w.WriteHeader(http.StatusMethodNotAllowed);return};var in struct{Supplement string `json:"supplement"`};if err:=json.NewDecoder(r.Body).Decode(&in);err!=nil{w.WriteHeader(http.StatusBadRequest);_=json.NewEncoder(w).Encode(map[string]any{"error":err.Error()});return};c,err:=historyService.OperatorSupplement(r.Context(),r.URL.Query().Get("id"),in.Supplement);if err!=nil{w.WriteHeader(http.StatusBadRequest);_=json.NewEncoder(w).Encode(map[string]any{"error":err.Error()});return};_=json.NewEncoder(w).Encode(c) }
	statusResearch := func(w http.ResponseWriter, r *http.Request) { if !adminGuard(w,r){return};if r.Method!=http.MethodPost{w.WriteHeader(http.StatusMethodNotAllowed);return};var in struct{Status historicalkb.Status `json:"status"`};if err:=json.NewDecoder(r.Body).Decode(&in);err!=nil{w.WriteHeader(http.StatusBadRequest);_=json.NewEncoder(w).Encode(map[string]any{"error":err.Error()});return};c,err:=historyService.SetStatus(r.Context(),r.URL.Query().Get("id"),in.Status);if err!=nil{w.WriteHeader(http.StatusBadRequest);_=json.NewEncoder(w).Encode(map[string]any{"error":err.Error()});return};_=json.NewEncoder(w).Encode(c) }
	resolveKnowledge := func(w http.ResponseWriter, r *http.Request) { w.Header().Set("Content-Type","application/json");if r.Method!=http.MethodPost{w.WriteHeader(http.StatusMethodNotAllowed);return};if historyStore==nil{http.Error(w,`{"error":"historical knowledge database is not configured"}`,http.StatusServiceUnavailable);return};var q historicalkb.KnowledgeQuery;if err:=json.NewDecoder(r.Body).Decode(&q);err!=nil{w.WriteHeader(http.StatusBadRequest);_=json.NewEncoder(w).Encode(map[string]any{"error":err.Error()});return};ctx,cancel:=context.WithTimeout(r.Context(),75*time.Second);defer cancel();result,err:=knowledgeService.Resolve(ctx,q);if err!=nil{w.WriteHeader(http.StatusBadGateway);_=json.NewEncoder(w).Encode(map[string]any{"error":err.Error(),"result":result});return};_=json.NewEncoder(w).Encode(result) }

	mux := http.NewServeMux()
	mux.Handle("/ws", wsserver.Handler{Network: network, Store: runtimeStore, Sessions: sessions})
	mux.HandleFunc("/api/world/bootstrap", bootstrapWorld)
	mux.HandleFunc("/api/centers", bootstrapWorld)
	mux.HandleFunc("/api/debug/export", newDebugExportHandler(store))
	mux.HandleFunc("/api/debug/materialization-lab", materializationLab.handler())
	mux.HandleFunc("/api/debug/materialization-lab-random", materializationLab.randomHandler())
	mux.HandleFunc("/api/debug/materialization-lab-allbody", materializationLab.allBodyHandler())
	mux.HandleFunc("/api/debug/materialization-lab-fresh", materializationLab.freshHandler())
	mux.HandleFunc("/api/debug/materialization-lab-fresh-view", materializationLab.freshViewerHandler())
	mux.HandleFunc("/api/debug/world/reset", resetWorld)
	mux.HandleFunc("/api/debug/host/reset", resetHost)
	mux.HandleFunc("/api/admin/research", listResearch)
	mux.HandleFunc("/api/admin/research/case", getResearch)
	mux.HandleFunc("/api/admin/research/new", createResearch)
	mux.HandleFunc("/api/admin/research/chat", chatResearch)
	mux.HandleFunc("/api/admin/research/supplement", supplementResearch)
	mux.HandleFunc("/api/admin/research/status", statusResearch)
	mux.HandleFunc("/api/internal/knowledge/resolve", resolveKnowledge)
	mux.HandleFunc("/api/poc/image-artifact", newImagePocHandler(cfg.OpenAIKey))
	mux.HandleFunc("/admin/research", func(w http.ResponseWriter,r *http.Request){w.Header().Set("Content-Type","text/html; charset=utf-8");_,_=w.Write([]byte(historicalkb.AdminPageHTML))})
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) { w.Header().Set("Content-Type", "application/json"); _ = json.NewEncoder(w).Encode(map[string]any{"ok":true,"world_date":cfg.WorldDate,"time":clock.Now(),"persistent_worlds":catalogStore!=nil,"historical_research":historyStore!=nil,"historical_knowledge":historyStore!=nil,"historical_references_enabled":cfg.HistoricalReferencesEnabled,"world_repository":true,"world_post_renderer":"openai-with-fallback","openai_model":cfg.OpenAIModel,"research_auth":"none-poc","debug_reset":cfg.DebugResetToken!="","materialization_lab":labEnabled(),"materialization_lab_auth":"none-test-only","materialization_lab_archive":freshArchive!=nil,"materialization_lab_daily_runs":publicLabDailyRuns}) })

	srv := &http.Server{Addr: cfg.Addr, Handler: cors(mux), ReadHeaderTimeout: 5*time.Second}
	log.Printf("zutto server listening on %s", cfg.Addr)
	log.Fatal(srv.ListenAndServe())
}

func cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-Zutto-Debug-Token, X-Zutto-Lab-Token")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		if r.Method == http.MethodOptions { w.WriteHeader(http.StatusNoContent); return }
		next.ServeHTTP(w,r)
	})
}
