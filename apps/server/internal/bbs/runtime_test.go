package bbs

import (
	"strings"
	"testing"

	"zutto-pccom/apps/server/internal/world"
)

func TestWritePersistsInMemory(t *testing.T) {
	store := world.NewMemoryStore()
	h, err := store.HostByPhone("0451234567")
	if err != nil {
		t.Fatal(err)
	}
	r := New(h, store)
	_, _ = r.HandleLine("W")
	_, _ = r.HandleLine("テスト")
	_, _ = r.HandleLine("これはテスト投稿です")
	out, _ := r.HandleLine("B")
	if !strings.Contains(out, "これはテスト投稿です") {
		t.Fatalf("post was not persisted: %s", out)
	}
}
