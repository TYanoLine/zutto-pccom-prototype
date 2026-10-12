package llm

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"

	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/transform"
)

// One line a resident says in a chat room (spec 017). The World has already
// decided who speaks, when, to whom and in answer to what; a ChatLineWriter
// only puts that decision into words.

// DefaultChatLineCells is the widest line, in half-width cells, when a request
// gives none. It is shared by every host program.
const DefaultChatLineCells = 64

// ChatLinePerson is someone in the room as the speaker knows them.
type ChatLinePerson struct {
	Handle string `json:"handle"`
	// Summary is the persona's profile and style; empty for a caller.
	Summary string `json:"summary,omitempty"`
}

// ChatLineTurn is a line already said in the room.
type ChatLineTurn struct {
	Handle string `json:"handle"`
	Body   string `json:"body"`
}

// ChatLineEvent is a world event the speaker brings up: who did what, where.
// It never carries a post's body.
type ChatLineEvent struct {
	Subject string    `json:"subject"`
	Kind    string    `json:"kind"`
	Board   string    `json:"board,omitempty"`
	Title   string    `json:"title,omitempty"`
	By      string    `json:"by,omitempty"`
	At      time.Time `json:"at"`
}

// ChatLineRequest is the World's decision and the materials for its words.
type ChatLineRequest struct {
	WorldTime  time.Time       `json:"world_time"`
	Station    string          `json:"station,omitempty"`
	Room       int             `json:"room"`
	Kind       string          `json:"kind"`
	Speaker    ChatLinePerson  `json:"speaker"`
	Addressee  *ChatLinePerson `json:"addressee,omitempty"`
	Relation   string          `json:"relation,omitempty"`
	Present    []string        `json:"present"`
	Transcript []ChatLineTurn  `json:"transcript"`
	Event      *ChatLineEvent  `json:"event,omitempty"`
	MaxCells   int             `json:"max_cells"`
}

// ChatLineWriter writes one chat line for a decided slot.
type ChatLineWriter interface {
	WriteChatLine(ctx context.Context, req ChatLineRequest) (string, error)
}

// ErrChatLineRejected wraps every validation failure.
var ErrChatLineRejected = errors.New("chat line rejected")

// ValidateChatLine checks a written line: one non-empty line within the width,
// writable in Shift_JIS, without the speaker's handle prefix or terms from
// after the world date. The caller normalizes it first (NormalizeChatLine).
func ValidateChatLine(line string, req ChatLineRequest) error {
	reject := func(format string, args ...any) error {
		return fmt.Errorf("%w: "+format, append([]any{ErrChatLineRejected}, args...)...)
	}
	if strings.TrimSpace(line) == "" {
		return reject("empty")
	}
	for _, r := range line {
		if unicode.IsControl(r) {
			return reject("control character %U", r)
		}
	}
	max := req.MaxCells
	if max <= 0 {
		max = DefaultChatLineCells
	}
	if w := ChatLineCells(line); w > max {
		return reject("%d cells, more than %d", w, max)
	}
	if _, _, err := transform.String(japanese.ShiftJIS.NewEncoder(), line); err != nil {
		return reject("not writable in Shift_JIS")
	}
	if h := strings.TrimSpace(req.Speaker.Handle); h != "" {
		rest, ok := strings.CutPrefix(line, h)
		rest = strings.TrimLeft(rest, " 　")
		if ok && (strings.HasPrefix(rest, ":") || strings.HasPrefix(rest, "：") || strings.HasPrefix(rest, ">")) {
			return reject("starts with the speaker's handle")
		}
	}
	if term := forbiddenFutureMetaTerm(strings.ToLower(line)); term != "" {
		return reject("contains %q", term)
	}
	return nil
}

// chatLineVariants maps characters models often write to the forms the
// Shift_JIS encoder accepts for the same JIS character (WAVE DASH and others
// that differ between JIS mappings).
var chatLineVariants = strings.NewReplacer("〜", "～", "−", "－", "‖", "∥", "¢", "￠", "£", "￡", "¬", "￢")

// NormalizeChatLine trims a written line and maps the JIS mapping variants.
func NormalizeChatLine(line string) string {
	return chatLineVariants.Replace(strings.TrimSpace(line))
}

// ChatLineCells is the width of a line on a Japanese terminal: ASCII and
// half-width katakana take one cell, everything else two.
func ChatLineCells(s string) int {
	w := 0
	for _, r := range s {
		if r <= 0x7F || (r >= 0xFF61 && r <= 0xFF9F) {
			w++
		} else {
			w += 2
		}
	}
	return w
}
