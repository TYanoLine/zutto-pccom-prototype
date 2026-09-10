from pathlib import Path


def replace_once(path: str, old: str, new: str) -> None:
    p = Path(path)
    s = p.read_text()
    if old not in s:
        raise SystemExit(f"pattern not found in {path}: {old[:120]!r}")
    p.write_text(s.replace(old, new, 1))


path = "apps/server/internal/llm/bbs_title_candidates.go"
replace_once(
    path,
    '''type BBSTitleDecision struct {
\tCandidate int    `json:"candidate"`
\tEventID   string `json:"event_id"`
\tSubject   string `json:"subject"`
\tReason    string `json:"reason"`
\tSummary   string `json:"summary"`
}''',
    '''type BBSTitleDecision struct {
\tCandidate int      `json:"candidate"`
\tEventID   string   `json:"event_id"`
\tSubject   string   `json:"subject"`
\tReason    string   `json:"reason"`
\tSummary   string   `json:"summary"`
\tDetails   []string `json:"details"`
}''',
)
replace_once(
    path,
    '''summaryには、採用時にworld側が正本化する「タイトルから直接読み取れる最小限の出来事・用件」だけを短く記してください。タイトルにない機種、場所、相手、原因、購入経路、進捗、クリア状況などを補わないでください。例: 「クロノ・トリガーを今さら始めました」なら「この人物が最近クロノ・トリガーを始め、そのことを話題にする」まで。「バーチャファイター2は凄い！」なら肯定的な意見までで、所有や購入は推定しない。不採用のsubjectとsummaryは空文字、reasonは具体的な理由。候補が重複したら片方を不採用。''',
    '''summaryには、採用時にworld側が正本化する「タイトルから直接読み取れる最小限の出来事・用件」だけを短く記してください。タイトルにない機種、場所、相手、原因、購入経路、進捗、クリア状況などをsummaryへ勝手に足さないでください。例: 「クロノ・トリガーを今さら始めました」なら「この人物が最近クロノ・トリガーを始め、そのことを話題にする」まで。「バーチャファイター2は凄い！」なら肯定的な意見までで、所有や購入は推定しない。
採用候補のdetailsには、本文を書く前にworld側へ正本化してよい「この記事だけの具体ディテール」を2〜4件入れてください。これはまだ提案であり、後段のEra検証とコード側検査を通った候補だけがcanonical world factになります。本文workerが「一つ見つけた」「その部分」「少し違った」のような抽象語だけで逃げなくてよい粒度にしてください。ページや欄、画面上の位置、何を見比べたか、試した順序、観察できた差、直後の結果、読者が確認できる手掛かり等から、その記事に自然なものを選びます。少なくとも2件は互いに別の具体情報にしてください。
detailsは記事内の一時的・局所的な事実を具体化するための欄です。既存PersonaFactsにない恒久的な所有物、購入歴、職歴、家族事情などを増やさないでください。また、subject・ExistingFacts・RecentBBSStateに無い新しい実在製品名/作品名/人物名/企業名/実在地名を導入せず、実在作品の設定、攻略情報、商品仕様、価格、発売日、実在出版物の正確なページ内容など外部史実をモデル記憶で発明しないでください。実在物が題名にある場合も、その存在から作品内容や仕様を連想補完しないでください。一方、実在物の外部史実にならない記事ローカルな観察・手順・位置関係は具体化して構いません。
不採用のsubject、summary、detailsは空にし、reasonは具体的な理由にしてください。候補が重複したら片方を不採用。''',
)
replace_once(
    path,
    '''\tfor _, key := range []string{"event_id", "subject", "reason", "summary"} {
\t\tfields[key] = map[string]any{"type": "string"}
\t}
\tfields["candidate"] = map[string]any{"type": "integer"}
\titem := map[string]any{"type": "object", "properties": fields, "required": []string{"candidate", "event_id", "subject", "reason", "summary"}, "additionalProperties": false}''',
    '''\tfor _, key := range []string{"event_id", "subject", "reason", "summary"} {
\t\tfields[key] = map[string]any{"type": "string"}
\t}
\tfields["candidate"] = map[string]any{"type": "integer"}
\tfields["details"] = map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "minItems": 0, "maxItems": 4}
\titem := map[string]any{"type": "object", "properties": fields, "required": []string{"candidate", "event_id", "subject", "reason", "summary", "details"}, "additionalProperties": false}''',
)
replace_once(
    path,
    '''\t\tif d.EventID == "" {
\t\t\tif d.Subject != "" || d.Summary != "" {
\t\t\t\treturn fmt.Errorf("rejected candidate has content")
\t\t\t}
\t\t\tcontinue
\t\t}''',
    '''\t\tif d.EventID == "" {
\t\t\tif d.Subject != "" || d.Summary != "" || len(d.Details) != 0 {
\t\t\t\treturn fmt.Errorf("rejected candidate has content")
\t\t\t}
\t\t\tcontinue
\t\t}''',
)
replace_once(
    path,
    '''\t\tif strings.TrimSpace(d.Subject) == "" || utf8.RuneCountInString(d.Subject) > 36 || strings.ContainsAny(d.Subject, "\\r\\n") || hasReplySubjectPrefix(d.Subject) || strings.TrimSpace(d.Summary) == "" {
\t\t\treturn fmt.Errorf("invalid accepted title %d", d.Candidate)
\t\t}
\t\tkey := strings.ToLower(strings.TrimSpace(d.Subject))''',
    '''\t\tif strings.TrimSpace(d.Subject) == "" || utf8.RuneCountInString(d.Subject) > 36 || strings.ContainsAny(d.Subject, "\\r\\n") || hasReplySubjectPrefix(d.Subject) || strings.TrimSpace(d.Summary) == "" {
\t\t\treturn fmt.Errorf("invalid accepted title %d", d.Candidate)
\t\t}
\t\tif len(d.Details) < 2 || len(d.Details) > 4 {
\t\t\treturn fmt.Errorf("accepted title %d needs 2-4 article details", d.Candidate)
\t\t}
\t\tfor _, detail := range d.Details {
\t\t\tdetail = strings.TrimSpace(detail)
\t\t\tif detail == "" || utf8.RuneCountInString(detail) > 160 || strings.ContainsAny(detail, "\\r\\n") {
\t\t\t\treturn fmt.Errorf("invalid article detail for title %d", d.Candidate)
\t\t\t}
\t\t}
\t\tkey := strings.ToLower(strings.TrimSpace(d.Subject))''',
)
p = Path(path)
s = p.read_text()
s = s.replace(
    '\t\td.Summary = ""\n\t\td.Reason = "検査結果不備：',
    '\t\td.Summary = ""\n\t\td.Details = nil\n\t\td.Reason = "検査結果不備：',
)
p.write_text(s)

