package world

import (
	"math"
	"time"
)

// Float64Source provides uniform pseudo-random values in [0.0, 1.0).
type Float64Source interface {
	Float64() float64
}

// StandardActiveMembers computes the standard active count for a station of given membership size.
func StandardActiveMembers(members int) int {
	if members <= 0 {
		return 10
	}
	n := int(math.Round(float64(members) * 0.08))
	if n < 3 {
		return 3
	}
	return n
}

// ActivePopulationAt calculates the active persona population at the given timestamp
// and the standard (daily average) active population using the unified rhythm model.
// active and standard are measured using the exact same metric so their ratio represents
// the relative diurnal activity factor at 'at'.
func ActivePopulationAt(host Host, board Board, personas []Persona, factsByPersona map[string][]PersonaFact, at time.Time) (active int, standard int) {
	if at.IsZero() {
		at = time.Now()
	}
	rhythmLayers := DefaultRhythmLayers()
	memberMix := board.Rhythm.MemberMix
	if len(memberMix) == 0 {
		if sr := StationRhythmForHost(host.ID); len(sr.MemberMix) > 0 {
			memberMix = sr.MemberMix
		} else {
			memberMix = DefaultMemberMix()
		}
	}

	if len(personas) == 0 {
		base := StandardActiveMembers(host.Members)
		cfg := DefaultRhythmConfig()
		if cfg == nil {
			return base, base
		}
		dayKind := cfg.dayKind(at)
		hour := toJST(at).Hour()
		if hour < 0 {
			hour = 0
		} else if hour > 23 {
			hour = 23
		}

		atFactor := 0.0
		avgFactor := 0.0
		totalWeight := 0.0
		for archName, weight := range memberMix {
			if weight <= 0 {
				continue
			}
			sched, ok := cfg.Archetypes[archName]
			if !ok {
				continue
			}
			var curve [24]float64
			switch dayKind {
			case "holiday":
				curve = sched.Holiday
			case "saturday":
				curve = sched.Saturday
			default:
				curve = sched.Weekday
			}
			atFactor += weight * curve[hour]
			sumH := 0.0
			for h := 0; h < 24; h++ {
				sumH += curve[h]
			}
			avgFactor += weight * (sumH / 24.0)
			totalWeight += weight
		}

		if totalWeight > 0 && avgFactor > 0 {
			atFactor /= totalWeight
			avgFactor /= totalWeight
			ratio := atFactor / avgFactor
			activeCount := int(math.Round(float64(base) * ratio))
			if activeCount < 1 {
				activeCount = 1
			}
			return activeCount, base
		}
		return base, base
	}

	archByPersona := make(map[string]string, len(personas))
	for _, p := range personas {
		var facts []PersonaFact
		if factsByPersona != nil {
			facts = factsByPersona[p.ID]
		}
		archByPersona[p.ID] = AssignArchetype(p, facts, memberMix)
	}

	activeSum := 0.0
	for _, p := range personas {
		r := ComposeRhythm(rhythmLayers, RhythmContext{
			Host:      host,
			Board:     board,
			Persona:   p,
			Archetype: archByPersona[p.ID],
			At:        at,
		})
		w := PersonaActivityWeight(p.ActivityPattern, p.LurkerTendency) * r
		activeSum += w
	}

	dailySum := 0.0
	jt := toJST(at)
	dayStart := time.Date(jt.Year(), jt.Month(), jt.Day(), 0, 30, 0, 0, jstZone)
	for h := 0; h < 24; h++ {
		sampleAt := dayStart.Add(time.Duration(h) * time.Hour)
		hourSum := 0.0
		for _, p := range personas {
			r := ComposeRhythm(rhythmLayers, RhythmContext{
				Host:      host,
				Board:     board,
				Persona:   p,
				Archetype: archByPersona[p.ID],
				At:        sampleAt,
			})
			w := PersonaActivityWeight(p.ActivityPattern, p.LurkerTendency) * r
			hourSum += w
		}
		dailySum += hourSum
	}
	dailyAvg := dailySum / 24.0

	active = int(math.Round(activeSum))
	if active < 1 {
		active = 1
	}
	standard = int(math.Round(dailyAvg))
	if standard < 1 {
		standard = 1
	}
	return active, standard
}

// AppealMultiplier maps appeal level (0..4) to the fictional reply rate multiplier.
var AppealMultiplier = [5]float64{0.2, 0.6, 1.0, 1.7, 2.6}

// ReplyCountFor computes the number of replies a thread draws using the given random source,
// effective board reply rate, appeal level (0..4), and currently active population count.
// Time factor scaling is disabled (factor = 1.0) when standard active is 0.
func ReplyCountFor(rng Float64Source, rate float64, appeal int, active int, maxReplies int) int {
	return ReplyCountForWithStandard(rng, rate, appeal, active, 0, maxReplies)
}

// ReplyCountForWithStandard computes the number of replies a thread draws, explicitly
// specifying the standard active population to compute the time factor.
func ReplyCountForWithStandard(rng Float64Source, rate float64, appeal int, active int, standardActive int, maxReplies int) int {
	if rate <= 0 || active <= 1 || rng == nil {
		return 0
	}
	if appeal < 0 {
		appeal = 0
	} else if appeal > 4 {
		appeal = 4
	}

	timeFactor := 1.0
	if standardActive > 0 {
		timeFactor = float64(active) / float64(standardActive)
		if timeFactor < 0.3 {
			timeFactor = 0.3
		} else if timeFactor > 1.5 {
			timeFactor = 1.5
		}
	}

	meanReplies := rate * AppealMultiplier[appeal] * timeFactor
	if meanReplies <= 0 {
		return 0
	}

	silent := math.Exp(-0.55 * meanReplies)
	if rng.Float64() < silent {
		return 0
	}

	answeredMean := meanReplies / (1.0 - silent)
	u := rng.Float64()
	if u >= 0.999999 {
		u = 0.999999
	}

	k := 1
	if answeredMean > 1.000001 {
		k = 1 + int(math.Floor(math.Log(1.0-u)/math.Log(1.0-1.0/answeredMean)))
	}

	if maxReplies <= 0 || maxReplies > MaxThreadReplies {
		maxReplies = MaxThreadReplies
	}
	if k > maxReplies {
		k = maxReplies
	}

	maxActive := active - 1
	if maxActive < 0 {
		maxActive = 0
	}
	if k > maxActive {
		k = maxActive
	}

	return k
}
