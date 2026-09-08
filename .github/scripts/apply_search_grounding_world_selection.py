from pathlib import Path

p = Path('apps/server/internal/worldrepo/materialization_search_grounding.go')
s = p.read_text()

s = s.replace(
'''type developmentSearchGroundingStats struct {
	Queries        int
	EvidenceHits   int
	Refined        int
	Rejected       int
	SearchFailures int
	RefineFailures int
}''',
'''type developmentSearchGroundingStats struct {
	Queries          int
	EvidenceHits     int
	CandidateOptions int
	Selections       int
	Refined          int
	Rejected         int
	SearchFailures   int
	RefineFailures   int
}''')

s = s.replace(
'''return fmt.Sprintf("search_queries=%d search_hits=%d grounded_refinements=%d grounding_rejected=%d search_failures=%d refine_failures=%d", stats.Queries, stats.EvidenceHits, stats.Refined, stats.Rejected, stats.SearchFailures, stats.RefineFailures)''',
'''return fmt.Sprintf("search_queries=%d search_hits=%d candidate_options=%d grounded_selections=%d grounded_refinements=%d grounding_rejected=%d search_failures=%d refine_failures=%d", stats.Queries, stats.EvidenceHits, stats.CandidateOptions, stats.Selections, stats.Refined, stats.Rejected, stats.SearchFailures, stats.RefineFailures)''')

s = s.replace('\tgrounding := map[string][]string{}\n\tvar mu sync.Mutex', '\toptionsByEvent := map[string][]developmentGroundingCandidate{}\n\tvar mu sync.Mutex')
s = s.replace('decision, err := r.Engine.ResolveEvidence(ctx, developmentSearchGroundingEvidenceRequest(item, proposal))', 'decision, err := r.Engine.ResolveEvidence(ctx, developmentSearchGroundingEvidenceRequest(item, proposal, factsByPersona[item.shell.persona.ID]))')

s = s.replace(
'''\t\t\tevidence := developmentUsableGroundingClaims(decision.Knowledge)
\t\t\tif len(evidence) == 0 {
\t\t\t\treturn
\t\t\t}
\t\t\tmu.Lock()
\t\t\tstats.EvidenceHits++
\t\t\tgrounding[item.eventID] = evidence
\t\t\tmu.Unlock()''',
'''\t\t\toptions := developmentGroundingCandidatesFromKnowledge(decision.Knowledge)
\t\t\tif len(options) == 0 {
\t\t\t\treturn
\t\t\t}
\t\t\tmu.Lock()
\t\t\tstats.EvidenceHits++
\t\t\tstats.CandidateOptions += len(options)
\t\t\toptionsByEvent[item.eventID] = options
\t\t\tmu.Unlock()''')

s = s.replace(
'''\twg.Wait()
\tif len(grounding) == 0 {
\t\tstoreStats()
\t\treturn accepted, GenerationUsage{}
\t}

\trefineRoots := make([]developmentWindowShell, 0, len(grounding))
\tfor _, root := range roots {
\t\tif len(grounding[root.eventID]) > 0 {
\t\t\trefineRoots = append(refineRoots, root)
\t\t}
\t}''',
'''\twg.Wait()
\tif len(optionsByEvent) == 0 {
\t\tstoreStats()
\t\treturn accepted, GenerationUsage{}
\t}

\tgrounding := map[string][]string{}
\tselectedNames := map[string]int{}
\trecentPosts := r.Base.ListPosts(host.ID)
\trefineRoots := make([]developmentWindowShell, 0, len(optionsByEvent))
\tfor _, root := range roots {
\t\toptions := optionsByEvent[root.eventID]
\t\tif len(options) == 0 {
\t\t\tcontinue
\t\t}
\t\tselected, ok := developmentSelectGroundingCandidate(host.ID, root, options, recentPosts, selectedNames)
\t\tif !ok {
\t\t\tcontinue
\t\t}
\t\tselectedNames[developmentNormalizeReferentName(selected.Name)]++
\t\tstats.Selections++
\t\tgrounding[root.eventID] = []string{developmentSelectedGroundingEvidence(selected)}
\t\trefineRoots = append(refineRoots, root)
\t}
\tif len(refineRoots) == 0 {
\t\tstoreStats()
\t\treturn accepted, GenerationUsage{}
\t}''')

