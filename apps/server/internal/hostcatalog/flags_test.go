package hostcatalog

import "testing"

func TestPresetFlagsDefaultToOff(t *testing.T) {
	p, err := ParsePreset("sample-bbs.yaml", []byte(samplePreset), Options{})
	if err != nil {
		t.Fatal(err)
	}
	if p.Debug != (DebugFlags{}) || p.GenerationFlags != (GenerationFlags{}) {
		t.Fatalf("a preset without flag blocks must have every flag off: %+v %+v", p.Debug, p.GenerationFlags)
	}
	d, err := p.Descriptor()
	if err != nil {
		t.Fatal(err)
	}
	if d.Debug != (DebugFlags{}) || d.GenerationFlags != (GenerationFlags{}) {
		t.Fatalf("descriptor flags must default to off: %+v %+v", d.Debug, d.GenerationFlags)
	}
}

func TestPresetFlagsAreParsedIntoTheDescriptor(t *testing.T) {
	src := samplePreset +
		"debug:\n" +
		"  reset_articles_on_connect: true\n" +
		"  generation_trace: true\n" +
		"  content_log: true\n" +
		"  http_endpoints: true\n" +
		"  snapshot: true\n" +
		"generation:\n" +
		"  freeform_body: true\n"
	p, err := ParsePreset("sample-bbs.yaml", []byte(src), Options{})
	if err != nil {
		t.Fatal(err)
	}
	d, err := p.Descriptor()
	if err != nil {
		t.Fatal(err)
	}
	wantDebug := DebugFlags{ResetArticlesOnConnect: true, GenerationTrace: true, ContentLog: true, HTTPEndpoints: true, Snapshot: true}
	if d.Debug != wantDebug || !d.GenerationFlags.FreeformBody {
		t.Fatalf("flags not carried into the descriptor: %+v %+v", d.Debug, d.GenerationFlags)
	}
}

// Each flag is independent: turning one on does not turn another on.
func TestPresetFlagsAreIndependent(t *testing.T) {
	for name, tc := range map[string]struct {
		block string
		want  DebugFlags
	}{
		"reset on connect": {"debug:\n  reset_articles_on_connect: true\n", DebugFlags{ResetArticlesOnConnect: true}},
		"generation trace": {"debug:\n  generation_trace: true\n", DebugFlags{GenerationTrace: true}},
		"content log":      {"debug:\n  content_log: true\n", DebugFlags{ContentLog: true}},
		"http endpoints":   {"debug:\n  http_endpoints: true\n", DebugFlags{HTTPEndpoints: true}},
		"snapshot":         {"debug:\n  snapshot: true\n", DebugFlags{Snapshot: true}},
	} {
		p, err := ParsePreset("sample-bbs.yaml", []byte(samplePreset+tc.block), Options{})
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if p.Debug != tc.want || p.GenerationFlags != (GenerationFlags{}) {
			t.Errorf("%s: got %+v %+v, want only %+v", name, p.Debug, p.GenerationFlags, tc.want)
		}
	}
}

// Decoding is strict, so a typo in a flag name never silently leaves it off.
func TestPresetFlagTyposAreRejected(t *testing.T) {
	for name, block := range map[string]string{
		"debug":      "debug:\n  snapshots: true\n",
		"generation": "generation:\n  freeform: true\n",
	} {
		_, err := ParsePreset("sample-bbs.yaml", []byte(samplePreset+block), Options{})
		if err == nil {
			t.Errorf("%s: an unknown flag was accepted", name)
		}
	}
}

// The embedded presets: HAKATA opts in to its evaluation behavior explicitly,
// and no other preset opts in to anything.
func TestEmbeddedPresetFlags(t *testing.T) {
	presets, err := LoadPresets(Options{})
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, p := range presets {
		seen[p.Key] = true
		if p.Key == "hakata-canal-net" {
			want := DebugFlags{ResetArticlesOnConnect: true, GenerationTrace: true, ContentLog: true, HTTPEndpoints: true, Snapshot: true}
			if p.Debug != want || !p.GenerationFlags.FreeformBody {
				t.Errorf("hakata-canal-net must opt in to its evaluation behavior: %+v %+v", p.Debug, p.GenerationFlags)
			}
			continue
		}
		if p.Debug != (DebugFlags{}) || p.GenerationFlags != (GenerationFlags{}) {
			t.Errorf("%s: a preset must not opt in to debug behavior unless it needs it: %+v %+v", p.Key, p.Debug, p.GenerationFlags)
		}
	}
	if !seen["hakata-canal-net"] {
		t.Error("hakata-canal-net preset is missing")
	}
}
