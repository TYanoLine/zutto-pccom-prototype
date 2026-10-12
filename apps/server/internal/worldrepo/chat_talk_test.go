package worldrepo

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"zutto-pccom/apps/server/internal/chatslot"
	"zutto-pccom/apps/server/internal/hostprogram/erikak"
	"zutto-pccom/apps/server/internal/llm"
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

// fakeChatWriter answers every slot with a fixed kind of line.
type fakeChatWriter struct {
	mu    sync.Mutex
	reqs  []llm.ChatLineRequest
	write func(llm.ChatLineRequest) (string, error)
}

func (w *fakeChatWriter) WriteChatLine(_ context.Context, req llm.ChatLineRequest) (string, error) {
	w.mu.Lock()
	w.reqs = append(w.reqs, req)
	w.mu.Unlock()
	if w.write != nil {
		return w.write(req)
	}
	return req.Kind + "です(^^)", nil
}

func (w *fakeChatWriter) requests() []llm.ChatLineRequest {
	w.mu.Lock()
	defer w.mu.Unlock()
	return append([]llm.ChatLineRequest(nil), w.reqs...)
}

// waitChatJobs waits for the writer calls a poll started.
func waitChatJobs(t *testing.T, repo *Repository) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		repo.chatTalk.mu.Lock()
		busy := 0
		for _, n := range repo.chatTalk.running {
			busy += n
		}
		repo.chatTalk.mu.Unlock()
		if busy == 0 {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("chat jobs did not finish")
}

// talkRoom stores observed plans that put up to four of the host's residents
// in room 0 around at: two already there, one coming in after the caller and
// one passing through. The small test host rarely fills a room by itself.
func talkRoom(t *testing.T, store *world.MemoryStore, host world.Host, at time.Time) int {
	t.Helper()
	personas := store.ListHostPersonas(host.ID)
	if len(personas) < 2 {
		t.Fatal("test host has fewer than two residents")
	}
	stays := [][2]time.Duration{{-15 * time.Minute, 60 * time.Minute}, {-10 * time.Minute, 25 * time.Minute}, {2 * time.Minute, 60 * time.Minute}, {5 * time.Minute, 12 * time.Minute}}
	var acts []world.CallActivity
	for i, p := range personas {
		if i == len(stays) {
			break
		}
		from, to := at.Add(stays[i][0]), at.Add(stays[i][1])
		start := from.Add(-5 * time.Minute)
		acts = append(acts, world.CallActivity{HostID: host.ID, Actor: world.ActorRef{Kind: world.ActorKindPersona, ID: p.ID}, Start: start, Segments: []world.CallSegment{
			{From: start, To: start.Add(2 * time.Minute), State: world.CallLogin},
			{From: start.Add(2 * time.Minute), To: from, State: world.CallIdle},
			{From: from, To: to, State: world.CallChat, Room: 0},
			{From: to, To: to.Add(10 * time.Minute), State: world.CallIdle},
		}})
	}
	store.SaveCallActivities(host.ID, acts)
	return 0
}

type talkPoll struct {
	from, to time.Time
	events   []world.ChatEvent
}

// drive polls room every step of world time for d, waiting for the lines each
// poll starts, as a caller sitting in the room would.
func drive(t *testing.T, repo *Repository, host world.Host, room int, start time.Time, d, step time.Duration) []talkPoll {
	t.Helper()
	var polls []talkPoll
	prev := start
	for at := start.Add(step); !at.After(start.Add(d)); at = at.Add(step) {
		current := at
		repo.SetWorldNow(func() time.Time { return current })
		polls = append(polls, talkPoll{prev, at, repo.ChatEvents(host, room, prev, at)})
		waitChatJobs(t, repo)
		prev = at
	}
	return polls
}

