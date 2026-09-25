package bbs

import (
	"strings"

	"zutto-pccom/apps/server/internal/hostprogram/replymodel"
	"zutto-pccom/apps/server/internal/world"
)

// ProjectReply belongs to the deliberately generic compatibility runtime, not
// to a historical host-program claim. It retains the prototype's Re:-style
// threaded presentation while historical runtimes provide their own mapping.
func ProjectReply(source world.Post, _ string) replymodel.Projection {
	return replymodel.Projection{
		ParentID: source.ID,
		Subject:  "Re: " + strings.TrimSpace(source.Subject),
	}
}
