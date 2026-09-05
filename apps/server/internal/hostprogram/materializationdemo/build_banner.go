package materializationdemo

import (
	"fmt"
	"strings"

	"zutto-pccom/apps/server/internal/buildinfo"
)

func runtimeBuildLine() string {
	info := buildinfo.Current()
	branch := strings.TrimSpace(info.Branch)
	if branch == "" {
		branch = "unknown"
	}
	return fmt.Sprintf("[DEV] BUILD         : %s / branch=%s / source=%s / started=%s\r\n", info.ShortCommit, branch, info.Source, info.StartedAt.Format("2006-01-02T15:04:05Z"))
}