func talkTestRepo(t *testing.T, writer llm.ChatLineWriter) (*Repository, *world.MemoryStore, world.Host, time.Time, int) {
	t.Helper()
	repo, store, host, at := chatTestRepo(t, true)
	room := talkRoom(t, store, host, at)
	if writer != nil {
		repo.SetChatLineWriter(writer)
	}
	return repo, store, host, at, room
}

var caller = world.ActorRef{Kind: world.ActorKindMember, ID: "call-1"}

func residentLines(store *world.MemoryStore, host world.Host, room int) []world.ChatUtterance {
	var out []world.ChatUtterance
	for _, u := range store.ListChatUtterances(host.ID, room, time.Time{}, time.Time{}) {
		if u.Slot != "" {
			out = append(out, u)
		}
	}
	return out
}

func TestResidentsSpeakWhileWatched(t *testing.T) {
	writer := &fakeChatWriter{}
	repo, store, host, at, room := talkTestRepo(t, writer)
	repo.EnterChatRoom(host, room, caller, "GUEST", at)
	polls := drive(t, repo, host, room, at, 15*time.Minute, 5*time.Second)

	lines := residentLines(store, host, room)
	if len(lines) == 0 {
		t.Fatal("residents said nothing in fifteen minutes")
	}
	shown := map[string]bool{}
	var last time.Time
	for _, p := range polls {
		for _, e := range p.events {
			if e.At.Before(last) {
				t.Fatalf("events out of order: %v after %v", e.At, last)
			}
			last = e.At
			if !e.At.After(p.from) || e.At.After(p.to) {
				t.Fatalf("event %+v outside (%v, %v]", e, p.from, p.to)
			}
			if e.Kind == world.ChatSay {
				if shown[e.UtteranceID] {
					t.Fatalf("line shown twice: %+v", e)
				}
				shown[e.UtteranceID] = true
			}
		}
	}
	plans := store.ListCallActivities(host.ID, time.Time{}, time.Time{})
	end := polls[len(polls)-1].to
	for _, u := range lines {
		if !u.At.After(end) && !shown[u.ID] {
			t.Fatalf("line %+v was recorded inside a window already shown", u)
		}
		inRoom := false
		for _, a := range plans {
			if seg, ok := a.SegmentAt(u.At); ok && a.Actor == u.Actor && seg.State == world.CallChat && seg.Room == room {
				inRoom = true
			}
		}
		if !inRoom {
			t.Fatalf("%s spoke at %v outside the room (or its plan was not saved)", u.Handle, u.At)
		}
		if u.Body == "" || u.Handle == "" || !strings.HasSuffix(u.Body, "です(^^)") {
			t.Fatalf("line %+v", u)
		}
	}
	for _, req := range writer.requests() {
		if req.Speaker.Summary == "" || len(req.Present) < 2 || req.Station == "" {
			t.Fatalf("request without materials: %+v", req)
		}
	}
}

func TestNobodyWatchingMeansNoLines(t *testing.T) {
	writer := &fakeChatWriter{}
	repo, store, host, at, room := talkTestRepo(t, writer)
	drive(t, repo, host, room, at, 10*time.Minute, 10*time.Second)
	if n := len(writer.requests()); n != 0 || len(residentLines(store, host, room)) != 0 {
		t.Fatalf("%d writer calls with nobody in the room", n)
	}

	repo2, store2, host2, at2, room2 := talkTestRepo(t, nil)
	repo2.EnterChatRoom(host2, room2, caller, "GUEST", at2)
	drive(t, repo2, host2, room2, at2, 10*time.Minute, 10*time.Second)
	if lines := residentLines(store2, host2, room2); len(lines) != 0 {
		t.Fatalf("residents spoke without a writer: %+v", lines)
	}
}

