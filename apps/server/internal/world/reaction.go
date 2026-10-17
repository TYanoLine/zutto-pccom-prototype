package world

import (
	"context"
	"fmt"
	"hash/fnv"
	"math"
	"sort"
	"strings"
	"time"
)

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
	Count        int               `json:"count,omitempty"`
	Appeal       int               `json:"appeal,omitempty"`
	Judged       bool              `json:"judged,omitempty"`
	Reason       string            `json:"reason,omitempty"`
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

// PlanInput bundles all deterministic inputs needed by PlanReactions.
type PlanInput struct {
	Host               Host
	Board              Board
	TargetPost         Post
	Ordinal            int
	ReplyRate          float64
	ReplyCountOverride *int
	ExistingPlan       *ReactionPlan
	Appeal             int
	Judged             bool
	Reason             string
	PlannedAt          time.Time
	Personas           []Persona
	FactsByPersona     map[string][]PersonaFact
	BoardPrior         map[string]float64
	Topics             []string
	Lexicon            *Lexicon
	Salience           []Salience
	RhythmLayers       []RhythmLayer
	MemberMix          map[string]float64
	Thread             ThreadState
	Advices            map[string]float64
}

// AdviceRequest is the input to ReactionAdvisor.AdviseReactors (plan 4.7).
type AdviceRequest struct {
	WorldDate  string
	Host       Host
	Board      Board
	Post       Post
	Candidates []Persona
}

// ReactionAdvisor is an optional plug-in for tagging open-vocabulary topics
// on untagged human posts and supplying a bounded prior score in [0, 1] for
// candidate reactors (plan 4.7). When nil (the default), Advice = 1.0.
type ReactionAdvisor interface {
	TagTopics(ctx context.Context, post Post) ([]string, error)
	AdviseReactors(ctx context.Context, req AdviceRequest) (map[string]float64, error)
}

// CandidatePoolForPlan returns the deterministic top-K candidate persona pool
// (capped at ReactionCandidateCap) for a PlanInput so an optional
// ReactionAdvisor can query priors for those candidates in a single batch.
func CandidatePoolForPlan(in PlanInput) []Persona {
	if len(in.Personas) == 0 {
		return nil
	}
	plannedAt := in.PlannedAt
	if plannedAt.IsZero() {
		plannedAt = in.TargetPost.CreatedAt
	}
	rhythmLayers := in.RhythmLayers
	if len(rhythmLayers) == 0 {
		rhythmLayers = DefaultRhythmLayers()
	}
	memberMix := in.MemberMix
	if len(memberMix) == 0 {
		if len(in.Board.Rhythm.MemberMix) > 0 {
			memberMix = in.Board.Rhythm.MemberMix
		} else if sr := StationRhythmForHost(in.Host.ID); len(sr.MemberMix) > 0 {
			memberMix = sr.MemberMix
		} else {
			memberMix = DefaultMemberMix()
		}
	}
	archByPersona := make(map[string]string, len(in.Personas))
	for _, p := range in.Personas {
		archByPersona[p.ID] = AssignArchetype(p, in.FactsByPersona[p.ID], memberMix)
	}
	return selectCandidatePersonas(in.Personas, func(p Persona) float64 {
		r := ComposeRhythm(rhythmLayers, RhythmContext{
			Host:      in.Host,
			Board:     in.Board,
			Persona:   p,
			Archetype: archByPersona[p.ID],
			At:        plannedAt,
		})
		return PersonaActivityWeight(p.ActivityPattern, p.LurkerTendency) * r
	}, ReactionCandidateCap, in.Thread.StarterPersonaID)
}

// NormalizeTopicKey normalizes an open-vocabulary topic key to lowercase
// without surrounding whitespace.
func NormalizeTopicKey(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.ReplaceAll(s, " ", "_")
	return s
}

