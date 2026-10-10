package world

import (
	"fmt"
	"hash/fnv"
	"math"
)

// MaxThreadReplies bounds one simulated thread. It is a reconstruction guard,
// not a historical limit of any host program.
const MaxThreadReplies = 30

// ThreadReplyCount is the deterministic number of replies the root with the
// given 1-based ordinal on a board eventually drew.
//
// A board's reply rate is its mean replies per root, but a conversation is not
// an even trickle: many roots are never answered, and a few draw long
// exchanges. The count is therefore zero-inflated and heavy-tailed: a root is
// silent with probability exp(-0.55*rate), and an answered root draws a
// geometric count whose mean is chosen so that the overall mean stays near
// rate. This is a fictional simulation heuristic, not a historical statistic.
// No reply is a normal outcome; it is decided here, by the world, never by the
// language model.
func ThreadReplyCount(hostID, boardID string, ordinal int, rate float64) int {
	if rate <= 0 || ordinal < 1 {
		return 0
	}
	silent := math.Exp(-.55 * rate)
	if threadUnit(hostID, boardID, ordinal, "silent") < silent {
		return 0
	}
	mean := rate / (1 - silent) // > 1.8 for every positive rate
	u := threadUnit(hostID, boardID, ordinal, "length")
	k := 1 + int(math.Floor(math.Log(1-u)/math.Log(1-1/mean)))
	if k > MaxThreadReplies {
		k = MaxThreadReplies
	}
	return k
}

func threadUnit(hostID, boardID string, ordinal int, salt string) float64 {
	h := fnv.New64a()
	_, _ = fmt.Fprintf(h, "%s|%s|%d|thread-replies-v1|%s", hostID, boardID, ordinal, salt)
	return (float64(h.Sum64()%1000000) + .5) / 1000000
}
