package main

import (
	"context"
	"log"
	"time"

	"zutto-pccom/apps/server/internal/world"
	"zutto-pccom/apps/server/internal/worldpersist"
)

const developmentMaterializationPhone = "0450000196"

// newRuntimeStore keeps the broad prototype on MemoryStore while adding durable
// persistence only for the expensive development materialization host. This is
// intentionally a stepping stone toward a proper canonical Postgres world store,
// not a claim that JSON snapshots are the final production schema.
func newRuntimeStore(databaseURL string) debugExportStore {
	base := world.NewMemoryStore()
	if databaseURL == "" {
		log.Printf("development materialization persistence disabled: DATABASE_URL is not set")
		return base
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	store, err := worldpersist.Open(ctx, databaseURL, base, developmentMaterializationPhone)
	if err != nil {
		log.Fatalf("initialize development materialization persistence: %v", err)
	}
	status := store.DevelopmentPersistenceStatus()
	log.Printf("development materialization persistence ready: backend=%s restored=%t", status.Backend, status.Loaded)
	return store
}
