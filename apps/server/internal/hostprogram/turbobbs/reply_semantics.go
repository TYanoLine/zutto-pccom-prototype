package turbobbs

import (
	"strings"

	"zutto-pccom/apps/server/internal/hostprogram/replymodel"
	"zutto-pccom/apps/server/internal/world"
)

// ProjectReply reflects the inspected TurboBBS snapshot where repto/reply fields
// exist but are dormant. A semantic response is therefore kept as a flat message
// rather than inventing native thread linkage. Every message still has its own
// subject field; the world planner supplies the proposed subject.
//
// This is deliberately narrower than claiming how every TurboBBS-derived system
// handled replies.
func ProjectReply(_ world.Post, proposedSubject string) replymodel.Projection {
	return replymodel.Projection{Subject: strings.TrimSpace(proposedSubject)}
}
