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

// debugEndpointHostLister is implemented by *world.MemoryStore and anything that
// embeds it, such as the debug snapshot store.
type debugEndpointHostLister interface {
	DebugEndpointHosts() []world.Host
}

// defaultDebugEndpointPhone returns the phone number of the host that has
// debug.http_endpoints when there is exactly one, and "" otherwise. The debug
// sample endpoint uses it when no phone is given.
func defaultDebugEndpointPhone(store world.Store) string {
	lister, ok := store.(debugEndpointHostLister)
	if !ok {
		return ""
	}
	hosts := lister.DebugEndpointHosts()
	if len(hosts) != 1 {
		return ""
	}
	return hosts[0].Phone
}

// newRuntimeStore returns the in-memory world store. Hosts whose preset sets
// debug.snapshot additionally get a debug snapshot in Postgres, so that their
// materialized world state (boards, posts, memberships, personas, persona facts)
// survives a restart. This is a development stopgap until the world is stored
// in normalized tables; every other host stays memory-only. The host definition
// is never read back from the snapshot: it always comes from the preset.
func newRuntimeStore(databaseURL string) runtimeWorldStore {
	base := world.NewMemoryStore()
	snapshotHosts := base.SnapshotHosts()
	if len(snapshotHosts) == 0 {
		log.Printf("debug world snapshot skipped: no host has debug.snapshot")
		return base
	}
	if databaseURL == "" {
		log.Printf("debug world snapshot unavailable: DATABASE_URL is not set")
		return base
	}
	targets := make([]worldpersist.HostTarget, 0, len(snapshotHosts))
	for _, h := range snapshotHosts {
		targets = append(targets, worldpersist.HostTarget{Phone: h.Phone})
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	store, err := worldpersist.Open(ctx, databaseURL, base, targets)
	if err != nil {
		log.Fatalf("initialize debug world snapshot: %v", err)
	}
	for _, h := range snapshotHosts {
		// The resident population is still keyed on the experiment role; restore
		// it after the snapshot so an older snapshot does not shrink it.
		if !h.IsExperiment() {
			continue
		}
		if added := store.EnsureHakataExperimentPopulation(h.Phone); added > 0 {
			log.Printf("HAKATA membership population restored: added_members=%d", added)
		}
	}
	status := store.DevelopmentPersistenceStatus()
	log.Printf("debug world snapshot ready: backend=%s restored=%t hosts=%d", status.Backend, status.Loaded, len(targets))
	return store
}
