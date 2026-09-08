package historicalkb

import "testing"

func TestResearchReasoningConfigUsesLowEffort(t *testing.T) {
	if got := researchReasoningConfig()["effort"]; got != "low" {
		t.Fatalf("reasoning effort=%v want low", got)
	}
}

func TestResearchResultTextConfigUsesStrictJSONSchema(t *testing.T) {
	text := researchResultTextConfig()
	format, ok := text["format"].(map[string]any)
	if !ok {
		t.Fatalf("format=%T want map", text["format"])
	}
	if got := format["type"]; got != "json_schema" {
		t.Fatalf("format.type=%v want json_schema", got)
	}
	if got := format["strict"]; got != true {
		t.Fatalf("format.strict=%v want true", got)
	}
	schema, ok := format["schema"].(map[string]any)
	if !ok {
		t.Fatalf("schema=%T want map", format["schema"])
	}
	if got := schema["additionalProperties"]; got != false {
		t.Fatalf("additionalProperties=%v want false", got)
	}
	properties, ok := schema["properties"].(map[string]any)
	if !ok {
		t.Fatalf("properties=%T want map", schema["properties"])
	}
	for _, key := range []string{"summary", "provisionalAnswer", "missingInfo", "confidence"} {
		if _, ok := properties[key]; !ok {
			t.Fatalf("missing schema property %q", key)
		}
	}
}
