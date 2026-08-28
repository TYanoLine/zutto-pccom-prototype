package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"zutto-pccom/apps/server/internal/config"
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
	sessions := wsserver.NewSessionManager(wsserver.DefaultReconnectGrace)

	mux := http.NewServeMux()
	mux.Handle("/ws", wsserver.Handler{Network: network, Store: store, Sessions: sessions})
	mux.HandleFunc("/api/centers", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"centers": dummyCenters()})
	})
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "world_date": cfg.WorldDate, "time": clock.Now()})
	})

	srv := &http.Server{Addr: cfg.Addr, Handler: cors(mux), ReadHeaderTimeout: 5 * time.Second}
	log.Printf("zutto server listening on %s", cfg.Addr)
	log.Fatal(srv.ListenAndServe())
}

func dummyCenters() []centerDirectoryEntry {
	// Fictional test names only.  The mix intentionally resembles the heterogeneous
	// naming styles found in 1990s Japanese personal BBS directories without
	// borrowing the identity of a specific historical station.
	names := []string{
		"MOONLIGHT NETWORK", "風の街ネット", "BLUE MOON STATION", "ぽぷら通信", "WINDY NET",
		"夢工房BBS", "GALAXY CLUB", "みなとネット", "ORANGE HOUSE", "星空通信",
		"MIDNIGHT BBS", "電脳茶屋", "SILVER STATION", "北の国ネット", "HARBOR LINK",
		"パソコン倶楽部ひまわり", "PENGUIN NET", "青空BBS", "MINT BASE", "こもれび通信",
	}
	bauds := []int{2400, 9600, 14400, 28800}
	centers := make([]centerDirectoryEntry, 0, 100)
	for i := 0; i < 100; i++ {
		area := 3 + (i % 7)
		phone := fmt.Sprintf("0%d%08d", area, 10000000+i)
		cycle := i / len(names)
		name := names[i%len(names)]
		if cycle > 0 {
			name = fmt.Sprintf("%s %d", name, cycle+1)
		}
		centers = append(centers, centerDirectoryEntry{
			ID:       fmt.Sprintf("dummy-%03d", i+1),
			Name:     name,
			Phone:    phone,
			DialMode: "tone",
			MaxBaud:  bauds[i%len(bauds)],
		})
	}
	return centers
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
