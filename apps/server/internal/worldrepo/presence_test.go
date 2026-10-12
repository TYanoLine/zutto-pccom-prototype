package worldrepo

import (
	"fmt"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"zutto-pccom/apps/server/internal/presence"
	"zutto-pccom/apps/server/internal/world"
)

const presenceTestPhone = "0920000196" // a preset host with a resident population

func presenceTestRepo(t *testing.T, now time.Time) (*Repository, *world.MemoryStore, world.Host) {
	t.Helper()
	store := world.NewMemoryStore()
	host, err := store.HostByPhone(presenceTestPhone)
	if err != nil {
		t.Fatal(err)
	}
	if len(store.ListHostPersonas(host.ID)) == 0 {
		t.Fatal("test host has no residents")
	}
	repo := New(store, nil, nil, now.Format(time.DateOnly))
	repo.SetWorldNow(func() time.Time { return now })
	return repo, store, host
}

// busiestTime finds a late-night time with residents online so the tests do not
// depend on tuning values.
func busiestTime(t *testing.T, repo *Repository, store *world.MemoryStore, host world.Host) time.Time {
	t.Helper()
	base := time.Date(1996, 10, 15, 23, 40, 0, 0, time.FixedZone("JST", 9*60*60))
	for i := 0; i < 60; i++ {
		ts := base.AddDate(0, 0, i)
		in := presence.Input{Host: host, Personas: store.ListHostPersonas(host.ID), Facts: store.ListPersonaFacts}
		if len(presence.Compute(in, ts).Sessions) > 0 {
			return ts
		}
	}
	t.Fatal("no resident online at 23:40 in sixty days")
	return time.Time{}
}

func TestObservePresenceStoresHistoryAndProfiles(t *testing.T) {
	repo, store, host := presenceTestRepo(t, time.Time{})
	now := busiestTime(t, repo, store, host)
	repo.SetWorldNow(func() time.Time { return now })

	first := repo.ObservePresenceNow(host)
	second := repo.ObservePresenceNow(host)
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("same time, different sessions:\n%v\n%v", first, second)
	}
	stored := store.ListObservedSessions(host.ID, now, now.Add(time.Second))
	if len(stored) != len(first) {
		t.Fatalf("stored %d sessions, observed %d", len(stored), len(first))
	}
	for _, s := range first {
		keys := map[string]bool{}
		for _, f := range store.ListPersonaFacts(s.Actor.ID) {
			keys[f.Key] = true
		}
		if !keys[presence.FactLifestyle] || !keys[presence.FactTeleho(host.ID)] {
			t.Fatalf("profile facts not stored for %s: %v", s.Actor.ID, keys)
		}
	}
}

func TestObservedPresenceSurvivesNewResidents(t *testing.T) {
	repo, store, host := presenceTestRepo(t, time.Time{})
	now := busiestTime(t, repo, store, host)
	repo.SetWorldNow(func() time.Time { return now })
	before := repo.ObservePresenceNow(host)

	for i := 0; i < 200; i++ {
		p := world.Persona{ID: fmt.Sprintf("newcomer-%03d", i), Handle: fmt.Sprintf("NEW%03d", i), ActivityPattern: "regular"}
		store.SavePersona(p)
		store.AddMembership(host.ID, p.ID)
	}
	after := repo.ObservePresenceNow(host)
	for _, s := range before {
		found := false
		for _, a := range after {
			if a == s {
				found = true
			}
		}
		if !found {
			t.Fatalf("observed session %v disappeared after new residents joined: %v", s, after)
		}
	}
}

func TestConcurrentObservationSavesOnce(t *testing.T) {
	repo, store, host := presenceTestRepo(t, time.Time{})
	now := busiestTime(t, repo, store, host)
	repo.SetWorldNow(func() time.Time { return now })
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); repo.ObservePresenceNow(host) }()
	}
	wg.Wait()
	seen := map[string]bool{}
	for _, s := range store.ListObservedSessions(host.ID, now.Add(-presence.Lookback), time.Time{}) {
		key := s.Actor.String() + s.Start.String()
		if seen[key] {
			t.Fatalf("session saved twice: %v", s)
		}
		seen[key] = true
	}
}

func TestHumanCallsHoldLines(t *testing.T) {
	now := time.Date(1996, 10, 15, 14, 0, 0, 0, time.FixedZone("JST", 9*60*60))
	repo, store, host := presenceTestRepo(t, now)
	host.Lines = 1 + len(repo.ObservePresenceNow(host))
	line, ok := repo.BeginHumanCall(host, "call-1", 9600)
	if !ok {
		t.Fatal("first human call should get the free line")
	}
	if _, ok := repo.DialLine(host, now); ok {
		t.Fatal("all lines are in use; the next dial must be BUSY")
	}
	if _, ok := repo.BeginHumanCall(host, "call-2", 9600); ok {
		t.Fatal("second human call must not get a line")
	}
	repo.SetHumanCallHandle("call-1", "GUEST")
	who := repo.ObservePresenceNow(host)
	found := false
	for _, s := range who {
		if s.Actor.ID == "call-1" && s.Line == line && s.Handle == "GUEST" {
			found = true
		}
	}
	if !found {
		t.Fatalf("human call not on line %d: %v", line, who)
	}
	later := now.Add(time.Minute)
	repo.SetWorldNow(func() time.Time { return later })
	repo.EndHumanCall("call-1")
	if got, ok := repo.DialLine(host, later.Add(time.Second)); !ok || got != line {
		t.Fatalf("line %d should be free after hang-up, got %d %v", line, got, ok)
	}
	if len(store.ListHumanCalls(host.ID)) != 0 {
		t.Fatal("ended call still live")
	}
	ended := store.ListObservedSessions(host.ID, now.Add(-time.Minute), time.Time{})
	hasEnded := false
	for _, s := range ended {
		if s.Actor.ID == "call-1" && !s.Live() && strings.EqualFold(s.Handle, "guest") {
			hasEnded = true
		}
	}
	if !hasEnded {
		t.Fatalf("ended call not kept as history: %v", ended)
	}
}
