package world

import (
	"fmt"
	"sync"

	"zutto-pccom/apps/server/internal/hostcatalog"
)

// HostFromDescriptor converts a HostDescriptor into the runtime Host.
//
// Host.ID is the descriptor Key. That keeps the IDs the in-memory store, the
// persisted snapshots and the experiment-specific code already use. When hosts
// become per-world, the runtime ID scheme must be decided explicitly: a bare key
// such as "world-001" is not unique across worlds.
func HostFromDescriptor(d hostcatalog.HostDescriptor) Host {
	return Host{
		ID:             d.Key,
		Role:           d.Role,
		Phone:          d.Phone,
		Name:           d.Name,
		Region:         d.Region.String(),
		Software:       d.SoftwareDisplay(),
		SoftwareID:     hostcatalog.RuntimeSoftwareID(d.Program),
		Lines:          d.Lines,
		Popularity:     d.Popularity,
		MaxBaud:        d.MaxBaud,
		Members:        d.Members,
		FoundedOn:      d.FoundedOn,
		ANSI:           d.Traits.ANSI,
		GuestAllowed:   d.Traits.GuestAllowed,
		TelehoFriendly: d.Traits.TelehoFriendly,
		Debug:          d.Debug,
		Generation:     d.GenerationFlags,
	}
}

var (
	presetHostsOnce sync.Once
	presetHostList  []Host
	presetHostErr   error
)

// presetHosts returns the hosts defined by the embedded YAML presets. The files
// are compiled into the binary and covered by tests, so a broken definition is a
// programming error: it panics instead of letting the process start without its
// hosts.
func presetHosts() []Host {
	presetHostsOnce.Do(func() {
		presets, err := hostcatalog.LoadPresets(hostcatalog.Options{})
		if err != nil {
			presetHostErr = err
			return
		}
		descriptors, err := hostcatalog.Descriptors(presets)
		if err != nil {
			presetHostErr = err
			return
		}
		for _, d := range descriptors {
			presetHostList = append(presetHostList, HostFromDescriptor(d))
		}
		if err := checkSingleExperiment(presetHostList); err != nil {
			presetHostErr = err
		}
	})
	if presetHostErr != nil {
		panic(fmt.Sprintf("world: invalid host presets: %v", presetHostErr))
	}
	return append([]Host(nil), presetHostList...)
}