// ArticleTopicWeights computes normalized topic weights T_a(x) for an article
// from the board's prior distribution, the article's topic tags (including
// broader propagation from lex), and date-bounded salience boosts (plan 4.1).
func ArticleTopicWeights(boardPrior map[string]float64, topics []string, lex *Lexicon, salience []Salience, on time.Time) map[string]float64 {
	tag := map[string]float64{}
	for _, raw := range topics {
		k := NormalizeTopicKey(raw)
		if k == "" {
			continue
		}
		tag[k] = 1.0
		for _, b := range lex.Broader(k) {
			if tag[b] < ReactionBroaderShare {
				tag[b] = ReactionBroaderShare
			}
		}
	}

	expandedPrior := map[string]float64{}
	for k, v := range boardPrior {
		nk := NormalizeTopicKey(k)
		if nk == "" || v <= 0 {
			continue
		}
		if v > expandedPrior[nk] {
			expandedPrior[nk] = v
		}
		for _, b := range lex.Broader(nk) {
			if bv := v * ReactionBroaderShare; bv > expandedPrior[b] {
				expandedPrior[b] = bv
			}
		}
	}

	keys := map[string]bool{}
	for k := range expandedPrior {
		keys[k] = true
	}
	for k := range tag {
		keys[k] = true
	}
	if len(keys) == 0 {
		return nil
	}

	var salList []Salience
	if lex != nil && len(lex.Salience) > 0 {
		salList = append(salList, lex.Salience...)
	}
	if len(salience) > 0 {
		salList = append(salList, salience...)
	}
	dateStr := ""
	if !on.IsZero() {
		dateStr = on.Format("2006-01-02")
	}
	salBoost := func(topic string) float64 {
		if dateStr == "" || len(salList) == 0 {
			return 1.0
		}
		b := 1.0
		for _, s := range salList {
			if NormalizeTopicKey(s.Topic) != topic || s.Boost <= 0 {
				continue
			}
			if s.From != "" && dateStr < s.From {
				continue
			}
			if s.To != "" && dateStr > s.To {
				continue
			}
			b *= s.Boost
		}
		return b
	}

	rawWeights := make(map[string]float64, len(keys))
	total := 0.0
	for k := range keys {
		prior := expandedPrior[k]
		t := tag[k]
		if prior <= 0 {
			if t <= 0 {
				continue
			}
			prior = 1.0
		}
		var factor float64
		if len(expandedPrior) == 0 && t > 0 && t < 1.0 {
			// Propagate broader topics at half the direct topic weight when no board prior is set.
			factor = (1.0 + ReactionTagWeightBoost) * t
		} else {
			factor = 1.0 + ReactionTagWeightBoost*t
		}
		w := prior * factor * salBoost(k)
		if w > 0 {
			rawWeights[k] = w
			total += w
		}
	}
	if total <= 0 {
		return nil
	}
	for k, w := range rawWeights {
		rawWeights[k] = w / total
	}
	return rawWeights
}

// Relevance computes R(p,a) in [0, 1] as the overlap between an article's
// normalized topic weights and a persona's interests, opinions, and confirmed
// PersonaFact topics (plan 4.2).
func Relevance(p Persona, facts []PersonaFact, weights map[string]float64) float64 {
	if len(weights) == 0 {
		return 0
	}
	personaTopics := map[string]float64{}
	for k, v := range p.Interests {
		nk := NormalizeTopicKey(k)
		if nk == "" || v <= 0 {
			continue
		}
		if cv := clamp01(v); cv > personaTopics[nk] {
			personaTopics[nk] = cv
		}
	}
	for k, v := range p.Opinions {
		nk := NormalizeTopicKey(k)
		if nk == "" {
			continue
		}
		if cv := clamp01(math.Abs(v)); cv > personaTopics[nk] {
			personaTopics[nk] = cv
		}
	}
	for _, f := range facts {
		if f.PersonaID != "" && p.ID != "" && f.PersonaID != p.ID {
			continue
		}
		nk := NormalizeTopicKey(f.Topic)
		if nk == "" {
			continue
		}
		if ReactionFactTopicWeight > personaTopics[nk] {
			personaTopics[nk] = ReactionFactTopicWeight
		}
	}
	if len(personaTopics) == 0 {
		return 0
	}
	sum := 0.0
	for k, tw := range weights {
		if tw <= 0 {
			continue
		}
		nk := NormalizeTopicKey(k)
		if pw, ok := personaTopics[nk]; ok && pw > 0 {
			sum += tw * pw
		}
	}
	return clamp01(sum)
}

