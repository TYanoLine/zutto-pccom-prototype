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

type presetData struct {
	hosts       []Host
	populations map[string]hostcatalog.Population
	details     map[string]hostcatalog.PresetDetail
}

var (
	presetDataOnce sync.Once
	presetDataVal  presetData
	presetDataErr  error
)

// presetHosts returns the hosts defined by the embedded YAML presets. The files
// are compiled into the binary and covered by tests, so a broken definition is a
// programming error: it panics instead of letting the process start without its
// hosts.
func presetHosts() []Host {
	return append([]Host(nil), loadPresetData().hosts...)
}

func presetPopulations() map[string]hostcatalog.Population {
	src := loadPresetData().populations
	out := make(map[string]hostcatalog.Population, len(src))
	for key, spec := range src {
		out[key] = spec
	}
	return out
}

func presetDetails() map[string]hostcatalog.PresetDetail {
	src := loadPresetData().details
	out := make(map[string]hostcatalog.PresetDetail, len(src))
	for key, detail := range src {
		out[key] = detail
	}
	return out
}

func loadPresetData() presetData {
	presetDataOnce.Do(func() {
		presets, err := hostcatalog.LoadPresets(hostcatalog.Options{})
		if err != nil {
			presetDataErr = err
			return
		}
		descriptors, err := hostcatalog.Descriptors(presets)
		if err != nil {
			presetDataErr = err
			return
		}
		populations, err := hostcatalog.Populations(presets)
		if err != nil {
			presetDataErr = err
			return
		}
		presetDataVal.details = make(map[string]hostcatalog.PresetDetail, len(presets))
		for _, p := range presets {
			presetDataVal.details[p.Key] = p.Detail
		}
		for _, d := range descriptors {
			presetDataVal.hosts = append(presetDataVal.hosts, HostFromDescriptor(d))
		}
		presetDataVal.populations = populations
	})
	if presetDataErr != nil {
		panic(fmt.Sprintf("world: invalid host presets: %v", presetDataErr))
	}
	return presetDataVal
}
