package erikak

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"zutto-pccom/apps/server/internal/hostcatalog"
	"zutto-pccom/apps/server/internal/world"
)

// chatWorld is a world whose line holders, plans and room events the test
// sets directly.
type chatWorld struct {
	*world.MemoryStore
	now      time.Time
	sessions []world.PresenceSession
	plans    []world.CallActivity
	events   []world.ChatEvent
	rooms    []hostcatalog.ChatRoom
	chat     *hostcatalog.ErikaKChat
	sent     []world.LineMessage
	said     []world.ChatUtterance
	stays    []string
	// incoming are residents' telegrams to the caller; asked records the
	// windows the host program asked for.
	incoming []world.LineMessage
	asked    [][2]time.Time
}

func (w *chatWorld) WorldNow() time.Time { return w.now }

func (w *chatWorld) HostDetail(string) (hostcatalog.PresetDetail, bool) {
	return hostcatalog.PresetDetail{ChatRooms: w.rooms, ErikaK: &hostcatalog.ErikaKDetail{Chat: w.chat}}, true
}

func (w *chatWorld) ObservePresenceNow(world.Host) []world.PresenceSession { return w.sessions }

func (w *chatWorld) CallActivitiesNow(world.Host) []world.CallActivity { return w.plans }

func (w *chatWorld) ChatEvents(_ world.Host, room int, from, to time.Time) []world.ChatEvent {
	var out []world.ChatEvent
	for _, e := range w.events {
		if e.Room == room && e.At.After(from) && !e.At.After(to) {
			out = append(out, e)
		}
	}
	return out
}

func (w *chatWorld) RecordChatUtterance(_ world.Host, u world.ChatUtterance) (world.ChatUtterance, error) {
	w.said = append(w.said, u)
	return u, nil
}

func (w *chatWorld) SendLineMessage(_ world.Host, m world.LineMessage) (world.LineMessage, error) {
	w.sent = append(w.sent, m)
	return m, nil
}

func (w *chatWorld) EnterChatRoom(_ world.Host, room int, actor world.ActorRef, _ string, _ time.Time) {
	w.stays = append(w.stays, fmt.Sprintf("enter %d %s", room, actor))
}

func (w *chatWorld) LeaveChatRoom(_ world.Host, room int, actor world.ActorRef, _ time.Time) {
	w.stays = append(w.stays, fmt.Sprintf("leave %d %s", room, actor))
}

var chatT0 = time.Date(1996, 4, 30, 23, 40, 0, 0, time.FixedZone("JST", 9*3600))

func testChatSection() *hostcatalog.ErikaKChat {
	return &hostcatalog.ErikaKChat{
		CallMenu:    hostcatalog.ErikaKCallMenu{Title: "【電報】C", Lines: []string{"-----", "MENU LINE", "-----"}, Hint: "[番号]=電報 [ﾘﾀｰﾝ]=WHO [.]戻る [X]=CHAT"},
		ChatMenu:    []string{"=====", "【0】 ROOM 0", "【1】 ROOM 1（定員 1名）", "====="},
		RoomNotices: map[int][]string{0: {"NOTICE 0"}},
	}
}