// PersonaActivityWeight returns the baseline activity weight for a persona's
// activity pattern and lurker tendency.
func PersonaActivityWeight(pattern string, lurker float64) float64 {
	var base float64
	switch strings.ToLower(strings.TrimSpace(pattern)) {
	case "regular":
		base = 1.00
	case "active":
		base = 0.82
	case "occasional":
		base = 0.56
	case "lurker":
		base = 0.05
	case "dormant":
		base = 0.015
	default:
		base = 0.48
	}
	if lurker > 0 {
		base *= 1.0 - 0.50*clamp01(lurker)
	}
	return base
}

// ReactionWeight computes the sampling weight w(p) for candidate p on the k-th
// reply (1-based) of a thread (plan 4.3).
func ReactionWeight(p Persona, rel, rhythm, advice float64, st ThreadState, k int) float64 {
	if p.ID != "" {
		if st.LastPersonaID != "" && p.ID == st.LastPersonaID {
			return 0
		}
		if st.ParentPersonaID != "" && p.ID == st.ParentPersonaID {
			// At k == 1, or when ParentPersonaID is not a returning thread starter,
			// the parent post's author cannot reply to themselves.
			isReturningStarter := k >= 2 && st.StarterPersonaID != "" && p.ID == st.StarterPersonaID && st.LastPersonaID != ""
			if !isReturningStarter {
				return 0
			}
		}
	}
	if rhythm <= 0 {
		rhythm = 1.0
	}
	if advice <= 0 {
		advice = 1.0
	}

	activity := PersonaActivityWeight(p.ActivityPattern, p.LurkerTendency) * rhythm
	relTerm := math.Pow(ReactionEpsilon+clamp01(rel), ReactionAlpha)
	replyTerm := ReactionDelta + clamp01(p.ReplyTendency)

	stateMult := 1.0
	if p.ID != "" && len(st.PriorReplyCounts) > 0 {
		if n := st.PriorReplyCounts[p.ID]; n > 0 {
			stateMult *= math.Pow(ReactionRepeatDecay, float64(n))
		}
	}
	if k >= 2 && p.Argumentativeness > 0 {
		stateMult *= 1.0 + ReactionArgumentFactor*clamp01(p.Argumentativeness)
	}
	if st.IsNewcomerPost && p.NewcomerOpenness > 0 {
		stateMult *= 1.0 + ReactionNewcomerFactor*clamp01(p.NewcomerOpenness)
	}
	if k >= 2 && p.ID != "" && st.StarterPersonaID != "" && p.ID == st.StarterPersonaID {
		stateMult *= ReactionThreadStarterMult
	}
	relEdge := st.Relationship
	if relEdge <= 0 {
		relEdge = ReactionRelationNeutral
	}
	stateMult *= relEdge

	w := activity * relTerm * replyTerm * stateMult * advice
	if w <= 0 || math.IsNaN(w) || math.IsInf(w, 0) {
		return 0
	}
	return w
}

// PickReactor deterministically selects one persona from candidates using
// Efraimidis-Spirakis weighted sampling without replacement (plan 4.4).
func PickReactor(seed string, candidates []Weighted) (personaID string, ok bool) {
	bestLogKey := math.Inf(-1)
	bestID := ""
	for _, c := range candidates {
		if c.PersonaID == "" || c.Weight <= 0 || math.IsNaN(c.Weight) {
			continue
		}
		u := reactionUnit(seed, c.PersonaID)
		// ln(u^(1/w)) = ln(u) / w, strictly monotonic with u^(1/w) and free of underflow.
		logKey := math.Log(u) / c.Weight
		if bestID == "" || logKey > bestLogKey || (logKey == bestLogKey && c.PersonaID < bestID) {
			bestLogKey = logKey
			bestID = c.PersonaID
		}
	}
	if bestID == "" {
		return "", false
	}
	return bestID, true
}

func patternDelayScale(pattern string) float64 {
	switch strings.ToLower(strings.TrimSpace(pattern)) {
	case "regular":
		return 0.85
	case "active":
		return 1.00
	case "occasional":
		return 1.25
	case "lurker":
		return 1.60
	case "dormant":
		return 2.00
	default:
		return 1.00
	}
}

