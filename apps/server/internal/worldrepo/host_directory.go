package worldrepo

import "zutto-pccom/apps/server/internal/world"

// ListedHosts returns the dialing directory of the underlying store, so the
// HTTP layer can read it through the repository. A store without the capability
// has an empty directory.
func (r *Repository) ListedHosts() []world.DirectoryEntry {
	if p, ok := r.Base.(world.HostDirectoryStore); ok {
		return p.ListedHosts()
	}
	return nil
}
