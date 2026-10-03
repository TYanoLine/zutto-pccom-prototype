package worldrepo

import "zutto-pccom/apps/server/internal/world"

// Tests need a quiet generic-runtime host. It is defined here so that no
// production station has to exist for them.
const (
	genericTestPhone  = "0450000010"
	genericTestHostID = "generic-test"
)

func genericTestHost() world.Host {
	return world.Host{ID: genericTestHostID, Phone: genericTestPhone, Name: "GENERIC TEST BBS", Region: "神奈川県", Software: "mmm compatible", SoftwareID: "generic", Lines: 8, Popularity: .05, MaxBaud: 28800, Members: 22, FoundedOn: "1996-05-05", GuestAllowed: true}
}

// newTestStore returns a memory store that also holds genericTestHost.
func newTestStore() *world.MemoryStore {
	s := world.NewMemoryStore()
	s.SaveHost(genericTestHost())
	return s
}