// ReactionDueAt computes the scheduled timestamp for the k-th reply by
// advancing a heavy-tailed effective delay along the cumulative rhythm
// integral Λ(t) = ∫ Rhythm(p, s) ds starting at from (plan 4.5).
func ReactionDueAt(seed string, from time.Time, r RhythmFunc, pattern string, k int) time.Time {
	u := reactionUnit(seed, fmt.Sprintf("gap|%d", k))
	if u < 0.002 {
		u = 0.002
	}
	effMinutes := ReactionBaseDelayMinutes * math.Pow(u, ReactionDelayTailExponent) * patternDelayScale(pattern)
	effDur := time.Duration(effMinutes * float64(time.Minute))
	if effDur < ReactionMinDelay {
		effDur = ReactionMinDelay
	}
	if effDur > ReactionMaxEffectiveDelay {
		effDur = ReactionMaxEffectiveDelay
	}
	if r == nil {
		return from.Add(effDur)
	}

	remainingSec := effDur.Seconds()
	cur := from
	const sliceSec = 10 * 60.0 // 10-minute integration slices
	maxSteps := 14 * 24 * 6    // guard: 14 days
	for step := 0; step < maxSteps && remainingSec > 0; step++ {
		rate := r(cur)
		if math.IsNaN(rate) || rate <= 0 {
			rate = 1.0
		}
		rate = clampFloat(rate, RhythmMinFactor, RhythmMaxFactor)
		effSlice := sliceSec * rate
		if effSlice >= remainingSec {
			realSec := remainingSec / rate
			cur = cur.Add(time.Duration(realSec * float64(time.Second)))
			remainingSec = 0
			break
		}
		remainingSec -= effSlice
		cur = cur.Add(time.Duration(sliceSec * float64(time.Second)))
	}
	if !cur.After(from) {
		cur = from.Add(ReactionMinDelay)
	}
	return cur
}

// QuantizeAdvice rounds an advice score in [0, 1] to the nearest 0.05 increment (plan 4.7).
func QuantizeAdvice(v float64) float64 {
	v = clamp01(v)
	return math.Round(v/ReactionAdviceQuantum) * ReactionAdviceQuantum
}

// AdviceMultiplier converts a quantized advice score in [0, 1] into its bounded
// weight multiplier 1 + β*(advice - 0.5)*2 with β <= 0.4 (plan 4.7).
func AdviceMultiplier(quantized float64) float64 {
	q := clamp01(quantized)
	return 1.0 + ReactionAdviceBeta*(q-0.5)*2.0
}

