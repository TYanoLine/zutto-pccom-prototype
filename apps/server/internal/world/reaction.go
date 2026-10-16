package world

import "time"

// 以下の係数はすべて架空の調整値であり、歴史統計ではない。
// All coefficients below are fictional simulation tuning values, not historical statistics.
const (
	ReactionEpsilon           = 0.15
	ReactionAlpha             = 1.5
	ReactionDelta             = 0.10
	ReactionCandidateCap      = 64
	ReactionWildcard          = 0.08
	ReactionAdviceBeta        = 0.40
	ReactionAdviceQuantum     = 0.05
	ReactionRepeatDecay       = 0.35
	ReactionArgumentFactor    = 0.80
	ReactionNewcomerFactor    = 1.00
	ReactionThreadStarterMult = 2.50
	ReactionRelationNeutral   = 1.00
	ReactionTagWeightBoost    = 2.00
	ReactionBroaderShare      = 0.50
	ReactionFactTopicWeight   = 0.80
	ReactionBaseDelayMinutes  = 15.0
	ReactionDelayTailExponent = -0.9
	ReactionMinDelay          = time.Minute
	ReactionMaxEffectiveDelay = 72 * time.Hour
	RhythmMinFactor           = 0.03
	RhythmMaxFactor           = 4.00
)

// ReactionPlan is the deterministic world-owned plan of replies that a post
// drew. It is computed without calling an LLM and persisted so later
// population or rhythm changes do not alter already-scheduled replies.
type ReactionPlan struct {
	TargetPostID int64             `json:"target_post_id"`
	BoardID      string            `json:"board_id,omitempty"`
	Ordinal      int               `json:"ordinal"`
	PlannedAt    time.Time         `json:"planned_at"`
	Replies      []PlannedReaction `json:"replies,omitempty"`
}

// PlannedReaction is one scheduled reply within a ReactionPlan.
type PlannedReaction struct {
	Index              int       `json:"index"`
	PersonaID          string    `json:"persona_id"`
	DueAt              time.Time `json:"due_at"`
	Advice             float64   `json:"advice,omitempty"`
	MaterializedPostID int64     `json:"materialized_post_id,omitempty"`
}

// Salience boosts a topic key during a date range (YYYY-MM-DD, inclusive).
type Salience struct {
	Topic string  `yaml:"topic" json:"topic"`
	From  string  `yaml:"from" json:"from"`
	To    string  `yaml:"to" json:"to"`
	Boost float64 `yaml:"boost" json:"boost"`
}

// ThreadState captures the current state of a thread when selecting the k-th
// replier (1-based).
type ThreadState struct {
	StarterPersonaID string
	ParentPersonaID  string
	LastPersonaID    string
	PriorReplyCounts map[string]int
	IsNewcomerPost   bool
	Relationship     float64
}

// Weighted pairs a candidate persona ID with its sampling weight.
type Weighted struct {
	PersonaID string
	Weight    float64
}

// RhythmFunc returns a persona's relative activity factor at world time t.
type RhythmFunc func(t time.Time) float64
