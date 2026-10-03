package main

import (
	"testing"

	"zutto-pccom/apps/server/internal/world"
)

func TestDefaultDebugEndpointPhoneFollowsTheFlag(t *testing.T) {
	store := world.NewMemoryStore()
	hosts := store.DebugEndpointHosts()
	if len(hosts) != 1 {
		t.Fatalf("want exactly one host with debug.http_endpoints, got %+v", hosts)
	}
	if got := defaultDebugEndpointPhone(store); got != hosts[0].Phone || got == "" {
		t.Fatalf("defaultDebugEndpointPhone = %q, want %q", got, hosts[0].Phone)
	}
}

type storeWithoutHostListing struct{ world.Store }

func TestDefaultDebugEndpointPhoneIsEmptyWithoutAHostLister(t *testing.T) {
	if got := defaultDebugEndpointPhone(storeWithoutHostListing{world.NewMemoryStore()}); got != "" {
		t.Fatalf("defaultDebugEndpointPhone = %q, want empty for a store that cannot list hosts", got)
	}
}

func TestSnapshotHostsFollowTheFlag(t *testing.T) {
	store := world.NewMemoryStore()
	hosts := store.SnapshotHosts()
	if len(hosts) != 1 {
		t.Fatalf("want exactly one host with debug.snapshot, got %+v", hosts)
	}
	for _, h := range hosts {
		if !h.Debug.Snapshot {
			t.Fatalf("host %s listed without debug.snapshot", h.ID)
		}
	}
}
