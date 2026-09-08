from pathlib import Path

p = Path('apps/server/internal/worldrepo/materialization_search_grounding.go')
s = p.read_text()

s = s.replace(
'''\tSearchErrors     []string
\tRefineErrors     []string
}''',
'''\tSearchErrors     []string
\tRefineErrors     []string
\tRejectionReasons []string
}''')

s = s.replace(
'''\tif len(stats.RefineErrors) > 0 {
\t\terrors := append([]string(nil), stats.RefineErrors...)
\t\tsort.Strings(errors)
\t\tdiagnostic += " refine_errors=" + strings.Join(errors, " | ")
\t}
\treturn diagnostic''',
'''\tif len(stats.RefineErrors) > 0 {
\t\terrors := append([]string(nil), stats.RefineErrors...)
\t\tsort.Strings(errors)
\t\tdiagnostic += " refine_errors=" + strings.Join(errors, " | ")
\t}
\tif len(stats.RejectionReasons) > 0 {
\t\treasons := append([]string(nil), stats.RejectionReasons...)
\t\tsort.Strings(reasons)
\t\tdiagnostic += " grounding_rejections=" + strings.Join(reasons, " | ")
\t}
\treturn diagnostic''')

s = s.replace('''\t\tif !found || !developmentSearchGroundingCompatible(original, refined) || !developmentRefinementUsesSelectedReferent(refined, selectedName) {
\t\t\tstats.Rejected++
\t\t\tcontinue
\t\t}''', '''\t\tif reason := developmentSearchGroundingRejectionReason(original, refined, found, selectedName); reason != "" {
\t\t\tstats.Rejected++
\t\t\tstats.RejectionReasons = append(stats.RejectionReasons, root.eventID+":"+reason)
\t\t\tcontinue
\t\t}''')

marker = '''func developmentSearchGroundingCompatible(original, refined developmentSituationProposal) bool {'''
helper = '''func developmentSearchGroundingRejectionReason(original, refined developmentSituationProposal, found bool, selectedName string) string {
\tif !found {
\t\treturn "missing_refinement"
\t}
\tif strings.TrimSpace(refined.objectClass) == "" || strings.TrimSpace(refined.occurrence) == "" || strings.TrimSpace(refined.actorObservation) == "" {
\t\treturn "blank_required_field"
\t}
\tif developmentNormalizeSituationKey(original.noveltyKey) != developmentNormalizeSituationKey(refined.noveltyKey) {
\t\treturn "novelty_key_changed"
\t}
\tif developmentSituationTextSimilarity(original.occurrence, refined.occurrence) < .18 {
\t\treturn "occurrence_semantics_changed"
\t}
\tif developmentSituationTextSimilarity(original.actorObservation, refined.actorObservation) < .12 {
\t\treturn "actor_observation_changed"
\t}
\tif !developmentRefinementUsesSelectedReferent(refined, selectedName) {
\t\treturn "selected_referent_missing"
\t}
\treturn ""
}

'''
if helper not in s:
    s = s.replace(marker, helper + marker)

s = s.replace('''\t\tfor _, evidence := range grounding[item.eventID] {
\t\t\texistingFacts = append(existingFacts, "GROUNDING VERIFIED HISTORICAL EVIDENCE - ALLOWED REAL REFERENTS: "+evidence)
\t\t}''', '''\t\tfor _, evidence := range grounding[item.eventID] {
\t\t\texistingFacts = append(existingFacts, evidence)
\t\t}''')

s = s.replace('候補を2〜5件返してください。', '候補を2〜4件返してください。')

p.write_text(s)

p = Path('apps/server/internal/worldrepo/materialization_search_grounding_test.go')
s = p.read_text()
s += '''\nfunc TestDevelopmentSearchGroundingRejectionReasonRequiresSelectedReferent(t *testing.T) {
\toriginal := developmentSituationProposal{objectClass: "branching game", occurrence: "A different route changed the result.", actorObservation: "She tried another route and saw a different result.", noveltyKey: "route-result"}
\trefined := original
\tif got := developmentSearchGroundingRejectionReason(original, refined, true, "かまいたちの夜"); got != "selected_referent_missing" {
\t\tt.Fatalf("reason=%q want selected_referent_missing", got)
\t}
\trefined.objectClass = "かまいたちの夜の分岐"
\trefined.occurrence = "In かまいたちの夜, a different route changed the result."
\tif got := developmentSearchGroundingRejectionReason(original, refined, true, "かまいたちの夜"); got != "" {
\t\tt.Fatalf("reason=%q want acceptance", got)
\t}
}
'''
p.write_text(s)
