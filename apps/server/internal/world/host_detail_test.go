package world

import "testing"

func TestHostDetailComesFromThePreset(t *testing.T) {
	s := NewMemoryStore()

	d, ok := s.HostDetail("hakata-canal-net")
	if !ok || d.ErikaK == nil {
		t.Fatalf("the sample station has no Erika-K detail: ok=%v detail=%+v", ok, d)
	}
	if len(d.ErikaK.Boards) != 29 {
		t.Fatalf("boards = %d, want 29", len(d.ErikaK.Boards))
	}
	if d.ErikaK.Texts.MainMenuTitle == "" || len(d.ErikaK.Texts.LoginBanner) == 0 {
		t.Fatalf("the sample station's texts are empty: %+v", d.ErikaK.Texts)
	}

	// A preset that does not run Erika-K has no Erika-K detail, and an unknown
	// host has no detail at all.
	if other, _ := s.HostDetail("busy-test"); other.ErikaK != nil {
		t.Fatalf("busy-test has Erika-K detail: %+v", other.ErikaK)
	}
	if _, ok := s.HostDetail("no-such-host"); ok {
		t.Fatal("an unknown host has detail")
	}
}

// The detail is part of the immutable host definition, so a store only ever
// serves what the presets define: registering a host does not invent any.
func TestHostDetailIsNotInventedForRegisteredHosts(t *testing.T) {
	s := NewMemoryStore()
	s.SaveHost(Host{ID: "registered-later", Phone: "0450000098", Name: "LATER"})
	if d, _ := s.HostDetail("registered-later"); d.ErikaK != nil {
		t.Fatalf("a host that is not in the presets has detail: %+v", d.ErikaK)
	}
}

func TestMemoryStoreIsAHostDetailStore(t *testing.T) {
	var _ HostDetailStore = NewMemoryStore()
}
