package world

import (
	"testing"
	"time"
)

func TestSaveCallActivitiesKeepsTheFirstPlan(t *testing.T) {
	s := NewMemoryStore()
	start := time.Date(1996, 4, 30, 23, 0, 0, 0, time.UTC)
	actor := ActorRef{Kind: ActorKindPersona, ID: "p1"}
	first := CallActivity{HostID: "h", Actor: actor, Start: start, Segments: []CallSegment{
		{From: start, To: start.Add(2 * time.Minute), State: CallLogin},
		{From: start.Add(2 * time.Minute), To: start.Add(20 * time.Minute), State: CallIdle},
	}}
	s.SaveCallActivities("h", []CallActivity{first})
	changed := first
	changed.Segments = []CallSegment{{From: start, To: start.Add(time.Hour), State: CallChat}}
	s.SaveCallActivities("h", []CallActivity{changed})

	got := s.ListCallActivities("h", start.Add(-time.Hour), time.Time{})
	if len(got) != 1 || len(got[0].Segments) != 2 || got[0].Segments[1].State != CallIdle {
		t.Fatalf("stored plans = %+v, want the first plan unchanged", got)
	}
	if later := s.ListCallActivities("h", start.Add(time.Minute), time.Time{}); len(later) != 0 {
		t.Fatalf("plans starting before from were listed: %+v", later)
	}
	if earlier := s.ListCallActivities("h", start.Add(-time.Hour), start); len(earlier) != 0 {
		t.Fatalf("plans starting at to were listed: %+v", earlier)
	}
	got[0].Segments[0].State = CallChat
	if again := s.ListCallActivities("h", start, time.Time{}); again[0].Segments[0].State != CallLogin {
		t.Fatal("a listed plan shares memory with the store")
	}
}

func TestCallActivitySegmentAt(t *testing.T) {
	start := time.Date(1996, 4, 30, 23, 0, 0, 0, time.UTC)
	a := CallActivity{Start: start, Segments: []CallSegment{
		{From: start, To: start.Add(2 * time.Minute), State: CallLogin},
		{From: start.Add(2 * time.Minute), To: start.Add(10 * time.Minute), State: CallChat, Room: 1},
	}}
	if seg, ok := a.SegmentAt(start.Add(2 * time.Minute)); !ok || seg.State != CallChat || seg.Room != 1 {
		t.Fatalf("SegmentAt(+2m) = %+v %v", seg, ok)
	}
	if _, ok := a.SegmentAt(start.Add(10 * time.Minute)); ok {
		t.Fatal("SegmentAt(end) found a segment")
	}
	if !a.End().Equal(start.Add(10 * time.Minute)) {
		t.Fatalf("End = %v", a.End())
	}
}
