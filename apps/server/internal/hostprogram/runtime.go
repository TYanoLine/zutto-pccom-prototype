package hostprogram

import (
	"strings"

	"zutto-pccom/apps/server/internal/bbs"
	"zutto-pccom/apps/server/internal/hostprogram/erikak"
	"zutto-pccom/apps/server/internal/world"
)

// Runtime is the deliberately small boundary shared by otherwise independent
// BBS host program implementations. Menu trees, prompts, board semantics and
// state machines belong to each host program, not to this interface.
type Runtime interface {
	Welcome() string
	HandleLine(line string) (output string, disconnect bool)
}

func New(host world.Host, store world.Store) Runtime {
	switch host.SoftwareID {
	case "erika-k":
		return erikak.New(host, store)
	}

	// Compatibility with older fixture data while SoftwareID is rolled out.
	software := strings.ToLower(host.Software)
	if strings.Contains(software, "絵理香k") || strings.Contains(software, "絵里香k") || strings.Contains(software, "erika k") {
		return erikak.New(host, store)
	}
	return bbs.New(host, store)
}
