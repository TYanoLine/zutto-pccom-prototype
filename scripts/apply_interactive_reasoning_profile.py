from pathlib import Path


def replace_once(path: str, old: str, new: str) -> None:
    p = Path(path)
    text = p.read_text()
    if old not in text:
        raise SystemExit(f"pattern not found in {path}: {old[:180]!r}")
    p.write_text(text.replace(old, new, 1))


# Keep existing callers on provider defaults, but allow latency-sensitive callers
# to pin reasoning explicitly.
path = "apps/server/internal/llm/openai_structured_timeline.go"
replace_once(
    path,
    '''func (p StructuredOpenAIProvider) responseTextWithJSONSchema(ctx context.Context, prompt, verbosity string, maxOutputTokens int, schemaName string, schema map[string]any) (responseTextResult, error) {\n''',
    '''func (p StructuredOpenAIProvider) responseTextWithJSONSchema(ctx context.Context, prompt, verbosity string, maxOutputTokens int, schemaName string, schema map[string]any) (responseTextResult, error) {\n\treturn p.responseTextWithJSONSchemaReasoning(ctx, prompt, verbosity, maxOutputTokens, schemaName, schema, "")\n}\n\nfunc (p StructuredOpenAIProvider) responseTextWithJSONSchemaReasoning(ctx context.Context, prompt, verbosity string, maxOutputTokens int, schemaName string, schema map[string]any, reasoningEffort string) (responseTextResult, error) {\n''',
)
replace_once(
    path,
    '''\tpayload := map[string]any{\n\t\t"model": p.Model,\n\t\t"input": prompt,\n\t\t"text": map[string]any{\n\t\t\t"verbosity": verbosity,\n\t\t\t"format": map[string]any{\n\t\t\t\t"type":   "json_schema",\n\t\t\t\t"name":   schemaName,\n\t\t\t\t"strict": true,\n\t\t\t\t"schema": schema,\n\t\t\t},\n\t\t},\n\t\t"max_output_tokens": maxOutputTokens,\n\t}\n''',
    '''\tpayload := map[string]any{\n\t\t"model": p.Model,\n\t\t"input": prompt,\n\t\t"text": map[string]any{\n\t\t\t"verbosity": verbosity,\n\t\t\t"format": map[string]any{\n\t\t\t\t"type":   "json_schema",\n\t\t\t\t"name":   schemaName,\n\t\t\t\t"strict": true,\n\t\t\t\t"schema": schema,\n\t\t\t},\n\t\t},\n\t\t"max_output_tokens": maxOutputTokens,\n\t}\n\tif effort := strings.TrimSpace(reasoningEffort); effort != "" {\n\t\tpayload["reasoning"] = map[string]any{"effort": effort}\n\t}\n''',
)

# Title generation/review: expose interactive variants with explicit reasoning.
path = "apps/server/internal/llm/bbs_title_candidates.go"
replace_once(
    path,
    '''type BBSTitleCandidatePlanner interface {\n\tGenerateBBSTitleCandidates(context.Context, string, string) (BBSTitleCandidates, error)\n\tReviewBBSTitleCandidates(context.Context, BBSTitleReviewRequest) (BBSTitleReview, error)\n}\n''',
    '''type BBSTitleCandidatePlanner interface {\n\tGenerateBBSTitleCandidates(context.Context, string, string) (BBSTitleCandidates, error)\n\tReviewBBSTitleCandidates(context.Context, BBSTitleReviewRequest) (BBSTitleReview, error)\n}\n\n// BBSTitleInteractiveCandidatePlanner is the latency-sensitive profile used only\n// by the ordinary development dial-up UI. The isolated Lab intentionally keeps\n// the normal provider profile so quality evaluation does not silently weaken.\ntype BBSTitleInteractiveCandidatePlanner interface {\n\tGenerateInteractiveBBSTitleCandidates(context.Context, string, string) (BBSTitleCandidates, error)\n\tReviewInteractiveBBSTitleCandidates(context.Context, BBSTitleReviewRequest) (BBSTitleReview, error)\n}\n''',
)
replace_once(
    path,
    '''func (p StructuredOpenAIProvider) GenerateBBSTitleCandidates(ctx context.Context, date, board string) (BBSTitleCandidates, error) {\n\tschema := map[string]any{"type": "object", "properties": map[string]any{"titles": map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "minItems": 20, "maxItems": 20}}, "required": []string{"titles"}, "additionalProperties": false}\n\tresult, err := p.responseTextWithJSONSchema(ctx, titleCandidatePrompt(date, board), "low", 2400, "bbs_title_candidates", schema)\n''',
    '''func (p StructuredOpenAIProvider) GenerateBBSTitleCandidates(ctx context.Context, date, board string) (BBSTitleCandidates, error) {\n\treturn p.generateBBSTitleCandidates(ctx, date, board, "")\n}\n\nfunc (p StructuredOpenAIProvider) GenerateInteractiveBBSTitleCandidates(ctx context.Context, date, board string) (BBSTitleCandidates, error) {\n\treturn p.generateBBSTitleCandidates(ctx, date, board, "none")\n}\n\nfunc (p StructuredOpenAIProvider) generateBBSTitleCandidates(ctx context.Context, date, board, reasoningEffort string) (BBSTitleCandidates, error) {\n\tschema := map[string]any{"type": "object", "properties": map[string]any{"titles": map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "minItems": 20, "maxItems": 20}}, "required": []string{"titles"}, "additionalProperties": false}\n\tresult, err := p.responseTextWithJSONSchemaReasoning(ctx, titleCandidatePrompt(date, board), "low", 2400, "bbs_title_candidates", schema, reasoningEffort)\n''',
)
replace_once(
    path,
    '''func (p StructuredOpenAIProvider) ReviewBBSTitleCandidates(ctx context.Context, req BBSTitleReviewRequest) (BBSTitleReview, error) {\n\tinput, err := json.Marshal(req)\n''',
    '''func (p StructuredOpenAIProvider) ReviewBBSTitleCandidates(ctx context.Context, req BBSTitleReviewRequest) (BBSTitleReview, error) {\n\treturn p.reviewBBSTitleCandidates(ctx, req, "")\n}\n\nfunc (p StructuredOpenAIProvider) ReviewInteractiveBBSTitleCandidates(ctx context.Context, req BBSTitleReviewRequest) (BBSTitleReview, error) {\n\treturn p.reviewBBSTitleCandidates(ctx, req, "low")\n}\n\nfunc (p StructuredOpenAIProvider) reviewBBSTitleCandidates(ctx context.Context, req BBSTitleReviewRequest, reasoningEffort string) (BBSTitleReview, error) {\n\tinput, err := json.Marshal(req)\n''',
)
replace_once(
    path,
    '''\tresult, err := p.responseTextWithJSONSchema(ctx, prompt, "low", 7000, "bbs_title_review", schema)\n''',
    '''\tresult, err := p.responseTextWithJSONSchemaReasoning(ctx, prompt, "low", 7000, "bbs_title_review", schema, reasoningEffort)\n''',
)

