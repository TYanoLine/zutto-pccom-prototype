package worldrepo

import (
	"zutto-pccom/apps/server/internal/hostcatalog"
	"zutto-pccom/apps/server/internal/world"
)

func (r *Repository) HostDetail(hostID string) (hostcatalog.PresetDetail, bool) {
	store, ok := r.Base.(world.HostDetailStore)
	if !ok {
		return hostcatalog.PresetDetail{}, false
	}
	return store.HostDetail(hostID)
}
