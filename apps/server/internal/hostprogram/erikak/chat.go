package erikak

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"zutto-pccom/apps/server/internal/world"
)

// The telegram (CALL) and chat screens (spec 016). Row shapes, prompts and the
// lines that announce people follow the 1996 GARAKUTA-KoBo connection log
// (docs/research/erika-k-sources/garakuta_D-00078.html); headings, menus and
// notices are the station's own wording from erika_k.chat. Who is on a line
// and what residents do there come from the world (world.ChatHost); this file
// only shows it.

// chatState is the caller's place in the telegram and chat screens.
type chatState struct {
	// room is the chat room the caller is in, and since the world time up to
	// which its joins and leaves were shown.
	room  int
	since time.Time
	// to and toLine are the recipient of the telegram being written.
	to     world.ActorRef
	toLine int
	// said holds the IDs of the lines this call typed, which were shown when
	// they were typed. Another call of the same user is someone else's screen.
	said map[string]bool
	// lineMark is the world time up to which the residents' telegrams to this
	// call were shown (spec 018). Zero until the call is past its login.
	lineMark time.Time
}

const (
	callPrompt     = "CALL> ->"
	roomPrompt     = "ルーム番号をどうぞ  （中止はﾘﾀｰﾝｷｰ） --> "
	telegramPrompt = "電報をどうぞ。 (最大全角150文字  中止:ﾘﾀｰﾝｷｰ)"
	chatExitHint   = "  チャット終了は 「..」 または 「CTRL+B」 を入力して下さい"
	// telegramCells is 全角150文字.
	telegramCells = 300
	// loginShown is how long a call shows "login" when the world gives no plan.
	loginShown = 2 * time.Minute
)

var (
	telegramRuler = strings.Repeat("---------+", 8)
	chatOnRule    = strings.Repeat("-", 36) + "CHAT ON" + strings.Repeat("-", 35)
	chatOffRule   = strings.Repeat("-", 36) + "CHAT OFF" + strings.Repeat("-", 34)
)

// callStateLabel shows the shared call states with Erika-K's labels.
func callStateLabel(state world.CallState) string {
	switch state {
	case world.CallPaging:
		return "CALL"
	case world.CallChat:
		return "CHAT"
	case world.CallLogin:
		return "login"
	}
	return "idle"
}

// lineHolder is who holds one line now, as the call screen shows it.
type lineHolder struct {
	line   int
	actor  world.ActorRef
	handle string
	baud   int
	self   bool
	state  world.CallState
	room   int
	// chatUntil is the end of the resident's stay in a room.
	chatUntil time.Time
}

// lineHolders returns who holds each line now, by line number.
func (r *Runtime) lineHolders(now time.Time) map[int]lineHolder {
	out := map[int]lineHolder{}
	plans := map[string]world.CallActivity{}
	if chat, ok := r.Store.(world.ChatHost); ok {
		for _, a := range chat.CallActivitiesNow(r.Host) {
			plans[a.Key()] = a
		}
	}
	if observer, ok := r.Store.(world.PresenceObserver); ok {
		for _, s := range observer.ObservePresenceNow(r.Host) {
			h := lineHolder{line: s.Line, actor: s.Actor, handle: s.Handle, baud: s.Baud, state: world.CallIdle}
			if r.isMyCall(s) {
				h.self = true
			}
			if a, ok := plans[world.CallActivity{Actor: s.Actor, Start: s.Start}.Key()]; ok {
				if seg, ok := a.SegmentAt(now); ok {
					h.state, h.room = seg.State, seg.Room
					if seg.State == world.CallChat {
						h.chatUntil = seg.To
					}
				}
			} else if s.Actor.Kind == world.ActorKindPersona && now.Sub(s.Start) < loginShown {
				h.state = world.CallLogin
			}
			out[s.Line] = h
		}
	}
	self := false
	for _, h := range out {
		self = self || h.self
	}
	if !self {
		line := r.line
		if line == 0 {
			line = 1
		}
		out[line] = lineHolder{line: line, self: true}
	}
	return out
}

