package world

import (
	"fmt"
	"sort"

	"zutto-pccom/apps/server/internal/hostcatalog"
)

// IsExperiment reports whether the host is the evaluation station (role:
// experiment in its preset). Only that host gets the debug auto-reset on connect
// and the debug reset/sample endpoints, the generation trace and generated-content
// log, the title-led prose experiment, durable snapshots, and its resident
// population.
func (h Host) IsExperiment() bool { return h.Role == hostcatalog.RoleExperiment }

// ExperimentHosts returns every host with the experiment role, ordered by phone.
func (s *MemoryStore) ExperimentHosts() []Host {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []Host
	for _, h := range s.hosts {
		if h.IsExperiment() {
			out = append(out, h)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Phone < out[j].Phone })
	return out
}

// checkSingleExperiment enforces that at most one host is the experiment host.
// The resident-population generator derives persona IDs from fixed names
// ("hakata-member-001" ...), so a second experiment host would silently share
// personas with the first.
func checkSingleExperiment(hosts []Host) error {
	var ids []string
	for _, h := range hosts {
		if h.IsExperiment() {
			ids = append(ids, h.ID)
		}
	}
	if len(ids) > 1 {
		return fmt.Errorf("at most one host may have role %q, found %v", hostcatalog.RoleExperiment, ids)
	}
	return nil
}
