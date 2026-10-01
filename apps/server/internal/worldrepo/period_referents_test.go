package worldrepo
import ("strings"; "testing")

func TestPeriodSupportPreservesOffAndExplicitTexture(t *testing.T) {
	off := (LLMMaterializer{}).withPeriodReferents("1996-08-29")
	if len(off.HistoricalTexture) != 0 || !strings.Contains(off.planningEraRules(), "HISTORICAL_REFERENCES=OFF") { t.Fatal("off changed") }
	m := LLMMaterializer{CuratedHistoricalReferences: true, HistoricalTexture: []string{"fixture"}}
	copy := m.withPeriodReferents("1996-08-29")
	copy.HistoricalTexture[0] = "changed"
	if m.HistoricalTexture[0] != "fixture" { t.Fatal("shared explicit texture mutated") }
	if got := m.withPeriodReferents("invalid"); len(got.HistoricalTexture) != 1 { t.Fatal("invalid date added evidence") }
}
