from pathlib import Path
import re


def read(path):
    return Path(path).read_text()


def write(path, text):
    Path(path).write_text(text)


def must_replace(text, old, new, label):
    count = text.count(old)
    if count != 1:
        raise SystemExit(f"{label}: expected exactly one match, got {count}")
    return text.replace(old, new, 1)

# 1) Title/slot review no longer tries to materialize details for all 20 candidates.
p = "apps/server/internal/llm/bbs_title_candidates.go"
s = read(p)
start = s.index("採用候補のdetailsには")
end_marker = "一方、実在物の外部史実にならない記事ローカルな観察・手順・位置関係は具体化して構いません。\n"
end = s.index(end_marker, start) + len(end_marker)
s = s[:start] + "detailsはこの人物・投稿枠検査では具体化しません。互換用フィールドとして、採用・不採用にかかわらず必ず空配列を返してください。本文用の具体ディテールは、Era検証と採用確定後に専用のArticle Detail Materializerが少数の採用記事だけを処理します。\n" + s[end:]
s = must_replace(s,
    'fields["details"] = map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "minItems": 0, "maxItems": 4}',
    'fields["details"] = map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "minItems": 0, "maxItems": 0}',
    "title detail schema")
start = s.index("\t\tif len(d.Details) < 2 || len(d.Details) > 4 {")
end = s.index("\t\tkey := strings.ToLower", start)
s = s[:start] + '\t\tif len(d.Details) != 0 {\n\t\t\treturn fmt.Errorf("title review must not materialize article details")\n\t\t}\n' + s[end:]
write(p, s)

# 2) Dedicated detail pass runs only for accepted+Era-validated titles.
p = "apps/server/internal/worldrepo/materialization_title_first.go"
s = read(p)
old = '''\teraValidator, ok := m.Renderer.(llm.BBSTitleEraValidator)\n\tif !ok {\n\t\treturn nil, fmt.Errorf("renderer does not support title era validation")\n\t}\n'''
new = old + '''\tdetailPlanner, ok := m.Renderer.(llm.BBSTitleArticleDetailPlanner)\n\tif !ok {\n\t\treturn nil, fmt.Errorf("renderer does not support title article details")\n\t}\n'''
s = must_replace(s, old, new, "detail planner assertion")
s = must_replace(s, "context.WithTimeout(context.Background(), 8*time.Minute)", "context.WithTimeout(context.Background(), 12*time.Minute)", "title-first timeout")
old_call = "r.developmentAssignTitleFirstBoard(ctx, host, board, asOf, eligibleTitles, originalCandidates, events[board.ID], planningBBSState(prior, 48), state, offset, boardsRemaining, planner, addUsage, out)"
new_call = "r.developmentAssignTitleFirstBoard(ctx, host, board, asOf, eligibleTitles, originalCandidates, events[board.ID], planningBBSState(prior, 48), state, offset, boardsRemaining, planner, detailPlanner, addUsage, out)"
s = must_replace(s, old_call, new_call, "assign call")
old_sig = "func (r *Repository) developmentAssignTitleFirstBoard(ctx context.Context, host world.Host, board world.Board, asOf string, eligibleTitles []string, originalCandidates []int, boardEvents []llm.BBSWorldWindowEvent, recentBBSState string, state *developmentTitleFirstState, offset int, boardsRemaining int, planner llm.BBSTitleCandidatePlanner, addUsage func(llm.TokenUsage), out map[string]developmentSparseSituation) error {"
new_sig = "func (r *Repository) developmentAssignTitleFirstBoard(ctx context.Context, host world.Host, board world.Board, asOf string, eligibleTitles []string, originalCandidates []int, boardEvents []llm.BBSWorldWindowEvent, recentBBSState string, state *developmentTitleFirstState, offset int, boardsRemaining int, planner llm.BBSTitleCandidatePlanner, detailPlanner llm.BBSTitleArticleDetailPlanner, addUsage func(llm.TokenUsage), out map[string]developmentSparseSituation) error {"
s = must_replace(s, old_sig, new_sig, "assign signature")
s = must_replace(s,
    "\tacceptedEvents := map[string]bool{}\n",
    "\tacceptedEvents := map[string]bool{}\n\tacceptedCandidateByEvent := map[string]int{}\n\tacceptedDetailSeeds := map[string]llm.BBSTitleArticleDetailSeed{}\n",
    "accepted detail maps")
