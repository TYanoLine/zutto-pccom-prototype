from pathlib import Path


def replace_once(path: str, old: str, new: str) -> None:
    p = Path(path)
    text = p.read_text()
    if old not in text:
        raise SystemExit(f"pattern not found in {path}: {old[:120]!r}")
    p.write_text(text.replace(old, new, 1))


replace_once(
    "apps/server/internal/worldrepo/materialization_demo_persona.go",
    '''\tif developmentConversationViewPoCEnabled(r) {\n\t\tposts, created := r.materializeConversationWorldWindow(host)\n\t\treturn filterBoard(posts, board.ID), created\n\t}\n''',
    '''\tif developmentConversationViewPoCEnabled(r) {\n\t\tif developmentInteractiveTitleFirstEnabled(r) {\n\t\t\treturn r.materializeInteractiveConversationBoardWindow(host, board)\n\t\t}\n\t\tposts, created := r.materializeConversationWorldWindow(host)\n\t\treturn filterBoard(posts, board.ID), created\n\t}\n''',
)

replace_once(
    "apps/server/internal/worldrepo/materialization_title_era.go",
    '''\t\tcase llm.BBSTitleEraResearch:\n\t\t\trow.EraStatus = "research"\n\t\t\trow.EraReason = d.Reason\n\t\t}\n\t\teligibleTitles = append(eligibleTitles, title)\n''',
    '''\t\tcase llm.BBSTitleEraResearch:\n\t\t\trow.EraStatus = "research"\n\t\t\trow.EraReason = d.Reason\n\t\t\tif developmentInteractiveTitleFirstEnabled(r) {\n\t\t\t\trow.Status = "era_rejected"\n\t\t\t\trow.Reason = "対話UIの一覧生成ではWeb史料確認を同期実行しないため候補外: " + d.Reason\n\t\t\t\tcontinue\n\t\t\t}\n\t\t}\n\t\teligibleTitles = append(eligibleTitles, title)\n''',
)

replace_once(
    "apps/server/internal/worldrepo/materialization_title_first.go",
    '''\tif len(acceptedDetailSeeds) > 0 {\n''',
    '''\tif len(acceptedDetailSeeds) > 0 && !developmentInteractiveTitleFirstEnabled(r) {\n''',
)

replace_once(
    "apps/server/internal/worldrepo/materialization_article_debug.go",
    '''func (r *Repository) materializeArticleBodyOnce(host world.Host, board world.Board, selected world.Post) (world.Post, bool, bool, string) {\n\trenderContext, contextStats := r.materializationRenderContext(host, board, selected)\n''',
    '''func (r *Repository) materializeArticleBodyOnce(host world.Host, board world.Board, selected world.Post) (world.Post, bool, bool, string) {\n\tdetailDiagnostic := ""\n\tif developmentInteractiveTitleFirstEnabled(r) {\n\t\tvar detailErr error\n\t\tselected, detailDiagnostic, detailErr = r.materializeInteractiveTitleArticleDetails(host, board, selected)\n\t\tif detailErr != nil {\n\t\t\treturn selected, true, false, detailDiagnostic\n\t\t}\n\t}\n\trenderContext, contextStats := r.materializationRenderContext(host, board, selected)\n''',
)

replace_once(
    "apps/server/internal/worldrepo/materialization_article_debug.go",
    '''\tdiagnostic := joinDevelopmentDiagnostics(formatGenerationUsage(usage), contextStats.String())\n''',
    '''\tdiagnostic := joinDevelopmentDiagnostics(detailDiagnostic, formatGenerationUsage(usage), contextStats.String())\n''',
)

runtime = "apps/server/internal/hostprogram/materializationdemo/runtime.go"
replace_once(
    runtime,
    '''\tbulkMu  sync.Mutex\n\tbulkJob *bulkBodyJob\n}\n''',
    '''\tbulkMu  sync.Mutex\n\tbulkJob *bulkBodyJob\n\n\tindexMu  sync.Mutex\n\tindexJob *articleIndexJob\n}\n''',
)

