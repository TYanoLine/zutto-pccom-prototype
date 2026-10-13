package callplan

import (
	"fmt"
	"reflect"
	"sort"
	"testing"
	"time"

	"zutto-pccom/apps/server/internal/presence"
	"zutto-pccom/apps/server/internal/world"
)

func at(s string) time.Time {
	t, err := time.ParseInLocation("2006-01-02 15:04", s, time.FixedZone("JST", 9*3600))
	if err != nil {
		panic(err)
	}
	return t
}

func testPersonas(n int) []world.Persona {
	patterns := []string{"regular", "active", "active", "occasional", "occasional", "lurker", "lurker", "dormant"}
	out := make([]world.Persona, n)
	for i := range out {
		out[i] = world.Persona{
			ID: fmt.Sprintf("p%04d", i), Handle: fmt.Sprintf("H%04d", i), ActivityPattern: patterns[i%len(patterns)],
			LurkerTendency: float64(i%10) / 10, ReplyTendency: float64((i*7)%10) / 10,
		}
	}
	return out
}

func byID(personas []world.Persona) map[string]world.Persona {
	out := make(map[string]world.Persona, len(personas))
	for _, p := range personas {
		out[p.ID] = p
	}
	return out
}

// sessionsBetween collects the resident sessions presence places on the host
// in [from, to]. Like the repository, it keeps what it saw as observed, so the
// sessions stay consistent with each other.
func sessionsBetween(host world.Host, personas []world.Persona, humans []world.HumanCall, from, to time.Time) ([]world.PresenceSession, map[string]presence.Profile) {
	seen := map[string]bool{}
	var out []world.PresenceSession
	profiles := map[string]presence.Profile{}
	for t := from; !t.After(to); t = t.Add(presence.Window) {
		var observed []world.PresenceSession
		for _, s := range out {
			if s.Overlaps(t.Add(-presence.Lookback), time.Time{}) {
				observed = append(observed, s)
			}
		}
		res := presence.Compute(presence.Input{Host: host, Personas: personas, Humans: humans, Observed: observed}, t)
		for _, s := range res.Sessions {
			key := activityKey(s)
			if s.Actor.Kind != world.ActorKindPersona || seen[key] {
				continue
			}
			seen[key] = true
			out = append(out, s)
		}
		for id, p := range res.Profiles {
			profiles[id] = p
		}
	}
	return out, profiles
}

var twoRooms = []world.ChatRoom{{Number: 0, Capacity: 0, Weight: .7}, {Number: 1, Capacity: 2, Weight: .3}}

func planDay(host world.Host, rooms []world.ChatRoom, personas []world.Persona, day string) ([]world.PresenceSession, map[string]world.CallActivity) {
	sessions, profiles := sessionsBetween(host, personas, nil, at(day+" 00:00"), at(day+" 23:50"))
	return sessions, Plan(Input{Host: host, Rooms: rooms, Chat: true, Sessions: sessions, Profiles: profiles, Personas: byID(personas)})
}

// checkPlan verifies the shape every plan must have: contiguous segments that
// cover the call, login first, and paging right before each chat.
func checkPlan(t *testing.T, s world.PresenceSession, a world.CallActivity, rooms []world.ChatRoom) {
	t.Helper()
	if len(a.Segments) == 0 || !a.Segments[0].From.Equal(s.Start) || !a.End().Equal(s.End) {
		t.Fatalf("plan %+v does not cover the call %v-%v", a, s.Start, s.End)
	}
	if a.Segments[0].State != world.CallLogin {
		t.Fatalf("plan does not start with login: %+v", a.Segments)
	}
	known := map[int]bool{}
	for _, r := range rooms {
		known[r.Number] = true
	}
	for i, seg := range a.Segments {
		if !seg.To.After(seg.From) {
			t.Fatalf("empty segment %+v", seg)
		}
		if i > 0 && !seg.From.Equal(a.Segments[i-1].To) {
			t.Fatalf("segments are not contiguous: %+v", a.Segments)
		}
		if seg.State == world.CallChat {
			if i == 0 || a.Segments[i-1].State != world.CallPaging {
				t.Fatalf("chat is not preceded by paging: %+v", a.Segments)
			}
			if !known[seg.Room] {
				t.Fatalf("chat in an undefined room %d", seg.Room)
			}
		}
	}
}

