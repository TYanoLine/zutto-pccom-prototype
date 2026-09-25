package hostprogram

import (
	"fmt"
	"strings"

	"zutto-pccom/apps/server/internal/bbs"
	"zutto-pccom/apps/server/internal/hostprogram/erikak"
	"zutto-pccom/apps/server/internal/hostprogram/materializationdemo"
	"zutto-pccom/apps/server/internal/hostprogram/replymodel"
	"zutto-pccom/apps/server/internal/hostprogram/turbobbs"
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

// ProjectReply delegates response article representation to the concrete host
// program. The shared world engine supplies only semantic causality; it must not
// impose a cross-host "Re:" convention or a universal parent/child topology.
func ProjectReply(host world.Host, source world.Post, proposedSubject string) (replymodel.Projection, error) {
	switch host.SoftwareID {
	case "erika-k":
		return erikak.ProjectReply(source, proposedSubject), nil
	case "materialization-demo":
		return materializationdemo.ProjectReply(source, proposedSubject), nil
	case "turbobbs":
		return turbobbs.ProjectReply(source, proposedSubject), nil
	case "generic":
		return bbs.ProjectReply(source, proposedSubject), nil
	}

	// Compatibility with older fixture data while SoftwareID is rolled out.
	software := strings.ToLower(host.Software)
	switch {
	case strings.Contains(software, "絵理香k"), strings.Contains(software, "絵里香k"), strings.Contains(software, "erika k"):
		return erikak.ProjectReply(source, proposedSubject), nil
	case strings.Contains(software, "turbobbs"), strings.Contains(software, "turbo bbs"):
		return turbobbs.ProjectReply(source, proposedSubject), nil
	case strings.TrimSpace(host.SoftwareID) == "" && strings.TrimSpace(host.Software) == "":
		return replymodel.Projection{}, fmt.Errorf("host %q has no host-program identity for reply projection", host.ID)
	default:
		return replymodel.Projection{}, fmt.Errorf("host program %q has no registered reply representation", host.SoftwareID)
	}
}

func New(host world.Host, store world.Store) Runtime {
	switch host.SoftwareID {
	case "erika-k":
		return erikak.New(host, store)
	case "materialization-demo":
		return materializationdemo.New(host, store)
	case "turbobbs":
		return turbobbs.New(host, store)
	}

	// Compatibility with older fixture data while SoftwareID is rolled out.
	software := strings.ToLower(host.Software)
	if strings.Contains(software, "絵理香k") || strings.Contains(software, "絵里香k") || strings.Contains(software, "erika k") {
		return erikak.New(host, store)
	}
	if strings.Contains(software, "turbobbs") || strings.Contains(software, "turbo bbs") {
		return turbobbs.New(host, store)
	}
	return bbs.New(host, store)
}