s = s.replace(
'''\t\trefined, found := plan.proposals[root.eventID]
\t\tif !found || !developmentSearchGroundingCompatible(original, refined) {''',
'''\t\trefined, found := plan.proposals[root.eventID]
\t\tselectedName := developmentSelectedGroundingName(grounding[root.eventID])
\t\tif !found || !developmentSearchGroundingCompatible(original, refined) || !developmentRefinementUsesSelectedReferent(refined, selectedName) {''')

start = s.index('func developmentSearchGroundingEvidenceRequest(')
end = s.index('\nfunc developmentUsableGroundingClaims', start)
new_func = '''func developmentSearchGroundingEvidenceRequest(root developmentWindowShell, proposal developmentSituationProposal, personaFacts []world.PersonaFact) worldengine.EvidenceRequest {
\tkind := historicalkb.KnowledgeGeneral
\tswitch strings.ToLower(strings.TrimSpace(root.shell.anchorKey)) {
\tcase "games", "software", "communications", "modem":
\t\tkind = historicalkb.KnowledgeProductAvailability
\tcase "music":
\t\tkind = historicalkb.KnowledgeCulturalSignal
\t}
\tworldDate := root.shell.createdAt.Format("2006-01-02")
\tactorContext := developmentGroundingActorContext(root.shell.persona, personaFacts)
\tneed := fmt.Sprintf("次の架空BBS内の出来事の種類・行動・観察はworld engineが既に選択済みです。あなたの役割は、この出来事を%s時点の日本の現実世界へ自然に接地できる実在の製品・作品・サービス・機種・番組・曲などの候補集合をWeb検索で作ることです。重要: これは過去の正解を推理する検索ではありません。候補のうちどれが今回このNPCの世界事実になるかは、この後world engineが決定します。したがってoriginal Situationから一意に名前を推理できる必要はありません。出来事の意味を変えず、その日までに日本で利用・発売・稼働・放送・鑑賞・言及可能で、普通にこの出来事の対象になり得る候補を2〜5件返してください。候補はできるだけ公式資料・当時資料・信頼できる保存資料で存在時期を確認してください。候補名は当時の利用者が会話で自然に使う短い正式名または一般的名称にしてください。人気作だけに偏らず、actor contextやboard contextに自然に合う順で並べてください。ただしactorが所有・購入・視聴・プレイした事実は推測しないでください。候補はその可能性を提供するだけです。各候補はProvisionalAnswer内で必ず独立した行に CANDIDATE: 名前 || その日までの存在/利用可能性と、このgeneric Situationに意味を変えず適合する短い根拠 の厳密な形式で書いてください。候補が本当に作れない場合だけ NO_CANDIDATE: 理由 としてください。単に候補が複数あることはNO_CANDIDATEの理由ではありません。必要な歴史条件が裏付けられた候補だけを出し、その場合missingInfoは空配列にしてください。NPCの今回の関与、所有歴、長期嗜好、個別店舗在庫、主観的結果は検索スコープ外なのでmissingInfoへ入れないでください。board=%s; actor_context=%s; object=%s; occurrence=%s; actor_observation=%s", worldDate, root.board.Name, actorContext, proposal.objectClass, proposal.occurrence, proposal.actorObservation)
\treturn worldengine.EvidenceRequest{
\t\tKind:            kind,
\t\tSubject:         proposal.objectClass + " / " + proposal.noveltyKey + " / candidate-pool-v1",
\t\tWorldDate:       worldDate,
\t\tRegion:          "JP",
\t\tAudience:        []string{"Japanese PC communication users"},
\t\tNeed:            need,
\t\tPersistence:     true,
\t\tImportance:      .7,
\t\tSpecificity:     .9,
\t\tHasProductModel: true,
\t}
}
'''
s = s[:start] + new_func + s[end:]
p.write_text(s)

