package hostcatalog

import (
	"errors"
	"slices"
	"strings"
	"testing"
	"testing/fstest"
)

func TestEmbeddedPresetsLoadAndValidate(t *testing.T) {
	presets, err := LoadPresets(Options{})
	if err != nil {
		t.Fatal(err)
	}
	var keys []string
	for _, p := range presets {
		keys = append(keys, p.Key)
	}
	wantKeys := []string{"busy-test", "hakata-canal-net", "quiet-test"}
	if !slices.Equal(keys, wantKeys) {
		t.Fatalf("preset keys = %v, want %v", keys, wantKeys)
	}
	descriptors, err := Descriptors(presets)
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range descriptors {
		if err := d.Validate(Options{}); err != nil {
			t.Errorf("%s: %v", d.Key, err)
		}
	}
}

// Policy guard: the real station is listed, fixtures are not.
func TestEmbeddedPresetListingPolicy(t *testing.T) {
	presets, err := LoadPresets(Options{})
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]struct {
		listed bool
		role   string
	}{
		"hakata-canal-net": {true, RoleExperiment},
		"quiet-test":       {false, RoleTest},
		"busy-test":        {false, RoleTest},
	}
	for _, p := range presets {
		w := want[p.Key]
		if p.Listed != w.listed || p.Role != w.role {
			t.Errorf("%s: listed=%v role=%q, want listed=%v role=%q", p.Key, p.Listed, p.Role, w.listed, w.role)
		}
	}
}

func TestEmbeddedPresetDialBehaviors(t *testing.T) {
	presets, _ := LoadPresets(Options{})
	got := map[string]DialBehavior{}
	for _, p := range presets {
		got[p.Key] = p.Dial
	}
	if got["quiet-test"].Kind != DialAlwaysConnect {
		t.Errorf("quiet-test dial = %+v", got["quiet-test"])
	}
	if got["busy-test"] != (DialBehavior{Kind: DialBusyFirstN, BusyFirstN: 5}) {
		t.Errorf("busy-test dial = %+v", got["busy-test"])
	}
	if got["hakata-canal-net"].Kind != DialNormal {
		t.Errorf("hakata dial = %+v", got["hakata-canal-net"])
	}
}

func TestEmbeddedPresetsRespectOptions(t *testing.T) {
	if _, err := LoadPresets(Options{ReservedPhones: map[string]struct{}{"0920000196": {}}}); err == nil || !strings.Contains(err.Error(), "reserved") {
		t.Fatalf("reserved phone not enforced: %v", err)
	}
	if _, err := LoadPresets(Options{Programs: func(string) bool { return false }}); err == nil || !strings.Contains(err.Error(), "unknown host program") {
		t.Fatalf("program checker not used: %v", err)
	}
}

func presetFS(files map[string]string) fstest.MapFS {
	fsys := fstest.MapFS{}
	for name, content := range files {
		fsys["presets/"+name] = &fstest.MapFile{Data: []byte(content)}
	}
	return fsys
}

func samplePresetWith(key, phone string) string {
	s := strings.Replace(samplePreset, "key: sample-bbs", "key: "+key, 1)
	return strings.Replace(s, `"0312345678"`, `"`+phone+`"`, 1)
}

func TestLoadPresetsFSIsAllOrNothing(t *testing.T) {
	cases := []struct {
		name  string
		files map[string]string
		want  string
	}{
		{"duplicate phone", map[string]string{
			"aaa-bbs.yaml": samplePresetWith("aaa-bbs", "0311111111"),
			"bbb-bbs.yaml": samplePresetWith("bbb-bbs", "0311111111"),
		}, "already used by aaa-bbs.yaml"},
		{"yml extension", map[string]string{"aaa-bbs.yml": samplePresetWith("aaa-bbs", "0311111111")}, "use the .yaml extension"},
		{"one bad file fails everything", map[string]string{
			"aaa-bbs.yaml": samplePresetWith("aaa-bbs", "0311111111"),
			"bbb-bbs.yaml": "schema: 9\n",
		}, "bbb-bbs.yaml"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			presets, err := LoadPresetsFS(presetFS(tc.files), "presets", Options{})
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("want error containing %q, got %v", tc.want, err)
			}
			if presets != nil {
				t.Fatalf("partial result returned: %v", presets)
			}
		})
	}
}

func TestLoadPresetsFSIgnoresOtherFilesAndSubdirs(t *testing.T) {
	fsys := presetFS(map[string]string{
		"aaa-bbs.yaml": samplePresetWith("aaa-bbs", "0311111111"),
		"README.md":    "docs",
	})
	fsys["presets/sub/zzz.yaml"] = &fstest.MapFile{Data: []byte("not yaml: [")}
	presets, err := LoadPresetsFS(fsys, "presets", Options{})
	if err != nil || len(presets) != 1 || presets[0].Key != "aaa-bbs" {
		t.Fatalf("presets=%v err=%v", presets, err)
	}
}

func TestLoadPresetsFSEmptyAndMissingDir(t *testing.T) {
	if presets, err := LoadPresetsFS(presetFS(map[string]string{"README.md": "x"}), "presets", Options{}); err != nil || len(presets) != 0 {
		t.Fatalf("empty dir: presets=%v err=%v", presets, err)
	}
	if _, err := LoadPresetsFS(fstest.MapFS{}, "presets", Options{}); err == nil {
		t.Fatal("missing directory must be an error")
	}
}

func TestDescriptorsReportsIncompletePresets(t *testing.T) {
	src := "schema: 1\nkey: partial-bbs\nrevision: 1\nlisted: true\nhost:\n  name: P\n  program: ktbbs\n"
	presets, err := LoadPresetsFS(presetFS(map[string]string{"partial-bbs.yaml": src}), "presets", Options{})
	if err != nil {
		t.Fatal(err)
	}
	_, err = Descriptors(presets)
	var inc *IncompleteError
	if !errors.As(err, &inc) {
		t.Fatalf("want *IncompleteError, got %v", err)
	}
}
