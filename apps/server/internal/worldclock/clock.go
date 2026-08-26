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
