package hostcatalog

import "testing"

func TestRuntimeSoftwareID(t *testing.T) {
	for program, want := range map[string]string{
		"erika-k": "erika-k", "turbobbs": "turbobbs",
		"ktbbs": "generic", "mmm": "generic", "big-model": "generic", "other": "generic", "generic": "generic",
	} {
		if got := RuntimeSoftwareID(program); got != want {
			t.Errorf("RuntimeSoftwareID(%q) = %q, want %q", program, got, want)
		}
	}
}

func TestKnownProgramCoversRuntimeIDs(t *testing.T) {
	for _, id := range []string{"erika-k", "turbobbs", "generic"} {
		if !KnownProgram(id) {
			t.Errorf("runtime program %q is not a known program", id)
		}
	}
	if KnownProgram("") || KnownProgram("nope") {
		t.Error("unknown program accepted")
	}
}
