package world

// HAKATA CANAL NET keeps a small code-owned cast skeleton during the generator
// evaluation phase. These are identities only, not article/content seeds.
// Concrete interests, life facts, opinions and writing details should still come
// from the world/persona layers when they become relevant.
var hakataExperimentHandles = []string{
	"MARI", "YUKI", "NORI", "KAZU", "TAKU",
	"NEKO", "KEN", "MAKO", "TOMO", "AKI",
	"RYO", "HIRO", "SACHI", "JUN", "MIDNIGHT",
}

func ensureHakataExperimentCastLocked(s *MemoryStore, hostID string) int {
	if s == nil || hostID == "" {
		return 0
	}
	existing := make(map[string]bool, len(s.memberships[hostID]))
	for _, id := range s.memberships[hostID] {
		existing[id] = true
	}
	added := 0
	for _, handle := range hakataExperimentHandles {
		id := "hakata-" + normalizeFixtureID(handle)
		if _, ok := s.personas[id]; !ok {
			s.personas[id] = Persona{ID: id, Handle: handle}
		}
		if existing[id] {
			continue
		}
		s.memberships[hostID] = append(s.memberships[hostID], id)
		existing[id] = true
		added++
	}
	return added
}

// EnsureHakataExperimentCast restores only the station's resident identity
// skeletons after loading an older snapshot. It never creates posts.
func (s *MemoryStore) EnsureHakataExperimentCast(phone string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	host, ok := s.hosts[phone]
	if !ok || host.ID == "" {
		return 0
	}
	return ensureHakataExperimentCastLocked(s, host.ID)
}

func normalizeFixtureID(handle string) string {
	out := make([]byte, 0, len(handle))
	for i := 0; i < len(handle); i++ {
		c := handle[i]
		if c >= 'A' && c <= 'Z' {
			c = c - 'A' + 'a'
		}
		if (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '-' {
			out = append(out, c)
		}
	}
	return string(out)
}
