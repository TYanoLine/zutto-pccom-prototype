package historicalkb

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"time"
)

type EvidenceLevel string

const (
	EvidenceAtmospheric EvidenceLevel = "atmospheric"
	EvidencePlausible   EvidenceLevel = "plausible"
	EvidenceVerified    EvidenceLevel = "verified"
)

type KnowledgeKind string

const (
	KnowledgeGeneral             KnowledgeKind = "general"
	KnowledgeProductAvailability KnowledgeKind = "product_availability"
	KnowledgePriceRange          KnowledgeKind = "price_range"
	KnowledgeTechnicalCapability KnowledgeKind = "technical_capability"
	KnowledgeCulturalSignal      KnowledgeKind = "cultural_signal"
	KnowledgeHistoricalEvent     KnowledgeKind = "historical_event"
	KnowledgeTerminology         KnowledgeKind = "terminology"
)

type FactStatus string

const (
	FactProvisional      FactStatus = "provisional"
	FactVerified         FactStatus = "verified"
	FactOperatorVerified FactStatus = "operator_verified"
	FactCanonical        FactStatus = "canonical"
	FactRejected         FactStatus = "rejected"
)

type KnowledgeQuery struct {
	Kind             KnowledgeKind `json:"kind"`
	Subject          string        `json:"subject"`
	WorldDate        string        `json:"worldDate"`
	Region           string        `json:"region"`
	Audience         []string      `json:"audience,omitempty"`
	Need             string        `json:"need"`
	RequiredEvidence EvidenceLevel `json:"requiredEvidence"`
}

type HistoricalFact struct {
	ID           string           `json:"id"`
	KnowledgeKey string           `json:"knowledgeKey"`
	Kind         KnowledgeKind    `json:"kind"`
	Subject      string           `json:"subject"`
	Claim        string           `json:"claim"`
	ValidFrom    string           `json:"validFrom,omitempty"`
	ValidUntil   string           `json:"validUntil,omitempty"`
	Region       string           `json:"region"`
	Audience     []string         `json:"audience,omitempty"`
	Confidence   float64          `json:"confidence"`
	Status       FactStatus       `json:"status"`
	Sources      []SourceEvidence `json:"sources"`
	ResearchID   string           `json:"researchId,omitempty"`
	CreatedAt    time.Time        `json:"createdAt"`
	UpdatedAt    time.Time        `json:"updatedAt"`
}

type KnowledgeGap struct { Description string `json:"description"` }

type KnowledgeResult struct {
	Query           KnowledgeQuery   `json:"query"`
	Facts           []HistoricalFact `json:"facts"`
	Coverage        float64          `json:"coverage"`
	Confidence      float64          `json:"confidence"`
	Missing         []KnowledgeGap   `json:"missing,omitempty"`
	Researched      bool             `json:"researched"`
	ResearchPending bool             `json:"researchPending"`
	ResearchID      string           `json:"researchId,omitempty"`
	CanUse          bool             `json:"canUse"`
}

// KnowledgeKey identifies a reusable historical concept. Date validity belongs
// to the fact rows, so a fact learned on one day can serve later world dates.
func KnowledgeKey(q KnowledgeQuery) string {
	parts := []string{strings.ToLower(strings.TrimSpace(string(q.Kind))), strings.ToLower(strings.TrimSpace(q.Subject)), strings.ToLower(strings.TrimSpace(q.Region)), strings.ToLower(strings.Join(q.Audience, ","))}
	sum := sha256.Sum256([]byte(strings.Join(parts, "|")))
	return hex.EncodeToString(sum[:16])
}

// ResearchKey is narrower than KnowledgeKey so concurrent research for different
// historical slices cannot incorrectly block each other.
func ResearchKey(q KnowledgeQuery) string { return KnowledgeKey(q)+"|"+strings.TrimSpace(q.WorldDate) }
