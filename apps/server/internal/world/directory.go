package world

// DirectoryEntry is what the dialing directory shows about one host. It is
// derived from the host definition, so it is as immutable as the definition.
type DirectoryEntry struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Software string `json:"software,omitempty"`
	Phone    string `json:"phone"`
	DialMode string `json:"dialMode"`
	MaxBaud  int    `json:"maxBaud"`
}

// HostDirectoryStore lists the hosts shown in the dialing directory: the hosts
// whose preset says listed: true. A host that is not listed can still be dialed.
type HostDirectoryStore interface {
	ListedHosts() []DirectoryEntry
}

// ListedHosts returns the directory entries in preset key order. The returned
// slice is a copy.
func (s *MemoryStore) ListedHosts() []DirectoryEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]DirectoryEntry(nil), s.directory...)
}