p = Path('apps/server/internal/llm/openai_world_situation_proposer.go')
s = p.read_text()
old = '- SEARCH-GROUNDED RETRY: an event may contain existing_facts beginning "GROUNDING ORIGINAL SITUATION - PRESERVE SEMANTICS:" and "GROUNDING VERIFIED HISTORICAL EVIDENCE - ALLOWED REAL REFERENTS:". For such an event, this pass is NOT permission to invent a different situation. Preserve the original activity, change, actor observation, impact, uncertainty and novelty_key. Use at most one searched real referent only when the ORIGINAL SITUATION contains discriminating semantic clues that make that referent materially better identified than ordinary contemporary alternatives. Historical availability or mere plausibility is not enough. Do NOT choose one name from a menu of unrelated candidates just to make prose concrete. If two or more unrelated candidates fit the original event about equally well, keep the original generic object. Likewise, if the evidence presents a named thing only as one possible example of a broad activity, keep it generic. Naming is allowed only when the original object/activity plus occurrence/observation narrow the identity or a historically meaningful small class; otherwise omission is the correct result. The searched evidence verifies historical availability, while this world-Situation pass decides the modest actor involvement. Do not turn alternative candidates into multiple events. If no candidate is discriminatively supported, keep the original generic object. Never emit a must_not rule that forbids a real name explicitly permitted by grounding evidence.'
new = '- SEARCH-GROUNDED RETRY: an event may contain existing_facts beginning "GROUNDING ORIGINAL SITUATION - PRESERVE SEMANTICS:" and "GROUNDING WORLD-SELECTED REFERENT - MUST USE:". For such an event, the search stage has already produced historically valid candidates and the Go world layer has already selected exactly one referent as the new world fact for this event. You MUST use that selected referent to replace the generic object while preserving the original activity, change, actor observation, impact, uncertainty and novelty_key. Do not choose an alternative candidate and do not fall back to a generic label merely because the original Situation did not uniquely imply the name; this is creation of a new canonical fact, not reconstruction of a hidden past fact. The selected actor may modestly play, use, hear, watch, read, visit or discuss that referent as required by the original occurrence. This does not establish ownership, purchase history, long-term fandom, unrelated biography or extra product facts. Never emit a must_not rule forbidding the selected referent; use must_not only to prevent unsupported extra details.'
if old not in s:
    raise SystemExit('search-grounded prompt block not found')
p.write_text(s.replace(old, new))