s = must_replace(s, "\t\t\trow.Details = append([]string(nil), d.Details...)\n", "", "remove pre-detail row assignment")

block_start = s.index("\t\t\tacceptedCandidates[originalCandidate] = true\n\t\t\tacceptedEvents[d.EventID] = true")
block_end_marker = '\t\t\tout[d.EventID] = developmentSparseSituation{kind: "title_first", summary: d.Summary, facts: situationFacts}\n'
block_end = s.index(block_end_marker, block_start) + len(block_end_marker)
new_block = '''\t\t\tacceptedCandidates[originalCandidate] = true
\t\t\tacceptedEvents[d.EventID] = true
\t\t\tacceptedCandidateByEvent[d.EventID] = originalCandidate
\t\t\te := eventByID[d.EventID]
\t\t\tacceptedDetailSeeds[d.EventID] = llm.BBSTitleArticleDetailSeed{
\t\t\t\tEventID: d.EventID, Subject: d.Subject, Summary: d.Summary,
\t\t\t\tAuthorHandle: e.AuthorHandle, CreatedAt: e.CreatedAt, DiscourseMode: e.DiscourseMode,
\t\t\t\tExistingFacts: append([]string(nil), e.ExistingFacts...),
\t\t\t}
\t\t\trow.Reason = fmt.Sprintf("時代[%s]: %s / 人物: %s", row.EraStatus, row.EraReason, d.Reason)
\t\t\trow.Status = "accepted"
\t\t\tif d.Subject != row.Original {
\t\t\t\trow.Status = "corrected"
\t\t\t}
\t\t\tsituationFacts := []string{
\t\t\t\t"title_first_subject=" + d.Subject,
\t\t\t\t"title_first_original=" + row.Original,
\t\t\t\t"title_first_review=" + d.Reason,
\t\t\t\t"world_adoption=title_candidate",
\t\t\t\t"world_adopted_summary=" + d.Summary,
\t\t\t\t"historical_check=title_era_" + row.EraStatus,
\t\t\t\t"subject_contract=Keep the accepted title verbatim. The accepted title and world_adopted_summary are canonical world facts for this post. Article-local specifics will be added only by the post-adoption Article Detail Materializer.",
\t\t\t}
\t\t\tout[d.EventID] = developmentSparseSituation{kind: "title_first", summary: d.Summary, facts: situationFacts}
'''
s = s[:block_start] + new_block + s[block_end:]