// newChatWorld puts the caller on line 1, an idle resident on line 2 and a
// resident in room 0 on line 3 (until 00:10); the other lines are free.
func newChatWorld(lines int) (*chatWorld, world.Host) {
	host := world.Host{ID: "chat-test", Phone: "0000", Name: "CHAT TEST", Lines: lines, MaxBaud: 28800, SoftwareID: "erika-k"}
	w := &chatWorld{MemoryStore: world.NewMemoryStore(), now: chatT0, chat: testChatSection(),
		rooms: []hostcatalog.ChatRoom{{Number: 0, Weight: 1}, {Number: 1, Capacity: 1, Weight: 1}}}
	idle := world.ActorRef{Kind: world.ActorKindPersona, ID: "idle"}
	chatting := world.ActorRef{Kind: world.ActorKindPersona, ID: "chatting"}
	start := chatT0.Add(-30 * time.Minute)
	w.sessions = []world.PresenceSession{
		{HostID: host.ID, Actor: world.ActorRef{Kind: world.ActorKindMember, ID: "call-1"}, Line: 1, Baud: 28800, Start: chatT0.Add(-5 * time.Minute)},
		{HostID: host.ID, Actor: idle, Handle: "NORI", Line: 2, Baud: 14400, Start: start, End: chatT0.Add(time.Hour)},
	}
	w.plans = []world.CallActivity{{HostID: host.ID, Actor: idle, Start: start, Segments: []world.CallSegment{
		{From: start, To: start.Add(2 * time.Minute), State: world.CallLogin},
		{From: start.Add(2 * time.Minute), To: chatT0.Add(time.Hour), State: world.CallIdle},
	}}}
	if lines >= 3 {
		w.sessions = append(w.sessions, world.PresenceSession{HostID: host.ID, Actor: chatting, Handle: "ｿﾈｾｲｼﾞ", Line: 3, Baud: 26400, Start: start, End: chatT0.Add(time.Hour)})
		w.plans = append(w.plans, world.CallActivity{HostID: host.ID, Actor: chatting, Start: start, Segments: []world.CallSegment{
			{From: start, To: start.Add(2 * time.Minute), State: world.CallLogin},
			{From: start.Add(2 * time.Minute), To: start.Add(4 * time.Minute), State: world.CallPaging},
			{From: start.Add(4 * time.Minute), To: chatT0.Add(30 * time.Minute), State: world.CallChat, Room: 0},
			{From: chatT0.Add(30 * time.Minute), To: chatT0.Add(time.Hour), State: world.CallIdle},
		}})
	}
	return w, host
}

func loggedInChatRuntime(t *testing.T, w *chatWorld, host world.Host) *Runtime {
	t.Helper()
	rt := New(host, w)
	rt.SetCallInfo(world.CallInfo{CallID: "call-1", Line: 1, Baud: 28800})
	rt.HandleLine("")
	if rt.state != "main" {
		t.Fatalf("not at the main menu after login: %s", rt.state)
	}
	return rt
}

func TestCallScreenShowsEveryLine(t *testing.T) {
	w, host := newChatWorld(4)
	rt := loggedInChatRuntime(t, w, host)
	out, _ := rt.HandleLine("4")
	want := "\r\n【電報】C\r\n-----\r\nMENU LINE\r\n-----\r\n\r\n" +
		"#01 GUEST     WHO       28K-...\r\n" +
		"#02 NORI      idle      14K-...\r\n" +
		"#03 ｿﾈｾｲｼﾞ    CHAT   #0 26K-...\r\n" +
		"#04 ring\r\n" +
		"\r\n[番号]=電報 [ﾘﾀｰﾝ]=WHO [.]戻る [X]=CHAT\r\nCALL> ->"
	if out != want {
		t.Fatalf("call screen:\n%q\nwant\n%q", out, want)
	}
	if rt.state != "call" {
		t.Fatalf("state = %s", rt.state)
	}
}

func TestWhoRowsFollowTheLineCount(t *testing.T) {
	for _, lines := range []int{1, 3, 8, 16} {
		w, host := newChatWorld(lines)
		if lines == 1 {
			w.sessions = w.sessions[:1]
		}
		rt := loggedInChatRuntime(t, w, host)
		rt.HandleLine("4")
		out, _ := rt.HandleLine("")
		rows := 0
		for _, line := range strings.Split(out, "\r\n") {
			if strings.HasPrefix(line, "#") {
				rows++
			}
		}
		if rows != lines {
			t.Fatalf("%d lines: %d WHO rows:\n%s", lines, rows, out)
		}
		if lines >= 8 && !strings.Contains(out, "#08 ring\r\n") {
			t.Fatalf("two-digit line numbers missing:\n%s", out)
		}
	}
}

