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

// fakeLines is a LineOccupancy stand-in that reports a fixed answer.
type fakeLines struct {
	world.Store
	line  int
	free  bool
	dials int
}

func (f *fakeLines) DialLine(world.Host, time.Time) (int, bool)         { f.dials++; return f.line, f.free }
func (f *fakeLines) BeginHumanCall(world.Host, string, string, int) (int, bool) { return f.line, f.free }
func (f *fakeLines) SetHumanCallHandle(string, string)                  {}
func (f *fakeLines) EndHumanCall(string)                                {}

func TestDialUsesLineOccupancy(t *testing.T) {
	at := time.Date(1996, 10, 15, 23, 10, 0, 0, time.FixedZone("JST", 9*60*60))
	full := &fakeLines{Store: world.NewMemoryStore(), free: false}
	if got := New(full, fixedClock{now: at}).Dial("0920000196", 1); got.Result != Busy || full.dials != 1 {
		t.Fatalf("full lines: %+v dials=%d", got, full.dials)
	}
	free := &fakeLines{Store: world.NewMemoryStore(), line: 3, free: true}
	if got := New(free, fixedClock{now: at}).Dial("0920000196", 1); got.Result != Connect || got.Line != 3 {
		t.Fatalf("free line: %+v", got)
	}
	// The BUSY-test fixture keeps its fixed behavior.
	if got := New(full, fixedClock{now: at}).Dial("0459999999", 5); got.Result != Connect {
		t.Fatalf("busy fixture attempt 5 = %+v", got)
	}
}

func TestDialBypassBusyProbabilistic(t *testing.T) {
	at := time.Date(1996, 10, 15, 23, 10, 0, 0, time.FixedZone("JST", 9*60*60))
	store := world.NewMemoryStore()
	n := New(store, fixedClock{now: at})
	// 0920000196 has BypassBusy: true
	for attempt := 1; attempt <= 10; attempt++ {
		got := n.Dial("0920000196", attempt)
		if got.Result != Connect {
			t.Fatalf("attempt %d failed to connect with BypassBusy: %+v", attempt, got)
		}
	}
}