// whoRows renders one row per line of the station: the holder's handle, state,
// a three-cell column and speed, or "ring" for a free line.
func (r *Runtime) whoRows(holders map[int]lineHolder) string {
	lines := r.Host.Lines
	if lines < 1 {
		lines = 1
	}
	var b strings.Builder
	for line := 1; line <= lines; line++ {
		h, ok := holders[line]
		if !ok {
			fmt.Fprintf(&b, "#%02d ring\r\n", line)
			continue
		}
		handle, state, icon, baud := h.handle, callStateLabel(h.state), "", h.baud
		if h.self {
			// The caller's own row always reads WHO: they are looking at it.
			handle, state = r.handle, "WHO"
			if r.baud > 0 {
				baud = r.baud
			}
		} else if h.state == world.CallChat {
			icon = "#" + strconv.Itoa(h.room)
		}
		if handle == "" {
			handle = "GUEST"
		}
		row := fmt.Sprintf("#%02d %s%s%s", line, padRunes(handle, 10), padRunes(state, 7), padRunes(icon, 3))
		if baud > 0 {
			row += fmt.Sprintf("%dK-...", baud/1000)
		}
		b.WriteString(strings.TrimRight(row, " ") + "\r\n")
	}
	return b.String()
}

func (r *Runtime) callPromptText() string {
	out := ""
	if hint := r.detail.Chat.CallMenu.Hint; hint != "" {
		out += hint + "\r\n"
	}
	return out + callPrompt
}

func (r *Runtime) renderCallScreen() string {
	menu := r.detail.Chat.CallMenu
	var b strings.Builder
	b.WriteString("\r\n")
	if menu.Title != "" {
		b.WriteString(menu.Title + "\r\n")
	}
	for _, line := range menu.Lines {
		b.WriteString(line + "\r\n")
	}
	b.WriteString("\r\n" + r.whoRows(r.lineHolders(world.WorldNowOf(r.Store))) + "\r\n" + r.callPromptText())
	return b.String()
}

func (r *Runtime) renderRoomMenu() string {
	var b strings.Builder
	for _, line := range r.detail.Chat.ChatMenu {
		b.WriteString(line + "\r\n")
	}
	return b.String() + roomPrompt
}

func (r *Runtime) handleCall(line string) (string, bool) {
	switch strings.ToUpper(line) {
	case "":
		now := world.WorldNowOf(r.Store)
		// Telegrams that came in since the last screen are shown first.
		shown := ""
		for _, m := range r.lineMessagesDue(now) {
			shown += lineMessageText(m)
		}
		if r.chat.lineMark.Before(now) {
			r.chat.lineMark = now
		}
		if shown != "" {
			return "\r\n" + shown + "\r\n" + r.whoRows(r.lineHolders(now)) + "\r\n" + r.callPromptText(), false
		}
		return "\r\n\r\n" + r.whoRows(r.lineHolders(world.WorldNowOf(r.Store))) + "\r\n" + r.callPromptText(), false
	case "X":
		r.state = "chat_menu"
		return "\r\n" + r.renderRoomMenu(), false
	case ".":
		r.state = "main"
		return r.renderMainMenu(), false
	}
	target, err := strconv.Atoi(line)
	if err == nil {
		h, held := r.lineHolders(world.WorldNowOf(r.Store))[target]
		if held && !h.self && target >= 1 && target <= r.Host.Lines {
			r.chat.to, r.chat.toLine = h.actor, target
			r.state = "telegram_body"
			return "\r\n" + telegramPrompt + "\r\n" + telegramRuler + "\r\n", false
		}
	}
	// "*" (everyone), P, S and the other menu letters are not part of this
	// reconstruction yet; neither is a telegram to a free line or to oneself.
	return "? COMMAND ERROR\r\n" + r.callPromptText(), false
}

