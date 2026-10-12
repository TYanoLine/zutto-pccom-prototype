package worldcalendar

import "time"

// Effect is one fictional tuning entry from data/effects.yaml. It is not a
// historical statistic.
type Effect struct {
	When              string   `yaml:"when"`
	Lifestyle         []string `yaml:"lifestyle"`
	Hours             []int    `yaml:"hours"`
	Factor            float64  `yaml:"factor"`
	UseHolidayProfile bool     `yaml:"use_holiday_profile"`
}

func (e Effect) appliesTo(lifestyle string) bool {
	if len(e.Lifestyle) == 0 {
		return true
	}
	for _, l := range e.Lifestyle {
		if l == lifestyle {
			return true
		}
	}
	return false
}

// matches reports whether the effect covers t, looking at the keyed day itself
// and, for hour ranges past 24, at the previous day.
func (c *Calendar) matches(e Effect, t time.Time) bool {
	hour := t.Hour()
	today := c.Day(t)
	if today.Has(e.When) && (len(e.Hours) == 0 || (e.Hours[0] <= hour && hour < e.Hours[1])) {
		return true
	}
	if len(e.Hours) == 2 && e.Hours[1] > 24 {
		yesterday := c.Day(t.AddDate(0, 0, -1))
		if yesterday.Has(e.When) && e.Hours[0] <= hour+24 && hour+24 < e.Hours[1] {
			return true
		}
	}
	return false
}

// Factor is the product of the calendar effects that cover t for a resident of
// the given lifestyle. With no applicable data it is 1.
func (c *Calendar) Factor(t time.Time, lifestyle string) float64 {
	f := 1.0
	for _, e := range c.effects {
		if e.Factor == 0 || !e.appliesTo(lifestyle) || !c.matches(e, t) {
			continue
		}
		f *= e.Factor
	}
	return f
}

// UseHolidayProfile reports whether a resident of the given lifestyle follows
// the holiday daily rhythm at t: weekends and holidays for everyone, plus the
// periods effects.yaml marks (school breaks for students).
func (c *Calendar) UseHolidayProfile(t time.Time, lifestyle string) bool {
	if c.Day(t).Weekend() {
		return true
	}
	for _, e := range c.effects {
		if e.UseHolidayProfile && e.appliesTo(lifestyle) && c.matches(e, t) {
			return true
		}
	}
	return false
}
