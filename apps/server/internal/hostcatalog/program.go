package hostcatalog

// knownPrograms is a stopgap list of host-program IDs. It is the union of the
// families the world catalog can generate and the IDs the runtime understands.
// It will be replaced by the ProgramSpec registry (single source of truth for
// program IDs, runtime factories and generation weights).
var knownPrograms = map[string]bool{
	"erika-k":   true,
	"turbobbs":  true,
	"generic":   true,
	"ktbbs":     true,
	"big-model": true,
	"mmm":       true,
	"rt-bbs":    true,
	"vs":        true,
	"other":     true,
}

// KnownProgram reports whether id is a recognised host-program ID.
func KnownProgram(id string) bool { return knownPrograms[id] }

// RuntimeSoftwareID maps a program ID to the world.Host.SoftwareID that
// hostprogram.New dispatches on. Programs without a dedicated runtime run on the
// generic BBS runtime, exactly as the existing fixtures do.
func RuntimeSoftwareID(program string) string {
	switch program {
	case "erika-k", "turbobbs":
		return program
	}
	return "generic"
}
