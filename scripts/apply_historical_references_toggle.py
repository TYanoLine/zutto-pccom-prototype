from pathlib import Path


def replace_one(path: str, old: str, new: str) -> None:
    p = Path(path)
    text = p.read_text()
    count = text.count(old)
    if count != 1:
        raise SystemExit(f"{path}: replacement count={count}, expected 1 for {old[:120]!r}")
    p.write_text(text.replace(old, new, 1))


# Runtime config: explicit switch, safe default OFF.
replace_one(
    "apps/server/internal/config/config.go",
    'import "os"\n',
    'import (\n\t"os"\n\t"strings"\n)\n',
)
replace_one(
    "apps/server/internal/config/config.go",
    '\tOpenAIModel             string\n\tWorldDate               string\n',
    '\tOpenAIModel                 string\n\tWorldDate                   string\n\tHistoricalReferencesEnabled bool\n',
)
replace_one(
    "apps/server/internal/config/config.go",
    '\t\tOpenAIModel:             env("OPENAI_MODEL", "gpt-5.6-luna"),\n\t\tWorldDate:               env("WORLD_DATE", "1996-08-26"),\n',
    '\t\tOpenAIModel:                 env("OPENAI_MODEL", "gpt-5.6-luna"),\n\t\tWorldDate:                   env("WORLD_DATE", "1996-08-26"),\n\t\tHistoricalReferencesEnabled: envBool("HISTORICAL_REFERENCES_ENABLED", false),\n',
)
replace_one(
    "apps/server/internal/config/config.go",
    'func env(key, fallback string) string {\n\tif v := os.Getenv(key); v != "" { return v }\n\treturn fallback\n}\n',
    '''func env(key, fallback string) string {\n\tif v := os.Getenv(key); v != "" {\n\t\treturn v\n\t}\n\treturn fallback\n}\n\nfunc envBool(key string, fallback bool) bool {\n\tswitch strings.ToLower(strings.TrimSpace(os.Getenv(key))) {\n\tcase "1", "true", "yes", "on":\n\t\treturn true\n\tcase "0", "false", "no", "off":\n\t\treturn false\n\tcase "":\n\t\treturn fallback\n\tdefault:\n\t\treturn fallback\n\t}\n}\n''',
)

# Materializer policy is the single generation boundary used by producer/planner/worker paths.
replace_one(
    "apps/server/internal/worldrepo/llm_materializer.go",
    'type LLMMaterializer struct {\n\tRenderer llm.BoardPostRenderer\n\tFallback Materializer\n}\n',
    'type LLMMaterializer struct {\n\tRenderer                    llm.BoardPostRenderer\n\tFallback                    Materializer\n\tHistoricalReferencesEnabled bool\n}\n',
)
replace_one(
    "apps/server/internal/worldrepo/llm_materializer.go",
    '\tif decision.Level == historicalkb.EvidenceVerified && !decision.Knowledge.CanUse {\n',
    '\tif m.HistoricalReferencesEnabled && decision.Level == historicalkb.EvidenceVerified && !decision.Knowledge.CanUse {\n',
)
replace_one(
    "apps/server/internal/worldrepo/llm_materializer.go",
    '\tfacts := usableClaims(decision)\n',
    '\tfacts := m.historicalFacts(decision)\n',
)
replace_one(
    "apps/server/internal/worldrepo/llm_materializer.go",
    '\t\tEraRules:         "世界時刻より未来の知識を使わない。具体的な歴史事実は supplied historical facts の範囲に限定する。局固有の架空設定と史実を混同しない。\\n" + llm.DiegeticWorldFrame,\n',
    '\t\tEraRules:         m.eraRules(),\n',
)
marker = 'func personaSummary(p world.Persona) string {\n'
p = Path("apps/server/internal/worldrepo/llm_materializer.go")
text = p.read_text()
if text.count(marker) != 1:
    raise SystemExit("llm_materializer.go: personaSummary marker missing/duplicated")
helpers = r'''func (m LLMMaterializer) historicalFacts(decision worldengine.EvidenceDecision) []string {
	if !m.HistoricalReferencesEnabled {
		return nil
	}
	return usableClaims(decision)
}

func (m LLMMaterializer) eraRules() string {
	if m.HistoricalReferencesEnabled {
		return "HISTORICAL_REFERENCES=ON. 世界時刻より未来の知識を使わない。新しい実在の製品名・作品名・サービス名・企業名・人物名・具体的地名・歴史上の出来事やニュースは、supplied historical facts または明示された canonical historical evidence にあるものだけ使用し、モデル記憶から補完しない。局固有の架空設定と史実を混同しない。セーブ、モデム、回線、駅、店、ゲーム、通信ソフト等の一般語彙は自然に使ってよい。\n" + llm.DiegeticWorldFrame
	}
	return "HISTORICAL_REFERENCES=OFF. 世界時刻より未来の知識を使わない。生成する題名・本文・意味計画へ、新しい実在の製品名・作品名・サービス名・企業名・人物名・具体的地名・歴史上の出来事やニュースを導入しない。既に canonical world state として明示的に供給された固有名詞を消去する必要はないが、そこから別の実在情報を連想・補完しない。セーブ、モデム、回線、駅、店、ゲーム、通信ソフト等の一般語彙は自然に使ってよく、具体性まで抽象語に潰さない。\n" + llm.DiegeticWorldFrame
}

'''
p.write_text(text.replace(marker, helpers + marker, 1))

