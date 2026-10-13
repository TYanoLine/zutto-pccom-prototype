package worldclock

import (
	"testing"
	"time"
)

type fixedClock struct{ now time.Time }

func (c fixedClock) Now() time.Time { return c.now }

func TestMappedClockPreservesJapanTimeOfDay(t *testing.T) {
	jst := time.FixedZone("JST", 9*60*60)
	source := fixedClock{now: time.Date(2026, 8, 26, 22, 59, 30, 0, jst)}
	clock, err := NewFrom(source, "1996-08-26", jst)
	if err != nil {
		t.Fatal(err)
	}

	want := time.Date(1996, 8, 26, 22, 59, 30, 0, jst)
	if got := clock.Now(); !got.Equal(want) {
		t.Fatalf("Now() = %v, want %v", got, want)
	}
}

func TestMappedClockCanCrossMidnight(t *testing.T) {
	jst := time.FixedZone("JST", 9*60*60)
	source := &fixedClock{now: time.Date(2026, 8, 26, 23, 59, 0, 0, jst)}
	clock, err := NewFrom(source, "1996-08-26", jst)
	if err != nil {
		t.Fatal(err)
	}

	source.now = source.now.Add(2 * time.Minute)
	want := time.Date(1996, 8, 27, 0, 1, 0, 0, jst)
	if got := clock.Now(); !got.Equal(want) {
		t.Fatalf("Now() = %v, want %v", got, want)
	}
}

// calendarClockCases is shared in content with
// apps/web/src/time/WorldClock.test.ts (spec 015 plan §3.3).
var calendarClockCases = []struct{ real, world string }{
	{"2026-10-12 11:54:00", "1996-10-12 11:54:00"},
	{"2026-12-24 22:59:59", "1996-12-24 22:59:59"},
	{"2026-12-24 23:00:00", "1996-12-24 23:00:00"},
	{"2026-12-31 23:59:59", "1996-12-31 23:59:59"},
	{"2027-01-01 00:00:00", "1997-01-01 00:00:00"},
}

func TestCalendarClockMapsRealCalendar(t *testing.T) {
	jst := time.FixedZone("JST", 9*60*60)
	const layout = "2006-01-02 15:04:05"
	for _, tc := range calendarClockCases {
		real, err := time.ParseInLocation(layout, tc.real, jst)
		if err != nil {
			t.Fatal(err)
		}
		clock, err := NewCalendarFrom(fixedClock{now: real}, jst, 1996, 2026)
		if err != nil {
			t.Fatal(err)
		}
		if got := clock.Now().Format(layout); got != tc.world {
			t.Errorf("real %s: world %s, want %s", tc.real, got, tc.world)
		}
	}
}

func TestCalendarClockWeekdayIsWorldWeekday(t *testing.T) {
	jst := time.FixedZone("JST", 9*60*60)
	clock, err := NewCalendarFrom(fixedClock{now: time.Date(2026, 12, 24, 21, 0, 0, 0, jst)}, jst, 1996, 2026)
	if err != nil {
		t.Fatal(err)
	}
	if got := clock.Now().Weekday(); got != time.Tuesday {
		t.Fatalf("1996-12-24 weekday = %v, want Tuesday", got)
	}
}

func TestCalendarClockOtherBaseYear(t *testing.T) {
	jst := time.FixedZone("JST", 9*60*60)
	clock, err := NewCalendarFrom(fixedClock{now: time.Date(2026, 10, 12, 11, 54, 0, 0, jst)}, jst, 1990, 2026)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := clock.Now(), time.Date(1990, 10, 12, 11, 54, 0, 0, jst); !got.Equal(want) {
		t.Fatalf("Now() = %v, want %v", got, want)
	}
}

func TestCalendarClockRejectsBaseYearAfterStart(t *testing.T) {
	jst := time.FixedZone("JST", 9*60*60)
	if _, err := NewCalendarFrom(fixedClock{}, jst, 2030, 2026); err == nil {
		t.Fatal("expected an error for a base year after the start real year")
	}
}
