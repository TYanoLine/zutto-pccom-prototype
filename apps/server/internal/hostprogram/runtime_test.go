package hostprogram

import (
	"strings"
	"testing"

	"zutto-pccom/apps/server/internal/world"
)

func TestNewSelectsTurboBBSRuntime(t *testing.T) {
	store := world.NewMemoryStore()
	host, err := store.HostByPhone("0470001080")
	if err != nil {
		t.Fatal(err)
	}
	runtime := New(host, store)

	if got := runtime.Welcome(); !strings.Contains(got, "TurboBBS version 1.08") {
		t.Fatalf("TurboBBS host used wrong runtime: %q", got)
	}
	boards := ObservationBoards(runtime)
	if len(boards) != 10 {
		t.Fatalf("TurboBBS observation catalog has %d sections, want 10", len(boards))
	}
}
