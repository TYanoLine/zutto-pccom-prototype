from pathlib import Path

# The rich diagnostic context remains available for stats, but the prose worker
# gets its separate compact context. Change only the second occurrence (the one
# immediately before the empty-body check), not the existing-body diagnostic path.
p = Path("apps/server/internal/worldrepo/materialization_article_debug.go")
s = p.read_text()
old = "\trenderContext, contextStats := r.materializationRenderContext(host, board, selected)\n\tif selected.Body != \"\" {\n"
new = "\t_, contextStats := r.materializationRenderContext(host, board, selected)\n\tif selected.Body != \"\" {\n"
if old not in s:
    raise SystemExit("second render-context pattern not found")
s = s.replace(old, new, 1)
p.write_text(s)

# The article worker does not need the full cross-stage diegetic contract. Keep
# the historical-reference mode/constraint first line and a short inside-the-era
# guardrail; planning stages retain the full rules.
p = Path("apps/server/internal/llm/board_post_prompt.go")
s = p.read_text()
s = s.replace("\teraRules := withDiegeticWorldFrame(req.EraRules)\n", "\teraRules := compactBoardPostEraRules(req.EraRules)\n", 1)
insert = r'''
func compactBoardPostEraRules(raw string) string {
	raw = strings.TrimSpace(raw)
	first := raw
	if i := strings.IndexByte(first, '\n'); i >= 0 {
		first = strings.TrimSpace(first[:i])
	}
	if first == "" {
		first = "世界日付より未来の知識や出来事を使わない。"
	}
	return first + "\n当時の本人として普通に書く。現代から振り返る説明、レトロ・懐古演出、時代解説はしない。"
}

'''
marker = "func validateBoardPostWorkerDraft(req BoardPostRequest, d BoardPostDraft) error {"
if marker not in s:
    raise SystemExit("board worker validator marker not found")
s = s.replace(marker, insert + marker, 1)
p.write_text(s)

# Replace the final Gemini transport test as a whole. It is deliberately last in
# the generated test file, so this is robust against quoting differences.
p = Path("apps/server/internal/llm/board_post_prompt_test.go")
s = p.read_text()
marker = "func TestGeminiProviderUsesInteractionsStructuredOutput(t *testing.T) {"
start = s.find(marker)
if start < 0:
    raise SystemExit("gemini test function not found")
new_test = r'''func TestGeminiProviderUsesInteractionsStructuredOutput(t *testing.T) {
	var got map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("x-goog-api-key") != "secret" {
			t.Errorf("missing gemini key header")
		}
		_ = json.NewDecoder(r.Body).Decode(&got)
		w.Header().Set("Content-Type", "application/json")
		inner, _ := json.Marshal(map[string]any{
			"author": "MARI", "subject": "YMOを聴き直しています",
			"body": "最近またYMOを聴いています。音の重なり方が前より気になります。",
		})
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status": "completed",
			"steps": []any{map[string]any{
				"type": "model_output",
				"content": []any{map[string]any{"type": "text", "text": string(inner)}},
			}},
			"usage": map[string]any{
				"total_input_tokens": 10, "total_output_tokens": 20,
				"total_thought_tokens": 3, "total_tokens": 33,
			},
		})
	}))
	defer srv.Close()

	p := GeminiProvider{APIKey: "secret", Model: "gemini-3.8-flash", Endpoint: srv.URL, Client: srv.Client()}
	draft, err := p.GenerateBoardPost(context.Background(), BoardPostRequest{
		BoardTopic: "音楽", WorldDate: "1996-06-07", AuthorHandle: "MARI",
		CanonicalSubject: "YMOを聴き直しています",
	})
	if err != nil {
		t.Fatal(err)
	}
	if draft.Usage.Model != "gemini-3.8-flash" || draft.Usage.TotalTokens != 33 {
		t.Fatalf("usage=%+v", draft.Usage)
	}
	if got["model"] != "gemini-3.8-flash" {
		t.Fatalf("model=%v", got["model"])
	}
	cfg := got["generation_config"].(map[string]any)
	if cfg["thinking_level"] != "low" {
		t.Fatalf("thinking=%v", cfg)
	}
	rf := got["response_format"].(map[string]any)
	if rf["mime_type"] != "application/json" {
		t.Fatalf("response_format=%v", rf)
	}
}
'''
p.write_text(s[:start] + new_test)

# The previous article-worker test intentionally asserted that the giant
# cross-stage contract was visible to prose. Replace only that expectation block.
p = Path("apps/server/internal/llm/openai_test.go")
s = p.read_text()
start_marker = '\tfor _, want := range []string{\n\t\t"DIEGETIC PRESENT / ERA NORMALITY",'
start = s.find(start_marker)
if start < 0:
    raise SystemExit("old OpenAI worker expectation block not found")
end_marker = '\t}\n}'
end = s.find(end_marker, start)
if end < 0:
    raise SystemExit("old OpenAI worker expectation end not found")
end += len('\t}\n')
replacement = r'''	for _, want := range []string{
		"世界日付は 1996-08-29",
		"everyday_baseline=[自宅のパソコンと通信環境は普段使いの道具]",
		"ヘッダを読み上げない",
		"最初の文から用件そのものに入って",
		"当時の本人として普通に書く",
	} {
		if !strings.Contains(capturedPrompt, want) {
			t.Fatalf("board-post prompt missing worker rule %q:\n%s", want, capturedPrompt)
		}
	}
'''
s = s[:start] + replacement + s[end:]
p.write_text(s)