path = "apps/server/internal/worldrepo/materialization_title_first.go"
replace_once(
    path,
    '''\tEraEvidence string `json:"era_evidence,omitempty"`
}''',
    '''\tEraEvidence string   `json:"era_evidence,omitempty"`
\tDetails     []string `json:"details,omitempty"`
}''',
)
replace_once(
    path,
    '''\t\t\trow.EventID = d.EventID
\t\t\trow.Author = e.AuthorHandle
\t\t\trow.Subject = d.Subject
\t\t\tif row.EraStatus != "research" {''',
    '''\t\t\trow.EventID = d.EventID
\t\t\trow.Author = e.AuthorHandle
\t\t\trow.Subject = d.Subject
\t\t\trow.Details = append([]string(nil), d.Details...)
\t\t\tif row.EraStatus != "research" {''',
)
replace_once(
    path,
    '''\t\t\tout[d.EventID] = developmentSparseSituation{kind: "title_first", summary: d.Summary, facts: []string{"title_first_subject=" + d.Subject, "title_first_original=" + row.Original, "title_first_review=" + d.Reason, "world_adoption=title_candidate", "world_adopted_summary=" + d.Summary, "historical_check=title_era_" + row.EraStatus, "subject_contract=Keep the accepted title verbatim. The accepted title and world_adopted_summary are canonical world facts for this post. You may state facts directly entailed by them plus existing persona/BBS facts; do not add further possessions, purchases, visits, progress, completions, technical causes, public events or personal history not entailed by the adopted event."}}
''',
    '''\t\t\tsituationFacts := []string{"title_first_subject=" + d.Subject, "title_first_original=" + row.Original, "title_first_review=" + d.Reason, "world_adoption=title_candidate", "world_adopted_summary=" + d.Summary, "historical_check=title_era_" + row.EraStatus}
\t\t\tfor _, detail := range d.Details {
\t\t\t\tdetail = strings.TrimSpace(detail)
\t\t\t\tif detail != "" {
\t\t\t\t\tsituationFacts = append(situationFacts, "article_detail="+detail)
\t\t\t\t}
\t\t\t}
\t\t\tsituationFacts = append(situationFacts,
\t\t\t\t"article_detail_contract=The article_detail facts are canonical article-local specifics chosen before prose. Materially express at least two distinct supplied details when available; do not collapse them into vague wording such as 'one thing', 'that part' or 'something was different'. Do not add new durable biography or external historical/product/game facts beyond the adopted title, supplied detail facts and existing canonical context.",
\t\t\t\t"subject_contract=Keep the accepted title verbatim. The accepted title, world_adopted_summary and article_detail facts are canonical world facts for this post. Do not add further possessions, purchases, visits, progress, completions, technical causes, public events or personal history beyond that adopted event and its explicit article details.",
\t\t\t)
\t\t\tout[d.EventID] = developmentSparseSituation{kind: "title_first", summary: d.Summary, facts: situationFacts}
''',
)

