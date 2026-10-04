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

// experimentTestHost is a host with every evaluation flag on. Its ID is
// deliberately not "hakata-canal-net": behavior must follow the flags, not the
// identity.
func experimentTestHost() world.Host {
	return world.Host{
		ID:   "experiment-test",
		Role: hostcatalog.RoleExperiment,
		Debug: hostcatalog.DebugFlags{
			ResetArticlesOnConnect: true,
			GenerationTrace:        true,
			ContentLog:             true,
			Snapshot:               true,
		},
		Generation: hostcatalog.GenerationFlags{FreeformBody: true},
	}
}

// newTestStore returns a memory store that also holds genericTestHost.
func newTestStore() *world.MemoryStore {
	s := world.NewMemoryStore()
	s.SaveHost(genericTestHost())
	return s
}

// The evaluation behaviors (generation trace, generated-content log, prose
// experiment) follow the explicit per-host flags. Neither the real station's
// ID nor the experiment role alone enables them.
func TestEvaluationBehaviorsFollowTheFlagsNotTheRoleOrID(t *testing.T) {
	r := &Repository{freeformBody: true, debugLogGeneratedContent: true}
	if !r.useFreeformBody(experimentTestHost()) || !r.shouldLogGeneratedContent(experimentTestHost()) {
		t.Fatal("a host with the flags on did not get the evaluation behavior")
	}
	lookalike := world.Host{ID: "hakata-canal-net", Phone: "0920000196"}
	if r.useFreeformBody(lookalike) || r.shouldLogGeneratedContent(lookalike) {
		t.Fatal("the station's ID without the flags must not enable evaluation behavior")
	}
	roleOnly := world.Host{ID: "role-only", Role: hostcatalog.RoleExperiment}
	if r.useFreeformBody(roleOnly) || r.shouldLogGeneratedContent(roleOnly) {
		t.Fatal("the experiment role alone must not enable evaluation behavior")
	}
}

// The process-wide switches still apply: a host that opted in is not affected
// while the global switch is off.
func TestEvaluationBehaviorsNeedTheProcessWideSwitch(t *testing.T) {
	r := &Repository{}
	if r.useFreeformBody(experimentTestHost()) || r.shouldLogGeneratedContent(experimentTestHost()) {
		t.Fatal("host flags must not enable evaluation behavior while the global switches are off")
	}
}
