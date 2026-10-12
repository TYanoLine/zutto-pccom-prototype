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
			if s.Actor.Kind == world.ActorKindMember && s.Actor.ID == r.callID && r.callID != "" {
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
		From: world.ActorRef{Kind: world.ActorKindMember, ID: r.callID}, To: r.chat.to, ToLine: r.chat.toLine,
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
		return out + padRunes(r.handle, 8) + ":..\r\n" +
			"\r\n" + chatLeaveLine(r.handle) + chatOffRule + "\r\n" +
			"\r\n" + r.whoRows(r.lineHolders(now)) + r.renderRoomMenu(), false
	}
	if line != "" {
		if chat, ok := r.Store.(world.ChatHost); ok {
			_, _ = chat.RecordChatUtterance(r.Host, world.ChatUtterance{
				Room: r.chat.room, Actor: world.ActorRef{Kind: world.ActorKindMember, ID: r.callID},
				Handle: r.handle, Body: line, At: now,
			})
		}
	}
	return out + padRunes(r.handle, 8) + ":" + line + "\r\n", false
}

// Poll shows who entered or left the caller's chat room since the last time.
// Nothing interrupts the other screens, the telegram body included.
func (r *Runtime) Poll(now time.Time) string {
	if r.state != "chat_room" {
		return ""
	}
	return r.roomEvents(now)
}

// roomEvents returns the joins and leaves in the caller's room since the last
// call and moves the mark to now.
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
		if e.Kind == world.ChatJoin {
			b.WriteString(chatJoinLine(e.Handle))
		} else {
			b.WriteString(chatLeaveLine(e.Handle))
		}
	}
	return b.String()
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