path = "apps/server/internal/worldrepo/materialization_title_first_test.go"
replace_once(
    path,
    '''\t\t\t\td.Summary = "感想を共有"
\t\t\t\td.Reason = "整合"''',
    '''\t\t\t\td.Summary = "感想を共有"
\t\t\t\td.Details = []string{"攻略本の142ページの一覧表3行目を確認した", "ゲーム画面と見比べて表記の違いに気づいた"}
\t\t\t\td.Reason = "整合"''',
)
replace_once(
    path,
    '''\t\thasWorldAdoption := false
\t\thasAdoptedSummary := false
\t\tfor _, fact := range post.Intent.SituationFacts {''',
    '''\t\thasWorldAdoption := false
\t\thasAdoptedSummary := false
\t\tdetailCount := 0
\t\tfor _, fact := range post.Intent.SituationFacts {''',
)
replace_once(
    path,
    '''\t\t\tif fact == "world_adopted_summary=感想を共有" {
\t\t\t\thasAdoptedSummary = true
\t\t\t}
\t\t}
\t\tif !hasWorldAdoption || !hasAdoptedSummary {
\t\t\tt.Fatalf("missing world adoption facts: %+v", post.Intent.SituationFacts)
\t\t}''',
    '''\t\t\tif fact == "world_adopted_summary=感想を共有" {
\t\t\t\thasAdoptedSummary = true
\t\t\t}
\t\t\tif strings.HasPrefix(fact, "article_detail=") {
\t\t\t\tdetailCount++
\t\t\t}
\t\t}
\t\tif !hasWorldAdoption || !hasAdoptedSummary || detailCount < 2 {
\t\t\tt.Fatalf("missing world adoption/detail facts: %+v", post.Intent.SituationFacts)
\t\t}''',
)
replace_once(
    path,
    '''\t\tif !found || !created || rendered.Subject != post.Subject || renderer.req.CanonicalSubject != post.Subject {
\t\t\tt.Fatalf("subject changed: %+v %s", rendered, diag)
\t\t}''',
    '''\t\tif !found || !created || rendered.Subject != post.Subject || renderer.req.CanonicalSubject != post.Subject {
\t\t\tt.Fatalf("subject changed: %+v %s", rendered, diag)
\t\t}
\t\tif !strings.Contains(renderer.req.PostIntent, "article_detail=") {
\t\t\tt.Fatalf("article details were not supplied to body worker: %s", renderer.req.PostIntent)
\t\t}''',
)

