package materializationdemo

import (
	"strings"

	"zutto-pccom/apps/server/internal/hostprogram/replymodel"
	"zutto-pccom/apps/server/internal/world"
)

// ProjectReply is development-lab presentation only. The lab intentionally uses
// an explicit Re: thread convention and must not be treated as historical host
// evidence.
func ProjectReply(source world.Post, _ string) replymodel.Projection {
	return replymodel.Projection{
		ParentID: source.ID,
		Subject:  "Re: " + strings.TrimSpace(source.Subject),
	}
}
