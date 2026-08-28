package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"zutto-pccom/apps/server/internal/config"
	"zutto-pccom/apps/server/internal/llm"
	"zutto-pccom/apps/server/internal/telephone"
	"zutto-pccom/apps/server/internal/world"
	"zutto-pccom/apps/server/internal/worldclock"
	wsserver "zutto-pccom/apps/server/internal/ws"
)

type centerDirectoryEntry struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Phone    string `json:"phone"`
	DialMode string `json:"dialMode"`
	MaxBaud  int    `json:"maxBaud"`
}

const temporaryCenterCount = 10

func main() {
	cfg := config.Load()
	store := world.NewMemoryStore()
	jst, err := time.LoadLocation("Asia/Tokyo")
	if err != nil { log.Fatalf("load Japan timezone: %v", err) }
	clock, err := worldclock.New(cfg.WorldDate, jst)
	if err != nil { log.Fatalf("create world clock: %v", err) }
	network := telephone.New(store, clock)
	sessions := wsserver.NewSessionManager(wsserver.DefaultReconnectGrace)
	catalog := llm.CenterCatalogGenerator{APIKey: cfg.OpenAIKey, Model: cfg.OpenAIModel}

	mux := http.NewServeMux()
	mux.Handle("/ws", wsserver.Handler{Network: network, Store: store, Sessions: sessions})
	mux.HandleFunc("/api/centers", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
		defer cancel()
		names, err := catalog.Generate(ctx, temporaryCenterCount, cfg.WorldDate)
		if err != nil {
			log.Printf("AI center catalog failed, using fictional fallback: %v", err)
			_ = json.NewEncoder(w).Encode(map[string]any{"centers": fallbackCenters(temporaryCenterCount), "source": "fallback", "error": err.Error()})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"centers": centersFromNames(names), "source": "openai", "model": cfg.OpenAIModel})
	})
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "world_date": cfg.WorldDate, "time": clock.Now()})
	})

	srv := &http.Server{Addr: cfg.Addr, Handler: cors(mux), ReadHeaderTimeout: 5 * time.Second}
	log.Printf("zutto server listening on %s", cfg.Addr)
	log.Fatal(srv.ListenAndServe())
}

func centersFromNames(names []llm.CenterName) []centerDirectoryEntry {
	bauds := []int{2400, 9600, 14400, 28800}
	centers := make([]centerDirectoryEntry, 0, len(names))
	for i, generated := range names {
		area := 3 + (i % 7)
		centers = append(centers, centerDirectoryEntry{
			ID: fmt.Sprintf("ai-%03d", i+1), Name: generated.Name,
			Phone: fmt.Sprintf("0%d%08d", area, 10000000+i), DialMode: "tone", MaxBaud: bauds[i%len(bauds)],
		})
	}
	return centers
}

func fallbackCenters(count int) []centerDirectoryEntry {
	names := []string{
		"MOONLIGHT NETWORK", "風の街ネット", "BLUE MOON STATION", "ぽぷら通信", "WINDY NET",
		"夢工房BBS", "GALAXY CLUB", "みなとネット", "ORANGE HOUSE", "星空通信",
		"MIDNIGHT BBS", "電脳茶屋", "SILVER STATION", "北の国ネット", "HARBOR LINK",
		"パソコン倶楽部ひまわり", "PENGUIN NET", "青空BBS", "MINT BASE", "こもれび通信",
	}
	generated := make([]llm.CenterName, count)
	for i := range generated { name := names[i%len(names)]; if i >= len(names) { name = fmt.Sprintf("%s %d", name, i/len(names)+1) }; generated[i] = llm.CenterName{Name: name} }
	return centersFromNames(generated)
}

func cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions { w.WriteHeader(http.StatusNoContent); return }
		next.ServeHTTP(w, r)
	})
}
