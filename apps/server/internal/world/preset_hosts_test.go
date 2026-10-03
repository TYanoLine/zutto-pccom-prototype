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
		{ID: "moonlight-yokohama", Phone: "0451234567", Name: "YOKOHAMA MOONLIGHT NETWORK", Region: "神奈川県横浜市", Software: "KTBBS compatible / customized", SoftwareID: "generic", Lines: 4, Popularity: .70, MaxBaud: 14400, Members: 187, FoundedOn: "1994-06-12", ANSI: true, GuestAllowed: true, TelehoFriendly: true},
		{ID: "hakata-canal-net", Phone: "0920000196", Name: "HAKATA CANAL NET", Region: "福岡県福岡市", Software: "絵理香K版", SoftwareID: "erika-k", Lines: 3, Popularity: .58, MaxBaud: 14400, Members: 326, FoundedOn: "1994-11-03", ANSI: false, GuestAllowed: true, TelehoFriendly: true},
		{ID: "silver-horizon-bbs", Phone: "0470001080", Name: "SILVER HORIZON BBS", Region: "千葉県", Software: "TurboBBS 1.08 compatible / customized", SoftwareID: "turbobbs", Lines: 1, Popularity: .18, MaxBaud: 2400, Members: 52, FoundedOn: "1989-08-20", ANSI: false, GuestAllowed: false, TelehoFriendly: true},
		{ID: "quiet-test", Phone: "0450000001", Name: "QUIET TEST BBS", Region: "神奈川県", Software: "mmm compatible", SoftwareID: "generic", Lines: 8, Popularity: .05, MaxBaud: 28800, Members: 22, FoundedOn: "1996-05-05", ANSI: false, GuestAllowed: true},
		{ID: "busy-test", Phone: "0459999999", Name: "POPULAR TEST BBS", Region: "神奈川県", Software: "BIG-Model compatible", SoftwareID: "generic", Lines: 1, Popularity: 1, MaxBaud: 14400, Members: 912, FoundedOn: "1993-09-15", ANSI: true, GuestAllowed: true},
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
		"moonlight-yokohama": 3,
		"silver-horizon-bbs": 4,
		"hakata-canal-net":   0, // evaluation station: no article seed at all
		"quiet-test":         0,
		"busy-test":          0,
	} {
		if got := len(store.ListPosts(key)); got != wantPosts {
			t.Errorf("%s: %d seed posts, want %d", key, got, wantPosts)
		}
	}
	if len(store.ListHostPersonas("hakata-canal-net")) == 0 {
		t.Error("HAKATA resident population was not created")
	}
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

func TestMustPresetHostPanicsWhenMissing(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected a panic for a missing preset host")
		}
	}()
	mustPresetHost(map[string]Host{}, "hakata-canal-net")
}
