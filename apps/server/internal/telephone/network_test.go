package telephone

import (
	"testing"
	"time"

	"zutto-pccom/apps/server/internal/world"
)

type fixedClock struct{ now time.Time }

func (c fixedClock) Now() time.Time { return c.now }

func networkAt(at time.Time) *Network {
	return New(world.NewMemoryStore(), fixedClock{now: at})
}

func TestUnknownNumberDoesNotAnswer(t *testing.T) {
	n := networkAt(time.Now())
	got := n.Dial("0455555555", 1)
	if got.Result != NoAnswer {
		t.Fatalf("expected no_answer, got %s", got.Result)
	}
}

func TestBusyFixtureConnectsOnFifthDial(t *testing.T) {
	n := networkAt(time.Date(1996, 8, 26, 22, 0, 0, 0, time.Local))
	for attempt := 1; attempt < 5; attempt++ {
		if got := n.Dial("0459999999", attempt); got.Result != Busy {
			t.Fatalf("attempt %d = %s, want busy", attempt, got.Result)
		}
	}
	if got := n.Dial("0459999999", 5); got.Result != Connect {
		t.Fatalf("attempt 5 = %s, want connect", got.Result)
	}
}

func TestBusyProbabilityTelehodaiRushBoundary(t *testing.T) {
	host := world.Host{Lines: 1, Popularity: 0.5}
	jst := time.FixedZone("JST", 9*60*60)
	tests := []struct {
		name string
		at   time.Time
		want float64
	}{
		{"before window", time.Date(1996, 8, 26, 22, 59, 59, 0, jst), 0.425},
		{"window starts", time.Date(1996, 8, 26, 23, 0, 0, 0, jst), 0.675},
		{"late peak ends", time.Date(1996, 8, 27, 2, 0, 0, 0, jst), 0.2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := busyProbability(host, tt.at); got != tt.want {
				t.Fatalf("busyProbability() = %v, want %v", got, tt.want)
			}
		})
	}
}
