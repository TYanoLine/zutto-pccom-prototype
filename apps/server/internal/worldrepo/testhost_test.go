package worldrepo

import (
	"testing"

	"zutto-pccom/apps/server/internal/hostcatalog"
	"zutto-pccom/apps/server/internal/world"
)

// Tests need a quiet generic-runtime host. It is defined here so that no
// production station has to exist for them.
const (
	genericTestPhone  = "0450000010"
	genericTestHostID = "generic-test"
)

func genericTestHost() world.Host {
	return world.Host{ID: genericTestHostID, Phone: genericTestPhone, Name: "GENERIC TEST BBS", Region: "神奈川県", Software: "mmm compatible", SoftwareID: "generic", Lines: 8, Popularity: .05, MaxBaud: 28800, Members: 22, FoundedOn: "1996-05-05", GuestAllowed: true}
}

// experimentTestHost is a host with the experiment role. Its ID is deliberately
// not "hakata-canal-net": behavior must follow the role, not the identity.
func experimentTestHost() world.Host {
	return world.Host{ID: "experiment-test", Role: hostcatalog.RoleExperiment}
}

// newTestStore returns a memory store that also holds genericTestHost.
func newTestStore() *world.MemoryStore {
	s := world.NewMemoryStore()
	s.SaveHost(genericTestHost())
	return s
}

// The experiment-only behaviors (generation trace, generated-content log, prose
// experiment) follow the role. The real station's ID without the role does not
// get them.
func TestExperimentOnlyBehaviorsFollowTheRoleNotTheID(t *testing.T) {
	r := &Repository{hakataFreeformBody: true, debugLogHAKATAGenerated: true}
	if !r.useHAKATAFreeformBody(experimentTestHost()) || !r.shouldLogHAKATAGenerated(experimentTestHost()) {
		t.Fatal("a host with the experiment role was not treated as the experiment host")
	}
	lookalike := world.Host{ID: "hakata-canal-net", Phone: "0920000196"}
	if r.useHAKATAFreeformBody(lookalike) || r.shouldLogHAKATAGenerated(lookalike) {
		t.Fatal("the station's ID without the role must not enable experiment-only behavior")
	}
}
