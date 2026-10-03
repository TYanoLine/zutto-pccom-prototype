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
