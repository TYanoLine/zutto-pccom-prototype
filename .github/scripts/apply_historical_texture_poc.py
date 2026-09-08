from pathlib import Path


def replace_once(path, old, new):
    p = Path(path)
    s = p.read_text()
    if old not in s:
        raise SystemExit(f'missing target in {path}: {old[:120]!r}')
    p.write_text(s.replace(old, new, 1))

replace_once(
    'apps/server/internal/llm/provider.go',
    'type BBSWorldSituationProposalRequest struct {\n\tHostName        string\n\tHostRegion      string\n\tHostSoftware    string\n\tWorldDate       string\n\tWindowStart     string\n\tWindowEnd       string\n\tEraRules        string\n\tRecentBBSState  string\n\tEvents          []BBSWorldWindowEvent\n\tAvoidSituations []string\n}',
    'type BBSWorldSituationProposalRequest struct {\n\tHostName        string\n\tHostRegion      string\n\tHostSoftware    string\n\tWorldDate       string\n\tWindowStart     string\n\tWindowEnd       string\n\tEraRules        string\n\tHistoricalFacts []string\n\tRecentBBSState  string\n\tEvents          []BBSWorldWindowEvent\n\tAvoidSituations []string\n}'
)

replace_once(
    'apps/server/internal/worldrepo/llm_materializer.go',
    'type LLMMaterializer struct {\n\tRenderer                    llm.BoardPostRenderer\n\tFallback                    Materializer\n\tHistoricalReferencesEnabled bool\n}',
    'type LLMMaterializer struct {\n\tRenderer                    llm.BoardPostRenderer\n\tFallback                    Materializer\n\tHistoricalReferencesEnabled bool\n\tHistoricalTexture           []string\n}'
)
replace_once(
    'apps/server/internal/worldrepo/llm_materializer.go',
    'func (m LLMMaterializer) historicalFacts(decision worldengine.EvidenceDecision) []string {\n\tif !m.HistoricalReferencesEnabled {\n\t\treturn nil\n\t}\n\treturn usableClaims(decision)\n}\n\nfunc (m LLMMaterializer) eraRules() string {\n\tif m.HistoricalReferencesEnabled {',
    'func (m LLMMaterializer) historicalFacts(decision worldengine.EvidenceDecision) []string {\n\tout := make([]string, 0, len(m.HistoricalTexture)+4)\n\tif m.HistoricalReferencesEnabled {\n\t\tout = append(out, usableClaims(decision)...)\n\t}\n\tfor _, fact := range m.HistoricalTexture {\n\t\tfact = strings.TrimSpace(fact)\n\t\tif fact != "" {\n\t\t\tout = append(out, fact)\n\t\t}\n\t}\n\treturn out\n}\n\nfunc (m LLMMaterializer) eraRules() string {\n\tif m.HistoricalReferencesEnabled || len(m.HistoricalTexture) > 0 {'
)
replace_once(
    'apps/server/internal/worldrepo/llm_materializer.go',
    'return "HISTORICAL_REFERENCES=ON. 世界時刻より未来の知識を使わない。新しい実在の製品名・作品名・サービス名・企業名・人物名・具体的地名・歴史上の出来事やニュースは、supplied historical facts または明示された canonical historical evidence にあるものだけ使用し、モデル記憶から補完しない。局固有の架空設定と史実を混同しない。セーブ、モデム、回線、駅、店、ゲーム、通信ソフト等の一般語彙は自然に使ってよい。\\n" + llm.DiegeticWorldFrame',
    'return "HISTORICAL_REFERENCES=ON. 世界時刻より未来の知識を使わない。新しい実在の製品名・作品名・サービス名・企業名・人物名・具体的地名・歴史上の出来事やニュースは、supplied historical facts / historical texture または明示された canonical historical evidence にあるものだけ使用し、モデル記憶から補完しない。supplied texture は話題リストではなく、その時点の世界に存在してよい背景語彙・参照対象である。必要な場合は曖昧な総称へ逃げず具体名を自然に使ってよいが、無関係な投稿へ時代小道具として挿入しない。局固有の架空設定と史実を混同しない。セーブ、モデム、回線、駅、店、ゲーム、通信ソフト等の一般語彙は自然に使ってよい。\\n" + llm.DiegeticWorldFrame'
)

replace_once(
    'apps/server/internal/worldrepo/materialization_batch_situation.go',
    '\t\tEraRules:        m.eraRules(),\n\t\tRecentBBSState:  recentBBS,',
    '\t\tEraRules:        m.eraRules(),\n\t\tHistoricalFacts: append([]string(nil), m.HistoricalTexture...),\n\t\tRecentBBSState:  recentBBS,'
)

