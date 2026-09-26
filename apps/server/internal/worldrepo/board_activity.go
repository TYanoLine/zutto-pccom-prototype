package worldrepo

import (
	"hash/fnv"
	"math"
	"strings"
	"time"

	"zutto-pccom/apps/server/internal/world"
)

const boardActivitySimulationBasis = "fictional deterministic host-age/member-growth/board-activity heuristic v1"

// BoardActivity returns canonical prose-free activity state for a board. The
// state is computed before title/body materialization and persisted when the
// underlying store supports it.
func (r *Repository) BoardActivity(host world.Host, board world.Board) (world.BoardActivityState, bool) {
	if strings.TrimSpace(host.ID) == "" || strings.TrimSpace(board.ID) == "" {
		return world.BoardActivityState{}, false
	}
	store, ok := r.Base.(world.BoardActivityStateStore)
	if !ok {
		return world.BoardActivityState{}, false
	}
	now := r.currentWorldTime()
	asOfDate := now.Format(time.DateOnly)
	weight := normalizedBoardActivityWeight(board.ActivityWeight)
	replyRate := normalizedBoardReplyRate(board.ReplyRate)

	if existing, found := store.BoardActivityState(host.ID, board.ID); found &&
		existing.AsOfDate == asOfDate &&
		existing.CurrentMembers == host.Members &&
		existing.ActivityWeight == weight &&
		existing.ReplyRate == replyRate {
		return existing, true
	}

	state, ok := planBoardActivity(host, board, now)
	if !ok {
		return world.BoardActivityState{}, false
	}
	store.SaveBoardActivityState(host.ID, state)
	return state, true
}

func planBoardActivity(host world.Host, board world.Board, now time.Time) (world.BoardActivityState, bool) {
	openedOn := strings.TrimSpace(board.OpenedOn)
	if openedOn == "" {
		openedOn = strings.TrimSpace(host.FoundedOn)
	}
	opened, err := time.ParseInLocation(time.DateOnly, openedOn, now.Location())
	if err != nil || !opened.Before(now) || host.Members <= 0 {
		return world.BoardActivityState{}, false
	}

	ageDays := int(now.Sub(opened).Hours()/24) + 1
	if ageDays < 1 {
		ageDays = 1
	}
	weight := normalizedBoardActivityWeight(board.ActivityWeight)
	replyRate := normalizedBoardReplyRate(board.ReplyRate)

	initialMembers := int(math.Round(float64(host.Members) * .025))
	if initialMembers < 3 {
		initialMembers = 3
	}
	if initialMembers > host.Members {
		initialMembers = host.Members
	}

	// The growth exponent is stable world fiction, not a historical statistic.
	// Some stations grow early then flatten; others accelerate later.
	growthExponent := .78 + float64(boardActivityHash(host.ID+"|membership-growth")%58)/100.0
	memberDays := 0.0
	for day := 1; day <= ageDays; day++ {
		progress := float64(day) / float64(ageDays)
		members := float64(initialMembers) +
			float64(host.Members-initialMembers)*math.Pow(progress, growthExponent)
		memberDays += members
	}
	averageMembers := memberDays / float64(ageDays)

	popularity := math.Max(0, math.Min(1, host.Popularity))
	rootRatePerMemberDay := .00045 + .00090*popularity
	boardVariation := .82 + float64(boardActivityHash(host.ID+"|"+board.ID+"|activity")%37)/100.0
	totalRoots := int(math.Round(memberDays * rootRatePerMemberDay * weight * boardVariation))
	if totalRoots < 1 && ageDays >= 30 && weight >= .10 {
		totalRoots = 1
	}

	replyVariation := .88 + float64(boardActivityHash(host.ID+"|"+board.ID+"|reply")%25)/100.0
	totalReplies := int(math.Round(float64(totalRoots) * replyRate * replyVariation))

	capRoots := board.RetainedRootCap
	if capRoots <= 0 {
		capRoots = 60
	}
	retainedRoots := totalRoots
	if retainedRoots > capRoots {
		retainedRoots = capRoots
	}
	retainedReplies := totalReplies
	if totalRoots > 0 && retainedRoots < totalRoots {
		retainedReplies = int(math.Round(float64(totalReplies) * float64(retainedRoots) / float64(totalRoots)))
	}

	retainedSince := opened
	if totalRoots > 0 && retainedRoots < totalRoots {
		fraction := float64(retainedRoots) / float64(totalRoots)
		span := time.Duration(float64(now.Sub(opened)) * fraction)
		retainedSince = now.Add(-span)
	}
	if retainedSince.Before(opened) {
		retainedSince = opened
	}

	lastPostAt := time.Time{}
	totalEvents := totalRoots + totalReplies
	if totalEvents > 0 {
		avgGap := now.Sub(opened) / time.Duration(totalEvents+1)
		maxLag := avgGap * 3
		if maxLag < time.Hour {
			maxLag = time.Hour
		}
		if maxLag > 7*24*time.Hour {
			maxLag = 7 * 24 * time.Hour
		}
		lagUnit := float64(boardActivityHash(host.ID+"|"+board.ID+"|last-post")%1001) / 1000.0
		lastPostAt = now.Add(-time.Duration(lagUnit * float64(maxLag)))
		if lastPostAt.Before(opened) {
			lastPostAt = opened
		}
	}

	return world.BoardActivityState{
		BoardID:         board.ID,
		AsOfDate:        now.Format(time.DateOnly),
		OpenedOn:        openedOn,
		CurrentMembers:  host.Members,
		AverageMembers:  averageMembers,
		TotalRoots:      totalRoots,
		TotalReplies:    totalReplies,
		RetainedRoots:   retainedRoots,
		RetainedReplies: retainedReplies,
		RetainedSince:   retainedSince,
		LastPostAt:      lastPostAt,
		ActivityWeight:  weight,
		ReplyRate:       replyRate,
		SimulationBasis: boardActivitySimulationBasis,
	}, true
}

func normalizedBoardActivityWeight(v float64) float64 {
	if v <= 0 {
		return .55
	}
	return math.Max(.02, math.Min(2.0, v))
}

func normalizedBoardReplyRate(v float64) float64 {
	if v <= 0 {
		return 1.10
	}
	return math.Max(0, math.Min(4.0, v))
}

func boardActivityHash(value string) uint64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(value))
	return h.Sum64()
}