p = Path('apps/server/internal/worldrepo/materialization_search_grounding_test.go')
s = p.read_text()
s = s.replace('developmentSearchGroundingEvidenceRequest(root, proposal)', 'developmentSearchGroundingEvidenceRequest(root, proposal, nil)')
start = s.index('func TestDevelopmentSearchGroundingEvidenceRequestScopesMissingInfoToHistoricalCandidate')
s = s[:start] + '''func TestDevelopmentSearchGroundingEvidenceRequestBuildsCandidatePool(t *testing.T) {
\tdate := time.Date(1996, 8, 22, 12, 0, 0, 0, time.FixedZone("JST", 9*3600))
\troot := developmentWindowShell{eventID: "game", board: world.Board{ID: "4", Name: "ゲーム"}, shell: developmentTimelineShell{createdAt: date, anchorKey: "games"}}
\tproposal := developmentSituationProposal{objectClass: "ゲーム内の進行場面", occurrence: "手がかりを見落として同じ場所を調べた", actorObservation: "同じ場所を何度か調べた", noveltyKey: "clue"}
\treq := developmentSearchGroundingEvidenceRequest(root, proposal, nil)
\tfor _, want := range []string{"world engineが決定", "CANDIDATE:", "一意に名前を推理できる必要はありません", "2〜5件", "candidate-pool-v1"} {
\t\tif !strings.Contains(req.Need+req.Subject, want) {
\t\t\tt.Fatalf("grounding request missing %q: %s / %s", want, req.Need, req.Subject)
\t\t}
\t}
}

func TestDevelopmentGroundingCandidateParser(t *testing.T) {
\tclaim := "CANDIDATE: MYST || 1994年発売で探索停滞の出来事に適合\\nCANDIDATE: 弟切草 || 1992年発売で選択肢のある遊びに適合"
\tgot := developmentParseGroundingCandidates(claim)
\tif len(got) != 2 || got[0].Name != "MYST" || got[1].Name != "弟切草" {
\t\tt.Fatalf("parsed candidates=%+v", got)
\t}
}

func TestDevelopmentSelectGroundingCandidatePenalizesRecentReuse(t *testing.T) {
\tdate := time.Date(1996, 8, 22, 12, 0, 0, 0, time.FixedZone("JST", 9*3600))
\troot := developmentWindowShell{eventID: "e1", shell: developmentTimelineShell{createdAt: date, persona: world.Persona{ID: "p1"}}}
\toptions := []developmentGroundingCandidate{{Name: "MYST", Evidence: "x", Rank: 0}, {Name: "弟切草", Evidence: "y", Rank: 1}}
\trecent := []world.Post{{Subject: "MYSTの話", Body: "MYSTを遊んだ"}}
\tselected, ok := developmentSelectGroundingCandidate("h", root, options, recent, map[string]int{})
\tif !ok || selected.Name != "弟切草" {
\t\tt.Fatalf("selected=%+v ok=%v", selected, ok)
\t}
}
'''
p.write_text(s)

