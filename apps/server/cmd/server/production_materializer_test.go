package main

import "testing"

func TestProductionMaterializerDoesNotInjectHistoricalCatalog(t *testing.T) {
	m := newProductionMaterializer(nil)
	if m.CuratedHistoricalReferences || m.HistoricalReferencesEnabled || len(m.HistoricalTexture) > 0 {
		t.Fatalf("live materializer enabled historical injection: %+v", m)
	}
	if !m.ModelHistoricalMemory {
		t.Fatal("model should retain its period knowledge without a supplied topic catalog")
	}
	if !m.ProductionMinimalHistoricalPrompt {
		t.Fatal("live prompting should use a short era context, not the Lab's rule stack")
	}
	if m.PreferConcreteHistoricalNames {
		t.Fatal("live world should not have a proper-noun quota")
	}
}
