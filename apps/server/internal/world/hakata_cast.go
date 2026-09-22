package world

import (
	"fmt"
	"math/rand"
	"strings"
)

// HAKATA CANAL NET is currently a generator-evaluation station. Its article
// state is reset between debug calls, but its membership population is stable.
// Keep only cheap identity/activity skeletons here; detailed life facts and prose
// traits remain lazy world state.
var hakataCoreHandles = []string{
	"MARI", "YUKI", "NORI", "KAZU", "TAKU",
	"NEKO", "KEN", "MAKO", "TOMO", "AKI",
	"RYO", "HIRO", "SACHI", "JUN", "MIDNIGHT",
}

var hakataHandleBases = []string{
	"AKI", "AYA", "EMI", "HIDE", "HIRO", "JUN", "KAZU", "KEN", "KOJI", "MAKO",
	"MARI", "MASA", "MIKI", "NAO", "NORI", "REI", "RYO", "SHIN", "TAKA", "TOMO",
	"YUKI", "YUJI", "SATOSHI", "TAKESHI", "KENTA", "MEG", "MAYU", "RINA", "ERI",
	"MINT", "WOLF", "RABBIT", "JOKER", "NOVA", "LUNA", "MARU", "KERO", "N88",
	"V30", "X68", "PC98", "COM", "MODEM",
}

func ensureHakataExperimentPopulationLocked(s *MemoryStore, host Host) int {
	if s == nil || host.ID == "" {
		return 0
	}
	target := host.Members
	if target <= 0 {
		target = 326
	}

	existingMembership := make(map[string]bool, len(s.memberships[host.ID]))
	usedHandles := map[string]bool{}
	for _, id := range s.memberships[host.ID] {
		existingMembership[id] = true
		if p, ok := s.personas[id]; ok && strings.TrimSpace(p.Handle) != "" {
			usedHandles[strings.ToLower(strings.TrimSpace(p.Handle))] = true
		}
	}

	rng := rand.New(rand.NewSource(199608260920))
	added := 0
	for i := 0; i < target; i++ {
		id := fmt.Sprintf("hakata-member-%03d", i+1)
		handle := ""
		if i < len(hakataCoreHandles) {
			handle = hakataCoreHandles[i]
			// Preserve the pre-population IDs so older snapshots/facts continue to
			// refer to the same core residents.
			id = "hakata-" + normalizeFixtureID(handle)
		} else {
			handle = nextHakataHandle(rng, usedHandles)
		}
		usedHandles[strings.ToLower(handle)] = true

		if _, ok := s.personas[id]; !ok {
			s.personas[id] = newHakataPersonaSkeleton(rng, id, handle)
		} else {
			// Older debug snapshots contained handle-only skeletons. Enrich only
			// missing activity state without replacing any already materialized
			// persona details.
			p := s.personas[id]
			if strings.TrimSpace(p.Handle) == "" {
				p.Handle = handle
			}
			if strings.TrimSpace(p.ActivityPattern) == "" {
				fillHakataActivitySkeleton(rng, &p)
			}
			if len(p.Interests) == 0 {
				p.Interests = randomHakataInterests(rng)
			}
			s.personas[id] = p
		}

		if existingMembership[id] {
			continue
		}
		s.memberships[host.ID] = append(s.memberships[host.ID], id)
		existingMembership[id] = true
		added++
	}
	return added
}

// EnsureHakataExperimentPopulation restores the full station membership count
// declared by Host.Members. It never creates posts or expensive persona details.
func (s *MemoryStore) EnsureHakataExperimentPopulation(phone string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	host, ok := s.hosts[phone]
	if !ok || host.ID == "" {
		return 0
	}
	return ensureHakataExperimentPopulationLocked(s, host)
}

func newHakataPersonaSkeleton(rng *rand.Rand, id, handle string) Persona {
	p := Persona{ID: id, Handle: handle}
	fillHakataActivitySkeleton(rng, &p)
	p.Interests = randomHakataInterests(rng)
	return p
}

func fillHakataActivitySkeleton(rng *rand.Rand, p *Persona) {
	x := rng.Float64()
	switch {
	case x < .10:
		p.ActivityPattern = "regular"
		p.LurkerTendency = .08 + .18*rng.Float64()
		p.ReplyTendency = .48 + .35*rng.Float64()
		p.ThreadStartTendency = .20 + .28*rng.Float64()
	case x < .28:
		p.ActivityPattern = "active"
		p.LurkerTendency = .16 + .28*rng.Float64()
		p.ReplyTendency = .38 + .34*rng.Float64()
		p.ThreadStartTendency = .16 + .25*rng.Float64()
	case x < .60:
		p.ActivityPattern = "occasional"
		p.LurkerTendency = .32 + .34*rng.Float64()
		p.ReplyTendency = .25 + .34*rng.Float64()
		p.ThreadStartTendency = .10 + .23*rng.Float64()
	case x < .92:
		p.ActivityPattern = "lurker"
		p.LurkerTendency = .70 + .24*rng.Float64()
		p.ReplyTendency = .08 + .24*rng.Float64()
		p.ThreadStartTendency = .03 + .12*rng.Float64()
	default:
		p.ActivityPattern = "dormant"
		p.LurkerTendency = .82 + .16*rng.Float64()
		p.ReplyTendency = .03 + .12*rng.Float64()
		p.ThreadStartTendency = .01 + .07*rng.Float64()
	}
	p.NewcomerOpenness = .20 + .62*rng.Float64()
	p.Argumentativeness = .04 + .34*rng.Float64()
}

func randomHakataInterests(rng *rand.Rand) map[string]float64 {
	// HAKATA is a mixed local/community station. These are routing affinities,
	// not facts that must be mentioned in posts.
	return map[string]float64{
		"local":          .34 + .58*rng.Float64(),
		"chat":           .28 + .58*rng.Float64(),
		"games":          .12 + .70*rng.Float64(),
		"communications": .14 + .68*rng.Float64(),
		"software":       .12 + .68*rng.Float64(),
		"music":          .08 + .62*rng.Float64(),
		"hardware":       .10 + .66*rng.Float64(),
	}
}

func nextHakataHandle(rng *rand.Rand, used map[string]bool) string {
	for attempt := 0; attempt < 80; attempt++ {
		base := hakataHandleBases[rng.Intn(len(hakataHandleBases))]
		variants := []string{
			base,
			fmt.Sprintf("%s.%c", base, 'A'+rune(rng.Intn(26))),
			fmt.Sprintf("%s-%c", base, 'A'+rune(rng.Intn(26))),
			fmt.Sprintf("%s%02d", base, 1+rng.Intn(99)),
			fmt.Sprintf("%s_%02d", base, 1+rng.Intn(99)),
			fmt.Sprintf("%s-%s", []string{"N88", "V30", "X68", "98", "COM"}[rng.Intn(5)], base),
		}
		for _, candidate := range variants {
			key := strings.ToLower(candidate)
			if !used[key] {
				return candidate
			}
		}
	}
	for n := 1; ; n++ {
		candidate := fmt.Sprintf("USER.%c%03d", 'A'+rune(rng.Intn(26)), n)
		if !used[strings.ToLower(candidate)] {
			return candidate
		}
	}
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
