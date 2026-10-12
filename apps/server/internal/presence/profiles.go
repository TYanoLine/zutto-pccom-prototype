package presence

// hourWeights are the relative chances, per hour of the day, that a resident
// of a lifestyle starts a call. Index 0 is 00:00-00:59. FICTIONAL TUNING
// VALUES, not historical statistics; adjust in a separate change.
type hourWeights [24]float64

var weekdayProfiles = map[string]hourWeights{
	SecondaryStudent:  {.30, .10, .03, .01, .01, .01, .05, .05, .02, .02, .02, .02, .02, .02, .02, .02, .30, .40, .30, .40, .60, .90, 1.0, 1.0},
	UniversityStudent: {1.0, .90, .60, .35, .15, .05, .03, .05, .08, .10, .15, .20, .30, .25, .20, .25, .30, .30, .35, .45, .55, .70, .85, 1.0},
	OfficeWorker:      {.80, .45, .15, .04, .02, .03, .12, .15, .03, .02, .02, .02, .06, .03, .02, .02, .02, .03, .08, .20, .45, .70, .85, 1.0},
	SelfEmployed:      {.70, .35, .10, .03, .02, .03, .05, .08, .10, .12, .15, .20, .50, .45, .20, .15, .15, .15, .20, .30, .50, .80, .90, 1.0},
	Homemaker:         {.20, .08, .02, .01, .01, .02, .05, .05, .10, .50, .90, .80, .60, .90, 1.0, .80, .40, .15, .08, .08, .15, .30, .45, .50},
	ShiftWorker:       {.50, .50, .50, .35, .35, .35, .35, .50, .30, .30, .30, .50, .50, .50, .50, .50, .50, .50, .50, .50, .50, .50, .50, .50},
}

var holidayProfiles = map[string]hourWeights{
	SecondaryStudent:  {.40, .20, .08, .03, .01, .01, .03, .05, .10, .20, .40, .50, .50, .60, .60, .60, .60, .50, .40, .50, .70, .90, 1.0, 1.0},
	UniversityStudent: {1.0, .95, .75, .50, .25, .08, .03, .03, .05, .10, .20, .35, .45, .50, .50, .50, .50, .45, .45, .55, .65, .80, .90, 1.0},
	OfficeWorker:      {.85, .55, .25, .08, .03, .02, .03, .05, .10, .20, .35, .40, .40, .45, .50, .50, .45, .40, .35, .45, .60, .75, .90, 1.0},
	SelfEmployed:      {.85, .55, .25, .08, .03, .02, .03, .05, .10, .20, .35, .40, .40, .45, .50, .50, .45, .40, .35, .45, .60, .75, .90, 1.0},
	Homemaker:         {.20, .08, .02, .01, .01, .01, .03, .05, .10, .20, .30, .30, .30, .60, .70, .60, .30, .15, .10, .15, .30, .45, .60, .50},
	ShiftWorker:       {.50, .50, .50, .35, .35, .35, .35, .50, .30, .30, .30, .50, .50, .50, .50, .50, .50, .50, .50, .50, .50, .50, .50, .50},
}

// share is the fraction of a lifestyle's daily starts that fall in the hour.
func share(lifestyle string, holiday bool, hour int) float64 {
	table := weekdayProfiles
	if holiday {
		table = holidayProfiles
	}
	w, ok := table[lifestyle]
	if !ok {
		w = table[OfficeWorker]
	}
	sum := 0.0
	for _, v := range w {
		sum += v
	}
	return w[hour] / sum
}

// dailyCalls is the expected number of calls per day for an activity pattern.
// Lurkers read without posting, so they call as often as occasional members.
func dailyCalls(activityPattern string) float64 {
	switch activityPattern {
	case "regular":
		return 1.2
	case "active":
		return .8
	case "occasional", "lurker":
		return .35
	case "dormant":
		return .03
	}
	return .35
}
