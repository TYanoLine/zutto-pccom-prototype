package hostprogram

import (
	"strings"

	"zutto-pccom/apps/server/internal/bbs"
	"zutto-pccom/apps/server/internal/hostprogram/erikak"
	"zutto-pccom/apps/server/internal/hostprogram/materializationdemo"
	"zutto-pccom/apps/server/internal/world"
)

// Runtime is the deliberately small boundary shared by otherwise independent
// BBS host program implementations. Menu trees, prompts, board semantics and
// state machines belong to each host program, not to this interface.
type Runtime interface {
	Welcome() string
	HandleLine(line string) (output string, disconnect bool)
}

// ObservationCatalog is optional and keeps board topology owned by each host
// program. The world layer may use it after CONNECT to start background catch-up,
// but it never invents a shared cross-host-program menu or board structure.
type ObservationCatalog interface {
	ObservationBoards() []world.Board
}

func ObservationBoards(runtime Runtime) []world.Board {
	if provider, ok := runtime.(ObservationCatalog); ok {
		return append([]world.Board(nil), provider.ObservationBoards()...)
	}
	return nil
}

func New(host world.Host, store world.Store) Runtime {
	switch host.SoftwareID {
	case "erika-k":
		return erikak.New(host, store)
	case "materialization-demo":
		return materializationdemo.New(host, store)
	}

	// Compatibility with older fixture data while SoftwareID is rolled out.
	software := strings.ToLower(host.Software)
	if strings.Contains(software, "絵理香k") || strings.Contains(software, "絵里香k") || strings.Contains(software, "erika k") {
		return erikak.New(host, store)
	}
	return bbs.New(host, store)
}