func TestOneResidentAnswersACallersLine(t *testing.T) {
	repo, store, host, at, room := talkTestRepo(t, &fakeChatWriter{})
	repo.EnterChatRoom(host, room, caller, "GUEST", at)
	var said []world.ChatUtterance
	for i := 0; i < 20; i++ {
		t0 := at.Add(time.Duration(i) * 40 * time.Second)
		repo.SetWorldNow(func() time.Time { return t0 })
		body := "なるほど〜"
		if i == 19 {
			body = "そろそろ落ちます"
		}
		u, err := repo.RecordChatUtterance(host, world.ChatUtterance{Room: room, Actor: caller, Handle: "GUEST", Body: body, At: t0})
		if err != nil {
			t.Fatal(err)
		}
		said = append(said, u)
		drive(t, repo, host, room, t0, 40*time.Second, 5*time.Second)
	}
	answers := map[string]map[string]int{}
	for _, u := range residentLines(store, host, room) {
		if u.RespondsTo == "" {
			continue
		}
		if answers[u.RespondsTo] == nil {
			answers[u.RespondsTo] = map[string]int{}
		}
		answers[u.RespondsTo][string(chatslot.KindOf(u.Slot))]++
	}
	if len(answers) == 0 {
		t.Fatal("no resident answered twenty lines; the check proved nothing")
	}
	for _, u := range said[:19] {
		if n := answers[u.ID]["reply"]; n > 1 {
			t.Fatalf("%d residents replied to %q", n, u.Body)
		}
	}
	if a := answers[said[19].ID]; a["reply"] != 0 || a["sendoff"] > 2 {
		t.Fatalf("answers to a leaving line: %v", a)
	}
}

func TestFailedOrRejectedLinesAreSilence(t *testing.T) {
	for name, write := range map[string]func(llm.ChatLineRequest) (string, error){
		"error":    func(llm.ChatLineRequest) (string, error) { return "", errors.New("boom") },
		"newline":  func(llm.ChatLineRequest) (string, error) { return "こん\nばんは", nil },
		"too long": func(llm.ChatLineRequest) (string, error) { return strings.Repeat("あ", 40), nil },
		"emoji":    func(llm.ChatLineRequest) (string, error) { return "やあ😀", nil },
		"future":   func(llm.ChatLineRequest) (string, error) { return "スマホから", nil },
	} {
		writer := &fakeChatWriter{write: write}
		repo, store, host, at, room := talkTestRepo(t, writer)
		repo.EnterChatRoom(host, room, caller, "GUEST", at)
		drive(t, repo, host, room, at, 5*time.Minute, 5*time.Second)
		if len(writer.requests()) == 0 {
			t.Fatalf("%s: the writer was never asked", name)
		}
		if lines := residentLines(store, host, room); len(lines) != 0 {
			t.Fatalf("%s: lines recorded: %+v", name, lines)
		}
	}
}

func TestSameSlotIsRecordedOnce(t *testing.T) {
	repo, store, host, at, room := talkTestRepo(t, &fakeChatWriter{})
	repo.EnterChatRoom(host, room, caller, "GUEST", at)
	for step := 1; step <= 60; step++ {
		now := at.Add(time.Duration(step) * 5 * time.Second)
		repo.SetWorldNow(func() time.Time { return now })
		var wg sync.WaitGroup
		for i := 0; i < 4; i++ {
			wg.Add(1)
			go func() { defer wg.Done(); repo.ChatEvents(host, room, now.Add(-5*time.Second), now) }()
		}
		wg.Wait()
		waitChatJobs(t, repo)
	}
	seen := map[string]bool{}
	lines := residentLines(store, host, room)
	if len(lines) == 0 {
		t.Fatal("no lines; the check proved nothing")
	}
	for _, u := range lines {
		if seen[u.Slot] {
			t.Fatalf("slot recorded twice: %s", u.Slot)
		}
		seen[u.Slot] = true
	}
}