func TestTelegramToAnIdleResident(t *testing.T) {
	w, host := newChatWorld(4)
	rt := loggedInChatRuntime(t, w, host)
	rt.HandleLine("4")
	out, _ := rt.HandleLine("2")
	if out != "\r\n電報をどうぞ。 (最大全角150文字  中止:ﾘﾀｰﾝｷｰ)\r\n"+strings.Repeat("---------+", 8)+"\r\n" {
		t.Fatalf("telegram prompt = %q", out)
	}
	if rt.Poll(chatT0.Add(time.Minute)) != "" {
		t.Fatal("Poll interrupted the telegram body")
	}
	out, _ = rt.HandleLine("おひさしぶりです(^^)")
	if out != "\r\n#02 に電報を送りました。\r\n\r\n[番号]=電報 [ﾘﾀｰﾝ]=WHO [.]戻る [X]=CHAT\r\nCALL> ->" {
		t.Fatalf("sent = %q", out)
	}
	if len(w.sent) != 1 {
		t.Fatalf("sent %d messages", len(w.sent))
	}
	m := w.sent[0]
	if m.To.ID != "idle" || m.ToLine != 2 || m.From.ID != "call-1" || m.Body != "おひさしぶりです(^^)" || !m.DeliverAt.Equal(chatT0) || !m.SentAt.Equal(chatT0) {
		t.Fatalf("message = %+v", m)
	}
}

func TestTelegramToAResidentInChatArrivesAfterChat(t *testing.T) {
	w, host := newChatWorld(4)
	rt := loggedInChatRuntime(t, w, host)
	rt.HandleLine("4")
	rt.HandleLine("3")
	out, _ := rt.HandleLine("あとで")
	if !strings.HasPrefix(out, "\r\n#03 チャットルームか会議室にちゃっかり入ってます。 CHAT終了後、電報を送ります。\r\n\r\n#01 GUEST") ||
		!strings.HasSuffix(out, "#04 ring\r\n\r\n[番号]=電報 [ﾘﾀｰﾝ]=WHO [.]戻る [X]=CHAT\r\nCALL> ->") {
		t.Fatalf("in-chat telegram = %q", out)
	}
	if len(w.sent) != 1 || !w.sent[0].DeliverAt.Equal(chatT0.Add(30*time.Minute)) {
		t.Fatalf("message = %+v, want delivery at the end of the chat", w.sent)
	}
}

func TestTelegramToAFreeOrOwnLineIsAnError(t *testing.T) {
	w, host := newChatWorld(4)
	rt := loggedInChatRuntime(t, w, host)
	rt.HandleLine("4")
	out, _ := rt.HandleLine("4")
	if !strings.HasPrefix(out, "その回線番号には誰もいません。\r\n") || !strings.HasSuffix(out, "CALL> ->") || rt.state != "call" {
		t.Fatalf("free line: %q (state %s)", out, rt.state)
	}
	out, _ = rt.HandleLine("1")
	if !strings.HasPrefix(out, "自分自身には電報を送れません。\r\n") || !strings.HasSuffix(out, "CALL> ->") || rt.state != "call" {
		t.Fatalf("own line: %q (state %s)", out, rt.state)
	}
	for _, input := range []string{"9", "*", "P"} {
		out, _ := rt.HandleLine(input)
		if !strings.HasPrefix(out, "? COMMAND ERROR\r\n") || !strings.HasSuffix(out, "CALL> ->") || rt.state != "call" {
			t.Fatalf("%q: %q (state %s)", input, out, rt.state)
		}
	}
	if len(w.sent) != 0 {
		t.Fatalf("messages were sent: %+v", w.sent)
	}
	out, _ = rt.HandleLine("2")
	if !strings.Contains(out, "電報をどうぞ") {
		t.Fatal(out)
	}
	out, _ = rt.HandleLine("")
	if out != "\r\n[番号]=電報 [ﾘﾀｰﾝ]=WHO [.]戻る [X]=CHAT\r\nCALL> ->" || len(w.sent) != 0 {
		t.Fatalf("an empty body did not cancel: %q %+v", out, w.sent)
	}
}