func TestPlanIsDeterministic(t *testing.T) {
	host := world.Host{ID: "h", Lines: 3, MaxBaud: 28800, TelehoFriendly: true}
	personas := testPersonas(300)
	sessions, a := planDay(host, twoRooms, personas, "1996-10-15")
	if len(a) == 0 {
		t.Fatal("no plans")
	}
	rev := append([]world.PresenceSession(nil), sessions...)
	sort.Slice(rev, func(i, j int) bool { return i > j })
	_, profiles := sessionsBetween(host, personas, nil, at("1996-10-15 00:00"), at("1996-10-15 23:50"))
	b := Plan(Input{Host: host, Rooms: twoRooms, Chat: true, Sessions: rev, Profiles: profiles, Personas: byID(personas)})
	if !reflect.DeepEqual(a, b) {
		t.Fatal("the same input gave different plans")
	}
	for _, s := range sessions {
		checkPlan(t, s, a[activityKey(s)], twoRooms)
	}
}

func TestPlanWithoutChatHasNoChat(t *testing.T) {
	host := world.Host{ID: "h", Lines: 3, MaxBaud: 28800, TelehoFriendly: true}
	personas := testPersonas(300)
	sessions, profiles := sessionsBetween(host, personas, nil, at("1996-10-15 00:00"), at("1996-10-15 23:50"))
	for name, in := range map[string]Input{
		"chat disabled": {Host: host, Rooms: twoRooms, Chat: false},
		"no rooms":      {Host: host, Chat: true},
	} {
		in.Sessions, in.Profiles, in.Personas = sessions, profiles, byID(personas)
		plans := Plan(in)
		if len(plans) != len(sessions) {
			t.Fatalf("%s: %d plans for %d sessions", name, len(plans), len(sessions))
		}
		for _, s := range sessions {
			a := plans[activityKey(s)]
			checkPlan(t, s, a, nil)
			for _, seg := range a.Segments {
				if seg.State == world.CallChat || seg.State == world.CallPaging {
					t.Fatalf("%s: plan has %s: %+v", name, seg.State, a.Segments)
				}
			}
		}
	}
}

func TestPlanIgnoresHumans(t *testing.T) {
	host := world.Host{ID: "h", Lines: 3, MaxBaud: 28800, TelehoFriendly: true}
	personas := testPersonas(300)
	sessions, profiles := sessionsBetween(host, personas, nil, at("1996-10-15 20:00"), at("1996-10-16 02:00"))
	in := Input{Host: host, Rooms: twoRooms, Chat: true, Sessions: sessions, Profiles: profiles, Personas: byID(personas)}
	without := Plan(in)

	human := world.HumanCall{CallID: "c1", HostID: host.ID, Handle: "GUEST", Line: 1, Start: at("1996-10-15 22:00")}
	in.Sessions = append(append([]world.PresenceSession(nil), sessions...), human.Session())
	with := Plan(in)
	if !reflect.DeepEqual(without, with) {
		t.Fatal("a human call changed the residents' plans")
	}
	if _, ok := with[activityKey(human.Session())]; ok {
		t.Fatal("the human call was planned")
	}
}

func TestPlanKeepsObservedPlans(t *testing.T) {
	host := world.Host{ID: "h", Lines: 3, MaxBaud: 28800, TelehoFriendly: true}
	personas := testPersonas(300)
	sessions, a := planDay(host, twoRooms, personas, "1996-10-15")
	first := sessions[0]
	stored := a[activityKey(first)]
	_, profiles := sessionsBetween(host, personas, nil, at("1996-10-15 00:00"), at("1996-10-15 23:50"))
	b := Plan(Input{Host: host, Rooms: twoRooms, Chat: true, Sessions: sessions, Profiles: profiles, Personas: byID(personas), Observed: []world.CallActivity{stored}})
	if _, ok := b[activityKey(first)]; ok {
		t.Fatal("an observed call was planned again")
	}
	if len(b) != len(a)-1 {
		t.Fatalf("%d plans, want %d", len(b), len(a)-1)
	}
}

func TestPlanChatsMoreInTelehoHours(t *testing.T) {
	host := world.Host{ID: "h", Lines: 64, MaxBaud: 28800}
	personas := testPersonas(400)
	var sessions []world.PresenceSession
	day := at("1996-09-01 00:00")
	for d := 0; d < 60; d++ {
		for i, p := range personas {
			if i%4 != d%4 {
				continue
			}
			for _, clock := range []time.Duration{14 * time.Hour, 23*time.Hour + 30*time.Minute} {
				start := day.AddDate(0, 0, d).Add(clock)
				sessions = append(sessions, world.PresenceSession{HostID: host.ID, Actor: world.ActorRef{Kind: world.ActorKindPersona, ID: p.ID},
					Handle: p.Handle, Start: start, End: start.Add(30 * time.Minute)})
			}
		}
	}
	plans := Plan(Input{Host: host, Rooms: twoRooms, Chat: true, Sessions: sessions, Personas: byID(personas)})
	afternoon, night := 0, 0
	for _, a := range plans {
		for _, seg := range a.Segments {
			if seg.State != world.CallChat {
				continue
			}
			if a.Start.Hour() == 14 {
				afternoon++
			} else {
				night++
			}
		}
	}
	if night <= afternoon {
		t.Fatalf("chat segments: 23:30 %d, 14:00 %d; want more at night", night, afternoon)
	}
}

