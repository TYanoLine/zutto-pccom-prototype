package worldrepo

import (
	"testing"

	"zutto-pccom/apps/server/internal/world"
)

func TestRepositoryServesTheDirectoryOfItsBaseStore(t *testing.T) {
	r := &Repository{Base: world.NewMemoryStore()}
	got := r.ListedHosts()
	if len(got) != 1 || got[0].ID != "hakata-canal-net" {
		t.Fatalf("directory = %+v", got)
	}
}

func TestRepositoryHasAnEmptyDirectoryWhenItsBaseStoreHasNone(t *testing.T) {
	r := &Repository{Base: storeWithoutDetail{world.NewMemoryStore()}}
	if got := r.ListedHosts(); len(got) != 0 {
		t.Fatalf("a base store without the capability yielded %+v", got)
	}
}

func TestRepositoryIsAHostDirectoryStore(t *testing.T) {
	var _ world.HostDirectoryStore = &Repository{}
}