replace_once(
    runtime,
    '''type bulkBodyTarget struct {\n\tboard  world.Board\n\tpostID int64\n}\n''',
    '''type articleIndexJob struct {\n\tboardID    string\n\tboardName  string\n\tstate      string\n\tstartedAt  time.Time\n\tfinishedAt time.Time\n\tenvelopes  int\n\tcreated    bool\n\tdiagnostic string\n}\n\ntype bulkBodyTarget struct {\n\tboard  world.Board\n\tpostID int64\n}\n''',
)

replace_once(
    runtime,
    '''\tr.board = boards[n-1]\n\tr.state = "articles"\n\treturn r.renderArticles(true), false\n}\n\nfunc (r *Runtime) renderArticles(showMaterialization bool) string {\n''',
    '''\tr.board = boards[n-1]\n\tr.state = "articles"\n\tif len(runtimeBoardPosts(r.Store.ListPosts(r.Host.ID), r.board.ID)) > 0 {\n\t\treturn r.renderArticles(true), false\n\t}\n\treturn r.startArticleIndexGeneration(), false\n}\n\nfunc runtimeBoardPosts(posts []world.Post, boardID string) []world.Post {\n\tout := make([]world.Post, 0)\n\tfor _, post := range posts {\n\t\tif post.BoardID == boardID {\n\t\t\tout = append(out, post)\n\t\t}\n\t}\n\treturn out\n}\n\nfunc (r *Runtime) articleIndexSnapshot(boardID string) (articleIndexJob, bool) {\n\tr.indexMu.Lock()\n\tdefer r.indexMu.Unlock()\n\tif r.indexJob == nil || r.indexJob.boardID != boardID {\n\t\treturn articleIndexJob{}, false\n\t}\n\treturn *r.indexJob, true\n}\n\nfunc (r *Runtime) startArticleIndexGeneration() string {\n\ts, ok := r.Store.(materializingStore)\n\tif !ok {\n\t\treturn "\\r\\nSTORE ERROR\\r\\n"\n\t}\n\tif len(runtimeBoardPosts(r.Store.ListPosts(r.Host.ID), r.board.ID)) > 0 {\n\t\treturn r.renderArticles(true)\n\t}\n\n\tr.indexMu.Lock()\n\tif r.indexJob != nil && r.indexJob.state == "RUNNING" {\n\t\tjob := *r.indexJob\n\t\tr.indexMu.Unlock()\n\t\treturn formatArticleIndexJob(job)\n\t}\n\tjob := &articleIndexJob{boardID: r.board.ID, boardName: r.board.Name, state: "RUNNING", startedAt: time.Now()}\n\tr.indexJob = job\n\tr.indexMu.Unlock()\n\n\tboard := r.board\n\tgo func() {\n\t\tposts, created := s.MaterializationPersonaArticleHeaders(r.Host, board)\n\t\tdiagnostic := s.MaterializationPlanningDiagnostic(r.Host.ID, board.ID)\n\t\tr.indexMu.Lock()\n\t\tdefer r.indexMu.Unlock()\n\t\tif r.indexJob != job {\n\t\t\treturn\n\t\t}\n\t\tjob.state = "COMPLETED"\n\t\tjob.finishedAt = time.Now()\n\t\tjob.envelopes = len(posts)\n\t\tjob.created = created\n\t\tjob.diagnostic = diagnostic\n\t}()\n\n\treturn formatArticleIndexJob(*job)\n}\n\nfunc formatArticleIndexJob(job articleIndexJob) string {\n\telapsedEnd := time.Now()\n\tif !job.finishedAt.IsZero() {\n\t\telapsedEnd = job.finishedAt\n\t}\n\telapsed := elapsedEnd.Sub(job.startedAt).Round(100 * time.Millisecond)\n\tif elapsed < 0 {\n\t\telapsed = 0\n\t}\n\tif job.state == "RUNNING" {\n\t\treturn fmt.Sprintf("\\r\\n[DEV] ARTICLE INDEX : GENERATING IN BACKGROUND / %s / elapsed=%s\\r\\n[DEV] この端末は待たされません。RETURN または R で再読込、Qで掲示板一覧へ戻れます。\\r\\n\\r\\nMSG No.を選択 / R=再読込 / Q=掲示板一覧 > ", job.boardName, elapsed)\n\t}\n\tline := fmt.Sprintf("\\r\\n[DEV] ARTICLE INDEX : READY / %s / envelopes=%d / elapsed=%s\\r\\n", job.boardName, job.envelopes, elapsed)\n\tif strings.TrimSpace(job.diagnostic) != "" {\n\t\tline += "[DEV] CAUSAL PLAN   : " + job.diagnostic + "\\r\\n"\n\t}\n\treturn line + "RETURN または R で記事一覧を表示 / Q=掲示板一覧 > "\n}\n\nfunc (r *Runtime) renderArticles(showMaterialization bool) string {\n''',
)

