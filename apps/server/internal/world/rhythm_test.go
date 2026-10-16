package world

import (
	"fmt"
	"testing"
	"time"
)

type fixedFactorLayer struct {
	name   string
	factor float64
}

func (l fixedFactorLayer) Name() string                  { return l.name }
func (l fixedFactorLayer) Factor(ctx RhythmContext) float64 { return l.factor }

func TestRhythmArchetypeContrast(t *testing.T) {
	// Property 11: Archetype contrast across time-of-day and weekday/holiday
	layers := DefaultRhythmLayers()
	if len(layers) < 6 {
		t.Fatalf("expected at least 6 default rhythm layers, got %d", len(layers))
	}

	// Weekday 11:00 JST (1996-05-08 Wednesday): homemaker > office
	weekday11 := time.Date(1996, 5, 8, 11, 0, 0, 0, jstZone)
	rHomemaker11 := ComposeRhythm(layers, RhythmContext{
		Persona:   Persona{ID: "p-home", ActivityPattern: "active"},
		Archetype: "homemaker",
		At:        weekday11,
	})
	rOffice11 := ComposeRhythm(layers, RhythmContext{
		Persona:   Persona{ID: "p-office", ActivityPattern: "active"},
		Archetype: "office",
		At:        weekday11,
	})
	if !(rHomemaker11 > rOffice11) {
		t.Fatalf("expected weekday 11:00 homemaker (%f) > office (%f)", rHomemaker11, rOffice11)
	}

	// Weekday 23:30 JST: office > homemaker
	weekday2330 := time.Date(1996, 5, 8, 23, 30, 0, 0, jstZone)
	rOffice23 := ComposeRhythm(layers, RhythmContext{
		Persona:   Persona{ID: "p-office", ActivityPattern: "active"},
		Archetype: "office",
		At:        weekday2330,
	})
	rHomemaker23 := ComposeRhythm(layers, RhythmContext{
		Persona:   Persona{ID: "p-home", ActivityPattern: "active"},
		Archetype: "homemaker",
		At:        weekday2330,
	})
	if !(rOffice23 > rHomemaker23) {
		t.Fatalf("expected weekday 23:30 office (%f) > homemaker (%f)", rOffice23, rHomemaker23)
	}

	// Holiday 14:00 JST vs Weekday 14:00 JST: office holiday > office weekday
	holiday14 := time.Date(1996, 5, 5, 14, 0, 0, 0, jstZone) // Children's Day
	weekday14 := time.Date(1996, 5, 8, 14, 0, 0, 0, jstZone)
	rOfficeHol14 := ComposeRhythm(layers, RhythmContext{
		Persona:   Persona{ID: "p-office", ActivityPattern: "active"},
		Archetype: "office",
		At:        holiday14,
	})
	rOfficeWkd14 := ComposeRhythm(layers, RhythmContext{
		Persona:   Persona{ID: "p-office", ActivityPattern: "active"},
		Archetype: "office",
		At:        weekday14,
	})
	if !(rOfficeHol14 > rOfficeWkd14) {
		t.Fatalf("expected office holiday 14:00 (%f) > weekday 14:00 (%f)", rOfficeHol14, rOfficeWkd14)
	}
}

