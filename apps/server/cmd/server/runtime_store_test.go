package main

import (
	"testing"

	"zutto-pccom/apps/server/internal/world"
)

func TestDefaultExperimentPhoneFollowsTheExperimentRole(t *testing.T) {
	store := world.NewMemoryStore()
	hosts := store.ExperimentHosts()
	if len(hosts) != 1 {
		t.Fatalf("want exactly one experiment host, got %+v", hosts)
	}
	if got := defaultExperimentPhone(store); got != hosts[0].Phone || got == "" {
		t.Fatalf("defaultExperimentPhone = %q, want %q", got, hosts[0].Phone)
	}
}

type storeWithoutHostListing struct{ world.Store }

func TestDefaultExperimentPhoneIsEmptyWithoutAHostLister(t *testing.T) {
	if got := defaultExperimentPhone(storeWithoutHostListing{world.NewMemoryStore()}); got != "" {
		t.Fatalf("defaultExperimentPhone = %q, want empty for a store that cannot list hosts", got)
	}
}
