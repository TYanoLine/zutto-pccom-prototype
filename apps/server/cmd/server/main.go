package main

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"zutto-pccom/apps/server/internal/config"
	"zutto-pccom/apps/server/internal/historicalkb"
	"zutto-pccom/apps/server/internal/hostprogram/erikak"
	"zutto-pccom/apps/server/internal/llm"
	"zutto-pccom/apps/server/internal/telephone"
	"zutto-pccom/apps/server/internal/world"
	"zutto-pccom/apps/server/internal/worldcatalog"
	"zutto-pccom/apps/server/internal/worldclock"
	"zutto-pccom/apps/server/internal/worldengine"
	"zutto-pccom/apps/server/internal/worldrepo"
	wsserver "zutto-pccom/apps/server/internal/ws"
)

const generatedCenterCount = 100

func main() {
	startedAt := time.Now()
	cfg := config.Load()
	store := newRuntimeStore(cfg.DatabaseURL)
	jst, err := time.LoadLocation("Asia/Tokyo")
	if err != nil {
		log.Fatalf("load Japan timezone: %v", err)
	}
	clock, err := worldclock.New(cfg.WorldDate, jst)
	if err != nil {
		log.Fatalf("create world clock: %v", err)
	}
	sessions := wsserver.NewSessionManager(wsserver.DefaultReconnectGrace)
	catalogGenerator := llm.CenterCatalogGenerator{Endpoint: cfg.AzureOpenAIEndpoint, APIKey: cfg.AzureOpenAIKey, Model: cfg.AzureOpenAIModel}

	var catalogStore *worldcatalog.Store
	var historyStore *historicalkb.Store
	if cfg.DatabaseURL == "" {
		log.Printf("DATABASE_URL is not set; persistent generated worlds and historical research are disabled")
	} else {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		catalogStore, err = worldcatalog.Open(ctx, cfg.DatabaseURL)
		if err == nil {
			err = catalogStore.EnsureSchema(ctx)
		}
		if err == nil {
			historyStore, err = historicalkb.Open(ctx, cfg.DatabaseURL)
		}
		if err == nil {
			err = historyStore.EnsureSchema(ctx)
		}
		cancel()
		if err != nil {
			log.Fatalf("initialize persistent stores: %v", err)
		}
		defer catalogStore.Close()
		defer historyStore.Close()
	}

	researcher := historicalkb.Researcher{Endpoint: cfg.AzureOpenAIEndpoint, APIKey: cfg.AzureOpenAIKey, Model: cfg.AzureOpenAIModel}
	historyService := historicalkb.Service{Store: historyStore, Researcher: researcher, WorldDate: cfg.WorldDate}
	knowledgeService := historicalkb.KnowledgeService{Store: historyStore, Researcher: researcher}
	worldEngine := worldengine.Engine{Knowledge: knowledgeService}
	if cfg.JevKey != "" {
		jevAdvisor := worldengine.JevAdvisor{
			APIKey: cfg.JevKey,
			Model:  cfg.JevModel,
			Client: &http.Client{Timeout: 4 * time.Second},
		}
		worldEngine.WriteAdvisor = jevAdvisor
		worldEngine.BehaviorAdvisor = jevAdvisor
		worldEngine.TitleAdvisor = jevAdvisor
	}
	// Candidate wording, article details, final prose, and bounded historical research
	// use Azure OpenAI. Jev remains an independent bounded semantic advisor for the
	// existing title/action/persona routes.
	azureOpenAIRenderer := llm.StructuredOpenAIProvider{OpenAIProvider: llm.OpenAIProvider{Endpoint: cfg.AzureOpenAIEndpoint, APIKey: cfg.AzureOpenAIKey, Model: cfg.AzureOpenAIModel, Client: &http.Client{Timeout: 90 * time.Second}}}
	postRenderer := azureOpenAIRenderer
	postMaterializer := newProductionMaterializer(postRenderer)
	runtimeStore := worldrepo.New(store, worldEngine, postMaterializer, cfg.WorldDate)
	runtimeStore.SetArticleDetailPlanner(postRenderer)
	runtimeStore.SetDebugLogBBSArticleDetails(cfg.DebugLogBBSArticleDetails)
	runtimeStore.SetDebugLogHAKATAGenerated(cfg.DebugLogHAKATAGenerated)
	runtimeStore.SetGenerationTraceEnabled(cfg.DebugHakataLLMTrace)
	runtimeStore.SetHAKATAFreeformBody(cfg.HakataFreeformBody)
	runtimeStore.SetWorldNow(clock.Now)
	network := telephone.New(runtimeStore, clock)

	generateNames := func(ctx context.Context, count int) ([]string, error) {
		generated, err := catalogGenerator.Generate(ctx, count, cfg.WorldDate)
		if err != nil {
			return nil, err
		}
		names := make([]string, len(generated))
		for i := range generated {
			names[i] = generated[i].Name
		}
		return names, nil
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
		catalog, err := catalogStore.GetOrCreate(ctx, worldKey, generatedCenterCount, generateNames)
		if err != nil {
			log.Printf("world bootstrap failed: %v", err)
			w.WriteHeader(http.StatusBadGateway)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": err.Error()})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"worldId": catalog.WorldID, "centers": catalog.Centers, "created": catalog.Created, "source": func() string {
			if catalog.Created {
				return "azure_openai"
			}
			return "postgres"
		}(), "model": cfg.AzureOpenAIModel})
	}

	adminGuard := func(w http.ResponseWriter, _ *http.Request) bool {
		w.Header().Set("Content-Type", "application/json")
		if historyStore == nil {
			http.Error(w, `{"error":"historical research database is not configured"}`, http.StatusServiceUnavailable)
			return false
		}
		return true
	}

	resetBBSArticles := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		if cfg.DebugResetToken == "" || subtle.ConstantTimeCompare([]byte(r.Header.Get("X-Zutto-Debug-Token")), []byte(cfg.DebugResetToken)) != 1 {
			w.WriteHeader(http.StatusForbidden)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "debug BBS reset requires DEBUG_RESET_TOKEN"})
			return
		}
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", "POST")
			w.WriteHeader(http.StatusMethodNotAllowed)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": "POST only"})
			return
		}
		phone := strings.TrimSpace(r.URL.Query().Get("phone"))
		// Public test deployment safety: expose the generic reset machinery only
		// for the persistent experiment station (role: experiment). An unknown
		// number and a non-experiment host get the same answer on purpose.
		host, err := runtimeStore.HostByPhone(phone)
		if err != nil || !host.IsExperiment() {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": "debug reset is limited to the experiment host"})
			return
		}
		if runtimeStore.MaterializationObservationRunning(host.ID) {
			w.WriteHeader(http.StatusConflict)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": "BBS observation/generation is still running; retry after it completes"})
			return
		}
		removed, kept, ok := runtimeStore.ResetBBSGeneratedArticles(host)
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
			"kept_posts":              kept,
			"kept":                    "seed/user history + boards + personas + host program configuration",
			"next":                    "visit a board again to run a fresh shared-engine catch-up batch",
		})
	}

	bbsSample := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		if cfg.DebugResetToken == "" || subtle.ConstantTimeCompare([]byte(r.Header.Get("X-Zutto-Debug-Token")), []byte(cfg.DebugResetToken)) != 1 {
			w.WriteHeader(http.StatusForbidden)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "debug BBS sample requires DEBUG_RESET_TOKEN"})
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodPost {
			w.Header().Set("Allow", "GET, POST")
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		phone := strings.TrimSpace(r.URL.Query().Get("phone"))
		if phone == "" {
			phone = defaultExperimentPhone(store)
		}
		host, err := runtimeStore.HostByPhone(phone)
		if err != nil || !host.IsExperiment() {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": "debug sample is limited to the experiment host"})
			return
		}
		boardID := strings.TrimSpace(r.URL.Query().Get("board"))
		if boardID == "" {
			boardID = "20/1"
		}
		var board world.Board
		for _, candidate := range store.ListBoards(host.ID) {
			if candidate.ID == boardID {
				board = candidate
				break
			}
		}
		// Erika-K owns its own board tree rather than storing that catalog in
		// the shared World BoardStore. Ask the host program for the canonical
		// board path instead of duplicating one debug-only GAME special case.
		if board.ID == "" && host.SoftwareID == "erika-k" {
			if resolved, ok := erikak.BoardByPath(boardID); ok {
				board = resolved
			}
		}
		if board.ID == "" {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": "unknown board"})
			return
		}

		action := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("action")))
		if action == "start" {
			if runtimeStore.MaterializationObservationRunning(host.ID) {
				w.WriteHeader(http.StatusConflict)
				_ = json.NewEncoder(w).Encode(map[string]any{"error": "BBS observation/generation is already running"})
				return
			}
			removed, kept, ok := runtimeStore.PrepareDebugBBSConnection(host)
			if !ok {
				w.WriteHeader(http.StatusConflict)
				_ = json.NewEncoder(w).Encode(map[string]any{"error": "debug BBS sample reset unavailable"})
				return
			}
			runtimeStore.BeginHostObservation(host, []world.Board{board})
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status":        "started",
				"phone":         phone,
				"host_id":       host.ID,
				"board_id":      board.ID,
				"board_name":    board.Name,
				"removed_posts": removed,
				"kept_posts":    kept,
			})
			return
		}
		if action != "" && action != "status" {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": "action must be start or status"})
			return
		}

		type samplePost struct {
			ID        int64     `json:"id"`
			Author    string    `json:"author"`
			Subject   string    `json:"subject"`
			ParentID  int64     `json:"parent_id,omitempty"`
			CreatedAt time.Time `json:"created_at"`
		}
		posts := make([]samplePost, 0)
		rootCount := 0
		for _, post := range runtimeStore.ListPosts(host.ID) {
			if post.BoardID != board.ID {
				continue
			}
			if post.ParentID == 0 {
				rootCount++
			}
			posts = append(posts, samplePost{ID: post.ID, Author: post.Author, Subject: post.Subject, ParentID: post.ParentID, CreatedAt: post.CreatedAt})
		}
		running := runtimeStore.MaterializationObservationRunning(host.ID)
		status := "idle"
		if running {
			status = "running"
		} else if len(posts) > 0 {
			status = "completed"
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status":     status,
			"phone":      phone,
			"host_id":    host.ID,
			"board_id":   board.ID,
			"board_name": board.Name,
			"post_count": len(posts),
			"root_count": rootCount,
			"posts":      posts,
		})
	}

	listResearch := func(w http.ResponseWriter, r *http.Request) {
		if !adminGuard(w, r) {
			return
		}
		cases, err := historyStore.List(r.Context(), 100)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": err.Error()})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"cases": cases})
	}
	getResearch := func(w http.ResponseWriter, r *http.Request) {
		if !adminGuard(w, r) {
			return
		}
		c, err := historyStore.Get(r.Context(), r.URL.Query().Get("id"))
		if err != nil {
			w.WriteHeader(http.StatusNotFound)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": err.Error()})
			return
		}
		_ = json.NewEncoder(w).Encode(c)
	}
	createResearch := func(w http.ResponseWriter, r *http.Request) {
		if !adminGuard(w, r) {
			return
		}
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		var in struct {
			Topic    string `json:"topic"`
			Question string `json:"question"`
		}
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": err.Error()})
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 90*time.Second)
		defer cancel()
		c, err := historyService.Create(ctx, in.Topic, in.Question)
		if err != nil {
			w.WriteHeader(http.StatusBadGateway)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": err.Error()})
			return
		}
		_ = json.NewEncoder(w).Encode(c)
	}
	chatResearch := func(w http.ResponseWriter, r *http.Request) {
		if !adminGuard(w, r) {
			return
		}
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		var in struct {
			Message string `json:"message"`
		}
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": err.Error()})
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 90*time.Second)
		defer cancel()
		c, err := historyService.Chat(ctx, r.URL.Query().Get("id"), in.Message)
		if err != nil {
			w.WriteHeader(http.StatusBadGateway)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": err.Error()})
			return
		}
		_ = json.NewEncoder(w).Encode(c)
	}
	supplementResearch := func(w http.ResponseWriter, r *http.Request) {
		if !adminGuard(w, r) {
			return
		}
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		var in struct {
			Supplement string `json:"supplement"`
		}
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": err.Error()})
			return
		}
		c, err := historyService.OperatorSupplement(r.Context(), r.URL.Query().Get("id"), in.Supplement)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": err.Error()})
			return
		}
		_ = json.NewEncoder(w).Encode(c)
	}
	statusResearch := func(w http.ResponseWriter, r *http.Request) {
		if !adminGuard(w, r) {
			return
		}
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		var in struct {
			Status historicalkb.Status `json:"status"`
		}
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": err.Error()})
			return
		}
		c, err := historyService.SetStatus(r.Context(), r.URL.Query().Get("id"), in.Status)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": err.Error()})
			return
		}
		_ = json.NewEncoder(w).Encode(c)
	}
	resolveKnowledge := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if historyStore == nil {
			http.Error(w, `{"error":"historical knowledge database is not configured"}`, http.StatusServiceUnavailable)
			return
		}
		var q historicalkb.KnowledgeQuery
		if err := json.NewDecoder(r.Body).Decode(&q); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": err.Error()})
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 75*time.Second)
		defer cancel()
		result, err := knowledgeService.Resolve(ctx, q)
		if err != nil {
			w.WriteHeader(http.StatusBadGateway)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": err.Error(), "result": result})
			return
		}
		_ = json.NewEncoder(w).Encode(result)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/api/version", newServerVersionHandler(serverBuildInfoFromEnvironment(startedAt)))
	mux.Handle("/ws", wsserver.Handler{Network: network, Store: runtimeStore, Sessions: sessions})
	mux.HandleFunc("/api/world/bootstrap", bootstrapWorld)
	mux.HandleFunc("/api/centers", bootstrapWorld)
	mux.HandleFunc("/api/debug/bbs/reset", resetBBSArticles)
	mux.HandleFunc("/api/debug/bbs/sample", bbsSample)
	mux.HandleFunc("/api/debug/bbs/generation-trace", newHakataTraceHandler(cfg.DebugHakataLLMTrace, runtimeStore))
	mux.HandleFunc("/api/admin/research", listResearch)
	mux.HandleFunc("/api/admin/research/case", getResearch)
	mux.HandleFunc("/api/admin/research/new", createResearch)
	mux.HandleFunc("/api/admin/research/chat", chatResearch)
	mux.HandleFunc("/api/admin/research/supplement", supplementResearch)
	mux.HandleFunc("/api/admin/research/status", statusResearch)
	mux.HandleFunc("/api/internal/knowledge/resolve", resolveKnowledge)
	mux.HandleFunc("/admin/research", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(historicalkb.AdminPageHTML))
	})
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "world_date": cfg.WorldDate, "time": clock.Now(), "persistent_worlds": catalogStore != nil, "historical_research": historyStore != nil, "historical_knowledge": historyStore != nil, "historical_references_enabled": postMaterializer.HistoricalReferencesEnabled, "debug_log_bbs_article_details": cfg.DebugLogBBSArticleDetails, "world_repository": true, "world_post_renderer": "azure-openai-article-worker-with-jev-title-advisor", "azure_openai_model": cfg.AzureOpenAIModel, "azure_openai_configured": cfg.AzureOpenAIEndpoint != "" && cfg.AzureOpenAIKey != "", "jev_model": cfg.JevModel, "jev_configured": cfg.JevKey != "", "jev_world_write_advisor": cfg.JevKey != "", "jev_world_behavior_advisor": cfg.JevKey != "", "jev_title_advisor": cfg.JevKey != "", "debug_reset": cfg.DebugResetToken != ""})
	})

	srv := &http.Server{Addr: cfg.Addr, Handler: cors(mux), ReadHeaderTimeout: 5 * time.Second}

	log.Printf("zutto server listening on %s", cfg.Addr)
	log.Fatal(srv.ListenAndServe())
}

func cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-Zutto-Debug-Token")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}