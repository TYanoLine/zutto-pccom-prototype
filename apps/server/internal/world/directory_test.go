package world

import "testing"

// The contract values of the sample station. They are the values the web client
// used to hard-code as DEFAULT_CENTERS, so a change here is a visible change to
// the directory players see.
func TestListedHostsIsTheSampleStation(t *testing.T) {
	got := NewMemoryStore().ListedHosts()
	want := DirectoryEntry{
		ID: "hakata-canal-net", Name: "HAKATA CANAL NET", Software: "絵理香K版",
		Phone: "0920000196", DialMode: "tone", MaxBaud: 14400,
	}
	if len(got) != 1 || got[0] != want {
		t.Fatalf("directory = %+v, want exactly [%+v]", got, want)
	}
}

// Listed and dialable are separate: a host that is not listed (busy-test) is
// missing from the directory but can still be reached by number.
func TestUnlistedHostsAreNotInTheDirectoryButCanBeDialed(t *testing.T) {
	s := NewMemoryStore()
	for _, e := range s.ListedHosts() {
		if e.ID == "busy-test" || e.Phone == "0459999999" {
			t.Fatalf("an unlisted host is in the directory: %+v", e)
		}
	}
	if h, err := s.HostByPhone("0459999999"); err != nil || h.ID != "busy-test" {
		t.Fatalf("the unlisted host cannot be reached by number: %v %+v", err, h)
	}
}

func TestRegisteringAHostDoesNotChangeTheDirectory(t *testing.T) {
	s := NewMemoryStore()
	s.SaveHost(Host{ID: "added-later", Phone: "0450000097", Name: "ADDED LATER"})
	if n := len(s.ListedHosts()); n != 1 {
		t.Fatalf("directory has %d entries, want 1", n)
	}
}

func TestListedHostsReturnsACopy(t *testing.T) {
	s := NewMemoryStore()
	first := s.ListedHosts()
	first[0].Name = "CHANGED"
	if got := s.ListedHosts()[0].Name; got != "HAKATA CANAL NET" {
		t.Fatalf("a caller modified the store's directory: %q", got)
	}
}

func TestMemoryStoreIsAHostDirectoryStore(t *testing.T) {
	var _ HostDirectoryStore = NewMemoryStore()
}
