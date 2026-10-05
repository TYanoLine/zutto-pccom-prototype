package worldrepo

import (
	"testing"

	"zutto-pccom/apps/server/internal/world"
)

// storeWithoutDetail wraps a store behind the plain world.Store interface, so
// the optional HostDetail capability of the wrapped store is not visible.
type storeWithoutDetail struct{ world.Store }

func TestRepositoryServesHostDetailFromItsBaseStore(t *testing.T) {
	r := &Repository{Base: world.NewMemoryStore()}
	d, ok := r.HostDetail("hakata-canal-net")
	if !ok || d.ErikaK == nil || len(d.ErikaK.Boards) != 29 {
		t.Fatalf("the repository did not pass the host detail through: ok=%v detail=%+v", ok, d)
	}
	if _, ok := r.HostDetail("no-such-host"); ok {
		t.Fatal("an unknown host has detail")
	}
}

func TestRepositoryHasNoHostDetailWhenItsBaseStoreHasNone(t *testing.T) {
	r := &Repository{Base: storeWithoutDetail{world.NewMemoryStore()}}
	if d, ok := r.HostDetail("hakata-canal-net"); ok || d.ErikaK != nil {
		t.Fatalf("a base store without the capability yielded detail: ok=%v detail=%+v", ok, d)
	}
}

func TestRepositoryIsAHostDetailStore(t *testing.T) {
	var _ world.HostDetailStore = &Repository{}
}
