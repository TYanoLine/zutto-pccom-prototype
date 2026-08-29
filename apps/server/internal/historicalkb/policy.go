package historicalkb

import "strings"

type DecisionContext struct {
	Persistence bool
	Importance  float64
	Specificity float64
	HasExactDate bool
	HasExactNumber bool
	HasProductModel bool
	HasTechnicalSpec bool
}

// RequiredEvidence keeps web research rare. Atmosphere and ordinary prose stay
// model-first; concrete persistent claims are held to stronger evidence.
func RequiredEvidence(c DecisionContext) EvidenceLevel {
	if c.Persistence && (c.HasExactDate || c.HasExactNumber || c.HasProductModel || c.HasTechnicalSpec || c.Specificity >= 0.75) {
		return EvidenceVerified
	}
	if c.Importance >= 0.55 || c.Specificity >= 0.5 {
		return EvidencePlausible
	}
	return EvidenceAtmospheric
}

func Sufficient(result KnowledgeResult, required EvidenceLevel) bool {
	switch required {
	case EvidenceAtmospheric:
		return true
	case EvidencePlausible:
		return result.Coverage >= 0.45 && result.Confidence >= 0.55 && len(result.Facts) > 0
	case EvidenceVerified:
		if result.Coverage < 0.8 || result.Confidence < 0.75 || len(result.Facts) == 0 {
			return false
		}
		for _, f := range result.Facts {
			if f.Status == FactVerified || f.Status == FactOperatorVerified || f.Status == FactCanonical {
				return true
			}
		return false
	default:
		return false
	}
}

func normalizeRegion(v string) string {
	v = strings.TrimSpace(v)
	if v == "" { return "JP" }
	return v
}
