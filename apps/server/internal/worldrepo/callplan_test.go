package worldrepo

import (
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"zutto-pccom/apps/server/internal/hostcatalog"
	"zutto-pccom/apps/server/internal/hostprogram/erikak"
	"zutto-pccom/apps/server/internal/presence"
	"zutto-pccom/apps/server/internal/world"
)

// roomsStore gives the test host chat rooms without touching any preset.
type roomsStore struct {
	*world.MemoryStore
	rooms []hostcatalog.ChatRoom
}

func (s roomsStore) HostDetail(hostID string) (hostcatalog.PresetDetail, bool) {
	d, _ := s.MemoryStore.HostDetail(hostID)
	d.ChatRooms = s.rooms
	return d, true
}

var testRooms = []hostcatalog.ChatRoom{{Number: 0, Capacity: 0, Weight: .7}, {Number: 1, Capacity: 3, Weight: .3}}

func chatTestRepo(t *testing.T, chat bool) (*Repository, *world.MemoryStore, world.Host, time.Time) {
	t.Helper()
	repo, store, host := presenceTestRepo(t, time.Time{})
	now := busiestTime(t, repo, store, host)
	repo = New(roomsStore{MemoryStore: store, rooms: testRooms}, nil, nil, now.Format(time.DateOnly))
	repo.SetWorldNow(func() time.Time { return now })
	repo.SetResidentChat(chat)
	return repo, store, host, now
}

func TestCallActivitiesNowMatchesTheLines(t *testing.T) {
	repo, store, host, now := chatTestRepo(t, true)
	acts := repo.CallActivitiesNow(host)
	sessions := repo.ObservePresenceNow(host)
	residents := 0
	for _, s := range sessions {
		if s.Actor.Kind == world.ActorKindPersona {
			residents++
		}
	}
	if residents == 0 || len(acts) != residents {
		t.Fatalf("%d plans for %d residents on the lines", len(acts), residents)
	}
	for _, a := range acts {
		if _, ok := a.SegmentAt(now); !ok {
			t.Fatalf("plan of %s does not cover now: %+v", a.Actor.ID, a.Segments)
		}
	}
	if stored := store.ListCallActivities(host.ID, now.Add(-presence.Lookback), time.Time{}); len(stored) != len(acts) {
		t.Fatalf("stored %d plans, observed %d", len(stored), len(acts))
	}
}

func TestObservedPlansSurviveNewResidents(t *testing.T) {
	repo, store, host, _ := chatTestRepo(t, true)
	before := repo.CallActivitiesNow(host)
	for i := 0; i < 200; i++ {
		p := world.Persona{ID: fmt.Sprintf("newcomer-%03d", i), Handle: fmt.Sprintf("NEW%03d", i), ActivityPattern: "regular", ReplyTendency: .9}
		store.SavePersona(p)
		store.AddMembership(host.ID, p.ID)
	}
	after := repo.CallActivitiesNow(host)
	byKey := map[string]world.CallActivity{}
	for _, a := range after {
		byKey[a.Key()] = a
	}
	for _, a := range before {
		got, ok := byKey[a.Key()]
		if !ok || fmt.Sprint(got.Segments) != fmt.Sprint(a.Segments) {
			t.Fatalf("observed plan of %s changed after new residents joined:\n%+v\n%+v", a.Actor.ID, a.Segments, got.Segments)
		}
	}
}

func TestConcurrentCallActivitiesSaveOnce(t *testing.T) {
	repo, store, host, now := chatTestRepo(t, true)
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); repo.CallActivitiesNow(host) }()
	}
	wg.Wait()
	seen := map[string]bool{}
	for _, a := range store.ListCallActivities(host.ID, now.Add(-presence.Lookback), time.Time{}) {
		if seen[a.Key()] {
			t.Fatalf("plan saved twice: %s", a.Key())
		}
		seen[a.Key()] = true
	}
	if len(seen) == 0 {
		t.Fatal("no plan saved")
	}
}

func TestChatEventsAreOrderedAndNotRepeated(t *testing.T) {
	repo, _, host, now := chatTestRepo(t, true)
	from := now.Add(-time.Hour)
	var events []world.ChatEvent
	for _, room := range []int{0, 1} {
		var roomEvents []world.ChatEvent
		for t0 := from; t0.Before(now.Add(time.Hour)); t0 = t0.Add(10 * time.Minute) {
			repo.SetWorldNow(func() time.Time { return t0 })
			roomEvents = append(roomEvents, repo.ChatEvents(host, room, t0, t0.Add(10*time.Minute))...)
		}
		for i := 1; i < len(roomEvents); i++ {
			if roomEvents[i].At.Before(roomEvents[i-1].At) {
				t.Fatalf("room %d events out of order: %+v then %+v", room, roomEvents[i-1], roomEvents[i])
			}
		}
		inRoom := map[string]bool{}
		for _, e := range roomEvents {
			if e.Room != room || e.Handle == "" {
				t.Fatalf("bad event %+v", e)
			}
			switch e.Kind {
			case world.ChatJoin:
				if inRoom[e.Actor.ID] {
					t.Fatalf("%s joined room %d twice", e.Actor.ID, room)
				}
				inRoom[e.Actor.ID] = true
			case world.ChatLeave:
				// A leave without a join is someone already in the room an hour ago.
				delete(inRoom, e.Actor.ID)
			}
		}
		events = append(events, roomEvents...)
	}
	if len(events) == 0 {
		t.Fatal("no chat events in two busy hours; the checks proved nothing")
	}

	// Asking again for the same window gives the same events.
	repo.SetWorldNow(func() time.Time { return now })
	a := repo.ChatEvents(host, 0, from, now.Add(time.Hour))
	b := repo.ChatEvents(host, 0, from, now.Add(time.Hour))
	if fmt.Sprint(a) != fmt.Sprint(b) {
		t.Fatalf("the same window gave different events:\n%v\n%v", a, b)
	}
}

