package world

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"testing"
)

var updatePopulationGolden = flag.Bool("update-population-golden", false, "rewrite testdata/population_golden.json from the current implementation")

const (
	populationGoldenPath  = "testdata/population_golden.json"
	populationGoldenPhone = "0920000196"
)

type goldenEntry struct {
	ID     string `json:"id"`
	Handle string `json:"handle"`
	SHA256 string `json:"sha256"`
}

// ensureResidents is the only place this file calls the population generator,
// so a later rename of that API changes exactly this one line.
func ensureResidents(s *MemoryStore, phone string) int {
	return s.EnsurePopulation(phone)
}

func goldenEntries(t *testing.T, personas []Persona) []goldenEntry {
	t.Helper()
	out := make([]goldenEntry, 0, len(personas))
	for _, p := range personas {
		data, err := json.Marshal(p)
		if err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256(data)
		out = append(out, goldenEntry{ID: p.ID, Handle: p.Handle, SHA256: hex.EncodeToString(sum[:])})
	}
	return out
}

func goldenHost(t *testing.T, s *MemoryStore) Host {
	t.Helper()
	h, err := s.HostByPhone(populationGoldenPhone)
	if err != nil {
		t.Fatal(err)
	}
	return h
}

// populationScenarios runs the four paths that matter in production (see plan.md).
func populationScenarios(t *testing.T) map[string][]goldenEntry {
	t.Helper()
	out := map[string][]goldenEntry{}

	// 1 and 2: a fresh process, then a restart over the existing population.
	fresh := NewMemoryStore()
	host := goldenHost(t, fresh)
	out["fresh_constructor"] = goldenEntries(t, fresh.ListHostPersonas(host.ID))
	ensureResidents(fresh, populationGoldenPhone)
	out["after_re_ensure"] = goldenEntries(t, fresh.ListHostPersonas(host.ID))

	// 3: older snapshots held handle-only skeletons.
	handleOnly := NewMemoryStore()
	for _, id := range append([]string(nil), handleOnly.memberships[host.ID]...) {
		p := handleOnly.personas[id]
		handleOnly.personas[id] = Persona{ID: p.ID, Handle: p.Handle}
	}
	ensureResidents(handleOnly, populationGoldenPhone)
	out["from_handle_only"] = goldenEntries(t, handleOnly.ListHostPersonas(host.ID))

	// 4: a snapshot taken when the station had fewer residents.
	partial := NewMemoryStore()
	keep := append([]string(nil), partial.memberships[host.ID][:20]...)
	keepSet := make(map[string]bool, len(keep))
	for _, id := range keep {
		keepSet[id] = true
	}
	for id := range partial.personas {
		if !keepSet[id] {
			delete(partial.personas, id)
		}
	}
	partial.memberships[host.ID] = keep
	ensureResidents(partial, populationGoldenPhone)
	out["after_partial_restore"] = goldenEntries(t, partial.ListHostPersonas(host.ID))
	return out
}

func TestPopulationMatchesGolden(t *testing.T) {
	got := populationScenarios(t)
	if *updatePopulationGolden {
		data, err := json.MarshalIndent(got, "", " ")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Dir(populationGoldenPath), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(populationGoldenPath, append(data, '\n'), 0o644); err != nil {
			t.Fatal(err)
		}
		t.Logf("rewrote %s", populationGoldenPath)
		return
	}

	data, err := os.ReadFile(populationGoldenPath)
	if err != nil {
		t.Fatalf("read golden (generate it once with -update-population-golden on the unchanged implementation): %v", err)
	}
	var want map[string][]goldenEntry
	if err := json.Unmarshal(data, &want); err != nil {
		t.Fatal(err)
	}
	if len(want) != len(got) {
		t.Fatalf("golden has %d scenarios, the test produces %d", len(want), len(got))
	}
	for name, w := range want {
		g, ok := got[name]
		if !ok {
			t.Errorf("scenario %q is missing", name)
			continue
		}
		if len(g) != len(w) {
			t.Errorf("%s: %d residents, golden has %d", name, len(g), len(w))
			continue
		}
		mismatches := 0
		for i := range w {
			if g[i] == w[i] {
				continue
			}
			mismatches++
			if mismatches <= 5 {
				t.Errorf("%s[%d]: got id=%s handle=%s sha=%s, want id=%s handle=%s sha=%s",
					name, i, g[i].ID, g[i].Handle, g[i].SHA256, w[i].ID, w[i].Handle, w[i].SHA256)
			}
		}
		if mismatches > 5 {
			t.Errorf("%s: %d residents differ in total", name, mismatches)
		}
	}
}

// A sanity check that does not depend on the golden file: the shape of the
// population is what the persisted data refers to.
func TestPopulationShape(t *testing.T) {
	for name, entries := range populationScenarios(t) {
		if len(entries) != 326 {
			t.Errorf("%s: %d residents, want 326", name, len(entries))
			continue
		}
		if entries[0].ID != "hakata-mari" || entries[0].Handle != "MARI" {
			t.Errorf("%s: first resident = %+v, want hakata-mari / MARI", name, entries[0])
		}
		if entries[14].ID != "hakata-midnight" {
			t.Errorf("%s: 15th resident id = %q, want hakata-midnight", name, entries[14].ID)
		}
		if entries[15].ID != "hakata-member-016" {
			t.Errorf("%s: 16th resident id = %q, want hakata-member-016", name, entries[15].ID)
		}
	}
}
