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
