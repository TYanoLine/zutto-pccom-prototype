package llm

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"reflect"
	"strings"
	"testing"
)

func TestStripUnsupportedSchemaConstraints(t *testing.T) {
	in := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"tags":  map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "maxItems": 2, "minItems": 3},
			"one":   map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "minItems": 1},
			"score": map[string]any{"type": "integer", "minimum": 0, "maximum": 5, "enum": []any{1, 2}},
			"list":  map[string]any{"anyOf": []map[string]any{{"type": "array", "maxItems": 4, "items": map[string]any{"type": "string"}}}},
		},
		"required":             []string{"tags"},
		"additionalProperties": false,
	}
	got := stripUnsupportedSchemaConstraints(in)
	want := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"tags":  map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
			"one":   map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "minItems": 1},
			"score": map[string]any{"type": "integer", "enum": []any{1, 2}},
			"list":  map[string]any{"anyOf": []any{map[string]any{"type": "array", "items": map[string]any{"type": "string"}}}},
		},
		"required":             []string{"tags"},
		"additionalProperties": false,
	}
	if !reflect.DeepEqual(got, want) {
		gj, _ := json.Marshal(got)
		wj, _ := json.Marshal(want)
		t.Fatalf("stripped schema mismatch\n got: %s\nwant: %s", gj, wj)
	}
	if _, ok := in["properties"].(map[string]any)["tags"].(map[string]any)["maxItems"]; !ok {
		t.Fatal("input schema was mutated")
	}
}

func TestSchemaCompatOnlyAppliesWhenEnabled(t *testing.T) {
	for _, compat := range []bool{false, true} {
		var sent string
		provider := StructuredOpenAIProvider{OpenAIProvider: OpenAIProvider{
			Endpoint: "https://test.openai.azure.com", APIKey: "k", Model: "m", SchemaCompat: compat,
			Client: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				raw, _ := io.ReadAll(req.Body)
				sent = string(raw)
				return &http.Response{StatusCode: 200, Status: "200 OK", Header: http.Header{}, Body: io.NopCloser(strings.NewReader(`{"output":[{"content":[{"type":"output_text","text":"{}"}]}]}`))}, nil
			})},
		}}
		schema := map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "maxItems": 2}
		_, _ = provider.responseTextWithJSONSchema(context.Background(), "p", "low", 100, "s", schema)
		if has := strings.Contains(sent, "maxItems"); has == compat {
			t.Fatalf("compat=%v but maxItems present=%v", compat, has)
		}
	}
}
