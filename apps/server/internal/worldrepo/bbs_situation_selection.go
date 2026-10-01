package worldrepo
import (
 "crypto/sha256"
 "encoding/binary"
 "strings"
 "zutto-pccom/apps/server/internal/world"
)
func demoStableUnit(parts ...string) float64 {
	sum := sha256.Sum256([]byte(strings.Join(parts, "\x1f")))
	value := binary.BigEndian.Uint64(sum[:8])
	return float64(value>>11) / float64(uint64(1)<<53)
}


var developmentRootDiscourseModes = []string{
	"share_observation",
	"share_experience",
	"state_opinion",
	"share_tip",
	"ask_peers",
}

// demoSelectRootDiscourseMode is a cheap world-layer decision about what kind of
// conversational act a root post is. It deliberately happens before semantic
// production so the LLM cannot turn every root into an engagement-seeking
// question. Each board rotates through all five modes in a stable order, which
// caps ask_peers at one slot per five consecutive roots while keeping the phase
// different across boards.
func demoSelectRootDiscourseMode(host world.Host, board world.Board, rootOrdinal int) string {
	if len(developmentRootDiscourseModes) == 0 {
		return ""
	}
	if rootOrdinal < 0 {
		rootOrdinal = 0
	}
	offset := int(demoStableUnit(host.ID, board.ID, "root-discourse-mode-v1") * float64(len(developmentRootDiscourseModes)))
	if offset >= len(developmentRootDiscourseModes) {
		offset = len(developmentRootDiscourseModes) - 1
	}
	return developmentRootDiscourseModes[(offset+rootOrdinal)%len(developmentRootDiscourseModes)]
}