insert_at = s.rindex("\tfor _, originalCandidate := range originalCandidates {")
detail_pass = '''\tif len(acceptedDetailSeeds) > 0 {
\t\tseeds := make([]llm.BBSTitleArticleDetailSeed, 0, len(acceptedDetailSeeds))
\t\tfor _, event := range boardEvents {
\t\t\tif seed, ok := acceptedDetailSeeds[event.EventID]; ok {
\t\t\t\tseeds = append(seeds, seed)
\t\t\t}
\t\t}
\t\tdetailDraft, detailErr := detailPlanner.MaterializeBBSTitleArticleDetails(ctx, llm.BBSTitleArticleDetailRequest{
\t\t\tBoardName: board.Name, WorldDate: asOf, RecentBBSState: recentBBSState, Articles: seeds,
\t\t})
\t\taddUsage(detailDraft.Usage)
\t\tif detailErr != nil {
\t\t\tfor eventID, originalCandidate := range acceptedCandidateByEvent {
\t\t\t\trow := &state.rows[offset+originalCandidate-1]
\t\t\t\trow.Status = "detail_rejected"
\t\t\t\trow.Reason += " / 記事detail具体化失敗: " + detailErr.Error()
\t\t\t\trow.Details = nil
\t\t\t\tdelete(out, eventID)
\t\t\t}
\t\t} else {
\t\t\tfor _, article := range detailDraft.Articles {
\t\t\t\toriginalCandidate, ok := acceptedCandidateByEvent[article.EventID]
\t\t\t\tif !ok {
\t\t\t\t\tcontinue
\t\t\t\t}
\t\t\t\trow := &state.rows[offset+originalCandidate-1]
\t\t\t\tsituation := out[article.EventID]
\t\t\t\trow.Details = nil
\t\t\t\tfor _, detail := range article.Details {
\t\t\t\t\tencoded := strings.TrimSpace(detail.Kind) + ":" + strings.TrimSpace(detail.Fact)
\t\t\t\t\trow.Details = append(row.Details, encoded)
\t\t\t\t\tsituation.facts = append(situation.facts, "article_detail="+encoded)
\t\t\t\t}
\t\t\t\tsituation.facts = append(situation.facts,
\t\t\t\t\t"article_detail_contract=The article_detail facts are canonical article-local specifics selected after title/persona/Era adoption. Materially express at least two distinct supplied details. A detail must add information beyond the title/summary; never collapse it back into vague wording. Do not add external historical/product/game facts, durable biography, or unexplained causes beyond canonical context.",
\t\t\t\t)
\t\t\t\tout[article.EventID] = situation
\t\t\t}
\t\t}
\t}
'''
s = s[:insert_at] + detail_pass + s[insert_at:]
s = must_replace(s,
    'if row.Status == "accepted" || row.Status == "corrected" || row.Status == "era_rejected" {',
    'if row.Status == "accepted" || row.Status == "corrected" || row.Status == "era_rejected" || row.Status == "detail_rejected" {',
    "final detail status")
write(p, s)

# 3) Source/root facts must never silently become reply-author facts.
p = "apps/server/internal/worldrepo/materialization_sparse_situation.go"
s = read(p)
start = s.index("func developmentSituationFromSource(")
end = s.index("\nfunc developmentSelectRootSituation", start)
new_func = r'''func developmentSituationFromSource(source world.Post, continuation bool) developmentSparseSituation {
	kind := strings.TrimSpace(source.Intent.SituationKind)
	if kind == "" {
		kind = "source_thread_context"
	}
	summary := strings.TrimSpace(source.Intent.SituationSummary)
	if summary == "" {
		summary = fmt.Sprintf("The canonical source post %04d is the situation boundary for this contribution.", source.ID)
	}
	facts := append([]string(nil), source.Intent.SituationFacts...)
	if continuation {
		summary = "A materially new development occurred inside the earlier canonical situation. " + summary
		facts = append(facts, "continuation=Add a genuinely new development; do not restate the earlier root.")
		return developmentSparseSituation{kind: kind, summary: summary, facts: facts}
	}
	if kind == "title_first" {
		facts = developmentTitleFirstReplySourceFacts(source.Intent.SituationFacts)
		summary = fmt.Sprintf("Reply to canonical source post %04d. Source event summary: %s", source.ID, summary)
		facts = append(facts,
			"reply_binding=Respond to the explicit canonical source; do not replace it with another topic or event.",
			"reply_source_contract=Every fact prefixed source_ belongs to the source post/source author, not to the reply author. You may acknowledge, question or comment on it, but never turn it into first-person experience, ownership, purchase, progress or observation unless the reply author's own canonical facts or already-written prose independently establish that fact.",
		)
		return developmentSparseSituation{kind: kind, summary: summary, facts: facts}
	}
	facts = append(facts, "reply_binding=Respond to the explicit canonical source; do not replace it with another topic or event.")
	return developmentSparseSituation{kind: kind, summary: summary, facts: facts}
}

func developmentTitleFirstReplySourceFacts(sourceFacts []string) []string {
	facts := make([]string, 0, len(sourceFacts))
	for _, fact := range sourceFacts {
		fact = strings.TrimSpace(fact)
		if fact == "" || strings.HasPrefix(fact, "article_detail_contract=") || strings.HasPrefix(fact, "subject_contract=") || strings.HasPrefix(fact, "reply_binding=") || strings.HasPrefix(fact, "reply_source_contract=") {
			continue
		}
		if strings.HasPrefix(fact, "source_") {
			facts = append(facts, fact)
			continue
		}
		if strings.Contains(fact, "=") {
			facts = append(facts, "source_"+fact)
			continue
		}
		facts = append(facts, "source_fact="+fact)
	}
	return facts
}
'''
s = s[:start] + new_func + s[end:]
write(p, s)

