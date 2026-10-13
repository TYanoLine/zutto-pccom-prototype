// Package worldcalendar holds the world's calendar facts (holidays, customary
// periods, notable days) and, separately, the fictional tuning values that say
// how those days change resident connection behavior. See spec 015 and
// docs/research/CALENDAR_1996.md.
package worldcalendar

import (
	"embed"
	"fmt"
	"io/fs"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

//go:embed data/*.yaml
var dataFS embed.FS

// Day is what the calendar knows about one world date.
type Day struct {
	Date    time.Time // world date at 00:00 in the caller's location
	Known   bool      // calendar data exists for this year
	Weekday time.Weekday
	Holiday bool     // national holiday or substitute holiday
	Periods []string // keys of periods covering the date
	Days    []string // keys of notable days on the date
}

// Weekend reports Saturday, Sunday or a holiday.
func (d Day) Weekend() bool {
	return d.Holiday || d.Weekday == time.Saturday || d.Weekday == time.Sunday
}

// Has reports whether key names a period or a day on this date.
func (d Day) Has(key string) bool {
	for _, k := range d.Periods {
		if k == key {
			return true
		}
	}
	for _, k := range d.Days {
		if k == key {
			return true
		}
	}
	return false
}

type span struct {
	key      string
	from, to string // YYYY-MM-DD, inclusive
}

// Calendar is the immutable set of loaded calendar years.
type Calendar struct {
	years    map[int]bool
	holidays map[string]string
	periods  []span
	days     map[string][]string
	effects  []Effect
}

type yearFile struct {
	Year     int `yaml:"year"`
	Holidays []struct {
		Date string `yaml:"date"`
		Name string `yaml:"name"`
	} `yaml:"holidays"`
	Periods []struct {
		Key      string `yaml:"key"`
		From     string `yaml:"from"`
		To       string `yaml:"to"`
		Evidence string `yaml:"evidence"`
	} `yaml:"periods"`
	Days []struct {
		Key      string `yaml:"key"`
		Date     string `yaml:"date"`
		Evidence string `yaml:"evidence"`
	} `yaml:"days"`
	Events []struct {
		Key      string `yaml:"key"`
		From     string `yaml:"from"`
		To       string `yaml:"to"`
		Evidence string `yaml:"evidence"`
	} `yaml:"events"`
}

var defaultCalendar = mustLoad()

// Default returns the calendar built from the embedded data.
func Default() *Calendar { return defaultCalendar }

func mustLoad() *Calendar {
	c, err := load(dataFS)
	if err != nil {
		panic(err)
	}
	return c
}

func load(fsys fs.FS) (*Calendar, error) {
	c := &Calendar{years: map[int]bool{}, holidays: map[string]string{}, days: map[string][]string{}}
	names, err := fs.Glob(fsys, "data/*.yaml")
	if err != nil {
		return nil, err
	}
	sort.Strings(names)
	for _, name := range names {
		raw, err := fs.ReadFile(fsys, name)
		if err != nil {
			return nil, err
		}
		if strings.HasSuffix(name, "/effects.yaml") {
			if err := yaml.Unmarshal(raw, &c.effects); err != nil {
				return nil, fmt.Errorf("%s: %w", name, err)
			}
			continue
		}
		var f yearFile
		if err := yaml.Unmarshal(raw, &f); err != nil {
			return nil, fmt.Errorf("%s: %w", name, err)
		}
		if f.Year == 0 || c.years[f.Year] {
			return nil, fmt.Errorf("%s: missing or duplicate year", name)
		}
		c.years[f.Year] = true
		for _, h := range f.Holidays {
			if err := checkDate(h.Date); err != nil {
				return nil, fmt.Errorf("%s holiday: %w", name, err)
			}
			c.holidays[h.Date] = h.Name
		}
		for _, p := range f.Periods {
			if err := checkSpan(p.Key, p.From, p.To); err != nil {
				return nil, fmt.Errorf("%s period: %w", name, err)
			}
			c.periods = append(c.periods, span{key: p.Key, from: p.From, to: p.To})
		}
		for _, e := range f.Events {
			if err := checkSpan(e.Key, e.From, e.To); err != nil {
				return nil, fmt.Errorf("%s event: %w", name, err)
			}
			c.periods = append(c.periods, span{key: e.Key, from: e.From, to: e.To})
		}
		for _, d := range f.Days {
			if err := checkDate(d.Date); err != nil || d.Key == "" {
				return nil, fmt.Errorf("%s day %q: invalid", name, d.Key)
			}
			c.days[d.Date] = append(c.days[d.Date], d.Key)
		}
	}
	for i, e := range c.effects {
		if e.When == "" || (len(e.Hours) != 0 && (len(e.Hours) != 2 || e.Hours[0] >= e.Hours[1])) {
			return nil, fmt.Errorf("effects.yaml entry %d is invalid", i)
		}
	}
	return c, nil
}

func checkDate(s string) error {
	_, err := time.Parse(time.DateOnly, s)
	return err
}

func checkSpan(key, from, to string) error {
	if key == "" {
		return fmt.Errorf("missing key")
	}
	if checkDate(from) != nil || checkDate(to) != nil || from > to {
		return fmt.Errorf("%s: invalid span %s..%s", key, from, to)
	}
	return nil
}

// Day returns the calendar facts for the world date of t.
func (c *Calendar) Day(t time.Time) Day {
	date := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
	key := date.Format(time.DateOnly)
	d := Day{Date: date, Known: c.years[date.Year()], Weekday: date.Weekday()}
	_, d.Holiday = c.holidays[key]
	for _, p := range c.periods {
		if p.from <= key && key <= p.to {
			d.Periods = append(d.Periods, p.key)
		}
	}
	d.Days = append(d.Days, c.days[key]...)
	return d
}

// Holidays returns the loaded holiday dates (YYYY-MM-DD) in order.
func (c *Calendar) Holidays() []string {
	out := make([]string, 0, len(c.holidays))
	for k := range c.holidays {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
