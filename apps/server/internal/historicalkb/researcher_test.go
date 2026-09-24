package historicalkb

import (
	"strings"
	"testing"
)

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


func TestHistoricalResearchPromptKeepsMissingInfoInsideRequestedClaim(t *testing.T) {
	prompt := historicalResearchPrompt(
		"PC-9801",
		"PC-9801が1996-08-26までに日本で存在・利用可能だったか。ProvisionalAnswerはERA_OK:またはERA_NG:で始めること。",
		"1996-08-26",
		"candidate title: PC-9801の起動画面",
	)
	for _, want := range []string{
		"missingInfo contains only unresolved evidence that is necessary to answer the exact Question",
		"Do not put intentionally out-of-scope details in missingInfo",
		"a person's ownership/use/preferences",
		"missingInfo must be an empty array",
		"ERA_OK: or ERA_NG:",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("prompt missing %q:\n%s", want, prompt)
		}
	}
	for _, bad := range []string{
		"Explicitly report what could not be verified",
	} {
		if strings.Contains(prompt, bad) {
			t.Fatalf("prompt retains over-broad missing-info instruction %q:\n%s", bad, prompt)
		}
	}
}