replace_once(
    'apps/server/internal/llm/openai_world_situation_proposer.go',
    '\tavoidJSON, err := json.Marshal(req.AvoidSituations)\n\tif err != nil {\n\t\treturn BBSWorldSituationProposalDraft{}, err\n\t}\n\trecent := strings.TrimSpace(req.RecentBBSState)',
    '\tavoidJSON, err := json.Marshal(req.AvoidSituations)\n\tif err != nil {\n\t\treturn BBSWorldSituationProposalDraft{}, err\n\t}\n\thistoricalFacts := "(none supplied)"\n\tif len(req.HistoricalFacts) > 0 {\n\t\thistoricalFacts = "- " + strings.Join(req.HistoricalFacts, "\\n- ")\n\t}\n\trecent := strings.TrimSpace(req.RecentBBSState)'
)
replace_once(
    'apps/server/internal/llm/openai_world_situation_proposer.go',
    '- Do not introduce new real product/work/service/company/person/place/event names unless supplied canonical evidence explicitly contains them.\n- Keep events mundane.',
    '- Do not introduce new real product/work/service/company/person/place/event names unless SUPPLIED HISTORICAL TEXTURE below explicitly permits them or supplied canonical evidence contains them.\n- SUPPLIED HISTORICAL TEXTURE is permission and contemporaneous background, not a topic menu. Use a supplied concrete name when it genuinely sharpens an already-plausible situation; do not mechanically insert names into every root. Some roots may naturally use one supplied referent and many may use none. Never extrapolate release dates, prices, specifications, plot/results, popularity rankings or other facts that the supplied line does not state.\n- Keep events mundane.'
)
replace_once(
    'apps/server/internal/llm/openai_world_situation_proposer.go',
    'EARLIER CANONICAL BBS STATE:\n%s\n\nWORLD-SELECTED ROOT SLOTS (JSON):',
    'SUPPLIED HISTORICAL TEXTURE — ALLOWED CONTEMPORARY REFERENTS, NOT REQUIRED TOPICS:\n%s\n\nEARLIER CANONICAL BBS STATE:\n%s\n\nWORLD-SELECTED ROOT SLOTS (JSON):'
)
replace_once(
    'apps/server/internal/llm/openai_world_situation_proposer.go',
    'Never mention AI, prompts, databases, social media, smartphones or anything after the world date.`, withDiegeticWorldFrame(req.EraRules), req.WorldDate, req.WindowStart, req.WindowEnd, req.HostName, req.HostRegion, req.HostSoftware, recent, string(eventsJSON), string(avoidJSON))',
    'Never mention AI, prompts, databases, social media, smartphones or anything after the world date.`, withDiegeticWorldFrame(req.EraRules), req.WorldDate, req.WindowStart, req.WindowEnd, req.HostName, req.HostRegion, req.HostSoftware, historicalFacts, recent, string(eventsJSON), string(avoidJSON))'
)

