package llm

// stripUnsupportedSchemaConstraints returns a deep copy of a JSON schema without
// the validation keywords that some structured-output backends reject
// (array size bounds beyond 0/1, numeric bounds). The shape of the schema is
// unchanged, so callers still validate the decoded result themselves.
func stripUnsupportedSchemaConstraints(node any) any {
	switch v := node.(type) {
	case map[string]any:
		out := make(map[string]any, len(v))
		for key, child := range v {
			switch key {
			case "maxItems", "minimum", "maximum", "exclusiveMinimum", "exclusiveMaximum", "multipleOf":
				continue
			case "minItems":
				if n, ok := schemaInt(child); ok && n > 1 {
					continue
				}
			}
			out[key] = stripUnsupportedSchemaConstraints(child)
		}
		return out
	case []any:
		out := make([]any, len(v))
		for i, child := range v {
			out[i] = stripUnsupportedSchemaConstraints(child)
		}
		return out
	case []map[string]any:
		out := make([]any, len(v))
		for i, child := range v {
			out[i] = stripUnsupportedSchemaConstraints(child)
		}
		return out
	default:
		return node
	}
}

func schemaInt(v any) (int, bool) {
	switch n := v.(type) {
	case int:
		return n, true
	case int64:
		return int(n), true
	case float64:
		return int(n), true
	}
	return 0, false
}