func TestPlanRespectsLinesAndCapacity(t *testing.T) {
	layouts := map[string][]world.ChatRoom{
		"one room":       {{Number: 0, Capacity: 0, Weight: 1}},
		"two rooms":      twoRooms,
		"tight rooms":    {{Number: 0, Capacity: 1, Weight: .5}, {Number: 3, Capacity: 2, Weight: .3}, {Number: 7, Capacity: 3, Weight: .2}},
		"unlimited trio": {{Number: 0, Weight: 1}, {Number: 1, Weight: 1}, {Number: 2, Weight: 1}},
	}
	personas := testPersonas(300)
	totalChats := 0
	for _, lines := range []int{1, 3, 8, 16} {
		host := world.Host{ID: fmt.Sprintf("h%d", lines), Lines: lines, MaxBaud: 28800, TelehoFriendly: true}
		sessions, profiles := sessionsBetween(host, personas, nil, at("1996-10-18 00:00"), at("1996-10-18 23:50"))
		for name, rooms := range layouts {
			t.Run(fmt.Sprintf("%d lines %s", lines, name), func(t *testing.T) {
				plans := Plan(Input{Host: host, Rooms: rooms, Chat: true, Sessions: sessions, Profiles: profiles, Personas: byID(personas)})
				capacity := map[int]int{}
				for _, r := range rooms {
					capacity[r.Number] = r.Capacity
				}
				var chats []world.CallSegment
				for _, s := range sessions {
					a := plans[activityKey(s)]
					checkPlan(t, s, a, rooms)
					for _, seg := range a.Segments {
						if seg.State == world.CallChat {
							chats = append(chats, seg)
						}
					}
				}
				for _, s := range sessions {
					online := 0
					for _, o := range sessions {
						if o.ActiveAt(s.Start) {
							online++
						}
					}
					if online > lines {
						t.Fatalf("%v: %d calls on %d lines", s.Start, online, lines)
					}
				}
				totalChats += len(chats)
				for _, c := range chats {
					in := 0
					for _, o := range chats {
						if o.Room == c.Room && !c.From.Before(o.From) && c.From.Before(o.To) {
							in++
						}
					}
					if limit := capacity[c.Room]; limit > 0 && in > limit {
						t.Fatalf("room %d holds %d at %v, capacity %d", c.Room, in, c.From, limit)
					}
				}
			})
		}
	}
	if totalChats == 0 {
		t.Fatal("nobody chatted on any host; the capacity checks proved nothing")
	}
}

func TestPlanNeverOverfillsAroundStoredPlans(t *testing.T) {
	host := world.Host{ID: "h", Lines: 16, MaxBaud: 28800}
	rooms := []world.ChatRoom{{Number: 0, Capacity: 1, Weight: 1}}
	start := at("1996-10-18 23:30")
	// A stored plan holds the single seat from 23:50 to 00:30.
	stored := world.CallActivity{HostID: host.ID, Actor: world.ActorRef{Kind: world.ActorKindPersona, ID: "stored"}, Start: start, Segments: []world.CallSegment{
		{From: start, To: start.Add(2 * time.Minute), State: world.CallLogin},
		{From: start.Add(2 * time.Minute), To: start.Add(19 * time.Minute), State: world.CallIdle},
		{From: start.Add(19 * time.Minute), To: start.Add(20 * time.Minute), State: world.CallPaging},
		{From: start.Add(20 * time.Minute), To: start.Add(time.Hour), State: world.CallChat},
	}}
	var sessions []world.PresenceSession
	personas := testPersonas(200)
	for i, p := range personas {
		s := start.Add(time.Duration(i%20) * time.Minute)
		sessions = append(sessions, world.PresenceSession{HostID: host.ID, Actor: world.ActorRef{Kind: world.ActorKindPersona, ID: p.ID}, Start: s, End: s.Add(90 * time.Minute)})
	}
	plans := Plan(Input{Host: host, Rooms: rooms, Chat: true, Sessions: sessions, Personas: byID(personas), Observed: []world.CallActivity{stored}})
	chats := 0
	for _, a := range plans {
		for _, seg := range a.Segments {
			if seg.State != world.CallChat {
				continue
			}
			chats++
			if seg.From.Before(start.Add(time.Hour)) && seg.To.After(start.Add(20*time.Minute)) {
				t.Fatalf("%s sits in the full room %v-%v while the stored plan holds it", a.Actor.ID, seg.From, seg.To)
			}
		}
	}
	if chats == 0 {
		t.Fatal("nobody used the room before or after the stored stay; the check proved nothing")
	}
}