func TestResidentChatOffMeansNoChat(t *testing.T) {
	repo, _, host, now := chatTestRepo(t, false)
	for _, a := range repo.CallActivitiesNow(host) {
		for _, seg := range a.Segments {
			if seg.State == world.CallChat || seg.State == world.CallPaging {
				t.Fatalf("plan has %s with resident chat off: %+v", seg.State, a.Segments)
			}
		}
	}
	if events := repo.ChatEvents(host, 0, now.Add(-time.Hour), now.Add(time.Hour)); len(events) != 0 {
		t.Fatalf("chat events with resident chat off: %+v", events)
	}
}

// TestCallActivitiesNowTiming measures one WHO of the chat screen on a
// 300-resident host (spec 016 T008).
func TestCallActivitiesNowTiming(t *testing.T) {
	repo, store, host, now := chatTestRepo(t, true)
	for i := len(store.ListHostPersonas(host.ID)); i < 300; i++ {
		p := world.Persona{ID: fmt.Sprintf("timing-%03d", i), Handle: fmt.Sprintf("T%03d", i), ActivityPattern: "active"}
		store.SavePersona(p)
		store.AddMembership(host.ID, p.ID)
	}
	if n := len(store.ListHostPersonas(host.ID)); n < 300 {
		t.Fatalf("%d residents, want 300", n)
	}
	// The first WHO at a time computes four hours of presence and saves what it
	// saw, which changes the cache fingerprint; from then on the window grid is
	// cached until something new is saved.
	var cold, warm time.Duration
	for i := 0; i < 5; i++ {
		at := now.Add(time.Duration(i) * 7 * time.Minute)
		repo.SetWorldNow(func() time.Time { return at })
		start := time.Now()
		repo.CallActivitiesNow(host)
		cold = max(cold, time.Since(start))
		repo.CallActivitiesNow(host)
		start = time.Now()
		repo.CallActivitiesNow(host)
		warm = max(warm, time.Since(start))
	}
	t.Logf("CallActivitiesNow on %d residents: worst of 5 = %v (repeated at the same time: %v)", len(store.ListHostPersonas(host.ID)), cold, warm)
}

// TestErikaKChatThroughRepository drives the station's own chat screens
// against the real world: the call screen shows the residents' states, and a
// caller sitting in a room sees joins and leaves in time order.
func TestErikaKChatThroughRepository(t *testing.T) {
	repo, store, host := presenceTestRepo(t, time.Time{})
	now := busiestTime(t, repo, store, host)
	current := now
	repo.SetWorldNow(func() time.Time { return current })
	repo.SetResidentChat(true)
	host.Lines = 16
	line, ok := repo.BeginHumanCall(host, "call-chat", "", 14400)
	if !ok {
		t.Fatal("no line for the caller")
	}
	rt := erikak.New(host, repo)
	rt.SetCallInfo(world.CallInfo{CallID: "call-chat", Line: line, Baud: 14400})
	rt.HandleLine("GUEST")
	out, _ := rt.HandleLine("4")
	if !strings.Contains(out, fmt.Sprintf("#%02d GUEST     WHO       14K-...", line)) || !strings.HasSuffix(out, "CALL> ->") {
		t.Fatalf("call screen:\n%s", out)
	}
	rows := 0
	for _, l := range strings.Split(out, "\r\n") {
		if strings.HasPrefix(l, "#") {
			rows++
		}
	}
	if rows != host.Lines {
		t.Fatalf("%d WHO rows for %d lines:\n%s", rows, host.Lines, out)
	}
	rt.HandleLine("X")
	if out, _ = rt.HandleLine("0"); !strings.Contains(out, "## chat on  GUEST ##") {
		t.Fatalf("room entry:\n%s", out)
	}
	var shown strings.Builder
	for i := 1; i <= 360; i++ {
		current = now.Add(time.Duration(i) * 20 * time.Second)
		shown.WriteString(rt.Poll(current))
	}
	if !strings.Contains(shown.String(), "## chat") {
		t.Fatalf("two hours in room 0 showed nobody coming or going")
	}
	for _, l := range strings.Split(strings.TrimSuffix(shown.String(), "\r\n"), "\r\n") {
		if !strings.HasPrefix(l, "\a\a## chat on  ") && !strings.HasPrefix(l, "\a\a## chat off ") {
			t.Fatalf("unexpected room line %q", l)
		}
	}
}