# Both semantic planning paths use the same toggle-aware era rules.
replace_one(
    "apps/server/internal/worldrepo/materialization_world_window_producer.go",
    '\t\tEraRules:       "世界時刻より未来の知識を使わない。外部世界の具体的な歴史事実・製品仕様は根拠なしに確定しない。架空住人の個人的事実と史実を区別する。\\n" + llm.DiegeticWorldFrame,\n',
    '\t\tEraRules:       m.eraRules(),\n',
)
replace_one(
    "apps/server/internal/worldrepo/materialization_intent_planner.go",
    '\t\t\tEraRules:       "世界時刻より未来の知識を使わない。外部世界の具体的な歴史事実・製品仕様は根拠なしに確定しない。架空住人の個人的事実と史実を区別する。\\n" + llm.DiegeticWorldFrame,\n',
    '\t\t\tEraRules:       m.eraRules(),\n',
)

# Main wiring and health expose the active value. The default remains OFF if env is absent.
replace_one(
    "apps/server/cmd/server/main.go",
    '\tpostMaterializer := worldrepo.LLMMaterializer{Renderer: postRenderer, Fallback: worldrepo.FallbackMaterializer{}}\n',
    '\tpostMaterializer := worldrepo.LLMMaterializer{Renderer: postRenderer, Fallback: worldrepo.FallbackMaterializer{}, HistoricalReferencesEnabled: cfg.HistoricalReferencesEnabled}\n',
)
replace_one(
    "apps/server/cmd/server/main.go",
    '"historical_knowledge":historyStore!=nil,"world_repository":true,',
    '"historical_knowledge":historyStore!=nil,"historical_references_enabled":cfg.HistoricalReferencesEnabled,"world_repository":true,',
)

# Env example documents current OFF setting.
replace_one(
    ".env.example",
    'WORLD_DATE=1996-08-26\n\n# Required when creating a new generated world.\n',
    'WORLD_DATE=1996-08-26\n# Allow verified real-world period proper nouns/events to enter generated BBS content.\n# Keep OFF while evaluating fictional-world prose without historical-reference mixing.\nHISTORICAL_REFERENCES_ENABLED=0\n\n# Required when creating a new generated world.\n',
)

# Existing historical-fact test now explicitly exercises ON; add OFF coverage.
replace_one(
    "apps/server/internal/worldrepo/llm_materializer_test.go",
    'm := LLMMaterializer{Renderer: renderer, Fallback: FallbackMaterializer{}}\n\tdecision := worldengine.EvidenceDecision',
    'm := LLMMaterializer{Renderer: renderer, Fallback: FallbackMaterializer{}, HistoricalReferencesEnabled: true}\n\tdecision := worldengine.EvidenceDecision',
)
insert_before = 'func TestLLMMaterializerBindsCanonicalPersonaAndEnvelope(t *testing.T) {\n'
p = Path("apps/server/internal/worldrepo/llm_materializer_test.go")
text = p.read_text()
if text.count(insert_before) != 1:
    raise SystemExit("llm_materializer_test.go: insertion marker missing/duplicated")
off_test = r'''func TestLLMMaterializerHistoricalReferencesOffDropsFactsAndAllowsGenericVocabulary(t *testing.T) {
	renderer := &fakeBoardRenderer{draft: llm.BoardPostDraft{Author: "NORI", Subject: "設定の話", Body: "本文"}}
	m := LLMMaterializer{Renderer: renderer, Fallback: FallbackMaterializer{}, HistoricalReferencesEnabled: false}
	decision := worldengine.EvidenceDecision{Level: historicalkb.EvidenceVerified, Knowledge: historicalkb.KnowledgeResult{CanUse: true, Facts: []historicalkb.HistoricalFact{
		{Claim: "REAL PRODUCT NAME", Status: historicalkb.FactVerified},
	}}}
	_, err := m.GenerateBoardPosts(context.Background(), BoardMaterializationRequest{Host: world.Host{Name: "TEST NET"}, BoardID: "1", BoardTopic: "通信", WorldDate: "1996-08-29"}, decision)
	if err != nil {
		t.Fatal(err)
	}
	if len(renderer.req.HistoricalFacts) != 0 {
		t.Fatalf("historical facts leaked while OFF: %#v", renderer.req.HistoricalFacts)
	}
	for _, want := range []string{"HISTORICAL_REFERENCES=OFF", "新しい実在", "セーブ", "モデム", "一般語彙"} {
		if !strings.Contains(renderer.req.EraRules, want) {
			t.Fatalf("OFF era rules missing %q: %s", want, renderer.req.EraRules)
		}
	}
}

'''
text = text.replace(insert_before, off_test + insert_before, 1)
text = text.replace('"fmt"\n\t"testing"', '"fmt"\n\t"strings"\n\t"testing"', 1)
p.write_text(text)

