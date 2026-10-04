package hostcatalog

import (
	"strings"
	"testing"
)

const samplePopulation = `population:
  seed: 0
  id_prefix: "sample"
  core_handles: ["ALPHA", "BETA"]
  handle_bases: ["BASE"]
  handle_prefixes: ["98"]
`

func TestPopulationParsingAndHandleSlug(t *testing.T) {
	p, err := ParsePreset("sample-bbs.yaml", []byte(samplePreset+samplePopulation), Options{})
	if err != nil {
		t.Fatal(err)
	}
	if p.Population == nil || p.Population.Seed != 0 || p.Population.IDPrefix != "sample" ||
		len(p.Population.CoreHandles) != 2 || p.Population.HandlePrefixes[0] != "98" {
		t.Fatalf("population = %+v", p.Population)
	}
	for input, want := range map[string]string{
		"MARI":     "mari",
		"N88":      "n88",
		"A.B_C-D":  "abc-d",
		"MIDNIGHT": "midnight",
	} {
		if got := HandleSlug(input); got != want {
			t.Errorf("HandleSlug(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestPopulationOmitted(t *testing.T) {
	p, err := ParsePreset("sample-bbs.yaml", []byte(samplePreset), Options{})
	if err != nil {
		t.Fatal(err)
	}
	if p.Population != nil {
		t.Fatalf("population = %+v, want nil", p.Population)
	}
}

func TestPopulationValidation(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want []string
	}{
		{"required", "population:\n  id_prefix: \"Bad Prefix\"\n  core_handles: [\"!!\", \"A.B\", \"AB\", \"a.b\"]\n  handle_bases: [\"\"]\n  handle_prefixes: [\"\"]\n", []string{"population.seed", "population.id_prefix", "population.core_handles[0]", "population.core_handles[3]", "population.handle_bases[0]", "population.handle_prefixes[0]"}},
		{"unknown key", samplePopulation + "  seeds: 1\n", []string{"seeds"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParsePreset("sample-bbs.yaml", []byte(samplePreset+tc.src), Options{})
			if err == nil {
				t.Fatal("expected validation error")
			}
			for _, want := range tc.want {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("error %q does not contain %q", err, want)
				}
			}
		})
	}
}

func TestPopulationsMemberValidation(t *testing.T) {
	seed := int64(1)
	p := Preset{Key: "x", Source: "x.yaml", Members: intPtr(1), Population: &Population{
		Seed: seed, IDPrefix: "x", CoreHandles: []string{"A", "B"},
	}}
	if _, err := Populations([]Preset{p}); err == nil || !strings.Contains(err.Error(), "core_handles") {
		t.Fatalf("core handle overflow: %v", err)
	}
	p.Members = intPtr(2)
	if _, err := Populations([]Preset{p}); err != nil {
		t.Fatal(err)
	}
	p.Members = intPtr(3)
	if _, err := Populations([]Preset{p}); err == nil || !strings.Contains(err.Error(), "handle_bases") {
		t.Fatalf("missing bases: %v", err)
	}
}

func TestPopulationPrefixMustBeUnique(t *testing.T) {
	srcA := strings.Replace(samplePopulation, `id_prefix: "sample"`, `id_prefix: "same"`, 1)
	srcB := strings.Replace(samplePopulation, `id_prefix: "sample"`, `id_prefix: "same"`, 1)
	_, err := LoadPresetsFS(presetFS(map[string]string{
		"a-bbs.yaml": samplePresetWith("a-bbs", "0311111111") + srcA,
		"b-bbs.yaml": samplePresetWith("b-bbs", "0311111112") + srcB,
	}), "presets", Options{})
	if err == nil || !strings.Contains(err.Error(), "population.id_prefix") {
		t.Fatalf("duplicate prefix accepted: %v", err)
	}
}

func TestEmbeddedPopulation(t *testing.T) {
	presets, err := LoadPresets(Options{})
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range presets {
		if p.Key != "hakata-canal-net" {
			continue
		}
		if p.Population == nil || p.Population.Seed != 199608260920 || p.Population.IDPrefix != "hakata" ||
			len(p.Population.CoreHandles) != 15 || p.Population.CoreHandles[0] != "MARI" ||
			p.Population.CoreHandles[14] != "MIDNIGHT" || len(p.Population.HandleBases) != 43 ||
			len(p.Population.HandlePrefixes) != 5 || p.Population.HandlePrefixes[3] != "98" {
			t.Fatalf("unexpected embedded population: %+v", p.Population)
		}
		return
	}
	t.Fatal("hakata preset not found")
}

func intPtr(v int) *int { return &v }
