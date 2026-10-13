// Package presence decides, deterministically and without an LLM, which
// residents are on a host's lines at a world time (spec 015). It only computes;
// callers persist what was observed. Every probability, weight and duration in
// this package is a FICTIONAL TUNING VALUE for the simulation, not a
// historical statistic.
package presence

import (
	"hash/fnv"
	"math"
)

// unit returns a stable value in [0, 1) for the given parts. It never consumes
// the population generator's random stream, so adding presence does not change
// persona generation.
func unit(parts ...any) float64 {
	h := fnv.New64a()
	for _, p := range parts {
		switch v := p.(type) {
		case string:
			_, _ = h.Write([]byte(v))
		case int64:
			for i := 0; i < 8; i++ {
				_, _ = h.Write([]byte{byte(v >> (8 * i))})
			}
		case int:
			for i := 0; i < 8; i++ {
				_, _ = h.Write([]byte{byte(int64(v) >> (8 * i))})
			}
		}
		_, _ = h.Write([]byte{0})
	}
	return float64(h.Sum64()>>11) / float64(1<<53)
}

// normal returns a stable standard-normal value (Box-Muller).
func normal(parts ...any) float64 {
	u1 := unit(append(parts, "n1")...)
	u2 := unit(append(parts, "n2")...)
	if u1 < 1e-12 {
		u1 = 1e-12
	}
	return math.Sqrt(-2*math.Log(u1)) * math.Cos(2*math.Pi*u2)
}