func TestRhythmLayerPluggabilityAndClamping(t *testing.T) {
	// Property 12: Adding a custom RhythmLayer changes Rhythm output without modifying existing layers
	baseLayers := DefaultRhythmLayers()
	ctx := RhythmContext{
		Persona:   Persona{ID: "p-1", ActivityPattern: "occasional"},
		Archetype: "student",
		At:        time.Date(1996, 5, 8, 18, 0, 0, 0, jstZone),
	}
	baseVal := ComposeRhythm(baseLayers, ctx)

	extendedLayers := append(append([]RhythmLayer(nil), baseLayers...), fixedFactorLayer{name: "custom_boost", factor: 1.5})
	extVal := ComposeRhythm(extendedLayers, ctx)
	if extVal <= baseVal {
		t.Fatalf("expected extended layers factor (%f) > base factor (%f)", extVal, baseVal)
	}

	// Verify log-space clamping [RhythmMinFactor, RhythmMaxFactor]
	huge := ComposeRhythm([]RhythmLayer{fixedFactorLayer{name: "huge", factor: 100.0}}, ctx)
	if huge != RhythmMaxFactor {
		t.Fatalf("expected clamp to RhythmMaxFactor (%f), got %f", RhythmMaxFactor, huge)
	}
	tiny := ComposeRhythm([]RhythmLayer{fixedFactorLayer{name: "tiny", factor: 0.0001}}, ctx)
	if tiny != RhythmMinFactor {
		t.Fatalf("expected clamp to RhythmMinFactor (%f), got %f", RhythmMinFactor, tiny)
	}
}

func TestAssignArchetypeOccupationAndMix(t *testing.T) {
	// Confirmed fact takes precedence
	p1 := Persona{ID: "p-1", Occupation: "会社員"}
	facts := []PersonaFact{{PersonaID: "p-1", Topic: "local_life", Value: "大学でレポートを書いています（大学生）"}}
	if got := AssignArchetype(p1, facts, nil); got != "student" {
		t.Fatalf("expected student from confirmed fact, got %q", got)
	}

	// Occupation string match when no fact matches
	p2 := Persona{ID: "p-2", Occupation: "システムエンジニア"}
	if got := AssignArchetype(p2, nil, nil); got != "office" {
		t.Fatalf("expected office from occupation, got %q", got)
	}

	// Fallback to deterministic hash against memberMix
	mix := map[string]float64{
		"night_owl": 0.70,
		"student":   0.30,
	}
	counts := map[string]int{}
	const total = 1000
	for i := 0; i < total; i++ {
		p := Persona{ID: fmt.Sprintf("anon-%d", i)}
		arch := AssignArchetype(p, nil, mix)
		counts[arch]++
	}
	nightShare := float64(counts["night_owl"]) / float64(total)
	if nightShare < 0.62 || nightShare > 0.78 {
		t.Fatalf("expected night_owl share near 0.70, got %.3f (%v)", nightShare, counts)
	}
}

func TestReactionDueAtRhythmCompressionAndStretch(t *testing.T) {
	// Property 13: ReactionDueAt monotonically increases and compresses during peak rhythm
	layers := DefaultRhythmLayers()
	rFunc := func(tm time.Time) float64 {
		return ComposeRhythm(layers, RhythmContext{
			Persona:   Persona{ID: "p-office", ActivityPattern: "active"},
			Archetype: "office",
			At:        tm,
		})
	}

	// Compare real wait duration starting at 23:00 (Telehodai peak) vs 03:30 (deep night lull)
	const samples = 200
	var sumWaitPeak, sumWaitLull time.Duration
	peakStart := time.Date(1996, 5, 8, 23, 0, 0, 0, jstZone)
	lullStart := time.Date(1996, 5, 8, 3, 30, 0, 0, jstZone)

	for i := 0; i < samples; i++ {
		seed := fmt.Sprintf("due-seed-%d", i)
		duePeak := ReactionDueAt(seed, peakStart, rFunc, "active", 1)
		dueLull := ReactionDueAt(seed, lullStart, rFunc, "active", 1)
		if !duePeak.After(peakStart) || !dueLull.After(lullStart) {
			t.Fatalf("expected DueAt strictly after start time")
		}
		sumWaitPeak += duePeak.Sub(peakStart)
		sumWaitLull += dueLull.Sub(lullStart)
	}

	if !(sumWaitPeak < sumWaitLull) {
		t.Fatalf("expected average wait starting at 23:00 peak (%v) < starting at 03:30 lull (%v)", sumWaitPeak/samples, sumWaitLull/samples)
	}
}
