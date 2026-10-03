package bbs

import (
	"strings"
	"testing"

	"zutto-pccom/apps/server/internal/world"
)

func TestWritePersistsInMemory(t *testing.T) {
	store := world.NewMemoryStore()
	h := world.Host{ID: "generic-test", Phone: "0450000010", Name: "GENERIC TEST BBS", SoftwareID: "generic", Lines: 1, MaxBaud: 2400}
	store.SaveHost(h)
	r := New(h, store)
	_, _ = r.HandleLine("W")
	_, _ = r.HandleLine("テスト")
	_, _ = r.HandleLine("これはテスト投稿です")
	out, _ := r.HandleLine("B")
	if !strings.Contains(out, "これはテスト投稿です") {
		t.Fatalf("post was not persisted: %s", out)
	}
}
