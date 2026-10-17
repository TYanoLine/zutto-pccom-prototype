package erikak

import (
	"testing"
	"time"

	"zutto-pccom/apps/server/internal/world"
)

// A resident's telegrams to the caller, which the test sets directly.
func (w *chatWorld) LineMessageEvents(_ world.Host, call world.LineMessageCall, from, to time.Time) []world.LineMessage {
	w.asked = append(w.asked, [2]time.Time{from, to})
	var out []world.LineMessage
	for _, m := range w.incoming {
		if m.To == call.Human && m.ToLine == call.Line && m.DeliverAt.After(from) && !m.DeliverAt.After(to) {
			out = append(out, m)
		}
	}
	return out
}

func incomingFrom(w *chatWorld, at time.Time, body string) {
	w.incoming = append(w.incoming, world.LineMessage{
		From: world.ActorRef{Kind: world.ActorKindPersona, ID: "idle"}, FromHandle: "NORI", FromLine: 2,
		To: world.ActorRef{Kind: world.ActorKindMember, ID: "call-1"}, ToLine: 1, Body: body, SentAt: at, DeliverAt: at,
	})
}

const callScreenTail = "#01 GUEST     WHO       28K-...\r\n" +
	"#02 NORI      idle      14K-...\r\n" +
	"#03 ｿﾈｾｲｼﾞ    CHAT   #0 26K-...\r\n" +
	"\r\n[番号]=電報 [ﾘﾀｰﾝ]=WHO [.]戻る [X]=CHAT\r\nCALL> ->"

func callRuntime(t *testing.T) (*chatWorld, *Runtime) {
	t.Helper()
	w, host := newChatWorld(4)
	rt := loggedInChatRuntime(t, w, host)
	rt.HandleLine("4")
	if out := rt.Poll(w.now); out != "" {
		t.Fatalf("first poll = %q", out)
	}
	return w, rt
}

func TestIncomingTelegramInterruptsTheCallPrompt(t *testing.T) {
	w, rt := callRuntime(t)
	incomingFrom(w, w.now.Add(10*time.Second), "こんばんは(^^)")
	w.now = w.now.Add(30 * time.Second)
	want := "\a\r\n#02 NORI>こんばんは(^^)\a\r\n\r\n" + callScreenTail
	if out := rt.Poll(w.now); out != want {
		t.Fatalf("poll = %q\nwant %q", out, want)
	}
	w.now = w.now.Add(time.Minute)
	if out := rt.Poll(w.now); out != "" {
		t.Fatalf("the same telegram twice: %q", out)
	}
}

func TestSeveralTelegramsComeOneByOneAndWhoOnce(t *testing.T) {
	w, rt := callRuntime(t)
	incomingFrom(w, w.now.Add(10*time.Second), "ひとつめ")
	incomingFrom(w, w.now.Add(20*time.Second), "ふたつめ")
	w.now = w.now.Add(30 * time.Second)
	want := "\a\r\n#02 NORI>ひとつめ\a\r\n\a\r\n#02 NORI>ふたつめ\a\r\n\r\n" + callScreenTail
	if out := rt.Poll(w.now); out != want {
		t.Fatalf("poll = %q\nwant %q", out, want)
	}
}

func TestTelegramsWaitWhileTheCallerWritesOrChats(t *testing.T) {
	w, rt := callRuntime(t)
	incomingFrom(w, w.now.Add(10*time.Second), "まっててね")
	w.now = w.now.Add(30 * time.Second)

	rt.HandleLine("3") // Start writing a telegram.
	if rt.state != "telegram_body" {
		t.Fatalf("state = %s", rt.state)
	}
	if out := rt.Poll(w.now); out != "" {
		t.Fatalf("interrupted a telegram being written: %q", out)
	}
	rt.HandleLine("") // Cancel; back at the call prompt.
	rt.state = "chat_room"
	rt.chat.room, rt.chat.since = 0, w.now
	if out := rt.Poll(w.now.Add(time.Second)); out != "" {
		t.Fatalf("interrupted the chat room: %q", out)
	}
	rt.state = "call"
	w.now = w.now.Add(time.Minute)
	want := "\a\r\n#02 NORI>まっててね\a\r\n\r\n" + callScreenTail
	if out := rt.Poll(w.now); out != want {
		t.Fatalf("after coming back: %q\nwant %q", out, want)
	}
}

func TestEnterAtTheCallPromptShowsWaitingTelegramsBeforeWho(t *testing.T) {
	w, rt := callRuntime(t)
	incomingFrom(w, w.now.Add(10*time.Second), "ぼくです")
	w.now = w.now.Add(30 * time.Second)
	out, _ := rt.HandleLine("")
	want := "\r\n#02 NORI>ぼくです\a\r\n\r\n" + callScreenTail
	if out != want {
		t.Fatalf("enter = %q\nwant %q", out, want)
	}
	if out := rt.Poll(w.now.Add(time.Second)); out != "" {
		t.Fatalf("shown twice: %q", out)
	}
}

func TestEnterWithoutTelegramsIsUnchanged(t *testing.T) {
	w, rt := callRuntime(t)
	w.now = w.now.Add(30 * time.Second)
	out, _ := rt.HandleLine("")
	want := "\r\n\r\n" + callScreenTail
	if out != want {
		t.Fatalf("enter = %q\nwant %q", out, want)
	}
}

func TestIncomingTelegramInTheRoomMenuIsShownBeforeItsPrompt(t *testing.T) {
	w, rt := callRuntime(t)
	rt.HandleLine("X")
	if rt.state != "chat_menu" {
		t.Fatalf("state = %s", rt.state)
	}
	incomingFrom(w, w.now.Add(10*time.Second), "ルームにおいでよ")
	w.now = w.now.Add(30 * time.Second)
	want := "\a\r\n#02 NORI>ルームにおいでよ\a\r\n\r\n" + roomPrompt
	if out := rt.Poll(w.now); out != want {
		t.Fatalf("poll = %q\nwant %q", out, want)
	}
}

func TestNoTelegramsBeforeLogin(t *testing.T) {
	w, host := newChatWorld(4)
	rt := New(host, w)
	rt.SetCallInfo(world.CallInfo{CallID: "call-1", Line: 1, Baud: 28800})
	incomingFrom(w, w.now.Add(10*time.Second), "はやすぎ")
	w.now = w.now.Add(30 * time.Second)
	if out := rt.Poll(w.now); out != "" || len(w.asked) != 0 {
		t.Fatalf("poll before login = %q after %d asks", out, len(w.asked))
	}
}
