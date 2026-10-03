package worldrepo

import (
	"testing"
	"time"

	"zutto-pccom/apps/server/internal/world"
)

func TestPlanBoardActivityUsesHostAgeMembersAndBoardWeight(t *testing.T) {
	now := time.Date(1996, 8, 26, 23, 30, 0, 0, time.Local)
	host := world.Host{
		ID:         "hakata-canal-net",
		Members:    326,
		Popularity: .58,
		FoundedOn:  "1994-11-03",
	}
	busy := world.Board{ID: "4", Name: "ふり～と～く", ActivityWeight: 1.25, ReplyRate: 2.0, RetainedRootCap: 60}
	quiet := world.Board{ID: "7", Name: "ＣＡＮＡＬ市場", ActivityWeight: .30, ReplyRate: .75, RetainedRootCap: 30}

	a, ok := planBoardActivity(host, busy, now)
	if !ok {
		t.Fatal("busy board activity was not planned")
	}
	b, ok := planBoardActivity(host, busy, now)
	if !ok || a != b {
		t.Fatalf("same world inputs were not deterministic: %#v != %#v", a, b)
	}
	q, ok := planBoardActivity(host, quiet, now)
	if !ok {
		t.Fatal("quiet board activity was not planned")
	}

	if a.OpenedOn != host.FoundedOn {
		t.Fatalf("board opened_on=%q, want host founding date %q", a.OpenedOn, host.FoundedOn)
	}
	if a.InitialMembers < 3 || a.InitialMembers >= a.CurrentMembers {
		t.Fatalf("unexpected membership curve endpoints: initial=%d current=%d", a.InitialMembers, a.CurrentMembers)
	}
	if a.AverageMembers <= float64(a.InitialMembers) || a.AverageMembers >= float64(a.CurrentMembers) {
		t.Fatalf("average members %.2f not between initial/current membership", a.AverageMembers)
	}
	if a.TotalRoots <= q.TotalRoots {
		t.Fatalf("busy roots=%d quiet roots=%d; board weight did not affect activity", a.TotalRoots, q.TotalRoots)
	}
	if a.TotalReplies <= a.TotalRoots {
		t.Fatalf("busy board replies=%d roots=%d; configured reply-heavy board did not differ", a.TotalReplies, a.TotalRoots)
	}
	if a.RetainedRoots > busy.RetainedRootCap || q.RetainedRoots > quiet.RetainedRootCap {
		t.Fatalf("retention caps ignored: busy=%d quiet=%d", a.RetainedRoots, q.RetainedRoots)
	}
	if a.LastPostAt.IsZero() || !a.LastPostAt.Before(now) {
		t.Fatalf("invalid last-post timestamp: %s", a.LastPostAt)
	}
}

func TestPlanBoardActivityChangesWithWorldDate(t *testing.T) {
	host := world.Host{
		ID:         "test-host",
		Members:    200,
		Popularity: .5,
		FoundedOn:  "1995-01-01",
	}
	board := world.Board{ID: "talk", ActivityWeight: 1.0, ReplyRate: 1.0, RetainedRootCap: 1000}
	early, ok := planBoardActivity(host, board, time.Date(1995, 8, 1, 12, 0, 0, 0, time.Local))
	if !ok {
		t.Fatal("early plan failed")
	}
	later, ok := planBoardActivity(host, board, time.Date(1996, 8, 1, 12, 0, 0, 0, time.Local))
	if !ok {
		t.Fatal("later plan failed")
	}
	if later.TotalRoots <= early.TotalRoots {
		t.Fatalf("later world date roots=%d, early=%d; host age did not accumulate activity", later.TotalRoots, early.TotalRoots)
	}
}