func (r *Runtime) handleTelegramBody(line string) (string, bool) {
	r.state = "call"
	if line == "" {
		return "\r\n" + r.callPromptText(), false
	}
	now := world.WorldNowOf(r.Store)
	msg := world.LineMessage{
		From: r.self(), FromHandle: r.handle, To: r.chat.to, ToLine: r.chat.toLine,
		Body: trimCells(line, telegramCells), SentAt: now, DeliverAt: now,
	}
	holders := r.lineHolders(now)
	h, still := holders[r.chat.toLine]
	inChat := still && h.actor == r.chat.to && h.state == world.CallChat
	if inChat {
		// A telegram to someone in a room arrives when they leave it.
		msg.DeliverAt = h.chatUntil
	}
	if chat, ok := r.Store.(world.ChatHost); ok {
		_, _ = chat.SendLineMessage(r.Host, msg)
	}
	if inChat {
		return fmt.Sprintf("\r\n#%02d チャットルームか会議室にちゃっかり入ってます。 CHAT終了後、電報を送ります。\r\n\r\n", r.chat.toLine) +
			r.whoRows(holders) + "\r\n" + r.callPromptText(), false
	}
	return fmt.Sprintf("\r\n#%02d に電報を送りました。\r\n\r\n", r.chat.toLine) + r.callPromptText(), false
}

func (r *Runtime) handleChatMenu(line string) (string, bool) {
	if line == "" {
		r.state = "main"
		return r.renderMainMenu(), false
	}
	number, err := strconv.Atoi(line)
	room, known := r.chatRoom(number)
	if err != nil || !known {
		return "? COMMAND ERROR\r\n" + r.renderRoomMenu(), false
	}
	now := world.WorldNowOf(r.Store)
	if room.Capacity > 0 {
		inRoom := 0
		for _, h := range r.lineHolders(now) {
			if !h.self && h.state == world.CallChat && h.room == number {
				inRoom++
			}
		}
		if room.Full(inRoom) {
			return "? COMMAND ERROR\r\n" + r.renderRoomMenu(), false
		}
	}
	r.state = "chat_room"
	r.chat.room, r.chat.since = number, now
	if chat, ok := r.Store.(world.ChatHost); ok {
		chat.EnterChatRoom(r.Host, number, r.self(), r.handle, now)
	}
	var b strings.Builder
	b.WriteString("\r\n\r\n" + chatExitHint + "\r\n")
	for _, notice := range r.detail.Chat.RoomNotices[number] {
		b.WriteString(notice + "\r\n")
	}
	b.WriteString(chatOnRule + "\r\n" + chatJoinLine(r.handle))
	return b.String(), false
}

func (r *Runtime) handleChatRoom(line string) (string, bool) {
	now := world.WorldNowOf(r.Store)
	out := "\r\n" + r.roomEvents(now)
	if line == ".." {
		r.state = "chat_menu"
		if chat, ok := r.Store.(world.ChatHost); ok {
			chat.LeaveChatRoom(r.Host, r.chat.room, r.self(), now)
		}
		return out + padRunes(r.handle, 8) + ":..\r\n" +
			"\r\n" + chatLeaveLine(r.handle) + chatOffRule + "\r\n" +
			"\r\n" + r.whoRows(r.lineHolders(now)) + r.renderRoomMenu(), false
	}
	if line != "" {
		if chat, ok := r.Store.(world.ChatHost); ok {
			if saved, err := chat.RecordChatUtterance(r.Host, world.ChatUtterance{
				Room: r.chat.room, Actor: r.self(),
				Handle: r.handle, Body: line, At: now,
			}); err == nil && saved.ID != "" {
				if r.chat.said == nil {
					r.chat.said = map[string]bool{}
				}
				r.chat.said[saved.ID] = true
			}
		}
	}
	return out + padRunes(r.handle, 8) + ":" + line + "\r\n", false
}

// Poll shows who entered or left the caller's chat room, and what was said
// there, since the last time.
// Nothing interrupts the other screens, the telegram body included.
func (r *Runtime) Poll(now time.Time) string {
	out := ""
	if r.state == "chat_room" {
		out = r.roomEvents(now)
	}
	return out + r.pollLineMessages(now)
}

// lineMessagesDue returns the residents' telegrams to this call whose time has
// come since lineMark, and starts the world deciding the coming ones. The
// caller moves lineMark when it can show what it got.
func (r *Runtime) lineMessagesDue(now time.Time) []world.LineMessage {
	host, ok := r.Store.(world.LineMessageHost)
	if !ok || r.callID == "" || r.state == "login_id" || r.state == "login_password" {
		return nil
	}
	if r.chat.lineMark.IsZero() {
		r.chat.lineMark = now
		return nil
	}
	if !now.After(r.chat.lineMark) {
		return nil
	}
	call := world.LineMessageCall{CallID: r.callID, Human: r.self(), Line: r.line, Login: r.handle}
	return host.LineMessageEvents(r.Host, call, r.chat.lineMark, now)
}