path = "apps/server/internal/llm/bbs_title_candidates_test.go"
p = Path(path)
s = p.read_text()
s = s.replace('Summary: "感想を共有"}', 'Summary: "感想を共有", Details: []string{"具体1", "具体2"}}')
s = s.replace('Summary: "用件1"}', 'Summary: "用件1", Details: []string{"具体1a", "具体1b"}}')
s = s.replace('Summary: "用件2"}', 'Summary: "用件2", Details: []string{"具体2a", "具体2b"}}')
s = s.replace('Summary: "用件3"}', 'Summary: "用件3", Details: []string{"具体3a", "具体3b"}}')
p.write_text(s)

path = "apps/server/internal/llm/openai.go"
replace_once(
    path,
    '''- If the intent contains claims=..., those are concrete fictional-world facts already decided for this person/post. Express them materially when relevant instead of replacing them with generic filler.
- goal is the free-form conversational purpose of this exact post.''',
    '''- If the intent contains claims=..., those are concrete fictional-world facts already decided for this person/post. Express them materially when relevant instead of replacing them with generic filler.
- If the intent or bbs_context contains article_detail=..., those are canonical article-local specifics fixed before prose. When two or more are supplied, materially express at least two distinct details in the root body. Do not compress them back into vague wording such as 「一つ見つけた」「その部分」「少し違った」 when a page/section/position/sequence/observed difference or other concrete locator was supplied. The point is that another member should understand what actually happened without guessing the hidden detail.
- article_detail is a ceiling as well as a floor: use the supplied concrete facts, but do not invent additional durable biography, external product/game canon, exact specifications, prices, release facts, public events or unexplained causes beyond canonical context.
- goal is the free-form conversational purpose of this exact post.''',
)
replace_once(
    path,
    '''- If bbs_context is supplied, read THREAD SO FAR like prior messages in a chat. Earlier body text is canonical prose; semantic-envelope entries are canonical meaning for posts whose prose has not been materialized yet. Use this context to avoid accidental repetition and to make references/replies coherent.
- RELATED EARLIER POSTS in bbs_context are retrieval hints''',
    '''- If bbs_context is supplied, read THREAD SO FAR like prior messages in a chat. Earlier body text is canonical prose; semantic-envelope entries are canonical meaning for posts whose prose has not been materialized yet. Use this context to avoid accidental repetition and to make references/replies coherent.
- For a reply, if the explicit source contains concrete page/position/value/step/result/observation details, respond to at least one actual supplied detail when it is relevant instead of giving only generic agreement such as 「そうですね」「気をつけた方がよさそうです」. Do not invent a second hidden detail just to sound specific.
- RELATED EARLIER POSTS in bbs_context are retrieval hints''',
)

path = "docs/MATERIALIZATION_LAB.md"
p = Path(path)
s = p.read_text()
marker = "world_adoption=title_candidate"
if marker in s and "article_detail=" not in s:
    s = s.replace(
        marker,
        marker + "\n- 採用候補のreviewは本文用の `details` を2〜4件提案し、Era/構造検査を通ったときだけ `article_detail=...` としてworld側へ正本化する。本文workerはその具体情報を実際に文章へ出し、抽象語へ丸め直さない。新しい外部史実や恒久的な人物設定はdetailで発明しない。",
        1,
    )
    p.write_text(s)
