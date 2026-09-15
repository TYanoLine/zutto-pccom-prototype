package worldrepo

import (
	"strings"
	"time"

	"zutto-pccom/apps/server/internal/historicalkb"
)

// Use the earliest event date, not the observation date, so a catch-up window
// cannot give earlier actors knowledge of a later release. Never mutate the
// materializer shared by concurrent requests or the Lab's explicit A/B list.
func (m LLMMaterializer) withPeriodReferents(dates ...string) LLMMaterializer {
	if !m.CuratedHistoricalReferences || len(dates) == 0 { return m }
	var earliest time.Time
	for _, raw := range dates {
		date, err := time.Parse("2006-01-02", raw)
		if err != nil { return m }
		if earliest.IsZero() || date.Before(earliest) { earliest = date }
	}
	facts := append([]string(nil), m.HistoricalTexture...)
	for _, item := range historicalkb.PeriodReferents(earliest.Format("2006-01-02")) {
		facts = append(facts, item.Claim)
	}
	m.HistoricalTexture = facts
	return m
}

// The semantic producer previously received permission rules but no referents,
// even when the situation proposer and article worker received the same facts.
func (m LLMMaterializer) planningEraRules() string {
	rules := m.eraRules()
	if len(m.HistoricalTexture) == 0 { return rules }
	return rules + "\nSUPPLIED HISTORICAL FACTS / TEXTURE (bounded claims, not topic quotas):\n- " + strings.Join(m.HistoricalTexture, "\n- ") + "\nUse only these stated claims. Existence does not establish ownership, purchase, experience or technical compatibility. Preserve previously established identities; do not rename unnamed canonical objects during prose rendering."
}