// PlanReactions computes the complete deterministic ReactionPlan for a post (plan 2 & 4).
func PlanReactions(in PlanInput) ReactionPlan {
	plannedAt := in.PlannedAt
	if plannedAt.IsZero() {
		plannedAt = in.TargetPost.CreatedAt
	}
	ordinal := in.Ordinal
	replyRate := in.ReplyRate
	plan := ReactionPlan{
		TargetPostID: in.TargetPost.ID,
		BoardID:      in.Board.ID,
		Ordinal:      ordinal,
		PlannedAt:    plannedAt,
	}
	n := ThreadReplyCount(in.Host.ID, in.Board.ID, ordinal, replyRate)
	if in.ExistingPlan != nil && in.ExistingPlan.Judged {
		n = in.ExistingPlan.Count
		plan.Count = in.ExistingPlan.Count
		plan.Appeal = in.ExistingPlan.Appeal
		plan.Judged = true
		plan.Reason = in.ExistingPlan.Reason
	} else if in.Judged {
		if in.ReplyCountOverride != nil {
			n = *in.ReplyCountOverride
		}
		plan.Count = n
		plan.Appeal = in.Appeal
		plan.Judged = true
		plan.Reason = in.Reason
	} else if in.ReplyCountOverride != nil {
		n = *in.ReplyCountOverride
		plan.Count = n
	} else {
		plan.Count = n
	}
	if n <= 0 || len(in.Personas) == 0 {
		return plan
	}

	lex := in.Lexicon
	if lex == nil {
		lex = DefaultLexicon()
	}
	rhythmLayers := in.RhythmLayers
	if len(rhythmLayers) == 0 {
		rhythmLayers = DefaultRhythmLayers()
	}
	boardPrior := in.BoardPrior
	if len(boardPrior) == 0 && len(in.Board.Topics) > 0 {
		boardPrior = in.Board.Topics
	}
	memberMix := in.MemberMix
	if len(memberMix) == 0 {
		if len(in.Board.Rhythm.MemberMix) > 0 {
			memberMix = in.Board.Rhythm.MemberMix
		} else if sr := StationRhythmForHost(in.Host.ID); len(sr.MemberMix) > 0 {
			memberMix = sr.MemberMix
		} else {
			memberMix = DefaultMemberMix()
		}
	}

	topics := append([]string(nil), in.Topics...)
	topics = append(topics, ExtractPostTopics(in.TargetPost, lex)...)
	topicWeights := ArticleTopicWeights(boardPrior, topics, lex, in.Salience, plannedAt)

	archByPersona := make(map[string]string, len(in.Personas))
	for _, p := range in.Personas {
		archByPersona[p.ID] = AssignArchetype(p, in.FactsByPersona[p.ID], memberMix)
	}

	// Candidate pool: top K personas by activity × rhythm at plannedAt (plan 2 & 4.3).
	candidatesPool := selectCandidatePersonas(in.Personas, func(p Persona) float64 {
		r := ComposeRhythm(rhythmLayers, RhythmContext{
			Host:      in.Host,
			Board:     in.Board,
			Persona:   p,
			Archetype: archByPersona[p.ID],
			At:        plannedAt,
		})
		return PersonaActivityWeight(p.ActivityPattern, p.LurkerTendency) * r
	}, ReactionCandidateCap, in.Thread.StarterPersonaID)

	personaByID := make(map[string]Persona, len(candidatesPool))
	for _, p := range candidatesPool {
		personaByID[p.ID] = p
	}

	st := ThreadState{
		StarterPersonaID: in.Thread.StarterPersonaID,
		ParentPersonaID:  in.Thread.ParentPersonaID,
		LastPersonaID:    in.Thread.LastPersonaID,
		PriorReplyCounts: make(map[string]int, len(in.Thread.PriorReplyCounts)),
		IsNewcomerPost:   in.Thread.IsNewcomerPost || in.Board.WelcomesNewcomers,
		Relationship:     in.Thread.Relationship,
	}
	for k, v := range in.Thread.PriorReplyCounts {
		st.PriorReplyCounts[k] = v
	}
	if st.ParentPersonaID == "" && in.TargetPost.AuthorPersonaID != "" {
		st.ParentPersonaID = in.TargetPost.AuthorPersonaID
	}
	if st.StarterPersonaID == "" && in.TargetPost.ParentID == 0 && in.TargetPost.AuthorPersonaID != "" {
		st.StarterPersonaID = in.TargetPost.AuthorPersonaID
	}

	curTime := in.TargetPost.CreatedAt
	if curTime.IsZero() {
		curTime = plannedAt
	}

	replies := make([]PlannedReaction, 0, n)
	for k := 1; k <= n; k++ {
		seed := fmt.Sprintf("%s|%s|%d|%d|%d", in.Host.ID, in.Board.ID, in.TargetPost.ID, ordinal, k)
		isWildcard := reactionUnit(seed, "wildcard") < ReactionWildcard

		weighted := make([]Weighted, 0, len(candidatesPool))
		for _, p := range candidatesPool {
			var rel float64
			if isWildcard {
				rel = 0.5
			} else {
				rel = Relevance(p, in.FactsByPersona[p.ID], topicWeights)
			}
			rhythm := ComposeRhythm(rhythmLayers, RhythmContext{
				Host:      in.Host,
				Board:     in.Board,
				Persona:   p,
				Archetype: archByPersona[p.ID],
				At:        curTime,
			})
			adviceMult := 1.0
			if rawAdv, ok := in.Advices[p.ID]; ok {
				adviceMult = AdviceMultiplier(QuantizeAdvice(rawAdv))
			}
			w := ReactionWeight(p, rel, rhythm, adviceMult, st, k)
			if w > 0 {
				weighted = append(weighted, Weighted{PersonaID: p.ID, Weight: w})
			}
		}

		chosenID, ok := PickReactor(seed, weighted)
		if !ok {
			break
		}
		chosenPersona := personaByID[chosenID]
		chosenArch := archByPersona[chosenID]
		var rFunc RhythmFunc
		if len(rhythmLayers) > 0 {
			rFunc = func(t time.Time) float64 {
				return ComposeRhythm(rhythmLayers, RhythmContext{
					Host:      in.Host,
					Board:     in.Board,
					Persona:   chosenPersona,
					Archetype: chosenArch,
					At:        t,
				})
			}
		}
		dueAt := ReactionDueAt(seed+"|"+chosenID, curTime, rFunc, chosenPersona.ActivityPattern, k)
		var qAdv float64
		if rawAdv, ok := in.Advices[chosenID]; ok {
			qAdv = QuantizeAdvice(rawAdv)
		}
		replies = append(replies, PlannedReaction{
			Index:     k,
			PersonaID: chosenID,
			DueAt:     dueAt,
			Advice:    qAdv,
		})

		curTime = dueAt
		st.LastPersonaID = chosenID
		st.PriorReplyCounts[chosenID]++
	}
	plan.Replies = replies
	return plan
}