# World-window Producer receives the same OFF/ON policy.
p = Path("apps/server/internal/worldrepo/materialization_discourse_mode_test.go")
text = p.read_text()
append = r'''

func TestPlanDevelopmentWorldWindowUsesHistoricalReferencePolicy(t *testing.T) {
	host := world.Host{ID: "h", Name: "H", Region: "R", Software: "S"}
	shell := developmentWindowShell{
		eventID: "board-1:event-0001",
		board:   world.Board{ID: "1", Name: "free"},
		shell: developmentTimelineShell{
			index:        1,
			persona:      world.Persona{ID: "p", Handle: "NEKO"},
			action:       "thread_start",
			anchorKey:    "local",
			causeKind:    "recent_salience",
			causeSummary: "a concrete occurrence",
		},
	}

	offRenderer := &discourseCaptureRenderer{}
	if _, err := (LLMMaterializer{Renderer: offRenderer}).PlanDevelopmentWorldWindow(context.Background(), host, "1996-08-26", []developmentWindowShell{shell}, nil, ""); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(offRenderer.req.EraRules, "HISTORICAL_REFERENCES=OFF") {
		t.Fatalf("producer OFF policy missing: %s", offRenderer.req.EraRules)
	}

	onRenderer := &discourseCaptureRenderer{}
	if _, err := (LLMMaterializer{Renderer: onRenderer, HistoricalReferencesEnabled: true}).PlanDevelopmentWorldWindow(context.Background(), host, "1996-08-26", []developmentWindowShell{shell}, nil, ""); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(onRenderer.req.EraRules, "HISTORICAL_REFERENCES=ON") {
		t.Fatalf("producer ON policy missing: %s", onRenderer.req.EraRules)
	}
}
'''
if 'TestPlanDevelopmentWorldWindowUsesHistoricalReferencePolicy' in text:
    raise SystemExit("policy test already present")
text = text.replace('"context"\n\t"testing"', '"context"\n\t"strings"\n\t"testing"', 1)
p.write_text(text + append)

# Config parsing tests.
Path("apps/server/internal/config/config_test.go").write_text(r'''package config

import "testing"

func TestHistoricalReferencesDefaultOff(t *testing.T) {
	t.Setenv("HISTORICAL_REFERENCES_ENABLED", "")
	if Load().HistoricalReferencesEnabled {
		t.Fatal("historical references must default to OFF")
	}
}

func TestHistoricalReferencesCanBeEnabledAndDisabled(t *testing.T) {
	for _, tc := range []struct {
		value string
		want  bool
	}{
		{value: "1", want: true},
		{value: "true", want: true},
		{value: "on", want: true},
		{value: "0", want: false},
		{value: "false", want: false},
		{value: "off", want: false},
	} {
		t.Run(tc.value, func(t *testing.T) {
			t.Setenv("HISTORICAL_REFERENCES_ENABLED", tc.value)
			if got := Load().HistoricalReferencesEnabled; got != tc.want {
				t.Fatalf("value=%q got=%v want=%v", tc.value, got, tc.want)
			}
		})
	}
}
''')

# Architecture doc: toggle controls introduction, not deletion of canonical state.
p = Path("docs/WORLD_WINDOW_PRODUCER.md")
text = p.read_text()
needle = 'When an external identity is not yet supported, the producer should make the episode concrete through actions/state/attempts/relationships while leaving the unsupported external name unspecified. A later structured Life Context / verified referent catalog can provide richer historically grounded objects without weakening this boundary.\n'
if text.count(needle) != 1:
    raise SystemExit("WORLD_WINDOW_PRODUCER.md specificity paragraph missing/duplicated")
addition = needle + r'''

### Historical-reference switch

`HISTORICAL_REFERENCES_ENABLED` controls whether generation may consume verified real-world period references. The safe/default and current setting is `0` (OFF).

- **OFF**: historical evidence is not passed to article rendering, and semantic Producer/Planner/Worker instructions forbid introducing new real product/work/service/company/person/place names or historical events/news from model memory. Period-normal generic vocabulary such as save, modem, line, station, shop, game, or communication software remains allowed; the switch must not force prose into vague abstractions.
- **ON**: verified/supplied historical evidence may be used, but the LLM still may not fill missing real-world facts from model memory.
- The switch does not delete or rewrite a proper noun that is already canonical world state. It governs new historical-reference introduction during generation.

The active value is exposed as `historical_references_enabled` on `/health`.
'''
p.write_text(text.replace(needle, addition, 1))