func TestChatRoomEntryEventsAndExit(t *testing.T) {
	w, host := newChatWorld(4)
	rt := loggedInChatRuntime(t, w, host)
	rt.HandleLine("4")
	out, _ := rt.HandleLine("X")
	if out != "\r\n=====\r\n【0】 ROOM 0\r\n【1】 ROOM 1（定員 1名）\r\n=====\r\nルーム番号をどうぞ  （中止はﾘﾀｰﾝｷｰ） --> " {
		t.Fatalf("room menu = %q", out)
	}
	w.events = []world.ChatEvent{
		{At: chatT0.Add(-time.Minute), Kind: world.ChatJoin, Room: 0, Handle: "BEFORE"},
		{At: chatT0.Add(20 * time.Second), Kind: world.ChatJoin, Room: 0, Handle: "まさや"},
		{At: chatT0.Add(30 * time.Second), Kind: world.ChatJoin, Room: 1, Handle: "OTHERROOM"},
		{At: chatT0.Add(40 * time.Second), Kind: world.ChatLeave, Room: 0, Handle: "ｿﾈｾｲｼﾞ"},
		{At: chatT0.Add(90 * time.Second), Kind: world.ChatLeave, Room: 0, Handle: "まさや"},
	}
	out, _ = rt.HandleLine("0")
	want := "\r\n\r\n  チャット終了は 「..」 または 「CTRL+B」 を入力して下さい\r\nNOTICE 0\r\n" +
		strings.Repeat("-", 36) + "CHAT ON" + strings.Repeat("-", 35) + "\r\n\a\a## chat on  GUEST ##\r\n"
	if out != want {
		t.Fatalf("entry:\n%q\nwant\n%q", out, want)
	}
	if got := rt.Poll(chatT0.Add(45 * time.Second)); got != "\a\a## chat on  まさや ##\r\n\a\a## chat off ｿﾈｾｲｼﾞ ##\r\n" {
		t.Fatalf("first poll = %q", got)
	}
	if got := rt.Poll(chatT0.Add(45 * time.Second)); got != "" {
		t.Fatalf("events repeated: %q", got)
	}

	w.now = chatT0.Add(time.Minute)
	out, _ = rt.HandleLine("こんばんは(^^)")
	if out != "\r\nGUEST   :こんばんは(^^)\r\n" {
		t.Fatalf("utterance = %q", out)
	}
	if len(w.said) != 1 || w.said[0].Room != 0 || w.said[0].Body != "こんばんは(^^)" || w.said[0].Actor.ID != "call-1" || !w.said[0].At.Equal(w.now) {
		t.Fatalf("recorded = %+v", w.said)
	}

	w.now = chatT0.Add(2 * time.Minute)
	out, _ = rt.HandleLine("..")
	wantExit := "\r\n\a\a## chat off まさや ##\r\n" +
		"GUEST   :..\r\n\r\n\a\a## chat off GUEST ##\r\n" +
		strings.Repeat("-", 36) + "CHAT OFF" + strings.Repeat("-", 34) + "\r\n\r\n" +
		"#01 GUEST     WHO       28K-...\r\n#02 NORI      idle      14K-...\r\n#03 ｿﾈｾｲｼﾞ    CHAT   #0 26K-...\r\n#04 ring\r\n" +
		"=====\r\n【0】 ROOM 0\r\n【1】 ROOM 1（定員 1名）\r\n=====\r\nルーム番号をどうぞ  （中止はﾘﾀｰﾝｷｰ） --> "
	if out != wantExit {
		t.Fatalf("exit:\n%q\nwant\n%q", out, wantExit)
	}
	if rt.state != "chat_menu" || rt.Poll(chatT0.Add(time.Hour)) != "" {
		t.Fatal("still in the room after ..")
	}
	out, _ = rt.HandleLine("")
	if rt.state != "main" || !strings.HasSuffix(out, "MAIN MENU [?]=HELP --> ") {
		t.Fatalf("an empty room number did not return to the main menu: %q", out)
	}
}

