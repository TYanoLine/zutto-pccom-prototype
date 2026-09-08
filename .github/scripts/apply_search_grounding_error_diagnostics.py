from pathlib import Path

p = Path('apps/server/internal/worldrepo/materialization_search_grounding.go')
s = p.read_text()

old = '''type developmentSearchGroundingStats struct {
\tQueries          int
\tEvidenceHits     int
\tCandidateOptions int
\tSelections       int
\tRefined          int
\tRejected         int
\tSearchFailures   int
\tRefineFailures   int
}'''
new = '''type developmentSearchGroundingStats struct {
\tQueries          int
\tEvidenceHits     int
\tCandidateOptions int
\tSelections       int
\tRefined          int
\tRejected         int
\tSearchFailures   int
\tRefineFailures   int
\tSearchErrors     []string
\tRefineErrors     []string
}'''
if old not in s:
    raise SystemExit('stats block not found')
s = s.replace(old, new)

old = '''\tstats := value.(developmentSearchGroundingStats)
\treturn fmt.Sprintf("search_queries=%d search_hits=%d candidate_options=%d grounded_selections=%d grounded_refinements=%d grounding_rejected=%d search_failures=%d refine_failures=%d", stats.Queries, stats.EvidenceHits, stats.CandidateOptions, stats.Selections, stats.Refined, stats.Rejected, stats.SearchFailures, stats.RefineFailures)
}'''
new = '''\tstats := value.(developmentSearchGroundingStats)
\tdiagnostic := fmt.Sprintf("search_queries=%d search_hits=%d candidate_options=%d grounded_selections=%d grounded_refinements=%d grounding_rejected=%d search_failures=%d refine_failures=%d", stats.Queries, stats.EvidenceHits, stats.CandidateOptions, stats.Selections, stats.Refined, stats.Rejected, stats.SearchFailures, stats.RefineFailures)
\tif len(stats.SearchErrors) > 0 {
\t\terrors := append([]string(nil), stats.SearchErrors...)
\t\tsort.Strings(errors)
\t\tdiagnostic += " search_errors=" + strings.Join(errors, " | ")
\t}
\tif len(stats.RefineErrors) > 0 {
\t\terrors := append([]string(nil), stats.RefineErrors...)
\t\tsort.Strings(errors)
\t\tdiagnostic += " refine_errors=" + strings.Join(errors, " | ")
\t}
\treturn diagnostic
}'''
if old not in s:
    raise SystemExit('diagnostic block not found')
s = s.replace(old, new)

old = '''\t\t\tif err != nil {
\t\t\t\tmu.Lock()
\t\t\t\tstats.SearchFailures++
\t\t\t\tmu.Unlock()
\t\t\t\treturn
\t\t\t}'''
new = '''\t\t\tif err != nil {
\t\t\t\tmu.Lock()
\t\t\t\tstats.SearchFailures++
\t\t\t\tstats.SearchErrors = append(stats.SearchErrors, developmentSearchGroundingError(item.eventID, err))
\t\t\t\tmu.Unlock()
\t\t\t\treturn
\t\t\t}'''
if old not in s:
    raise SystemExit('search error block not found')
s = s.replace(old, new, 1)

old = '''\tplan, err := refiner.RefineDevelopmentWorldSituationsWithSearchGrounding(ctx, host, r.WorldDate, refineRoots, factsByPersona, planningBBSState(r.Base.ListPosts(host.ID), 24), accepted, grounding)
\tif err != nil {
\t\tstats.RefineFailures++
\t\tstoreStats()
\t\treturn accepted, GenerationUsage{}
\t}'''
new = '''\tplan, err := refiner.RefineDevelopmentWorldSituationsWithSearchGrounding(ctx, host, r.WorldDate, refineRoots, factsByPersona, planningBBSState(r.Base.ListPosts(host.ID), 24), accepted, grounding)
\tif err != nil {
\t\tstats.RefineFailures++
\t\tstats.RefineErrors = append(stats.RefineErrors, developmentSearchGroundingError("refine", err))
\t\tstoreStats()
\t\treturn accepted, GenerationUsage{}
\t}'''
if old not in s:
    raise SystemExit('refine error block not found')
s = s.replace(old, new)

insert_before = '\nfunc developmentSearchGroundingEvidenceRequest('
helper = r'''
func developmentSearchGroundingError(eventID string, err error) string {
\tif err == nil {
\t\treturn strings.TrimSpace(eventID) + ":unknown error"
\t}
\tmessage := strings.Join(strings.Fields(err.Error()), " ")
\trunes := []rune(message)
\tif len(runes) > 220 {
\t\tmessage = string(runes[:220]) + "…"
\t}
\treturn strings.TrimSpace(eventID) + ":" + message
}
'''
if insert_before not in s:
    raise SystemExit('evidence function marker not found')
s = s.replace(insert_before, '\n' + helper + insert_before, 1)
p.write_text(s)

p = Path('apps/server/internal/worldrepo/materialization_search_grounding_test.go')
s = p.read_text()
s += r'''

func TestDevelopmentSearchGroundingErrorCompactsAndBoundsMessage(t *testing.T) {
\tgot := developmentSearchGroundingError("event-1", fmt.Errorf("first line\\n%s", strings.Repeat("x", 300)))
\tif !strings.HasPrefix(got, "event-1:first line") {
\t\tt.Fatalf("unexpected compact error: %q", got)
\t}
\tif strings.Contains(got, "\\n") {
\t\tt.Fatalf("error should be single-line: %q", got)
\t}
\tif len([]rune(got)) > 240 {
\t\tt.Fatalf("error not bounded: %d %q", len([]rune(got)), got)
\t}
}
'''
# Existing test imports use a standalone strings import and a grouped block; add fmt to the grouped block.
s = s.replace('import (\n\t"testing"', 'import (\n\t"fmt"\n\t"testing"', 1)
p.write_text(s)