# Era classification: same independent validator, but interactive classification is
# intentionally a no-reasoning structured classification request.
path = "apps/server/internal/llm/bbs_title_era.go"
replace_once(
    path,
    '''type BBSTitleEraValidator interface {\n\tValidateBBSTitleEra(context.Context, BBSTitleEraRequest) (BBSTitleEraReview, error)\n}\n''',
    '''type BBSTitleEraValidator interface {\n\tValidateBBSTitleEra(context.Context, BBSTitleEraRequest) (BBSTitleEraReview, error)\n}\n\n// BBSTitleInteractiveEraValidator keeps Era validation independent while using\n// a latency-oriented inference profile in the interactive development terminal.\ntype BBSTitleInteractiveEraValidator interface {\n\tValidateInteractiveBBSTitleEra(context.Context, BBSTitleEraRequest) (BBSTitleEraReview, error)\n}\n''',
)
replace_once(
    path,
    '''func (p StructuredOpenAIProvider) ValidateBBSTitleEra(ctx context.Context, req BBSTitleEraRequest) (BBSTitleEraReview, error) {\n\tinput, err := json.Marshal(req)\n''',
    '''func (p StructuredOpenAIProvider) ValidateBBSTitleEra(ctx context.Context, req BBSTitleEraRequest) (BBSTitleEraReview, error) {\n\treturn p.validateBBSTitleEra(ctx, req, "")\n}\n\nfunc (p StructuredOpenAIProvider) ValidateInteractiveBBSTitleEra(ctx context.Context, req BBSTitleEraRequest) (BBSTitleEraReview, error) {\n\treturn p.validateBBSTitleEra(ctx, req, "none")\n}\n\nfunc (p StructuredOpenAIProvider) validateBBSTitleEra(ctx context.Context, req BBSTitleEraRequest, reasoningEffort string) (BBSTitleEraReview, error) {\n\tinput, err := json.Marshal(req)\n''',
)
replace_once(
    path,
    '''\tresult, err := p.responseTextWithJSONSchema(ctx, prompt, "low", 3600, "bbs_title_era_review", schema)\n''',
    '''\tresult, err := p.responseTextWithJSONSchemaReasoning(ctx, prompt, "low", 3600, "bbs_title_era_review", schema, reasoningEffort)\n''',
)

# Select the interactive interfaces only for the ordinary development dial path.
path = "apps/server/internal/worldrepo/materialization_title_first.go"
replace_once(
    path,
    '''\teraValidator, ok := m.Renderer.(llm.BBSTitleEraValidator)\n\tif !ok {\n\t\treturn nil, fmt.Errorf("renderer does not support title era validation")\n\t}\n\tdetailPlanner, ok := m.Renderer.(llm.BBSTitleArticleDetailPlanner)\n''',
    '''\teraValidator, ok := m.Renderer.(llm.BBSTitleEraValidator)\n\tif !ok {\n\t\treturn nil, fmt.Errorf("renderer does not support title era validation")\n\t}\n\tif developmentInteractiveTitleFirstEnabled(r) {\n\t\tplanner = developmentInteractiveTitlePlanner(m.Renderer, planner)\n\t\teraValidator = developmentInteractiveTitleEraValidator(m.Renderer, eraValidator)\n\t}\n\tdetailPlanner, ok := m.Renderer.(llm.BBSTitleArticleDetailPlanner)\n''',
)

print("interactive reasoning profile applied")