# 4) Make source ownership explicit to the prose worker too.
p = "apps/server/internal/llm/openai.go"
s = read(p)
needle = "- article_detail is a ceiling as well as a floor:"
pos = s.index(needle)
line_end = s.index("\n", pos)
addition = "\n- For replies, source_article_detail=... and every other source_... fact describe the source post/source author only. Use them as concrete material to react to, but do NOT convert them into first-person claims about the reply author. In particular, never write '私も始めた/買った/使っている/行った/見つけた' merely because the source author did; that requires independent canonical support for the reply author."
s = s[:line_end] + addition + s[line_end:]
write(p, s)

# 5) Update test renderer: title review returns no details; dedicated pass returns them.
p = "apps/server/internal/worldrepo/materialization_title_first_test.go"
s = read(p)
s = s.replace('\n\t\t\t\td.Details = []string{"攻略本の142ページの一覧表3行目を確認した", "ゲーム画面と見比べて表記の違いに気づいた"}', '', 1)
insert_marker = "\nfunc TestTitleFirstPreservesSubjectAndArchivesRejectedCandidates"
pos = s.index(insert_marker)
method = r'''
func (f *titleFirstTestRenderer) MaterializeBBSTitleArticleDetails(_ context.Context, r llm.BBSTitleArticleDetailRequest) (llm.BBSTitleArticleDetailDraft, error) {
	articles := make([]llm.BBSTitleArticleDetailSet, 0, len(r.Articles))
	for _, seed := range r.Articles {
		articles = append(articles, llm.BBSTitleArticleDetailSet{EventID: seed.EventID, Details: []llm.BBSArticleDetail{
			{Kind: "locator", Fact: "手元の資料の142ページ、一覧表の3行目だった"},
			{Kind: "comparison", Fact: "資料の表記と画面で確認した表記が食い違っていた"},
		}})
	}
	return llm.BBSTitleArticleDetailDraft{Articles: articles}, nil
}
'''
s = s[:pos] + method + s[pos:]
write(p, s)

# 6) Old title-review unit fixtures must no longer carry article details.
p = "apps/server/internal/llm/bbs_title_candidates_test.go"
s = read(p)
s = re.sub(r',?\s*Details:\s*\[\]string\{[^}]*\}', '', s)
write(p, s)

# 7) Document the two-stage adoption/detail boundary.
p = "docs/MATERIALIZATION_LAB.md"
s = read(p)
old = "- 採用が確定した時点で、タイトルとreview summaryがその投稿の `title_first` canonical world eventになる。summaryはタイトルから直接読み取れる最小限の出来事だけを正本化し、タイトルにない機種・場所・原因・購入経路・進捗等は追加しない。本文workerはこの採用済みeventと既存Persona/BBS factsの範囲だけを文章化する。"
new = old + "\n- title/persona/Era採用後にだけ専用Article Detail Materializerを実行し、採用記事ごとに2〜4件の `article_detail=<kind>:<fact>` を正本化する。detailはタイトル/summaryの言い換えや『読者に尋ねる』等の編集指示を禁止し、locator/timing/sequence/comparison/observation/question_scope/decision/reaction_contextのうち2種類以上で、本文を具体化する記事ローカル情報を固定する。20候補すべてにdetailを作らない。\n- replyではrootのdetailを `source_article_detail` 等の `source_` namespaceへ移し、source authorの事実として扱う。返信者自身の購入・利用・開始・訪問・発見等へ一人称で継承してはならない。"
s = must_replace(s, old, new, "docs title-first detail boundary")
write(p, s)