func TestChatMenuRejectsUnknownAndFullRooms(t *testing.T) {
	w, host := newChatWorld(4)
	// The resident on line 3 sits in room 1, whose capacity is 1.
	w.plans[1].Segments[2].Room = 1
	rt := loggedInChatRuntime(t, w, host)
	rt.HandleLine("4")
	rt.HandleLine("X")
	for _, input := range []string{"7", "A", "1"} {
		out, _ := rt.HandleLine(input)
		if !strings.HasPrefix(out, "? COMMAND ERROR\r\n") || !strings.HasSuffix(out, "--> ") || rt.state != "chat_menu" {
			t.Fatalf("%q: %q (state %s)", input, out, rt.state)
		}
	}
	if out, _ := rt.HandleLine("0"); !strings.Contains(out, "## chat on  GUEST ##") {
		t.Fatalf("room 0 refused: %q", out)
	}
}

func TestStationWithoutChatKeepsThePlaceholder(t *testing.T) {
	w, host := newChatWorld(4)
	w.chat = nil
	rt := loggedInChatRuntime(t, w, host)
	out, _ := rt.HandleLine("4")
	if rt.state != "chat" || out != rt.renderChatMenu() {
		t.Fatalf("placeholder changed: state %s\n%q", rt.state, out)
	}
	if rt.Poll(chatT0.Add(time.Hour)) != "" {
		t.Fatal("Poll printed on a station without chat")
	}
}

// TestChatRoomShowsLines shows the lines said in the room in time order with
// the joins and leaves, without the caller's own lines, and tells the world
// when the caller enters and leaves (spec 017).
func TestChatRoomShowsLines(t *testing.T) {
	w, host := newChatWorld(4)
	rt := loggedInChatRuntime(t, w, host)
	self := world.ActorRef{Kind: world.ActorKindMember, ID: "call-1"}
	rt.HandleLine("4")
	rt.HandleLine("X")
	w.events = []world.ChatEvent{
		{At: chatT0.Add(10 * time.Second), Kind: world.ChatSay, Room: 0, Actor: world.ActorRef{Kind: world.ActorKindPersona, ID: "chatting"}, Handle: "ｿﾈｾｲｼﾞ", Body: "こんばんは〜"},
		{At: chatT0.Add(20 * time.Second), Kind: world.ChatJoin, Room: 0, Handle: "まさや"},
		{At: chatT0.Add(25 * time.Second), Kind: world.ChatSay, Room: 0, Actor: self, Handle: "GUEST", Body: "どうも"},
		{At: chatT0.Add(30 * time.Second), Kind: world.ChatSay, Room: 0, Actor: world.ActorRef{Kind: world.ActorKindPersona, ID: "m"}, Handle: "まさや", Body: "ばんわ(^^)"},
	}
	rt.HandleLine("0")
	got := rt.Poll(chatT0.Add(time.Minute))
	want := "ｿﾈｾｲｼﾞ  :こんばんは〜\r\n\a\a## chat on  まさや ##\r\nまさや  :ばんわ(^^)\r\n"
	if got != want {
		t.Fatalf("poll:\n%q\nwant\n%q", got, want)
	}
	rt.HandleLine("..")
	if fmt.Sprint(w.stays) != fmt.Sprintf("[enter 0 %s leave 0 %s]", self, self) {
		t.Fatalf("stays told to the world: %v", w.stays)
	}
}
