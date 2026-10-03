package hostcatalog

import (
	"testing"

	"zutto-pccom/apps/server/internal/world"
)

// Migration guard: the YAML presets must describe exactly the hosts that are
// hard-coded in world.NewMemoryStore today. Delete this test together with the
// hard-coded fixtures once the presets are wired in.
func TestPresetsMatchLegacyFixtures(t *testing.T) {
	presets, err := LoadPresets(Options{})
	if err != nil {
		t.Fatal(err)
	}
	descriptors, err := Descriptors(presets)
	if err != nil {
		t.Fatal(err)
	}
	store := world.NewMemoryStore()
	for _, d := range descriptors {
		legacy, err := store.HostByPhone(d.Phone)
		if err != nil {
			t.Errorf("%s: no legacy host for %s: %v", d.Key, d.Phone, err)
			continue
		}
		if got := d.WorldHost(); got != legacy {
			t.Errorf("%s differs from legacy fixture:\n yaml:   %+v\n legacy: %+v", d.Key, got, legacy)
		}
	}
	if len(descriptors) != 5 {
		t.Errorf("expected 5 presets for the 5 legacy fixtures, got %d", len(descriptors))
	}
}

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

func TestWorldHostUsesKeyAsID(t *testing.T) {
	h := validDescriptor().WorldHost()
	if h.ID != "sample-bbs" || h.Software != "ktbbs" || h.SoftwareID != "generic" || h.Region != "東京都渋谷区" {
		t.Fatalf("unexpected world host: %+v", h)
	}
}
