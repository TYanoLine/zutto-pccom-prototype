package world

import (
	"fmt"
	"sort"
	"time"
)

// Ledger channels and audience scopes of chat rooms and line messages
// (spec 016 FR-006). They are shared by every host program.
const (
	ChannelChatUtterance = "chat_utterance"
	ChannelLineMessage   = "line_message"
)

// ChatRoomScope is the audience of what is said in a chat room: the people in
// that room.
func ChatRoomScope(hostID string, room int) string {
	return fmt.Sprintf("chat_room:%s:%d", hostID, room)
}

// PrivateScope is the audience of a message addressed to one caller.
func PrivateScope(hostID string) string { return "private:" + hostID }

// ChatUtterance is one line said in a chat room.
type ChatUtterance struct {
	ID     string    `json:"id"`
	HostID string    `json:"host_id"`
	Room   int       `json:"room"`
	Actor  ActorRef  `json:"actor"`
	Handle string    `json:"handle,omitempty"`
	Body   string    `json:"body"`
	At     time.Time `json:"at"`
	// Slot is the key of the World's decision a resident's line verbalizes
	// (spec 017); empty for a human's line. RespondsTo is the ID of the
	// utterance the line answers, if any.
	Slot       string `json:"slot,omitempty"`
	RespondsTo string `json:"responds_to,omitempty"`
}

// LineMessage is a short message sent to whoever holds a line. DeliverAt is
// when it reaches the recipient; the host program decides it (for example,
// after the recipient leaves a chat room).
type LineMessage struct {
	ID        string    `json:"id"`
	HostID    string    `json:"host_id"`
	From      ActorRef  `json:"from"`
	To        ActorRef  `json:"to"`
	ToLine    int       `json:"to_line"`
	Body      string    `json:"body"`
	SentAt    time.Time `json:"sent_at"`
	DeliverAt time.Time `json:"deliver_at"`

	// FromHandle is a human sender's login ID (spec 019).
	FromHandle string `json:"from_handle,omitempty"`
}

// ChatRecordStore keeps chat utterances and line messages as world history.
// Save assigns the ID; records never change once saved.
type ChatRecordStore interface {
	SaveChatUtterance(u ChatUtterance) ChatUtterance
	// ListChatUtterances returns room's utterances with At in [from, to); a
	// zero to means no upper bound.
	ListChatUtterances(hostID string, room int, from, to time.Time) []ChatUtterance
	SaveLineMessage(m LineMessage) LineMessage
	ListLineMessages(hostID string) []LineMessage
}

var _ ChatRecordStore = (*MemoryStore)(nil)

func (s *MemoryStore) SaveChatUtterance(u ChatUtterance) ChatUtterance {
	s.mu.Lock()
	defer s.mu.Unlock()
	u.ID = fmt.Sprintf("%s:%d", u.HostID, len(s.chatLines[u.HostID])+1)
	s.chatLines[u.HostID] = append(s.chatLines[u.HostID], u)
	sort.SliceStable(s.chatLines[u.HostID], func(i, j int) bool {
		return s.chatLines[u.HostID][i].At.Before(s.chatLines[u.HostID][j].At)
	})
	return u
}

func (s *MemoryStore) ListChatUtterances(hostID string, room int, from, to time.Time) []ChatUtterance {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []ChatUtterance
	for _, u := range s.chatLines[hostID] {
		if u.Room == room && !u.At.Before(from) && (to.IsZero() || u.At.Before(to)) {
			out = append(out, u)
		}
	}
	return out
}

func (s *MemoryStore) SaveLineMessage(m LineMessage) LineMessage {
	s.mu.Lock()
	defer s.mu.Unlock()
	m.ID = fmt.Sprintf("%s:%d", m.HostID, len(s.lineMessages[m.HostID])+1)
	s.lineMessages[m.HostID] = append(s.lineMessages[m.HostID], m)
	return m
}

func (s *MemoryStore) ListLineMessages(hostID string) []LineMessage {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]LineMessage(nil), s.lineMessages[hostID]...)
}
