from pathlib import Path

# The rich diagnostic render context is still built for stats, but prose uses the
# compact worker context. Avoid keeping an unused local.
p = Path("apps/server/internal/worldrepo/materialization_article_debug.go")
s = p.read_text()
s = s.replace("\trenderContext, contextStats := r.materializationRenderContext(host, board, selected)\n", "\t_, contextStats := r.materializationRenderContext(host, board, selected)\n", 1)
p.write_text(s)

# Make the Gemini mock response through json.Encoder so nested JSON text is not
# hand-escaped incorrectly.
p = Path("apps/server/internal/llm/board_post_prompt_test.go")
s = p.read_text()
old = '''        w.Header().Set("Content-Type", "application/json")
        _, _ = w.Write([]byte(`{"status":"completed","steps":[{"type":"model_output","content":[{"type":"text","text":"{\\"author\\":\\"MARI\\",\\"subject\\":\\"YMOを聴き直しています\\",\\"body\\":\\"最近またYMOを聴いています。音の重なり方が前より気になります。\\"}"}]}],"usage":{"total_input_tokens":10,"total_output_tokens":20,"total_thought_tokens":3,"total_tokens":33}}`))
'''
new = '''        w.Header().Set("Content-Type", "application/json")
        inner, _ := json.Marshal(map[string]any{"author":"MARI", "subject":"YMOを聴き直しています", "body":"最近またYMOを聴いています。音の重なり方が前より気になります。"})
        _ = json.NewEncoder(w).Encode(map[string]any{
            "status":"completed",
            "steps":[]any{map[string]any{"type":"model_output", "content":[]any{map[string]any{"type":"text", "text":string(inner)}}}},
            "usage":map[string]any{"total_input_tokens":10,"total_output_tokens":20,"total_thought_tokens":3,"total_tokens":33},
        })
'''
if old not in s:
    raise SystemExit("gemini mock response pattern not found")
s = s.replace(old, new, 1)
p.write_text(s)

# Old contract required internal routing/debug metadata to be visible to the prose
# model. The new contract intentionally excludes it while retaining diegetic rules.
p = Path("apps/server/internal/llm/openai_test.go")
s = p.read_text()
old = '''\tfor _, want := range []string{
\t\t"DIEGETIC PRESENT / ERA NORMALITY",
\t\t"Ordinary baseline conditions stay implicit",
\t\t"internal_routing_domain=...",
\t\t"everyday_baseline=[...]",
\t\t"Never convert them into novelty, rediscovery, nostalgia",
\t\t"Write from inside the actor's present",
\t\t"Never add period props",
\t} {
\t\tif !strings.Contains(capturedPrompt, want) {
\t\t\tt.Fatalf("board-post prompt missing diegetic rule %q:\\n%s", want, capturedPrompt)
\t\t}
\t}
'''
new = '''\tfor _, want := range []string{
\t\t"DIEGETIC PRESENT / ERA NORMALITY",
\t\t"Ordinary baseline conditions stay implicit",
\t\t"everyday_baseline=[自宅のパソコンと通信環境は普段使いの道具]",
\t\t"ヘッダを読み上げない",
\t\t"最初の文から用件そのものに入って",
\t} {
\t\tif !strings.Contains(capturedPrompt, want) {
\t\t\tt.Fatalf("board-post prompt missing worker rule %q:\\n%s", want, capturedPrompt)
\t\t}
\t}
'''
if old not in s:
    raise SystemExit("old openai prompt expectation not found")
s = s.replace(old, new, 1)
p.write_text(s)