func TestResidentLinesInTheLedger(t *testing.T) {
	repo, store, host, at, room := talkTestRepo(t, &fakeChatWriter{})
	repo.EnterChatRoom(host, room, caller, "GUEST", at)
	u, err := repo.RecordChatUtterance(host, world.ChatUtterance{Room: room, Actor: caller, Handle: "GUEST", Body: "こんばんは", At: at})
	if err != nil {
		t.Fatal(err)
	}
	drive(t, repo, host, room, at, 10*time.Minute, 5*time.Second)
	events := map[string]world.InteractionEvent{}
	for _, e := range store.ListInteractionEvents(host.ID, 0) {
		events[e.EventID] = e
	}
	lines := residentLines(store, host, room)
	if len(lines) == 0 {
		t.Fatal("no resident lines")
	}
	for _, l := range lines {
		e, ok := events[world.ChannelChatUtterance+":"+l.ID]
		if !ok || e.Channel != world.ChannelChatUtterance || e.SourceKind != "npc_world_selected" || e.Actor != l.Actor ||
			e.AudienceScope != world.ChatRoomScope(host.ID, room) || !e.OccurredAt.Equal(l.At) || len(e.AddressedTo) == 0 {
			t.Fatalf("ledger event for %+v: %+v", l, e)
		}
		if l.RespondsTo != "" && e.RespondsToEventID != world.ChannelChatUtterance+":"+l.RespondsTo {
			t.Fatalf("answer %+v without its ledger link: %+v", l, e)
		}
	}
	_ = u
}

func TestResidentLinesStayWithinLimits(t *testing.T) {
	repo, store, host, at, room := talkTestRepo(t, &fakeChatWriter{})
	repo.EnterChatRoom(host, room, caller, "GUEST", at)
	drive(t, repo, host, room, at, time.Hour, 10*time.Second)
	lines := residentLines(store, host, room)
	for i, l := range lines {
		n := 0
		for _, o := range lines[i:] {
			if o.At.Before(l.At.Add(time.Minute)) {
				n++
			}
		}
		if n > 12 {
			t.Fatalf("%d resident lines in the minute from %v", n, l.At)
		}
		if i > 0 && l.At.Sub(lines[i-1].At) < 3*time.Second {
			t.Fatalf("resident lines %v apart", l.At.Sub(lines[i-1].At))
		}
	}
	if len(lines) > 360 {
		t.Fatalf("%d resident lines in an hour", len(lines))
	}
}

func TestResidentBringsUpACallersPost(t *testing.T) {
	writer := &fakeChatWriter{}
	repo, store, host, at, room := talkTestRepo(t, writer)
	boards := []world.Board{{ID: "chat-test-board", Name: "雑談"}}
	store.SaveBoards(host.ID, boards)
	post := store.AddPost(host.ID, world.Post{BoardID: boards[0].ID, Author: "GUEST", AuthorMemberID: "GUEST", Subject: "モデム買いました", CreatedAt: at.Add(-5 * time.Minute)})
	member := world.ActorRef{Kind: world.ActorKindMember, ID: "GUEST"}
	if err := store.AppendInteractionEvent(world.InteractionEvent{EventID: fmt.Sprintf("post:%s:%d", host.ID, post.ID), HostID: host.ID, Channel: "board_root",
		Actor: member, ContentRef: fmt.Sprint(post.ID), OccurredAt: post.CreatedAt, SourceKind: "human_authored"}); err != nil {
		t.Fatal(err)
	}
	// Every resident in the room is a regular of the caller.
	for _, a := range repo.CallActivitiesNow(host) {
		if seg, ok := a.SegmentAt(at); ok && seg.State == world.CallChat && seg.Room == room {
			if err := store.UpdateRelationship(world.RelationshipEdge{HostID: host.ID, FromActor: a.Actor, ToActor: member, BoardReplyCount: 3}); err != nil {
				t.Fatal(err)
			}
		}
	}
	repo.EnterChatRoom(host, room, caller, "GUEST", at)
	drive(t, repo, host, room, at, 10*time.Minute, 5*time.Second)
	var news []llm.ChatLineRequest
	for _, req := range writer.requests() {
		if req.Kind == "news" {
			news = append(news, req)
		}
	}
	if len(news) == 0 {
		t.Skip("no resident drew news in this room; the chance is a tuning value")
	}
	for _, req := range news {
		if req.Event == nil || req.Event.Title != "モデム買いました" || req.Event.Board != boards[0].Name || req.Event.Subject != "GUEST" || req.Relation != "常連仲間" {
			t.Fatalf("news request: %+v / %+v", req, req.Event)
		}
	}
	if len(news) > 1 {
		t.Fatalf("%d residents brought up one post", len(news))
	}
}

