package world

import "testing"

func TestHakataExperimentInterestsAreSparseAndNotComputerUniversal(t *testing.T) {
	store := NewMemoryStore()
	host, err := store.HostByPhone("0920000196")
	if err != nil {
		t.Fatal(err)
	}
	personas := store.ListHostPersonas(host.ID)
	if len(personas) != host.Members {
		t.Fatalf("personas=%d, want members=%d", len(personas), host.Members)
	}

	counts := map[string]int{}
	totalKeys := 0
	for _, persona := range personas {
		if len(persona.Interests) < 3 {
			t.Fatalf("persona %s has too little routing texture: %+v", persona.Handle, persona.Interests)
		}
		totalKeys += len(persona.Interests)
		for key := range persona.Interests {
			counts[key]++
		}
	}
	for _, key := range []string{"games", "communications", "software", "hardware"} {
		if counts[key] >= len(personas) {
			t.Fatalf("%s is still a universal HAKATA interest", key)
		}
	}
	if counts["daily_life"] == 0 || counts["local"] == 0 || counts["food"] == 0 || counts["shopping"] == 0 {
		t.Fatalf("everyday routing domains are missing: %+v", counts)
	}
	avg := float64(totalKeys) / float64(len(personas))
	if avg >= 9 {
		t.Fatalf("interests are not sparse enough: average keys=%.2f", avg)
	}
}

func TestEnsureHakataExperimentPopulationRefreshesOldBiasedRoutingInterests(t *testing.T) {
	store := NewMemoryStore()
	host, err := store.HostByPhone("0920000196")
	if err != nil {
		t.Fatal(err)
	}
	personas := store.ListHostPersonas(host.ID)
	if len(personas) == 0 {
		t.Fatal("missing HAKATA personas")
	}
	p := personas[0]
	p.Interests = map[string]float64{
		"games": 1, "communications": 1, "software": 1, "hardware": 1,
	}
	store.SavePersona(p)
	store.EnsureHakataExperimentPopulation(host.Phone)

	refreshed, ok := store.PersonaByID(p.ID)
	if !ok {
		t.Fatal("persona disappeared")
	}
	if len(refreshed.Interests) == 4 &&
		refreshed.Interests["games"] == 1 &&
		refreshed.Interests["communications"] == 1 &&
		refreshed.Interests["software"] == 1 &&
		refreshed.Interests["hardware"] == 1 {
		t.Fatalf("old biased routing interests survived refresh: %+v", refreshed.Interests)
	}
}
