package ws

import (
	"testing"

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

func TestPrepareDebugBBSConnectionRunsOnlyForHakata(t *testing.T) {
	base := world.NewMemoryStore()
	hakata, err := base.HostByPhone("0920000196")
	if err != nil {
		t.Fatal(err)
	}
	other, err := base.HostByPhone("0470001080")
	if err != nil {
		t.Fatal(err)
	}
	store := &debugPrepareStore{MemoryStore: base, removed: 5, kept: 920, ok: true}

	if !prepareDebugBBSConnection(store, other) {
		t.Fatal("non-debug host was rejected")
	}
	if store.calls != 0 {
		t.Fatalf("non-debug host triggered reset: calls=%d", store.calls)
	}
	if !prepareDebugBBSConnection(store, hakata) {
		t.Fatal("HAKATA debug reset was rejected")
	}
	if store.calls != 1 {
		t.Fatalf("HAKATA reset calls=%d, want 1", store.calls)
	}
}

func TestPrepareDebugBBSConnectionFailsClosedWhenResetUnavailable(t *testing.T) {
	base := world.NewMemoryStore()
	hakata, err := base.HostByPhone("0920000196")
	if err != nil {
		t.Fatal(err)
	}
	store := &debugPrepareStore{MemoryStore: base, ok: false}
	if prepareDebugBBSConnection(store, hakata) {
		t.Fatal("HAKATA connection should fail closed when reset cannot complete")
	}
}
