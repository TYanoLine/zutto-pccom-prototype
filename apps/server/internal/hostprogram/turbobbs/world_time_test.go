package turbobbs

import (
	"testing"
	"time"

	"zutto-pccom/apps/server/internal/world"
)

type worldTimeStore struct {
	*world.MemoryStore
	now time.Time
}

func (s *worldTimeStore) WorldNow() time.Time { return s.now }

// A stored message is stamped with world time, not the real clock.
func TestStoredMessageUsesWorldTime(t *testing.T) {
	worldNow := time.Date(1996, 10, 12, 23, 14, 0, 0, time.FixedZone("JST", 9*60*60))
	store := &worldTimeStore{MemoryStore: world.NewMemoryStore(), now: worldNow}
	host := sampleHost()
	store.SaveHost(host)
	runtime := New(host, store)
	loginRegular(t, runtime)
	for _, line := range []string{"E", "ALL", "TIME TEST", "1", "HELLO FROM 1996", "", "S"} {
		if _, disconnect := runtime.HandleLine(line); disconnect {
			t.Fatalf("input %q disconnected", line)
		}
	}
	for _, p := range store.ListPosts(host.ID) {
		if p.Subject == "TIME TEST" {
			if !p.CreatedAt.Equal(worldNow) {
				t.Fatalf("CreatedAt = %v, want world time %v", p.CreatedAt, worldNow)
			}
			return
		}
	}
	t.Fatal("message was not stored")
}
