package erikak

import (
	"zutto-pccom/apps/server/internal/hostprogram/replymodel"
	"zutto-pccom/apps/server/internal/world"
)

// ProjectReply maps a semantic response to Erika-K's parent-message + append
// model. The append has no independent subject in the documented/readback
// representation; the root subject remains the thread heading.
func ProjectReply(source world.Post, _ string) replymodel.Projection {
	return replymodel.Projection{ParentID: source.ID}
}
