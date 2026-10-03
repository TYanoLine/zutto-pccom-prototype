package world

import (
	"testing"

	"zutto-pccom/apps/server/internal/hostcatalog"
)

// Golden record of the hosts the store has always provided. They used to be
// hard-coded in NewMemoryStore and now come from hostcatalog/presets/*.yaml, so
// this guards both the YAML content and the descriptor -> Host conversion.
func TestPresetHostsMatchHistoricalFixtures(t *testing.T) {
	want := []Host{
		{
			ID: "hakata-canal-net", Role: "experiment", Phone: "0920000196", Name: "HAKATA CANAL NET", Region: "福岡県福岡市", Software: "絵理香K版", SoftwareID: "erika-k", Lines: 3, Popularity: .58, MaxBaud: 14400, Members: 326, FoundedOn: "1994-11-03", ANSI: false, GuestAllowed: true, TelehoFriendly: true,
			Debug: hostcatalog.DebugFlags{
				ResetArticlesOnConnect: true,
				GenerationTrace:        true,
				ContentLog:             true,
				HTTPEndpoints:          true,
				Snapshot:               true,
			},
			Generation: hostcatalog.GenerationFlags{FreeformBody: true},
		},
		{ID: "busy-test", Role: "test", Phone: "0459999999", Name: "POPULAR TEST BBS", Region: "神奈川県", Software: "BIG-Model compatible", SoftwareID: "generic", Lines: 1, Popularity: 1, MaxBaud: 14400, Members: 912, FoundedOn: "1993-09-15", ANSI: true, GuestAllowed: true},
	}
	store := NewMemoryStore()
	for _, w := range want {
		got, err := store.HostByPhone(w.Phone)
		if err != nil {
			t.Errorf("%s: %v", w.ID, err)
			continue
		}
		if got != w {
			t.Errorf("%s changed:\n got  %+v\n want %+v", w.ID, got, w)
		}
	}
	if n := len(presetHosts()); n != len(want) {
		t.Errorf("presets define %d hosts, want %d", n, len(want))
	}
}

func TestMemoryStoreSeedDataIsKeyedToPresetHosts(t *testing.T) {
	store := NewMemoryStore()
	for key, wantPosts := range map[string]int{
		"hakata-canal-net": 0, // evaluation station: no article seed at all
		"busy-test":        0,
	} {
		if got := len(store.ListPosts(key)); got != wantPosts {
			t.Errorf("%s: %d seed posts, want %d", key, got, wantPosts)
		}
	}
	if len(store.ListHostPersonas("hakata-canal-net")) == 0 {
		t.Error("HAKATA resident population was not created")
	}
}

// genericTestHost registers a quiet generic-runtime host for tests that need one,
// so no production station has to exist for them.
func genericTestHost(store *MemoryStore) Host {
	h := Host{ID: "generic-test", Phone: "0450000010", Name: "GENERIC TEST BBS", Region: "神奈川県", Software: "mmm compatible", SoftwareID: "generic", Lines: 8, Popularity: .05, MaxBaud: 28800, Members: 22, FoundedOn: "1996-05-05", GuestAllowed: true}
	store.SaveHost(h)
	return h
}

func TestHostFromDescriptor(t *testing.T) {
	d := hostcatalog.HostDescriptor{
		Key: "sample-bbs", Phone: "0312345678", Name: "SAMPLE BBS", Program: "ktbbs",
		Region: hostcatalog.Region{Prefecture: "東京都", City: "渋谷区"},
		Lines:  2, MaxBaud: 9600, FoundedOn: "1995-01-02", Popularity: 0.4, Members: 10,
		Traits: hostcatalog.Traits{ANSI: true},
	}
	h := HostFromDescriptor(d)
	if h.ID != "sample-bbs" || h.Software != "ktbbs" || h.SoftwareID != "generic" || h.Region != "東京都渋谷区" || !h.ANSI || h.GuestAllowed {
		t.Fatalf("unexpected host: %+v", h)
	}
	d.SoftwareLabel, d.Program = "絵理香K版", "erika-k"
	if h := HostFromDescriptor(d); h.Software != "絵理香K版" || h.SoftwareID != "erika-k" {
		t.Fatalf("label/program mapping: %+v", h)
	}
}
