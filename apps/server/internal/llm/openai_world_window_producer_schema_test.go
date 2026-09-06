package llm

import "testing"

func TestBBSWorldWindowProductionSchemaConstrainsBriefCount(t *testing.T) {
	schema := bbsWorldWindowProductionSchema(15)
	properties, ok := schema["properties"].(map[string]any)
	if !ok { t.Fatal("properties missing") }
	briefs, ok := properties["briefs"].(map[string]any)
	if !ok { t.Fatal("briefs schema missing") }
	if got := briefs["minItems"]; got != 15 { t.Fatalf("minItems=%v want 15", got) }
	if got := briefs["maxItems"]; got != 15 { t.Fatalf("maxItems=%v want 15", got) }
}