// pollLineMessages shows telegrams that arrived while the caller waits at the
// CALL prompt or the room menu, as they interrupted the screen of 1996: a
// bell, the line and handle, the text, and the screen again. On other screens
// they wait, and are shown when the caller comes back.
func (r *Runtime) pollLineMessages(now time.Time) string {
	msgs := r.lineMessagesDue(now)
	if r.state != "call" && r.state != "chat_menu" {
		return ""
	}
	if r.chat.lineMark.Before(now) {
		r.chat.lineMark = now
	}
	if len(msgs) == 0 {
		return ""
	}
	var b strings.Builder
	for _, m := range msgs {
		b.WriteString("\a\r\n" + lineMessageText(m))
	}
	if r.state == "chat_menu" {
		return b.String() + "\r\n" + roomPrompt
	}
	return b.String() + "\r\n" + r.whoRows(r.lineHolders(now)) + "\r\n" + r.callPromptText()
}

// lineMessageText is one telegram shown on the screen: the sender's line and
// handle, then the text, then the bell.
func lineMessageText(m world.LineMessage) string {
	return fmt.Sprintf("#%02d %s>%s\a\r\n", m.FromLine, m.FromHandle, m.Body)
}

// roomEvents returns the joins, leaves and lines in the caller's room since
// the last call and moves the mark to now. The caller's own lines were shown
// when they were typed.
func (r *Runtime) roomEvents(now time.Time) string {
	if !now.After(r.chat.since) {
		return ""
	}
	from := r.chat.since
	r.chat.since = now
	chat, ok := r.Store.(world.ChatHost)
	if !ok {
		return ""
	}
	var b strings.Builder
	for _, e := range chat.ChatEvents(r.Host, r.chat.room, from, now) {
		switch e.Kind {
		case world.ChatJoin:
			b.WriteString(chatJoinLine(e.Handle))
		case world.ChatLeave:
			b.WriteString(chatLeaveLine(e.Handle))
		case world.ChatSay:
			if !r.saidByMe(e) {
				b.WriteString(padRunes(e.Handle, 8) + ":" + e.Body + "\r\n")
			}
		}
	}
	return b.String()
}

// self is the caller as the world knows them in rooms and telegrams: the user
// (spec 019). A call made without a human ID is named by its call ID.
func (r *Runtime) self() world.ActorRef {
	id := r.humanID
	if id == "" {
		id = r.callID
	}
	return world.ActorRef{Kind: world.ActorKindMember, ID: id}
}

// isMyCall reports whether a presence session is this caller's own call. A
// session that does not name its call is matched by the actor it names, as
// before spec 019.
func (r *Runtime) isMyCall(s world.PresenceSession) bool {
	if r.callID == "" {
		return false
	}
	if s.CallID != "" {
		return s.CallID == r.callID
	}
	return s.Actor.Kind == world.ActorKindMember && s.Actor.ID == r.callID
}

// saidByMe reports whether a line of the room was typed on this call.
func (r *Runtime) saidByMe(e world.ChatEvent) bool {
	if e.UtteranceID != "" {
		return r.chat.said[e.UtteranceID]
	}
	return e.Actor == r.self()
}

func chatJoinLine(handle string) string  { return "\a\a## chat on  " + handle + " ##\r\n" }
func chatLeaveLine(handle string) string { return "\a\a## chat off " + handle + " ##\r\n" }

func (r *Runtime) chatRoom(number int) (world.ChatRoom, bool) {
	store, ok := r.Store.(world.HostDetailStore)
	if !ok {
		return world.ChatRoom{}, false
	}
	detail, ok := store.HostDetail(r.Host.ID)
	if !ok {
		return world.ChatRoom{}, false
	}
	for _, room := range world.ChatRoomsOf(detail) {
		if room.Number == number {
			return room, true
		}
	}
	return world.ChatRoom{}, false
}

// trimCells cuts s to at most width display cells.
func trimCells(s string, width int) string {
	used := 0
	for i, r := range s {
		used += runeCellWidth(r)
		if used > width {
			return s[:i]
		}
	}
	return s
}
