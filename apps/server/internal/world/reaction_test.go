package world

import (
	"fmt"
	"math"
	"math/rand"
	"reflect"
	"testing"
	"time"
)

func testPopulation(seed int64, count int) []Persona {
	out := make([]Persona, count)
	for i := 0; i < count; i++ {
		rng := rand.New(rand.NewSource(seed + int64(i+1)*7919))
		id := fmt.Sprintf("canal-member-%03d", i+1)
		out[i] = newPersonaSkeleton(rng, id, fmt.Sprintf("USER%03d", i+1))
	}
	return out
}

func TestPlanReactionsDeterminism(t *testing.T) {
	// Property 1: Same inputs produce deep-equal ReactionPlan
	pop := testPopulation(42, 200)
	in := PlanInput{
		Host:         Host{ID: "canal", Region: "fukuoka"},
		Board:        Board{ID: "pc-tech"},
		TargetPost:   Post{ID: 101, BoardID: "pc-tech", Subject: "PC-9821のメモリ増設", Body: "Cバスにボードを挿したのですが", CreatedAt: time.Date(1996, 5, 10, 23, 15, 0, 0, jstZone)},
		Ordinal:      3, // ThreadReplyCount("canal", "pc-tech", 3, 1.0) > 0
		ReplyRate:    1.8,
		PlannedAt:    time.Date(1996, 5, 10, 23, 15, 0, 0, jstZone),
		Personas:     pop,
		BoardPrior:   map[string]float64{"computers": 1.0, "retro_tech": 0.6},
		Topics:       []string{"pc98"},
		Lexicon:      DefaultLexicon(),
		RhythmLayers: DefaultRhythmLayers(),
		MemberMix:    DefaultMemberMix(),
	}

	p1 := PlanReactions(in)
	p2 := PlanReactions(in)
	if !reflect.DeepEqual(p1, p2) {
		t.Fatalf("expected identical ReactionPlan across repeated calls:\np1=%+v\np2=%+v", p1, p2)
	}
	if len(p1.Replies) == 0 {
		t.Fatalf("expected non-empty replies for ordinal 3")
	}
	prevDue := in.TargetPost.CreatedAt
	for i, r := range p1.Replies {
		if r.Index != i+1 {
			t.Fatalf("expected reply index %d, got %d", i+1, r.Index)
		}
		if !r.DueAt.After(prevDue) {
			t.Fatalf("expected DueAt[%d]=%v > prev=%v", i, r.DueAt, prevDue)
		}
		prevDue = r.DueAt
	}
}

func TestReactionWeightSelfAndImmediateExclusion(t *testing.T) {
	// Property 2: Parent author (at k==1 or non-starter) and LastPersonaID always have weight 0
	const trials = 5000
	rng := rand.New(rand.NewSource(1001))
	for i := 0; i < trials; i++ {
		authorID := fmt.Sprintf("p-%d", rng.Intn(20))
		lastID := fmt.Sprintf("p-%d", rng.Intn(20))
		candidates := make([]Weighted, 0, 20)
		for j := 0; j < 20; j++ {
			p := Persona{
				ID:              fmt.Sprintf("p-%d", j),
				ActivityPattern: "active",
				ReplyTendency:   0.5,
			}
			st := ThreadState{
				StarterPersonaID: "p-starter",
				ParentPersonaID:  authorID,
				LastPersonaID:    lastID,
			}
			w := ReactionWeight(p, 0.6, 1.0, 1.0, st, 1)
			if p.ID == authorID || p.ID == lastID {
				if w != 0 {
					t.Fatalf("trial %d: expected weight 0 for author/last %q, got %f", i, p.ID, w)
				}
			}
			if w > 0 {
				candidates = append(candidates, Weighted{PersonaID: p.ID, Weight: w})
			}
		}
		chosen, ok := PickReactor(fmt.Sprintf("trial-%d", i), candidates)
		if !ok {
			t.Fatalf("trial %d: PickReactor failed", i)
		}
		if chosen == authorID || chosen == lastID {
			t.Fatalf("trial %d: forbidden persona %q was chosen", i, chosen)
		}
	}
}

