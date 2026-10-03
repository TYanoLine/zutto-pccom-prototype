package main

import (
	"context"
	"log"
	"time"

	"zutto-pccom/apps/server/internal/world"
	"zutto-pccom/apps/server/internal/worldpersist"
)

// A world store with the optional canonical board/persona capabilities required
// by the operational BBS inspection endpoint.
type runtimeWorldStore interface {
	world.Store
	world.BoardStore
	world.PersonaStore
	world.PersonaFactStore
}

// experimentHostLister is implemented by *world.MemoryStore and anything that
// embeds it, such as the durable snapshot store.
type experimentHostLister interface{ ExperimentHosts() []world.Host }

// defaultExperimentPhone returns the phone number of the experiment host when
// there is exactly one, and "" otherwise.
func defaultExperimentPhone(store world.Store) string {
	lister, ok := store.(experimentHostLister)
	if !ok {
		return ""
	}
	hosts := lister.ExperimentHosts()
	if len(hosts) != 1 {
		return ""
	}
	return hosts[0].Phone
}

// Until the full world model is represented by transactional Postgres tables,
// the HAKATA evaluation station uses a durable snapshot. The explicit
// reconnect reset is separate: it is preserved for generation-quality testing.
// The station is whichever host has the experiment role in its preset.
func newRuntimeStore(databaseURL string) runtimeWorldStore {
	base := world.NewMemoryStore()
	experiment := base.ExperimentHosts()
	if databaseURL == "" {
		log.Printf("HAKATA world persistence unavailable: DATABASE_URL is not set")
		return base
	}
	if len(experiment) == 0 {
		log.Printf("HAKATA world persistence skipped: no host has the experiment role")
		return base
	}
	targets := make([]worldpersist.HostTarget, 0, len(experiment))
	for _, h := range experiment {
		targets = append(targets, worldpersist.HostTarget{Phone: h.Phone, KeepSeedHostConfig: true})
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	store, err := worldpersist.Open(ctx, databaseURL, base, targets)
	if err != nil {
		log.Fatalf("initialize HAKATA world persistence: %v", err)
	}
	for _, h := range experiment {
		if added := store.EnsureHakataExperimentPopulation(h.Phone); added > 0 {
			log.Printf("HAKATA membership population restored: added_members=%d", added)
		}
		if host, hostErr := store.HostByPhone(h.Phone); hostErr == nil {
			removed := store.ClearHostPosts(host.ID)
			log.Printf("HAKATA startup article baseline cleared: removed_posts=%d", removed)
		}
	}
	status := store.DevelopmentPersistenceStatus()
	log.Printf("HAKATA world persistence ready: backend=%s restored=%t", status.Backend, status.Loaded)
	return store
}
