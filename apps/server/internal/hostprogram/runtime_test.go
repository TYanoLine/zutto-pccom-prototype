package hostprogram

import (
	"strings"
	"testing"

	"zutto-pccom/apps/server/internal/world"
)

// turboTestHost is a TurboBBS station defined only for these tests.
func turboTestHost() world.Host {
	return world.Host{ID: "turbobbs-test", Name: "TURBOBBS TEST", Software: "TurboBBS 1.08 compatible", SoftwareID: "turbobbs", Lines: 1, MaxBaud: 2400}
}

func TestNewSelectsTurboBBSRuntime(t *testing.T) {
	store := world.NewMemoryStore()
	runtime := New(turboTestHost(), store)

	if got := runtime.Welcome(); !strings.Contains(got, "TurboBBS version 1.08") {
		t.Fatalf("TurboBBS host used wrong runtime: %q", got)
	}
	boards := ObservationBoards(runtime)
	if len(boards) != 10 {
		t.Fatalf("TurboBBS observation catalog has %d sections, want 10", len(boards))
	}
}


func TestReplyRepresentationIsHostProgramSpecific(t *testing.T) {
	store := world.NewMemoryStore()
	source := world.Post{ID: 42, Subject: "元の件名"}

	erika, err := store.HostByPhone("0920000196")
	if err != nil {
		t.Fatal(err)
	}
	got, err := ProjectReply(erika, source, "返信用件名")
	if err != nil {
		t.Fatal(err)
	}
	if got.ParentID != source.ID || got.Subject != "" {
		t.Fatalf("Erika-K reply projection=%+v, want append child with no subject", got)
	}

	got, err = ProjectReply(turboTestHost(), source, "返信用件名")
	if err != nil {
		t.Fatal(err)
	}
	if got.ParentID != 0 || got.Subject != "返信用件名" {
		t.Fatalf("TurboBBS reply projection=%+v, want flat message with proposed subject", got)
	}

	generic := world.Host{ID: "generic-test", SoftwareID: "generic"}
	got, err = ProjectReply(generic, source, "ignored")
	if err != nil {
		t.Fatal(err)
	}
	if got.ParentID != source.ID || got.Subject != "Re: 元の件名" {
		t.Fatalf("generic compatibility reply projection=%+v", got)
	}
}

func TestUnknownHistoricalHostHasNoImplicitReplyConvention(t *testing.T) {
	_, err := ProjectReply(world.Host{ID: "unknown", SoftwareID: "future-host"}, world.Post{ID: 1, Subject: "x"}, "y")
	if err == nil {
		t.Fatal("unknown host program unexpectedly inherited an implicit reply convention")
	}
}