func TestReactionActivitySuppressionStatistical(t *testing.T) {
	// Property 3: Over 5,000 samples on a generated population, lurker <= 5% and dormant <= 1%
	pop := testPopulation(77, 200)
	patternByID := make(map[string]string, len(pop))
	for _, p := range pop {
		patternByID[p.ID] = p.ActivityPattern
	}

	candidates := make([]Weighted, 0, len(pop))
	for _, p := range pop {
		w := ReactionWeight(p, 0.5, 1.0, 1.0, ThreadState{}, 1)
		if w > 0 {
			candidates = append(candidates, Weighted{PersonaID: p.ID, Weight: w})
		}
	}

	const samples = 5000
	counts := map[string]int{}
	for i := 0; i < samples; i++ {
		id, ok := PickReactor(fmt.Sprintf("act-stat-%d", i), candidates)
		if !ok {
			t.Fatalf("PickReactor returned false at sample %d", i)
		}
		counts[patternByID[id]]++
	}

	lurkerShare := float64(counts["lurker"]) / float64(samples)
	dormantShare := float64(counts["dormant"]) / float64(samples)
	if lurkerShare > 0.05 {
		t.Fatalf("expected lurker share <= 0.05, got %.4f (%d/%d)", lurkerShare, counts["lurker"], samples)
	}
	if dormantShare > 0.01 {
		t.Fatalf("expected dormant share <= 0.01, got %.4f (%d/%d)", dormantShare, counts["dormant"], samples)
	}
}

func TestTopicAffinityDominanceStatistical(t *testing.T) {
	// Property 4: Topic affinity dominance over 5,000 samples
	var personas []Persona
	highSet := map[string]bool{}
	lowSet := map[string]bool{}

	for i := 0; i < 30; i++ {
		id := fmt.Sprintf("high-retro-%d", i)
		highSet[id] = true
		personas = append(personas, Persona{
			ID:              id,
			ActivityPattern: "active",
			LurkerTendency:  0.15,
			ReplyTendency:   0.50,
			Interests:       map[string]float64{"retro_tech": 0.90, "computers": 0.70},
		})
	}
	for i := 0; i < 30; i++ {
		id := fmt.Sprintf("low-retro-%d", i)
		lowSet[id] = true
		personas = append(personas, Persona{
			ID:              id,
			ActivityPattern: "active",
			LurkerTendency:  0.15,
			ReplyTendency:   0.50,
			Interests:       map[string]float64{"food": 0.90, "cars": 0.70},
		})
	}

	weights := ArticleTopicWeights(map[string]float64{"retro_tech": 1.0}, []string{"retro_tech"}, DefaultLexicon(), nil, time.Date(1996, 5, 10, 22, 0, 0, 0, jstZone))
	const samples = 5000
	highCount, lowCount := 0, 0

	for i := 0; i < samples; i++ {
		seed := fmt.Sprintf("affinity-seed-%d", i)
		isWild := reactionUnit(seed, "wildcard") < ReactionWildcard
		cands := make([]Weighted, 0, len(personas))
		for _, p := range personas {
			rel := Relevance(p, nil, weights)
			if isWild {
				rel = 0.5
			}
			w := ReactionWeight(p, rel, 1.0, 1.0, ThreadState{}, 1)
			cands = append(cands, Weighted{PersonaID: p.ID, Weight: w})
		}
		id, ok := PickReactor(seed, cands)
		if !ok {
			t.Fatalf("PickReactor failed at %d", i)
		}
		if highSet[id] {
			highCount++
		} else if lowSet[id] {
			lowCount++
		}
	}

	if highCount < 3*lowCount {
		t.Fatalf("expected high-interest group (%d) >= 3x low-interest group (%d)", highCount, lowCount)
	}
	if lowCount == 0 {
		t.Fatalf("expected non-zero selections from low-interest group via epsilon/wildcard, got 0")
	}
}

