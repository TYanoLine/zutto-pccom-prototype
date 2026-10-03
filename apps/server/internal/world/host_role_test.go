package world

import (
	"strings"
	"testing"

	"zutto-pccom/apps/server/internal/hostcatalog"
)

func TestIsExperimentFollowsTheRoleNotTheIdentity(t *testing.T) {
	if !(Host{Role: hostcatalog.RoleExperiment}).IsExperiment() {
		t.Fatal("a host with the experiment role must be the experiment host")
	}
	// Same ID and phone as the real station, but no role: not the experiment host.
	if (Host{ID: "hakata-canal-net", Phone: "0920000196"}).IsExperiment() {
		t.Fatal("identity alone must not make a host the experiment host")
	}
	if (Host{Role: hostcatalog.RoleTest}).IsExperiment() {
		t.Fatal("a test-role host is not the experiment host")
	}
}

func TestExperimentHostsComesFromThePresets(t *testing.T) {
	hosts := NewMemoryStore().ExperimentHosts()
	if len(hosts) != 1 || hosts[0].ID != "hakata-canal-net" || hosts[0].Phone != "0920000196" {
		t.Fatalf("experiment hosts = %+v, want only hakata-canal-net", hosts)
	}
}

func TestExperimentHostsAreOrderedByPhone(t *testing.T) {
	s := NewMemoryStore()
	s.SaveHost(Host{ID: "exp-b", Phone: "0990000002", Role: hostcatalog.RoleExperiment})
	s.SaveHost(Host{ID: "exp-a", Phone: "0910000001", Role: hostcatalog.RoleExperiment})
	got := s.ExperimentHosts()
	if len(got) != 3 || got[0].Phone != "0910000001" || got[1].Phone != "0920000196" || got[2].Phone != "0990000002" {
		t.Fatalf("not ordered by phone: %+v", got)
	}
}

func TestOnlyExperimentHostsGetTheResidentPopulation(t *testing.T) {
	s := NewMemoryStore()
	if n := len(s.ListHostPersonas("hakata-canal-net")); n == 0 {
		t.Fatal("the experiment host has no resident population")
	}
	if n := len(s.ListHostPersonas("busy-test")); n != 0 {
		t.Fatalf("a non-experiment host got %d resident personas", n)
	}
}

func TestPresetsAllowAtMostOneExperimentHost(t *testing.T) {
	one := []Host{{ID: "a", Role: hostcatalog.RoleExperiment}, {ID: "b"}}
	if err := checkSingleExperiment(one); err != nil {
		t.Fatal(err)
	}
	two := []Host{{ID: "a", Role: hostcatalog.RoleExperiment}, {ID: "b", Role: hostcatalog.RoleExperiment}}
	if err := checkSingleExperiment(two); err == nil || !strings.Contains(err.Error(), "at most one") {
		t.Fatalf("two experiment hosts accepted: %v", err)
	}
}

func TestHostFromDescriptorCarriesTheRole(t *testing.T) {
	d := hostcatalog.HostDescriptor{Key: "x", Role: hostcatalog.RoleExperiment}
	if h := HostFromDescriptor(d); !h.IsExperiment() {
		t.Fatalf("role was not carried into the host: %+v", h)
	}
}

func TestHostFromDescriptorCarriesTheFlags(t *testing.T) {
	d := hostcatalog.HostDescriptor{
		Key: "x",
		Debug: hostcatalog.DebugFlags{
			ResetArticlesOnConnect: true,
			GenerationTrace:        true,
			ContentLog:             true,
			HTTPEndpoints:          true,
			Snapshot:               true,
		},
		GenerationFlags: hostcatalog.GenerationFlags{FreeformBody: true},
	}
	h := HostFromDescriptor(d)
	if h.Debug != d.Debug || h.Generation != d.GenerationFlags {
		t.Fatalf("flags were not carried into the host: %+v", h)
	}
	// A host definition without flags has everything off.
	if off := HostFromDescriptor(hostcatalog.HostDescriptor{Key: "y"}); off.Debug != (hostcatalog.DebugFlags{}) || off.Generation != (hostcatalog.GenerationFlags{}) {
		t.Fatalf("flags must default to off: %+v", off)
	}
}

func TestFlagBasedHostListsComeFromThePresets(t *testing.T) {
	s := NewMemoryStore()
	for name, hosts := range map[string][]Host{
		"snapshot":      s.SnapshotHosts(),
		"http endpoint": s.DebugEndpointHosts(),
	} {
		if len(hosts) != 1 || hosts[0].ID != "hakata-canal-net" {
			t.Fatalf("%s hosts = %+v, want only hakata-canal-net", name, hosts)
		}
	}
	// The generic test preset opts in to nothing.
	busy, err := s.HostByPhone("0459999999")
	if err != nil {
		t.Fatal(err)
	}
	if busy.Debug != (hostcatalog.DebugFlags{}) || busy.Generation != (hostcatalog.GenerationFlags{}) {
		t.Fatalf("a preset without flag blocks must have every flag off: %+v", busy)
	}
}

func TestFlagBasedHostListsFollowTheFlagsNotTheRole(t *testing.T) {
	s := NewMemoryStore()
	s.SaveHost(Host{ID: "role-only", Phone: "0910000001", Role: hostcatalog.RoleExperiment})
	s.SaveHost(Host{ID: "flag-only", Phone: "0990000002", Debug: hostcatalog.DebugFlags{Snapshot: true, HTTPEndpoints: true}})
	for name, got := range map[string][]Host{
		"snapshot":      s.SnapshotHosts(),
		"http endpoint": s.DebugEndpointHosts(),
	} {
		if len(got) != 2 || got[0].Phone != "0920000196" || got[1].Phone != "0990000002" {
			t.Fatalf("%s hosts = %+v, want hakata-canal-net then flag-only, ordered by phone and without role-only", name, got)
		}
	}
}
