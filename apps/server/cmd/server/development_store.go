package main

import (
	"context"
	"log"
	"time"

	"zutto-pccom/apps/server/internal/world"
	"zutto-pccom/apps/server/internal/worldpersist"
)

const (
	developmentMaterializationPhone = "0450000196"
	erikaKExperimentPhone           = "0920000196"
)

var developmentSnapshotTargets = []worldpersist.HostTarget{
	{Phone: developmentMaterializationPhone},
	// HAKATA CANAL NET is currently a fixed fixture at the host-program/config
	// layer. Persist its evolving world state, but keep code-defined host settings
	// authoritative across restarts. Future AI host generation can change this
	// policy when host configuration itself becomes canonical world state.
	{Phone: erikaKExperimentPhone, KeepSeedHostConfig: true},
}

var developmentGrassrootsBoardCatalog = []world.Board{
	{ID: "1", Name: "フリートーク"},
	{ID: "2", Name: "パソコン通信・モデム"},
	{ID: "3", Name: "地域の話題"},
	{ID: "4", Name: "ゲーム"},
	{ID: "5", Name: "音楽"},
	{ID: "6", Name: "ソフトウェア"},
	{ID: "7", Name: "はじめまして・初心者"},
	{ID: "8", Name: "オフ会・仲間募集"},
	{ID: "9", Name: "売ります・買います"},
	{ID: "10", Name: "PC・周辺機器"},
	{ID: "11", Name: "通信ソフト・設定"},
	{ID: "12", Name: "ファイル・アップロード"},
	{ID: "13", Name: "局への要望・連絡"},
	{ID: "14", Name: "雑談・独り言"},
	{ID: "15", Name: "ゲーム攻略・情報"},
	{ID: "16", Name: "音楽・CD・ライブ"},
}

// ensureDevelopmentBoardCatalog expands only the development observation host.
// Existing/persisted boards win: this appends missing IDs instead of replacing
// operator-created or previously materialized board state.
func ensureDevelopmentBoardCatalog(store debugExportStore) {
	host, err := store.HostByPhone(developmentMaterializationPhone)
	if err != nil {
		return
	}
	existing := store.ListBoards(host.ID)
	seen := make(map[string]bool, len(existing))
	for _, board := range existing {
		seen[board.ID] = true
	}
	merged := append([]world.Board(nil), existing...)
	for _, board := range developmentGrassrootsBoardCatalog {
		if seen[board.ID] {
			continue
		}
		merged = append(merged, board)
		seen[board.ID] = true
	}
	if len(merged) != len(existing) {
		store.SaveBoards(host.ID, merged)
	}
}

// newRuntimeStore keeps the broad prototype on MemoryStore while adding durable
// JSONB snapshot persistence only for selected experiment hosts. This is
// intentionally a stepping stone toward a proper canonical Postgres world store,
// not a claim that JSON snapshots are the final production schema.
func newRuntimeStore(databaseURL string) debugExportStore {
	base := world.NewMemoryStore()
	if databaseURL == "" {
		ensureDevelopmentBoardCatalog(base)
		log.Printf("development materialization persistence disabled: DATABASE_URL is not set")
		return base
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	store, err := worldpersist.Open(ctx, databaseURL, base, developmentSnapshotTargets)
	if err != nil {
		log.Fatalf("initialize experiment host persistence: %v", err)
	}
	// HAKATA is currently a pure generator-evaluation fixture: keep resident
	// identities but no article baseline. Older snapshots may contain the former
	// 40-per-board seed or prior generated/user posts, so clear them immediately
	// on process startup and persist the empty article state.
	if added := store.EnsureHakataExperimentCast(erikaKExperimentPhone); added > 0 {
		log.Printf("HAKATA CANAL NET resident cast restored: added_members=%d", added)
	}
	if host, hostErr := store.HostByPhone(erikaKExperimentPhone); hostErr == nil {
		removed := store.ClearHostPosts(host.ID)
		log.Printf("HAKATA CANAL NET article baseline cleared: removed_posts=%d", removed)
	}
	ensureDevelopmentBoardCatalog(store)
	status := store.DevelopmentPersistenceStatus()
	log.Printf("experiment host persistence ready: backend=%s restored=%t targets=%d", status.Backend, status.Loaded, len(developmentSnapshotTargets))
	return store
}