func TestBehaviorParametersMonotonicity(t *testing.T) {
	// Property 7: ReplyTendency, Argumentativeness (k>=2 vs k==1), NewcomerOpenness (IsNewcomerPost)
	base := Persona{
		ID:                "p-base",
		ActivityPattern:   "active",
		LurkerTendency:    0.1,
		ReplyTendency:     0.2,
		Argumentativeness: 0.1,
		NewcomerOpenness:  0.1,
	}
	highReply := base
	highReply.ReplyTendency = 0.9
	if !(ReactionWeight(highReply, 0.5, 1.0, 1.0, ThreadState{}, 1) > ReactionWeight(base, 0.5, 1.0, 1.0, ThreadState{}, 1)) {
		t.Fatalf("expected higher ReplyTendency to increase weight")
	}

	highArg := base
	highArg.Argumentativeness = 0.9
	wArgK1 := ReactionWeight(highArg, 0.5, 1.0, 1.0, ThreadState{}, 1)
	wBaseK1 := ReactionWeight(base, 0.5, 1.0, 1.0, ThreadState{}, 1)
	if math.Abs(wArgK1-wBaseK1) > 1e-9 {
		t.Fatalf("expected Argumentativeness to have no effect at k=1 (%f vs %f)", wArgK1, wBaseK1)
	}
	wArgK2 := ReactionWeight(highArg, 0.5, 1.0, 1.0, ThreadState{}, 2)
	wBaseK2 := ReactionWeight(base, 0.5, 1.0, 1.0, ThreadState{}, 2)
	if !(wArgK2 > wBaseK2) {
		t.Fatalf("expected higher Argumentativeness to increase weight at k>=2 (%f vs %f)", wArgK2, wBaseK2)
	}

	highNew := base
	highNew.NewcomerOpenness = 0.9
	wNewFalse := ReactionWeight(highNew, 0.5, 1.0, 1.0, ThreadState{IsNewcomerPost: false}, 1)
	wBaseFalse := ReactionWeight(base, 0.5, 1.0, 1.0, ThreadState{IsNewcomerPost: false}, 1)
	if math.Abs(wNewFalse-wBaseFalse) > 1e-9 {
		t.Fatalf("expected NewcomerOpenness to have no effect when IsNewcomerPost=false")
	}
	wNewTrue := ReactionWeight(highNew, 0.5, 1.0, 1.0, ThreadState{IsNewcomerPost: true}, 1)
	wBaseTrue := ReactionWeight(base, 0.5, 1.0, 1.0, ThreadState{IsNewcomerPost: true}, 1)
	if !(wNewTrue > wBaseTrue) {
		t.Fatalf("expected higher NewcomerOpenness to increase weight when IsNewcomerPost=true")
	}
}

func TestThreadReturnAndRepeatDecay(t *testing.T) {
	// Property 8: Thread starter boost at k >= 2 (when not LastPersonaID) and 0.35^n repeat decay
	p := Persona{
		ID:              "p-starter",
		ActivityPattern: "active",
		ReplyTendency:   0.5,
	}
	stK2 := ThreadState{
		StarterPersonaID: "p-starter",
		ParentPersonaID:  "p-starter",
		LastPersonaID:    "p-other",
		PriorReplyCounts: map[string]int{},
	}
	wStarter := ReactionWeight(p, 0.5, 1.0, 1.0, stK2, 2)

	pNonStarter := p
	pNonStarter.ID = "p-regular"
	wNonStarter := ReactionWeight(pNonStarter, 0.5, 1.0, 1.0, stK2, 2)
	ratio := wStarter / wNonStarter
	if math.Abs(ratio-ReactionThreadStarterMult) > 1e-6 {
		t.Fatalf("expected starter weight ratio %f, got %f", ReactionThreadStarterMult, ratio)
	}

	// Repeat decay: 0.35^n
	w0 := ReactionWeight(pNonStarter, 0.5, 1.0, 1.0, ThreadState{PriorReplyCounts: map[string]int{"p-regular": 0}}, 2)
	w1 := ReactionWeight(pNonStarter, 0.5, 1.0, 1.0, ThreadState{PriorReplyCounts: map[string]int{"p-regular": 1}}, 2)
	w2 := ReactionWeight(pNonStarter, 0.5, 1.0, 1.0, ThreadState{PriorReplyCounts: map[string]int{"p-regular": 2}}, 2)
	if !(w0 > w1 && w1 > w2) {
		t.Fatalf("expected repeat decay w0(%f) > w1(%f) > w2(%f)", w0, w1, w2)
	}
	if math.Abs(w1/w0-ReactionRepeatDecay) > 1e-6 {
		t.Fatalf("expected w1/w0 == %f, got %f", ReactionRepeatDecay, w1/w0)
	}
}

