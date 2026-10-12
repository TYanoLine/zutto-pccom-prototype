package worldrepo

import (
	"testing"
	"time"

	"zutto-pccom/apps/server/internal/world"
)

func TestRepositoryIsAWorldTimeSource(t *testing.T) {
	now := time.Date(1996, 12, 24, 22, 0, 0, 0, time.FixedZone("JST", 9*60*60))
	repo := New(world.NewMemoryStore(), nil, nil, now.Format(time.DateOnly))
	repo.SetWorldNow(func() time.Time { return now })
	if got := world.WorldNowOf(repo); !got.Equal(now) {
		t.Fatalf("WorldNowOf(repository) = %v, want %v", got, now)
	}
}
