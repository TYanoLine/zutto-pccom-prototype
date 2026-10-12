package worldclock

import (
	"fmt"
	"time"
)

// WorldClock is the single time source for world-facing behavior.
// Implementations may map the real Japanese clock onto the 1996 calendar.
type WorldClock interface {
	Now() time.Time
}

type systemClock struct {
	location *time.Location
}

func (c systemClock) Now() time.Time { return time.Now().In(c.location) }

// MappedClock preserves elapsed time and time-of-day while changing the
// calendar origin. It therefore continues across midnight instead of pinning
// every call to one date.
type MappedClock struct {
	source      WorldClock
	realOrigin  time.Time
	worldOrigin time.Time
}

func New(date string, location *time.Location) (*MappedClock, error) {
	return NewFrom(systemClock{location: location}, date, location)
}

func NewFrom(source WorldClock, date string, location *time.Location) (*MappedClock, error) {
	if source == nil {
		return nil, fmt.Errorf("world clock source is required")
	}
	if location == nil {
		return nil, fmt.Errorf("world clock location is required")
	}

	worldDate, err := time.ParseInLocation(time.DateOnly, date, location)
	if err != nil {
		return nil, fmt.Errorf("parse world date: %w", err)
	}
	realNow := source.Now().In(location)
	realOrigin := time.Date(realNow.Year(), realNow.Month(), realNow.Day(), 0, 0, 0, 0, location)
	return &MappedClock{source: source, realOrigin: realOrigin, worldOrigin: worldDate}, nil
}

func (c *MappedClock) Now() time.Time {
	return c.worldOrigin.Add(c.source.Now().In(c.realOrigin.Location()).Sub(c.realOrigin))
}

// The base year belongs to the world, not to the deployment: worlds are to be
// created per user or group, each choosing its base year at creation. Until
// that exists, the single world uses these values. They are deliberately not
// read from the environment.
const (
	CurrentWorldBaseYear      = 1996
	CurrentWorldStartRealYear = 2026
)

// CalendarClock maps the real Japanese wall clock onto the world calendar. The
// world's base year corresponds to the real year in which the world was
// started; the difference is subtracted from every real time. Month, day and
// time of day are kept, and the world crosses New Year together with the real
// calendar, so the world date does not depend on when the process started.
//
// A real 29 February that maps onto a common world year is normalized by
// AddDate to 1 March (a known limitation, see docs/WORLD_SIMULATION.md "Time").
type CalendarClock struct {
	source     WorldClock
	location   *time.Location
	yearOffset int // startRealYear - baseYear
}

func NewCalendar(location *time.Location, baseYear, startRealYear int) (*CalendarClock, error) {
	return NewCalendarFrom(systemClock{location: location}, location, baseYear, startRealYear)
}

func NewCalendarFrom(source WorldClock, location *time.Location, baseYear, startRealYear int) (*CalendarClock, error) {
	if source == nil {
		return nil, fmt.Errorf("world clock source is required")
	}
	if location == nil {
		return nil, fmt.Errorf("world clock location is required")
	}
	if baseYear > startRealYear {
		return nil, fmt.Errorf("world base year %d is after the start real year %d", baseYear, startRealYear)
	}
	return &CalendarClock{source: source, location: location, yearOffset: startRealYear - baseYear}, nil
}

func (c *CalendarClock) Now() time.Time {
	return c.source.Now().In(c.location).AddDate(-c.yearOffset, 0, 0)
}
