package world

import (
	"context"
	"math"
	"time"
)

// Float64Source provides uniform pseudo-random values in [0.0, 1.0).
type Float64Source interface {
	Float64() float64
}

// AppealRoot describes one root post submitted for reply appeal assessment.
type AppealRoot struct {
	ID      int64
	Subject string
	Summary string // Materialized situation summary
}

// ReplyAppealRequest bundles the context for judging thread appeal for a batch of roots.
type ReplyAppealRequest struct {
	Host         string              // Host station display name
	Board        string              // Board display name
	BaseRate     float64             // Board effective reply rate
	Roots        []AppealRoot        // Roots in this batch
	PostedAt     map[int64]time.Time // World timestamp per root
	Members      int                 // Total station members
	ActiveByRoot map[int64]int       // Active personas at root timestamp
	RecentCounts []int               // Recent thread reply counts on this board
}

// ReplyAppealResult contains the qualitative appeal level (0..4) and rationale.
type ReplyAppealResult struct {
	Appeal map[int64]int    // 0..4 (2 is baseline)
	Reason map[int64]string // One sentence rationale for logs
}

// ReplyAppealJudge evaluates how likely threads are to draw replies based on topic, timing, and activity.
type ReplyAppealJudge interface {
	JudgeReplyAppeal(context.Context, ReplyAppealRequest) (ReplyAppealResult, error)
}

// StandardActiveMembers computes the standard active count for a station of given membership size.
var StandardActiveMembers = func(members int) int {
	if members <= 0 {
		return 10
	}
	n := int(math.Round(float64(members) * 0.08))
	if n < 3 {
		return 3
	}
	return n
}

// CurrentStandardActive is the default standard active count used by ReplyCountFor
// when station membership is not specified. When <= 0, time factor scaling is disabled (factor = 1.0).
var CurrentStandardActive = 0

// AppealMultiplier maps appeal level (0..4) to the fictional reply rate multiplier.
var AppealMultiplier = [5]float64{0.2, 0.6, 1.0, 1.7, 2.6}

// ReplyCountFor computes the number of replies a thread draws using the given random source,
// effective board reply rate, appeal level (0..4), and currently active population count.
func ReplyCountFor(rng Float64Source, rate float64, appeal int, active int, maxReplies int) int {
	return ReplyCountForWithStandard(rng, rate, appeal, active, CurrentStandardActive, maxReplies)
}

// ReplyCountForWithStandard computes the number of replies a thread draws, explicitly
// specifying the standard active population to compute the time factor.
func ReplyCountForWithStandard(rng Float64Source, rate float64, appeal int, active int, standardActive int, maxReplies int) int {
	if rate <= 0 || active <= 1 || rng == nil {
		return 0
	}
	if appeal < 0 {
		appeal = 0
	} else if appeal > 4 {
		appeal = 4
	}

	timeFactor := 1.0
	if standardActive > 0 {
		timeFactor = float64(active) / float64(standardActive)
		if timeFactor < 0.3 {
			timeFactor = 0.3
		} else if timeFactor > 1.5 {
			timeFactor = 1.5
		}
	}

	meanReplies := rate * AppealMultiplier[appeal] * timeFactor
	if meanReplies <= 0 {
		return 0
	}

	silent := math.Exp(-0.55 * meanReplies)
	if rng.Float64() < silent {
		return 0
	}

	answeredMean := meanReplies / (1.0 - silent)
	u := rng.Float64()
	if u >= 0.999999 {
		u = 0.999999
	}

	k := 1
	if answeredMean > 1.000001 {
		k = 1 + int(math.Floor(math.Log(1.0-u)/math.Log(1.0-1.0/answeredMean)))
	}

	if maxReplies <= 0 || maxReplies > MaxThreadReplies {
		maxReplies = MaxThreadReplies
	}
	if k > maxReplies {
		k = maxReplies
	}

	maxActive := active - 1
	if maxActive < 0 {
		maxActive = 0
	}
	if k > maxActive {
		k = maxActive
	}

	return k
}
