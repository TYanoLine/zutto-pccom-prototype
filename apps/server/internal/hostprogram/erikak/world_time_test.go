package erikak

import (
	"fmt"
	"testing"
	"time"

	"zutto-pccom/apps/server/internal/world"
)

// worldTimeStore is a store with a world clock, as worldrepo.Repository is.
type worldTimeStore struct {
	*world.MemoryStore
	now time.Time
}

func (s *worldTimeStore) WorldNow() time.Time { return s.now }

// What the human writes is stamped with world time, not the real clock, so it
// shares the timeline of resident activity.
func TestHumanPostsUseWorldTime(t *testing.T) {
	worldNow := time.Date(1996, 10, 12, 23, 14, 0, 0, time.FixedZone("JST", 9*60*60))
	store := &worldTimeStore{MemoryStore: world.NewMemoryStore(), now: worldNow}
	host, err := store.HostByPhone("0920000196")
	if err != nil {
		t.Fatalf("sample host: %v", err)
	}
	runtime := New(host, store)
	root := store.AddPost(host.ID, world.Post{BoardID: "1", Author: "SYSOP", Subject: "テスト記事", Body: "本文", CreatedAt: worldNow.Add(-time.Hour)})

	// Board 2 takes new posts from members; the root on board 1 takes appends.
	inputs := []string{"TESTER", "dummy", "1", "2", "W", "時刻のテスト", "新規の本文です", "/", "1", "1", fmt.Sprintf("%d", root.ID), "A", "アペの本文です"}
	for _, line := range inputs {
		if _, disconnect := runtime.HandleLine(line); disconnect {
			t.Fatalf("input %q disconnected", line)
		}
	}

	checked := 0
	for _, post := range store.ListPosts(host.ID) {
		if post.Author != "TESTER" {
			continue
		}
		checked++
		if !post.CreatedAt.Equal(worldNow) {
			t.Fatalf("human post %q CreatedAt = %v, want world time %v", post.Body, post.CreatedAt, worldNow)
		}
	}
	if checked != 2 {
		t.Fatalf("stored %d human posts, want a new post and an append", checked)
	}
}
