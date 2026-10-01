package main

import (
	"context"
	"log"
	"time"

	"zutto-pccom/apps/server/internal/world"
	"zutto-pccom/apps/server/internal/worldpersist"
)

const erikaKExperimentPhone = "0920000196"

// A world store with the optional canonical board/persona capabilities required
// by the operational BBS inspection endpoint.
type runtimeWorldStore interface {
	world.Store
	world.BoardStore
	world.PersonaStore
	world.PersonaFactStore
}

// Until the full world model is represented by transactional Postgres tables,
// the HAKATA evaluation station uses a durable snapshot. The explicit
// reconnect reset is separate: it is preserved for generation-quality testing.
func newRuntimeStore(databaseURL string) runtimeWorldStore {
	base := world.NewMemoryStore()
	if databaseURL == "" {
		log.Printf("HAKATA world persistence unavailable: DATABASE_URL is not set")
		return base
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	store, err := worldpersist.Open(ctx, databaseURL, base, []worldpersist.HostTarget{
		{Phone: erikaKExperimentPhone, KeepSeedHostConfig: true},
	})
	if err != nil {
		log.Fatalf("initialize HAKATA world persistence: %v", err)
	}
	if added := store.EnsureHakataExperimentPopulation(erikaKExperimentPhone); added > 0 {
		log.Printf("HAKATA membership population restored: added_members=%d", added)
	}
	if host, hostErr := store.HostByPhone(erikaKExperimentPhone); hostErr == nil {
		removed := store.ClearHostPosts(host.ID)
		log.Printf("HAKATA startup article baseline cleared: removed_posts=%d", removed)
	}
	status := store.DevelopmentPersistenceStatus()
	log.Printf("HAKATA world persistence ready: backend=%s restored=%t", status.Backend, status.Loaded)
	return store
}