replace_once(
    'apps/server/cmd/server/materialization_lab_fresh.go',
    '\tSituationMode       string                        `json:"situation_mode,omitempty"`\n\tBoardCount',
    '\tSituationMode       string                        `json:"situation_mode,omitempty"`\n\tHistoricalTexture   string                        `json:"historical_texture,omitempty"`\n\tBoardCount'
)
insert_after = '''func normalizeFreshSituationMode(raw string) (string, bool) {
\tmode := strings.ToLower(strings.TrimSpace(raw))
\tswitch mode {
\tcase "", "facets":
\t\treturn "facets", true
\tcase "facetless":
\t\treturn "facetless", true
\tcase "batch":
\t\treturn "batch", true
\tdefault:
\t\treturn "", false
\t}
}
'''
replace_once(
    'apps/server/cmd/server/materialization_lab_fresh.go',
    insert_after,
    insert_after + '''
func normalizeFreshHistoricalTexture(raw string) (string, bool) {
\tmode := strings.ToLower(strings.TrimSpace(raw))
\tswitch mode {
\tcase "", "off":
\t\treturn "off", true
\tcase "1996-08-curated":
\t\treturn mode, true
\tdefault:
\t\treturn "", false
\t}
}
'''
)
replace_once(
    'apps/server/cmd/server/materialization_lab_fresh.go',
    '\t\treturn\n\t}\n\tboardCount, ok := freshIntParam(r.URL.Query().Get("board_count"), 3, 3, 6)',
    '\t\treturn\n\t}\n\thistoricalTexture, ok := normalizeFreshHistoricalTexture(r.URL.Query().Get("historical_texture"))\n\tif !ok {\n\t\tw.WriteHeader(http.StatusBadRequest)\n\t\t_ = json.NewEncoder(w).Encode(map[string]string{"error": "historical_texture must be off or 1996-08-curated"})\n\t\treturn\n\t}\n\tboardCount, ok := freshIntParam(r.URL.Query().Get("board_count"), 3, 3, 6)'
)
replace_once(
    'apps/server/cmd/server/materialization_lab_fresh.go',
    'job := &materializationFreshJob{ID: id, Status: "queued", Phone: phone, SituationMode: situationMode, BoardCount: boardCount, ShellLimit: shellLimit, Boards: freshScaleBoards(boardCount), CreatedAt: time.Now().UTC()}',
    'job := &materializationFreshJob{ID: id, Status: "queued", Phone: phone, SituationMode: situationMode, HistoricalTexture: historicalTexture, BoardCount: boardCount, ShellLimit: shellLimit, Boards: freshScaleBoards(boardCount), CreatedAt: time.Now().UTC()}'
)
replace_once(
    'apps/server/cmd/server/materialization_lab_fresh.go',
    '\trepo := worldrepo.New(base, l.engine, l.materializer, l.worldDate)\n\trepo.EnableDevelopmentConversationViewPoC()',
    '''\tlabMaterializer := l.materializer
\tif facts := freshHistoricalTextureFacts(job.HistoricalTexture); len(facts) > 0 {
\t\tswitch m := l.materializer.(type) {
\t\tcase worldrepo.LLMMaterializer:
\t\t\tm.HistoricalReferencesEnabled = true
\t\t\tm.HistoricalTexture = append([]string(nil), facts...)
\t\t\tlabMaterializer = m
\t\tcase *worldrepo.LLMMaterializer:
\t\t\tclone := *m
\t\t\tclone.HistoricalReferencesEnabled = true
\t\t\tclone.HistoricalTexture = append([]string(nil), facts...)
\t\t\tlabMaterializer = &clone
\t\tdefault:
\t\t\tfinishFreshError(id, fmt.Errorf("historical texture requires LLMMaterializer, got %T", l.materializer))
\t\t\treturn
\t\t}
\t}
\trepo := worldrepo.New(base, l.engine, labMaterializer, l.worldDate)
\trepo.EnableDevelopmentConversationViewPoC()'''
)

replace_once(
    'apps/server/cmd/server/materialization_lab_fresh_archive.go',
    '\tSituationMode string    `json:"situation_mode,omitempty"`\n\tBoardCount',
    '\tSituationMode string    `json:"situation_mode,omitempty"`\n\tHistoricalTexture string   `json:"historical_texture,omitempty"`\n\tBoardCount'
)
replace_once(
    'apps/server/cmd/server/materialization_lab_fresh_archive.go',
    "  situation_mode text NOT NULL DEFAULT '',\n  board_count integer NOT NULL DEFAULT 0,",
    "  situation_mode text NOT NULL DEFAULT '',\n  historical_texture text NOT NULL DEFAULT '',\n  board_count integer NOT NULL DEFAULT 0,"
)
replace_once(
    'apps/server/cmd/server/materialization_lab_fresh_archive.go',
    'CREATE INDEX IF NOT EXISTS development_materialization_fresh_archives_created_idx',
    "ALTER TABLE development_materialization_fresh_archives ADD COLUMN IF NOT EXISTS historical_texture text NOT NULL DEFAULT '';\nCREATE INDEX IF NOT EXISTS development_materialization_fresh_archives_created_idx"
)
replace_once(
    'apps/server/cmd/server/materialization_lab_fresh_archive.go',
    '  (id,status,situation_mode,board_count,shell_limit,post_count,body_count,failures,created_at,finished_at,payload)\nVALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11::jsonb)',
    '  (id,status,situation_mode,historical_texture,board_count,shell_limit,post_count,body_count,failures,created_at,finished_at,payload)\nVALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12::jsonb)'
)
replace_once(
    'apps/server/cmd/server/materialization_lab_fresh_archive.go',
    '  situation_mode=EXCLUDED.situation_mode,\n  board_count=EXCLUDED.board_count,',
    '  situation_mode=EXCLUDED.situation_mode,\n  historical_texture=EXCLUDED.historical_texture,\n  board_count=EXCLUDED.board_count,'
)
replace_once(
    'apps/server/cmd/server/materialization_lab_fresh_archive.go',
    '`, job.ID, job.Status, job.SituationMode, job.BoardCount, job.ShellLimit, job.PostCount, job.BodyCount, job.Failures, job.CreatedAt, job.FinishedAt, payload)',
    '`, job.ID, job.Status, job.SituationMode, job.HistoricalTexture, job.BoardCount, job.ShellLimit, job.PostCount, job.BodyCount, job.Failures, job.CreatedAt, job.FinishedAt, payload)'
)
replace_once(
    'apps/server/cmd/server/materialization_lab_fresh_archive.go',
    'SELECT id,status,situation_mode,board_count,shell_limit,post_count,body_count,failures,created_at,finished_at',
    'SELECT id,status,situation_mode,historical_texture,board_count,shell_limit,post_count,body_count,failures,created_at,finished_at'
)
replace_once(
    'apps/server/cmd/server/materialization_lab_fresh_archive.go',
    '&item.ID, &item.Status, &item.SituationMode, &item.BoardCount, &item.ShellLimit, &item.PostCount, &item.BodyCount, &item.Failures, &item.CreatedAt, &item.FinishedAt',
    '&item.ID, &item.Status, &item.SituationMode, &item.HistoricalTexture, &item.BoardCount, &item.ShellLimit, &item.PostCount, &item.BodyCount, &item.Failures, &item.CreatedAt, &item.FinishedAt'
)
replace_once(
    'apps/server/cmd/server/materialization_lab_fresh_archive.go',
    '\t\tID: job.ID, Status: job.Status, SituationMode: job.SituationMode,\n\t\tBoardCount:',
    '\t\tID: job.ID, Status: job.Status, SituationMode: job.SituationMode, HistoricalTexture: job.HistoricalTexture,\n\t\tBoardCount:'
)

