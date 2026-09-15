package historicalkb

import "time"

type Status string

const (
	StatusProvisional       Status = "provisional"
	StatusNeedsReview       Status = "needs_review"
	StatusOperatorVerified  Status = "operator_verified"
	StatusCanonical         Status = "canonical"
	StatusRejected          Status = "rejected"
)

type SourceEvidence struct {
	URL   string `json:"url"`
	Title string `json:"title"`
}

type Message struct {
	Role      string    `json:"role"`
	Body      string    `json:"body"`
	CreatedAt time.Time `json:"createdAt"`
}

type ResearchCase struct {
	ID                string           `json:"id"`
	Topic             string           `json:"topic"`
	Question          string           `json:"question"`
	WorldDate         string           `json:"worldDate"`
	Status            Status           `json:"status"`
	Summary           string           `json:"summary"`
	ProvisionalAnswer string           `json:"provisionalAnswer"`
	MissingInfo       []string         `json:"missingInfo"`
	Confidence        float64          `json:"confidence"`
	Sources           []SourceEvidence `json:"sources"`
	Messages          []Message        `json:"messages"`
	CreatedAt         time.Time        `json:"createdAt"`
	UpdatedAt         time.Time        `json:"updatedAt"`
}

type ResearchResult struct {
	Summary           string           `json:"summary"`
	ProvisionalAnswer string           `json:"provisionalAnswer"`
	MissingInfo       []string         `json:"missingInfo"`
	Confidence        float64          `json:"confidence"`
	Sources           []SourceEvidence `json:"sources"`
}