func TestRelationshipFactorMonotonicity(t *testing.T) {
	// Property 9: Relationship factor (affinity > neutral 1.0 > friction)
	p := Persona{ID: "p-1", ActivityPattern: "active", ReplyTendency: 0.5}
	wFriction := ReactionWeight(p, 0.5, 1.0, 1.0, ThreadState{Relationship: 0.5}, 1)
	wNeutral := ReactionWeight(p, 0.5, 1.0, 1.0, ThreadState{Relationship: 1.0}, 1)
	wAffinity := ReactionWeight(p, 0.5, 1.0, 1.0, ThreadState{Relationship: 1.5}, 1)
	if !(wAffinity > wNeutral && wNeutral > wFriction) {
		t.Fatalf("expected relationship monotonicity wAffinity(%f) > wNeutral(%f) > wFriction(%f)", wAffinity, wNeutral, wFriction)
	}
}

func TestSinglePersonaDegeneracy(t *testing.T) {
	// Property 10: When only the author is in Personas, Replies is empty without panic
	author := Persona{ID: "only-me", ActivityPattern: "regular", ReplyTendency: 0.9}
	plan := PlanReactions(PlanInput{
		Host:       Host{ID: "canal"},
		Board:      Board{ID: "general"},
		TargetPost: Post{ID: 55, AuthorPersonaID: "only-me", CreatedAt: time.Date(1996, 5, 10, 20, 0, 0, 0, jstZone)},
		Ordinal:    3,
		ReplyRate:  2.0,
		Personas:   []Persona{author},
	})
	if len(plan.Replies) != 0 {
		t.Fatalf("expected 0 replies when only the post author exists in Personas, got %v", plan.Replies)
	}
}

func TestAdviceQuantizationAndBoundedInfluence(t *testing.T) {
	// Property 14: Advice quantization (0.05 step) and bounded multiplier [1-beta, 1+beta]
	if got := QuantizeAdvice(0.73); math.Abs(got-0.75) > 1e-9 {
		t.Fatalf("expected QuantizeAdvice(0.73) == 0.75, got %f", got)
	}
	if got := QuantizeAdvice(-0.5); got != 0 {
		t.Fatalf("expected QuantizeAdvice(-0.5) == 0, got %f", got)
	}
	if got := QuantizeAdvice(1.5); got != 1.0 {
		t.Fatalf("expected QuantizeAdvice(1.5) == 1.0, got %f", got)
	}

	minMult := AdviceMultiplier(QuantizeAdvice(0.0))
	midMult := AdviceMultiplier(QuantizeAdvice(0.5))
	maxMult := AdviceMultiplier(QuantizeAdvice(1.0))
	if math.Abs(minMult-(1.0-ReactionAdviceBeta)) > 1e-9 {
		t.Fatalf("expected min advice multiplier %f, got %f", 1.0-ReactionAdviceBeta, minMult)
	}
	if math.Abs(midMult-1.0) > 1e-9 {
		t.Fatalf("expected neutral advice multiplier 1.0, got %f", midMult)
	}
	if math.Abs(maxMult-(1.0+ReactionAdviceBeta)) > 1e-9 {
		t.Fatalf("expected max advice multiplier %f, got %f", 1.0+ReactionAdviceBeta, maxMult)
	}

	// Ensure advice cannot turn a 0-weight (self/last) candidate into positive weight,
	// nor make a positive-weight candidate 0.
	pSelf := Persona{ID: "p-self", ActivityPattern: "regular", ReplyTendency: 0.8}
	if w := ReactionWeight(pSelf, 0.9, 1.0, maxMult, ThreadState{ParentPersonaID: "p-self"}, 1); w != 0 {
		t.Fatalf("expected self-reply weight to remain 0 even with max advice, got %f", w)
	}
	pValid := Persona{ID: "p-valid", ActivityPattern: "occasional", ReplyTendency: 0.4}
	if w := ReactionWeight(pValid, 0.2, 1.0, minMult, ThreadState{}, 1); w <= 0 {
		t.Fatalf("expected valid candidate weight to remain > 0 even with min advice, got %f", w)
	}
}
