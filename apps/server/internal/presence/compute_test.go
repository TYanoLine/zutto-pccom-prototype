package presence

import (
	"flag"
	"fmt"
	"reflect"
	"testing"
	"time"

	"zutto-pccom/apps/server/internal/world"
	"zutto-pccom/apps/server/internal/worldcalendar"
)

var histogram = flag.Bool("presence-histogram", false, "print hourly connection counts in TestPresenceHistogram")

func testPersonas(n int) []world.Persona {
	patterns := []string{"regular", "active", "active", "occasional", "occasional", "occasional", "lurker", "lurker", "lurker", "dormant"}
	out := make([]world.Persona, n)
	for i := range out {
		out[i] = world.Persona{ID: fmt.Sprintf("p%04d", i), Handle: fmt.Sprintf("H%04d", i), ActivityPattern: patterns[i%len(patterns)]}
	}
	return out
}

func TestComputeIsDeterministic(t *testing.T) {
	host := world.Host{ID: "h", Lines: 4, MaxBaud: 14400, TelehoFriendly: true}
	in := Input{Host: host, Personas: testPersonas(300)}
	a := Compute(in, at("1996-10-15 23:40"))
	rev := append([]world.Persona(nil), in.Personas...)
	for i, j := 0, len(rev)-1; i < j; i, j = i+1, j-1 {
		rev[i], rev[j] = rev[j], rev[i]
	}
	b := Compute(Input{Host: host, Personas: rev}, at("1996-10-15 23:40"))
	if !reflect.DeepEqual(a.Sessions, b.Sessions) {
		t.Fatalf("results differ:\n%v\n%v", a.Sessions, b.Sessions)
	}
	if len(a.Sessions) == 0 {
		t.Fatal("expected someone online at 23:40 on a 300-member host")
	}
}

func TestComputeNeverExceedsLines(t *testing.T) {
	host := world.Host{ID: "busy", Lines: 2, MaxBaud: 9600, TelehoFriendly: true}
	in := Input{Host: host, Personas: testPersonas(300), Humans: []world.HumanCall{{CallID: "c1", HostID: "busy", Line: 2, Start: at("1996-10-15 18:00")}}}
	start := at("1996-10-14 00:00")
	for i := 0; i < 1000; i++ {
		ts := start.Add(time.Duration(i) * 17 * time.Minute)
		got := Compute(in, ts).Sessions
		if len(got) > Lines(host) {
			t.Fatalf("%v: %d sessions on %d lines", ts, len(got), Lines(host))
		}
		seen := map[int]bool{}
		for _, s := range got {
			if seen[s.Line] || s.Line < 1 || s.Line > Lines(host) {
				t.Fatalf("%v: bad line assignment %v", ts, got)
			}
			seen[s.Line] = true
		}
	}
}

// statsHost has enough lines that capacity never hides the rhythm.
var statsHost = world.Host{ID: "stats", Lines: 500, MaxBaud: 28800, TelehoFriendly: true}

func countAt(in Input, ts time.Time) int { return len(Compute(in, ts).Sessions) }

func TestTelehoRush(t *testing.T) {
	in := Input{Host: statsHost, Personas: testPersonas(2000)}
	before, after := 0, 0
	day := at("1996-09-01 00:00")
	for i := 0; i < 60; i++ {
		d := day.AddDate(0, 0, i)
		before += countAt(in, d.Add(22*time.Hour+50*time.Minute))
		after += countAt(in, d.Add(23*time.Hour+10*time.Minute))
	}
	if float64(after) <= 1.5*float64(before) {
		t.Fatalf("23:10 total %d, 22:50 total %d; want 23:10 > 1.5x", after, before)
	}
}

func TestWeekdayAfternoonIsQuiet(t *testing.T) {
	in := Input{Host: statsHost, Personas: testPersonas(2000)}
	weekday := countAt(in, at("1996-10-15 14:00")) // Tuesday
	saturday := countAt(in, at("1996-10-19 14:00"))
	if weekday >= saturday {
		t.Fatalf("weekday 14:00 %d, saturday 14:00 %d", weekday, saturday)
	}
	substitute := countAt(in, at("1996-11-04 14:00"))
	mondays := 0
	for _, d := range []string{"1996-10-14", "1996-10-21", "1996-10-28", "1996-11-11"} {
		mondays += countAt(in, at(d+" 14:00"))
	}
	if float64(substitute) <= float64(mondays)/4 {
		t.Fatalf("1996-11-04 14:00 %d, ordinary Mondays average %.1f", substitute, float64(mondays)/4)
	}
}

func TestObservedSessionsWin(t *testing.T) {
	host := world.Host{ID: "h", Lines: 2, MaxBaud: 9600}
	ts := at("1996-10-15 23:30")
	stored := world.PresenceSession{HostID: "h", Actor: world.ActorRef{Kind: world.ActorKindPersona, ID: "gone"}, Handle: "GONE", Line: 1, Baud: 9600, Start: at("1996-10-15 23:00"), End: at("1996-10-16 00:00")}
	got := Compute(Input{Host: host, Personas: testPersonas(300), Observed: []world.PresenceSession{stored}}, ts).Sessions
	if len(got) == 0 || got[0] != stored {
		t.Fatalf("stored session must keep line 1: %v", got)
	}
}

func TestPresenceHistogram(t *testing.T) {
	if !*histogram {
		t.Skip("run with -presence-histogram -v to print hourly counts")
	}
	in := Input{Host: world.Host{ID: "hist", Lines: 8, MaxBaud: 14400, TelehoFriendly: true}, Personas: testPersonas(300), Calendar: worldcalendar.Default()}
	for _, d := range []string{"1996-10-15", "1996-10-19", "1996-12-24", "1996-12-31"} {
		line := d + ":"
		for h := 0; h < 24; h++ {
			line += fmt.Sprintf(" %02d=%d", h, countAt(in, at(fmt.Sprintf("%s %02d:30", d, h))))
		}
		t.Log(line)
	}
}