Path('apps/server/internal/worldrepo/materialization_search_grounding_selection.go').write_text('''package worldrepo

import (
\t"fmt"
\t"hash/fnv"
\t"strings"

\t"zutto-pccom/apps/server/internal/historicalkb"
\t"zutto-pccom/apps/server/internal/world"
)

type developmentGroundingCandidate struct {
\tName     string
\tEvidence string
\tRank     int
}

func developmentGroundingCandidatesFromKnowledge(result historicalkb.KnowledgeResult) []developmentGroundingCandidate {
\tif !result.CanUse {
\t\treturn nil
\t}
\tout := make([]developmentGroundingCandidate, 0, 8)
\tseen := map[string]struct{}{}
\tfor _, fact := range result.Facts {
\t\tif fact.Status != historicalkb.FactVerified && fact.Status != historicalkb.FactOperatorVerified && fact.Status != historicalkb.FactCanonical {
\t\t\tcontinue
\t\t}
\t\tfor _, candidate := range developmentParseGroundingCandidates(fact.Claim) {
\t\t\tkey := developmentNormalizeReferentName(candidate.Name)
\t\t\tif key == "" {
\t\t\t\tcontinue
\t\t\t}
\t\t\tif _, ok := seen[key]; ok {
\t\t\t\tcontinue
\t\t\t}
\t\t\tcandidate.Rank = len(out)
\t\t\tseen[key] = struct{}{}
\t\t\tout = append(out, candidate)
\t\t}
\t}
\treturn out
}

func developmentParseGroundingCandidates(claim string) []developmentGroundingCandidate {
\tlines := strings.Split(strings.ReplaceAll(claim, "\\r\\n", "\\n"), "\\n")
\tout := make([]developmentGroundingCandidate, 0, 5)
\tfor _, line := range lines {
\t\tline = strings.TrimSpace(strings.TrimLeft(line, "-*• "))
\t\tif !strings.HasPrefix(strings.ToUpper(line), "CANDIDATE:") {
\t\t\tcontinue
\t\t}
\t\trest := strings.TrimSpace(line[len("CANDIDATE:"):])
\t\tparts := strings.SplitN(rest, "||", 2)
\t\tname := strings.TrimSpace(parts[0])
\t\tif name == "" {
\t\t\tcontinue
\t\t}
\t\tevidence := ""
\t\tif len(parts) == 2 {
\t\t\tevidence = strings.TrimSpace(parts[1])
\t\t}
\t\tout = append(out, developmentGroundingCandidate{Name: name, Evidence: evidence, Rank: len(out)})
\t}
\treturn out
}

func developmentGroundingActorContext(persona world.Persona, facts []world.PersonaFact) string {
\tparts := []string{personaSummary(persona)}
\tfor i, fact := range facts {
\t\tif i >= 8 {
\t\t\tbreak
\t\t}
\t\tkey := strings.TrimSpace(fact.Key)
\t\tvalue := strings.TrimSpace(fact.Value)
\t\tif key != "" && value != "" {
\t\t\tparts = append(parts, key+"="+value)
\t\t}
\t}
\treturn strings.Join(parts, "; ")
}

func developmentSelectGroundingCandidate(hostID string, root developmentWindowShell, options []developmentGroundingCandidate, recentPosts []world.Post, selectedNames map[string]int) (developmentGroundingCandidate, bool) {
\tif len(options) == 0 {
\t\treturn developmentGroundingCandidate{}, false
\t}
\tbest := options[0]
\tbestScore := -1 << 30
\tfor _, option := range options {
\t\tnormalized := developmentNormalizeReferentName(option.Name)
\t\tif normalized == "" {
\t\t\tcontinue
\t\t}
\t\tscore := 1000 - option.Rank*35
\t\tscore -= developmentReferentRecentCount(option.Name, recentPosts) * 300
\t\tscore -= selectedNames[normalized] * 700
\t\tscore += int(developmentStableGroundingHash(hostID+"|"+root.eventID+"|"+root.shell.persona.ID+"|"+option.Name) % 97)
\t\tif score > bestScore {
\t\t\tbestScore = score
\t\t\tbest = option
\t\t}
\t}
\treturn best, bestScore > (-1 << 30)
}

func developmentReferentRecentCount(name string, posts []world.Post) int {
\tnormalized := developmentNormalizeReferentName(name)
\tif normalized == "" {
\t\treturn 0
\t}
\tcount := 0
\tfor _, post := range posts {
\t\thaystack := developmentNormalizeReferentName(post.Subject + " " + post.Body)
\t\tif strings.Contains(haystack, normalized) {
\t\t\tcount++
\t\t}
\t}
\treturn count
}

func developmentStableGroundingHash(value string) uint32 {
\th := fnv.New32a()
\t_, _ = h.Write([]byte(value))
\treturn h.Sum32()
}

func developmentNormalizeReferentName(value string) string {
\treplacer := strings.NewReplacer(" ", "", "　", "", "『", "", "』", "", "「", "", "」", "", "・", "", "-", "", "_", "")
\treturn strings.ToLower(replacer.Replace(strings.TrimSpace(value)))
}

func developmentSelectedGroundingEvidence(candidate developmentGroundingCandidate) string {
\treturn fmt.Sprintf("GROUNDING WORLD-SELECTED REFERENT - MUST USE: %s || VERIFIED SEARCH FIT: %s", candidate.Name, candidate.Evidence)
}

func developmentSelectedGroundingName(evidence []string) string {
\tconst prefix = "GROUNDING WORLD-SELECTED REFERENT - MUST USE:"
\tfor _, item := range evidence {
\t\tif !strings.HasPrefix(item, prefix) {
\t\t\tcontinue
\t\t}
\t\trest := strings.TrimSpace(strings.TrimPrefix(item, prefix))
\t\tif idx := strings.Index(rest, "||"); idx >= 0 {
\t\t\trest = strings.TrimSpace(rest[:idx])
\t\t}
\t\treturn rest
\t}
\treturn ""
}

func developmentRefinementUsesSelectedReferent(refined developmentSituationProposal, selected string) bool {
\tselected = developmentNormalizeReferentName(selected)
\tif selected == "" {
\t\treturn false
\t}
\tcombined := developmentNormalizeReferentName(strings.Join([]string{refined.objectClass, refined.occurrence, refined.actorObservation}, " "))
\treturn strings.Contains(combined, selected)
}
''')