func selectCandidatePersonas(personas []Persona, scoreFn func(Persona) float64, capLimit int, ensureID string) []Persona {
	if capLimit <= 0 || len(personas) <= capLimit {
		out := make([]Persona, len(personas))
		copy(out, personas)
		return out
	}
	type scored struct {
		p     Persona
		score float64
	}
	ranked := make([]scored, len(personas))
	for i, p := range personas {
		ranked[i] = scored{p: p, score: scoreFn(p)}
	}
	sort.SliceStable(ranked, func(i, j int) bool {
		if ranked[i].score == ranked[j].score {
			return ranked[i].p.ID < ranked[j].p.ID
		}
		return ranked[i].score > ranked[j].score
	})
	out := make([]Persona, 0, capLimit+1)
	hasEnsure := false
	for i := 0; i < capLimit && i < len(ranked); i++ {
		out = append(out, ranked[i].p)
		if ensureID != "" && ranked[i].p.ID == ensureID {
			hasEnsure = true
		}
	}
	if ensureID != "" && !hasEnsure {
		for i := capLimit; i < len(ranked); i++ {
			if ranked[i].p.ID == ensureID {
				out = append(out, ranked[i].p)
				break
			}
		}
	}
	return out
}

func reactionUnit(seed, salt string) float64 {
	h := fnv.New64a()
	_, _ = fmt.Fprintf(h, "%s|%s|react-v1", seed, salt)
	return (float64(h.Sum64()%1000000) + 0.5) / 1000000.0
}

func clamp01(v float64) float64 {
	return clampFloat(v, 0, 1)
}

func clampFloat(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// ExtractPostTopics extracts deterministic topic keys from a post's structured
// intent and lexicon matches without calling an LLM.
func ExtractPostTopics(post Post, lex *Lexicon) []string {
	var out []string
	seen := map[string]bool{}
	add := func(k string) {
		nk := NormalizeTopicKey(k)
		if nk == "" || seen[nk] {
			return
		}
		seen[nk] = true
		out = append(out, nk)
	}
	for _, t := range post.Intent.Topics {
		add(t)
	}
	if post.Intent.AnchorKey != "" {
		add(post.Intent.AnchorKey)
	}
	if post.Intent.Topic != "" {
		add(post.Intent.Topic)
	}
	for _, ref := range post.Intent.ProducerReferents {
		add(ref)
	}
	for _, fact := range post.Intent.SituationFacts {
		if strings.HasPrefix(fact, "referent:") {
			add(strings.TrimPrefix(fact, "referent:"))
		} else if strings.HasPrefix(fact, "object_class=") {
			add(strings.TrimPrefix(fact, "object_class="))
		}
	}
	if lex != nil {
		for _, m := range lex.Match(post.Subject + "\n" + post.Body + "\n" + post.Intent.Topic + "\n" + post.Intent.AnchorKey + "\n" + post.Intent.SituationSummary) {
			add(m)
		}
	}
	return out
}
