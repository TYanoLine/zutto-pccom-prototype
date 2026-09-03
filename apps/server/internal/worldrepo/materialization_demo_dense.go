package worldrepo

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"math"
	"strings"
	"time"

	"zutto-pccom/apps/server/internal/world"
)

type demoPostCandidate struct {
	persona   world.Persona
	createdAt time.Time
}

// MaterializationDenseArticleHeaders is kept as a compatibility alias for older
// development callers. Content generation now lives exclusively in the generic
// persona timeline path; there is no separate dense fixture/topic catalog.
func (r *Repository) MaterializationDenseArticleHeaders(host world.Host, board world.Board) ([]world.Post, bool) {
	return r.MaterializationPersonaArticleHeaders(host, board)
}

func demoActivityProbability(p world.Persona, board world.Board) float64 {
	affinity := demoBoardAffinity(p, board)
	return .03 + (1-p.LurkerTendency)*.12 + affinity*.25 + math.Min(1, p.ReplyTendency+p.ThreadStartTendency)*.05
}

// demoBoardAffinity is behavioral metadata for the development fixture, not a
// content template. It affects whether a persona is likely to appear on a board;
// it never selects a topic, subject, claim, question, or piece of prose.
func demoBoardAffinity(p world.Persona, board world.Board) float64 {
	interest := func(key string) float64 { return p.Interests[key] }
	var values []float64
	switch board.ID {
	case "2":
		values = []float64{interest("modem"), interest("software"), interest("pc98") * .85, interest("bbs") * .65}
	case "3":
		values = []float64{interest("local"), interest("chat") * .55, interest("games") * .20}
	default:
		values = []float64{interest("chat"), interest("music") * .80, interest("games") * .75, interest("local") * .55, interest("bbs") * .30}
	}
	best := 0.0
	for _, value := range values {
		if value > best {
			best = value
		}
	}
	return clamp01(best)
}

func demoShouldReply(host world.Host, board world.Board, p world.Persona, at time.Time, ordinal int) bool {
	chance := .12 + p.ReplyTendency*.48 - p.ThreadStartTendency*.12
	chance = math.Max(.08, math.Min(.62, chance))
	return demoStableUnit(host.ID, board.ID, p.ID, at.Format(time.RFC3339), fmt.Sprintf("reply-%d", ordinal)) < chance
}

// demoChooseRecentReplyRoot selects only reply topology. It deliberately knows
// nothing about a fixed semantic topic catalog. Content semantics are proposed
// later from the actual board/persona/history context.
func demoChooseRecentReplyRoot(host world.Host, board world.Board, p world.Persona, at time.Time, roots []developmentTimelineShell, ordinal int) (int, bool) {
	if len(roots) == 0 {
		return -1, false
	}
	bestScore := -10.0
	bestIndex := -1
	start := len(roots) - 1
	stop := start - 7
	if stop < 0 {
		stop = 0
	}
	for i := start; i >= stop; i-- {
		root := roots[i]
		age := at.Sub(root.createdAt)
		if age < 0 || age > 6*24*time.Hour {
			continue
		}
		score := 1 - age.Hours()/(6*24)
		if root.persona.ID == p.ID {
			score -= .65
		}
		score += demoStableUnit(host.ID, board.ID, p.ID, fmt.Sprint(root.index), fmt.Sprintf("target-%d", ordinal)) * .22
		if score > bestScore {
			bestScore = score
			bestIndex = root.index
		}
	}
	return bestIndex, bestIndex >= 0
}

func demoPersonaTimestampForDay(day time.Time, p world.Persona, hostID, boardID string) time.Time {
	peak, spread := demoPersonaClockProfile(p)
	offsetRoll := demoStableIndex(spread*2+1, hostID, boardID, p.ID, day.Format("2006-01-02"), "hour") - spread
	hour := peak + offsetRoll
	if hour < 0 {
		hour += 24
	}
	if hour >= 24 {
		hour -= 24
	}
	minute := demoStableIndex(60, hostID, boardID, p.ID, day.Format("2006-01-02"), "minute")
	return time.Date(day.Year(), day.Month(), day.Day(), hour, minute, 0, 0, day.Location())
}

func demoPersonaClockProfile(p world.Persona) (peakHour, spreadHours int) {
	// The current Persona model still stores human-readable activity text. Keep
	// this development-only schedule interpretation until persistence gains
	// structured activity windows.
	pattern := p.ActivityPattern
	switch {
	case strings.Contains(pattern, "0:00-03:00"):
		return 1, 2
	case strings.Contains(pattern, "21:00-00:30"):
		return 22, 2
	case strings.Contains(pattern, "22:30-02:30"):
		return 0, 2
	case strings.Contains(pattern, "23:00-02:00"):
		return 0, 2
	case strings.Contains(pattern, "23:30"):
		return 23, 1
	default:
		return 23, 2
	}
}

func demoStableUnit(parts ...string) float64 {
	sum := sha256.Sum256([]byte(strings.Join(parts, "\x1f")))
	value := binary.BigEndian.Uint64(sum[:8])
	return float64(value>>11) / float64(uint64(1)<<53)
}

func demoStableIndex(n int, parts ...string) int {
	if n <= 1 {
		return 0
	}
	return int(demoStableUnit(parts...) * float64(n))
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}
