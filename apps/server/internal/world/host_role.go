package world

import (
	"sort"

	"zutto-pccom/apps/server/internal/hostcatalog"
)

// IsExperiment reports whether the host has the experiment role in its preset.
//
// The role is a label. Debug and experimental behavior (the CONNECT-time article
// reset, the generation trace, the generated-content log, the debug HTTP
// endpoints, the title-led prose experiment, debug snapshots) is switched on
// per host by Host.Debug and Host.Generation, never by the role, a phone number
// or an ID. The role is only a label: the resident population is defined by the
// preset's population block, not by the role.
func (h Host) IsExperiment() bool { return h.Role == hostcatalog.RoleExperiment }

// ExperimentHosts returns every host with the experiment role, ordered by phone.
func (s *MemoryStore) ExperimentHosts() []Host {
	return s.hostsWhere(func(h Host) bool { return h.IsExperiment() })
}

// SnapshotHosts returns every host with debug.snapshot, ordered by phone.
func (s *MemoryStore) SnapshotHosts() []Host {
	return s.hostsWhere(func(h Host) bool { return h.Debug.Snapshot })
}

// DebugEndpointHosts returns every host with debug.http_endpoints, ordered by
// phone.
func (s *MemoryStore) DebugEndpointHosts() []Host {
	return s.hostsWhere(func(h Host) bool { return h.Debug.HTTPEndpoints })
}

func (s *MemoryStore) hostsWhere(keep func(Host) bool) []Host {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []Host
	for _, h := range s.hosts {
		if keep(h) {
			out = append(out, h)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Phone < out[j].Phone })
	return out
}