replace_once(
    'apps/web/src/poc/MaterializationLabViewerPage.tsx',
    "id: string; status: string; situation_mode?: string; board_count?: number; shell_limit?: number;",
    "id: string; status: string; situation_mode?: string; historical_texture?: string; board_count?: number; shell_limit?: number;"
)
replace_once(
    'apps/web/src/poc/MaterializationLabViewerPage.tsx',
    "type JobSummary = Pick<Job, 'id'|'status'|'situation_mode'|'board_count'|'shell_limit'|'created_at'|'finished_at'|'post_count'|'body_count'|'failures'>;",
    "type JobSummary = Pick<Job, 'id'|'status'|'situation_mode'|'historical_texture'|'board_count'|'shell_limit'|'created_at'|'finished_at'|'post_count'|'body_count'|'failures'>;"
)
replace_once(
    'apps/web/src/poc/MaterializationLabViewerPage.tsx',
    "{summaries.map(s=><option key={s.id} value={s.id}>{fmt(s.finished_at || s.created_at)} · {s.situation_mode || '-'} · {s.post_count || 0}件 · {s.id}</option>)}",
    "{summaries.map(s=><option key={s.id} value={s.id}>{fmt(s.finished_at || s.created_at)} · {s.situation_mode || '-'} · texture:{s.historical_texture || 'off'} · {s.post_count || 0}件 · {s.id}</option>)}"
)
replace_once(
    'apps/web/src/poc/MaterializationLabViewerPage.tsx',
    "<span>MODE <b>{job.situation_mode || '-'}</b></span><span>BOARDS",
    "<span>MODE <b>{job.situation_mode || '-'}</b></span><span>TEXTURE <b>{job.historical_texture || 'off'}</b></span><span>BOARDS"
)

p = Path('docs/MATERIALIZATION_LAB.md')
s = p.read_text()
s = s.replace('`phone`, `situation_mode=facets|facetless|batch`, `board_count=3..6`, `shell_limit=1..10`', '`phone`, `situation_mode=facets|facetless|batch`, `historical_texture=off|1996-08-curated`, `board_count=3..6`, `shell_limit=1..10`')
if '### Historical Texture A/B' not in s:
    s += '''\n\n### Historical Texture A/B\n\n`historical_texture=1996-08-curated` is a fresh-Lab-only experiment. It does not change ordinary runtime or the service-wide `HISTORICAL_REFERENCES_ENABLED` setting. The isolated Lab clones the LLM materializer, supplies a small curated set of contemporary real referents, and allows the batch Situation proposer/article worker to use those names only when they naturally sharpen an already-selected event. The texture is permission/background, not a topic quota. `off` preserves the previous generic-name behavior. Completed jobs archive the texture label so the read-only viewer can compare runs. See `docs/research/HISTORICAL_TEXTURE_POC.md`.\n'''
p.write_text(s)