// TestChatEventsTiming measures one poll of a watched room on a 300-resident
// host (spec 017 T007).
func TestChatEventsTiming(t *testing.T) {
	repo, store, host, at, room := talkTestRepo(t, &fakeChatWriter{})
	for i := len(store.ListHostPersonas(host.ID)); i < 300; i++ {
		p := world.Persona{ID: fmt.Sprintf("timing-%03d", i), Handle: fmt.Sprintf("T%03d", i), ActivityPattern: "active"}
		store.SavePersona(p)
		store.AddMembership(host.ID, p.ID)
	}
	repo.EnterChatRoom(host, room, caller, "GUEST", at)
	var worst time.Duration
	prev := at
	for i := 1; i <= 30; i++ {
		now := at.Add(time.Duration(i) * 10 * time.Second)
		repo.SetWorldNow(func() time.Time { return now })
		start := time.Now()
		repo.ChatEvents(host, room, prev, now)
		worst = max(worst, time.Since(start))
		waitChatJobs(t, repo)
		prev = now
	}
	t.Logf("ChatEvents on %d residents with a caller in the room: worst of 30 polls = %v", len(store.ListHostPersonas(host.ID)), worst)
}

// TestErikaKChatShowsResidentLines drives the station's chat room against the
// real world with a writer: residents' lines appear among the joins and
// leaves, and the caller's own lines are not shown again.
func TestErikaKChatShowsResidentLines(t *testing.T) {
	repo, _, host, at, room := talkTestRepo(t, &fakeChatWriter{})
	current := at
	repo.SetWorldNow(func() time.Time { return current })
	rt := erikak.New(host, repo)
	rt.SetCallInfo(world.CallInfo{CallID: "call-chat", Line: 1, Baud: 14400})
	rt.HandleLine("GUEST")
	rt.HandleLine("4")
	rt.HandleLine("X")
	if out, _ := rt.HandleLine(fmt.Sprint(room)); !strings.Contains(out, "## chat on  GUEST ##") {
		t.Fatalf("room entry:\n%s", out)
	}
	var shown strings.Builder
	for i := 1; i <= 120; i++ {
		current = at.Add(time.Duration(i) * 5 * time.Second)
		if i == 30 {
			out, _ := rt.HandleLine("みなさんこんばんは")
			if !strings.Contains(out, "GUEST   :みなさんこんばんは") {
				t.Fatalf("own line:\n%s", out)
			}
			shown.WriteString(strings.TrimPrefix(strings.ReplaceAll(out, "GUEST   :みなさんこんばんは\r\n", ""), "\r\n"))
		}
		shown.WriteString(rt.Poll(current))
		waitChatJobs(t, repo)
	}
	said := 0
	for _, l := range strings.Split(strings.TrimSuffix(shown.String(), "\r\n"), "\r\n") {
		switch {
		case l == "":
		case strings.HasPrefix(l, "\a\a## chat on  "), strings.HasPrefix(l, "\a\a## chat off "):
		case strings.Contains(l, ":") && strings.HasSuffix(l, "です(^^)"):
			said++
		default:
			t.Fatalf("unexpected room line %q in:\n%s", l, shown.String())
		}
		if strings.Contains(l, "みなさんこんばんは") {
			t.Fatalf("own line shown again: %q", l)
		}
	}
	if said == 0 {
		t.Fatalf("no resident line in ten minutes:\n%s", shown.String())
	}
}
