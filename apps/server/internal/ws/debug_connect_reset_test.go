package ws

import (
	"testing"

	"zutto-pccom/apps/server/internal/hostcatalog"
	"zutto-pccom/apps/server/internal/world"
)

type debugPrepareStore struct {
	*world.MemoryStore
	calls   int
	removed int
	kept    int
	ok      bool
}

func (s *debugPrepareStore) PrepareDebugBBSConnection(world.Host) (int, int, bool) {
	s.calls++
	return s.removed, s.kept, s.ok
}

func TestPrepareDebugBBSConnectionRunsOnlyForHostsThatOptIn(t *testing.T) {
	base := world.NewMemoryStore()
	// The HAKATA preset opts in with debug.reset_articles_on_connect.
	optedIn, err := base.HostByPhone("0920000196")
	if err != nil {
		t.Fatal(err)
	}
	if !optedIn.Debug.ResetArticlesOnConnect {
		t.Fatal("test fixture: the HAKATA preset must enable debug.reset_articles_on_connect")
	}
	other := world.Host{ID: "other-test", Phone: "0450000010", Name: "OTHER TEST BBS"}
	store := &debugPrepareStore{MemoryStore: base, removed: 5, kept: 920, ok: true}

	if !prepareDebugBBSConnection(store, other) {
		t.Fatal("host without the flag was rejected")
	}
	if store.calls != 0 {
		t.Fatalf("host without the flag triggered reset: calls=%d", store.calls)
	}
	if !prepareDebugBBSConnection(store, optedIn) {
		t.Fatal("debug reset was rejected for an opted-in host")
	}
	if store.calls != 1 {
		t.Fatalf("opted-in reset calls=%d, want 1", store.calls)
	}
}

// The reset follows the explicit flag, not the role, a phone number or an ID:
// HAKATA's own identity without the flag and a host with only the experiment
// role must not trigger it, and any other host with the flag must.
func TestPrepareDebugBBSConnectionFollowsTheFlag(t *testing.T) {
	base := world.NewMemoryStore()
	store := &debugPrepareStore{MemoryStore: base, ok: true}

	sameIdentityNoFlag := world.Host{ID: "hakata-canal-net", Phone: "0920000196"}
	if !prepareDebugBBSConnection(store, sameIdentityNoFlag) || store.calls != 0 {
		t.Fatalf("a host without the flag was reset: calls=%d", store.calls)
	}
	roleOnly := world.Host{ID: "role-only", Phone: "0450000011", Role: hostcatalog.RoleExperiment}
	if !prepareDebugBBSConnection(store, roleOnly) || store.calls != 0 {
		t.Fatalf("the experiment role alone triggered a reset: calls=%d", store.calls)
	}
	otherWithFlag := world.Host{
		ID:    "another-station",
		Phone: "0450000010",
		Debug: hostcatalog.DebugFlags{ResetArticlesOnConnect: true},
	}
	if !prepareDebugBBSConnection(store, otherWithFlag) || store.calls != 1 {
		t.Fatalf("a host with the flag was not reset: calls=%d", store.calls)
	}
}

func TestPrepareDebugBBSConnectionFailsClosedWhenResetUnavailable(t *testing.T) {
	base := world.NewMemoryStore()
	optedIn, err := base.HostByPhone("0920000196")
	if err != nil {
		t.Fatal(err)
	}
	store := &debugPrepareStore{MemoryStore: base, ok: false}
	if prepareDebugBBSConnection(store, optedIn) {
		t.Fatal("an opted-in connection should fail closed when reset cannot complete")
	}
}