replace_once(
    runtime,
    '''\tposts, created := s.MaterializationPersonaArticleHeaders(r.Host, r.board)\n\tstatus := "STORED REUSE"\n\tif created {\n\t\tstatus = "SPARSE CAUSAL ENVELOPES MATERIALIZED + STORED"\n\t}\n''',
    '''\tvar posts []world.Post\n\tcreated := false\n\tif job, exists := r.articleIndexSnapshot(r.board.ID); exists {\n\t\tif job.state == "RUNNING" {\n\t\t\treturn formatArticleIndexJob(job)\n\t\t}\n\t\tposts = runtimeBoardPosts(r.Store.ListPosts(r.Host.ID), r.board.ID)\n\t\tcreated = job.created\n\t} else {\n\t\tposts, created = s.MaterializationPersonaArticleHeaders(r.Host, r.board)\n\t}\n\tstatus := "STORED REUSE"\n\tif created {\n\t\tstatus = "SPARSE CAUSAL ENVELOPES MATERIALIZED + STORED"\n\t}\n''',
)

replace_once(
    runtime,
    '''func (r *Runtime) handleArticles(line string) (string, bool) {\n\tif strings.EqualFold(line, "Q") || line == "/" {\n\t\tr.state = "boards"\n\t\treturn r.renderBoards(), false\n\t}\n\tid, err := strconv.ParseInt(line, 10, 64)\n''',
    '''func (r *Runtime) handleArticles(line string) (string, bool) {\n\tif strings.EqualFold(line, "Q") || line == "/" {\n\t\tr.state = "boards"\n\t\treturn r.renderBoards(), false\n\t}\n\tif strings.TrimSpace(line) == "" || strings.EqualFold(line, "R") || strings.EqualFold(line, "REFRESH") {\n\t\treturn r.renderArticles(false), false\n\t}\n\tif job, exists := r.articleIndexSnapshot(r.board.ID); exists && job.state == "RUNNING" {\n\t\treturn formatArticleIndexJob(job), false\n\t}\n\tid, err := strconv.ParseInt(line, 10, 64)\n''',
)

replace_once(
    runtime,
    '''func (r *Runtime) resetConversation() string {\n\tif r.bulkJobIsRunning() {\n''',
    '''func (r *Runtime) resetConversation() string {\n\tif job, exists := r.articleIndexSnapshot(r.board.ID); exists && job.state == "RUNNING" {\n\t\treturn "\\r\\n[DEV] RESET BLOCKED : article index generation is still running. Wait for READY, then RESET.\\r\\nDEV> "\n\t}\n\tif r.bulkJobIsRunning() {\n''',
)

replace_once(
    runtime,
    '''\tr.board = world.Board{}\n\treturn fmt.Sprintf("\\r\\n[DEV] CONVERSATION RESET''',
    '''\tr.board = world.Board{}\n\tr.indexMu.Lock()\n\tr.indexJob = nil\n\tr.indexMu.Unlock()\n\treturn fmt.Sprintf("\\r\\n[DEV] CONVERSATION RESET''',
)

print("patched interactive title index latency path")
