package worldrepo

import (
	"testing"
	"time"

	"zutto-pccom/apps/server/internal/world"
)

func TestViewerStaysOpenAndClose(t *testing.T) {
	repo, _, host, now := chatTestRepo(t, true)
	me := world.ActorRef{Kind: world.ActorKindMember, ID: "call-1"}
	repo.EnterChatRoom(host, 0, me, "GUEST", now)
	if got := repo.watchRoom(host, 0, now, now.Add(30*time.Second)); len(got) != 1 || !got[0].To.IsZero() || got[0].Handle != "GUEST" {
		t.Fatalf("open stay = %+v", got)
	}
	if got := repo.watchRoom(host, 1, now, now.Add(30*time.Second)); len(got) != 0 {
		t.Fatalf("stay in another room = %+v", got)
	}
	repo.LeaveChatRoom(host, 0, me, now.Add(time.Minute))
	got := repo.watchRoom(host, 0, now, now.Add(2*time.Minute))
	if len(got) != 1 || !got[0].To.Equal(now.Add(time.Minute)) {
		t.Fatalf("closed stay = %+v", got)
	}
	if got := repo.watchRoom(host, 0, now.Add(90*time.Second), now.Add(2*time.Minute)); len(got) != 0 {
		t.Fatalf("stay that ended before the window = %+v", got)
	}
}

func TestViewerStayWithoutLeaveGoesStale(t *testing.T) {
	repo, _, host, now := chatTestRepo(t, true)
	me := world.ActorRef{Kind: world.ActorKindMember, ID: "call-1"}
	repo.EnterChatRoom(host, 0, me, "GUEST", now)
	repo.watchRoom(host, 0, now, now.Add(20*time.Second))
	// The line drops: no poll and no leave for longer than staleViewer.
	later := now.Add(20*time.Second + staleViewer + time.Second)
	got := repo.watchRoom(host, 0, now, later)
	if len(got) != 1 || !got[0].To.Equal(now.Add(20*time.Second)) {
		t.Fatalf("stale stay = %+v", got)
	}
	// Entering again opens a new stay.
	repo.EnterChatRoom(host, 0, me, "GUEST", later)
	if got := repo.watchRoom(host, 0, later, later.Add(time.Second)); len(got) != 1 || !got[0].To.IsZero() {
		t.Fatalf("stays after entering again = %+v", got)
	}
}
