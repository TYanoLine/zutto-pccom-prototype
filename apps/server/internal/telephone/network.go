package telephone

import (
	"hash/fnv"
	"math"
	"time"

	"zutto-pccom/apps/server/internal/world"
	"zutto-pccom/apps/server/internal/worldclock"
)

type Result string

const (
	Connect   Result = "connect"
	Busy      Result = "busy"
	NoCarrier Result = "no_carrier"
	NoAnswer  Result = "no_answer"
)

type DialResult struct {
	Result Result     `json:"result"`
	Host   world.Host `json:"host"`
	Baud   int        `json:"baud"`
	Line   int        `json:"line"`
}

type Network struct {
	store world.Store
	clock worldclock.WorldClock
}

func New(store world.Store, clock worldclock.WorldClock) *Network {
	return &Network{store: store, clock: clock}
}

func (n *Network) Dial(phone string, attempt int) DialResult {
	return n.dialAt(phone, n.clock.Now(), attempt)
}

func (n *Network) dialAt(phone string, at time.Time, attempt int) DialResult {
	host, err := n.store.HostByPhone(phone)
	if err != nil {
		return DialResult{Result: NoAnswer}
	}

	// Hidden prototype fixtures.
	if phone == "0450000001" {
		return DialResult{Result: Connect, Host: host, Baud: host.MaxBaud, Line: 1}
	}
	if phone == "0459999999" {
		if attempt < 5 {
			return DialResult{Result: Busy, Host: host}
		}
		return DialResult{Result: Connect, Host: host, Baud: host.MaxBaud, Line: 1}
	}

	p := busyProbability(host, at)
	// Deterministic-ish per 10-second bucket + attempt; repeated redial changes outcome.
	bucket := at.Unix() / 10
	r := stableUnit(phone, bucket, int64(attempt))
	if r < p {
		return DialResult{Result: Busy, Host: host}
	}

	baud := negotiatedBaud(host.MaxBaud, stableUnit(phone+"baud", bucket, int64(attempt)))
	line := int(math.Mod(float64(bucket+int64(attempt)), float64(max(host.Lines, 1)))) + 1
	return DialResult{Result: Connect, Host: host, Baud: baud, Line: line}
}

func busyProbability(h world.Host, at time.Time) float64 {
	hour := at.Hour()
	factor := 0.35
	switch {
	case hour >= 23 || hour < 2:
		factor = 1.35 // teleho rush / late-night peak
	case hour >= 21 && hour < 23:
		factor = 0.85
	case hour >= 17 && hour < 21:
		factor = 0.60
	case hour >= 2 && hour < 6:
		factor = 0.40
	case hour >= 6 && hour < 9:
		factor = 0.20
	}
	lineRelief := math.Sqrt(float64(max(h.Lines, 1)))
	p := h.Popularity * factor / lineRelief
	return math.Max(0.01, math.Min(0.90, p))
}

func negotiatedBaud(maxBaud int, r float64) int {
	if maxBaud >= 28800 && r > 0.15 {
		return 28800
	}
	if maxBaud >= 14400 && r > 0.08 {
		return 14400
	}
	if maxBaud >= 9600 {
		return 9600
	}
	return 2400
}

func stableUnit(s string, values ...int64) float64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(s))
	for _, v := range values {
		for i := 0; i < 8; i++ {
			_, _ = h.Write([]byte{byte(v >> (8 * i))})
		}
	}
	return float64(h.Sum64()%10000) / 10000.0
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
