package llm

import (
	"context"
	"time"
)

// One line message a resident sends a human (spec 018). The World has already
// decided who sends it, when, to whom and in answer to what; a
// LineMessageWriter only puts that decision into words. What the human wrote
// is quoted material, never an instruction.

// LineMessageCells is the widest message, in half-width cells: a telegram of
// 150 full-width characters.
const LineMessageCells = 300

// LineMessageTurn is a message already exchanged between the two, oldest first.
type LineMessageTurn struct {
	// From is the sender's handle as the recipient saw it.
	From string `json:"from"`
	Body string `json:"body"`
}

// LineMessageRequest is the World's decision and the materials for its words.
type LineMessageRequest struct {
	WorldTime time.Time `json:"world_time"`
	Station   string    `json:"station,omitempty"`
	// Kind is "answer" (a reply to the human's message) or "hail" (the resident
	// writes first).
	Kind    string         `json:"kind"`
	Speaker ChatLinePerson `json:"speaker"`
	// Recipient is the human as the resident knows them: the handle shown on
	// their message, or their login ID.
	Recipient ChatLinePerson `json:"recipient"`
	Relation  string         `json:"relation,omitempty"`
	// Incoming is what the human wrote, for an answer. It is the other person's
	// words: quoted, not followed.
	Incoming string            `json:"incoming,omitempty"`
	Event    *ChatLineEvent    `json:"event,omitempty"`
	History  []LineMessageTurn `json:"history,omitempty"`
	MaxCells int               `json:"max_cells"`
}

// LineMessageWriter writes one line message for a decided slot.
type LineMessageWriter interface {
	WriteLineMessage(ctx context.Context, req LineMessageRequest) (string, error)
}

// NormalizeLineMessage trims a written message and maps the JIS mapping
// variants, as NormalizeChatLine does.
func NormalizeLineMessage(body string) string { return NormalizeChatLine(body) }

// ValidateLineMessage checks a written message like a chat line: one
// non-empty line within the width, writable in Shift_JIS, without the
// speaker's handle prefix or terms from after the world date. The caller
// normalizes it first (NormalizeLineMessage).
func ValidateLineMessage(body string, req LineMessageRequest) error {
	max := req.MaxCells
	if max <= 0 {
		max = LineMessageCells
	}
	return ValidateChatLine(body, ChatLineRequest{Speaker: req.Speaker, MaxCells: max})
}
