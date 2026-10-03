package hostcatalog

import "zutto-pccom/apps/server/internal/world"

// WorldHost converts the descriptor into the runtime world.Host.
//
// world.Host.ID is the descriptor Key. That matches the existing in-memory
// fixtures, whose posts and snapshots are keyed by host ID. When hosts become
// per-world, the runtime ID scheme must be decided explicitly (a bare key such
// as "world-001" is not unique across worlds).
func (d HostDescriptor) WorldHost() world.Host {
	return world.Host{
		ID:             d.Key,
		Phone:          d.Phone,
		Name:           d.Name,
		Region:         d.Region.String(),
		Software:       d.SoftwareDisplay(),
		SoftwareID:     RuntimeSoftwareID(d.Program),
		Lines:          d.Lines,
		Popularity:     d.Popularity,
		MaxBaud:        d.MaxBaud,
		Members:        d.Members,
		FoundedOn:      d.FoundedOn,
		ANSI:           d.Traits.ANSI,
		GuestAllowed:   d.Traits.GuestAllowed,
		TelehoFriendly: d.Traits.TelehoFriendly,
	}
}
