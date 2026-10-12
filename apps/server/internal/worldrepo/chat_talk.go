package worldrepo

import (
	"sort"
	"sync"
	"time"

	"zutto-pccom/apps/server/internal/world"
)

// Residents' lines in chat rooms (spec 017). The World decides who speaks,
// when and to what (internal/chatslot); this file keeps who is watching each
// room, writes the decided lines through an optional generator and records
// them. Lines are written only while a caller is in the room.

// staleViewer is how long a caller's stay in a room stays open without a
// ChatEvents call for that room. A dropped line never says it left the room;
// with nobody polling nothing is written anyway.
const staleViewer = time.Minute

// chatTalk is the repository's spec 017 state.
type chatTalk struct {
	mu sync.Mutex
	// viewers are the callers' stays, by host and room.
	viewers map[viewerRoom][]viewerStay
}

type viewerRoom struct {
	host string
	room int
}

// viewerStay is a caller in a room from From until To (zero while open).
// Seen is the last world time the room was polled during the stay.
type viewerStay struct {
	Actor    world.ActorRef
	Handle   string
	From, To time.Time
	Seen     time.Time
}

// EnterChatRoom records that a caller entered a room at a world time.
func (r *Repository) EnterChatRoom(host world.Host, room int, actor world.ActorRef, handle string, at time.Time) {
	t := &r.chatTalk
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.viewers == nil {
		t.viewers = map[viewerRoom][]viewerStay{}
	}
	key := viewerRoom{host.ID, room}
	stays := closeStale(t.viewers[key], at)
	for i, s := range stays {
		if s.Actor == actor && s.To.IsZero() {
			// Entering again without leaving closes the earlier stay.
			stays[i].To = at
		}
	}
	t.viewers[key] = append(stays, viewerStay{Actor: actor, Handle: handle, From: at, Seen: at})
}

// LeaveChatRoom records that a caller left a room at a world time.
func (r *Repository) LeaveChatRoom(host world.Host, room int, actor world.ActorRef, at time.Time) {
	t := &r.chatTalk
	t.mu.Lock()
	defer t.mu.Unlock()
	key := viewerRoom{host.ID, room}
	stays := t.viewers[key]
	for i, s := range stays {
		if s.Actor == actor && s.To.IsZero() {
			stays[i].To = at
		}
	}
}

// watchRoom marks room as polled at now and returns the callers' stays in it
// that overlap [since, now], oldest first. A stay unpolled for staleViewer is
// closed at its last poll.
func (r *Repository) watchRoom(host world.Host, room int, since, now time.Time) []viewerStay {
	t := &r.chatTalk
	t.mu.Lock()
	defer t.mu.Unlock()
	key := viewerRoom{host.ID, room}
	stays := closeStale(t.viewers[key], now)
	var out []viewerStay
	for i, s := range stays {
		if s.To.IsZero() && now.After(s.Seen) {
			stays[i].Seen = now
			s.Seen = now
		}
		if s.From.After(now) || (!s.To.IsZero() && s.To.Before(since)) {
			continue
		}
		out = append(out, s)
	}
	t.viewers[key] = stays
	sort.SliceStable(out, func(i, j int) bool { return out[i].From.Before(out[j].From) })
	return out
}

// closeStale closes the open stays last polled more than staleViewer before
// now, at their last poll.
func closeStale(stays []viewerStay, now time.Time) []viewerStay {
	for i, s := range stays {
		if s.To.IsZero() && now.Sub(s.Seen) > staleViewer {
			stays[i].To = s.Seen
		}
	}
	return stays
}
