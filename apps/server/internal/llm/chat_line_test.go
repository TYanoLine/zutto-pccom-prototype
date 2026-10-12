package llm

import (
	"errors"
	"strings"
	"testing"
)

func TestValidateChatLine(t *testing.T) {
	req := ChatLineRequest{Speaker: ChatLinePerson{Handle: "NORI"}, MaxCells: 20}
	for name, line := range map[string]string{
		"empty":       "  ",
		"newline":     "こんばんは\nどうも",
		"tab":         "こんば\tんは",
		"too long":    "こんばんは、今日は寒いですね",
		"not sjis":    "いいね😀",
		"handle":      "NORI:こんばんは",
		"handle wide": "NORI ：どうも",
		"future":      "スマホで見てます",
	} {
		if err := ValidateChatLine(line, req); !errors.Is(err, ErrChatLineRejected) {
			t.Errorf("%s: %q passed (%v)", name, line, err)
		}
	}
	for _, line := range []string{"こんばんは(^^)", "NORIです〜", "ｺﾝﾊﾞﾝﾜ ﾃﾞｽ", "おつかれー(^^;", " 1−2 "} {
		if err := ValidateChatLine(NormalizeChatLine(line), req); err != nil {
			t.Errorf("%q rejected: %v", line, err)
		}
	}
	if err := ValidateChatLine(strings.Repeat("あ", 32), ChatLineRequest{}); err != nil {
		t.Errorf("default width rejected 64 cells: %v", err)
	}
	if err := ValidateChatLine(strings.Repeat("あ", 33), ChatLineRequest{}); err == nil {
		t.Error("default width allowed 66 cells")
	}
}

func TestChatLineCells(t *testing.T) {
	for s, want := range map[string]int{"abc": 3, "あい": 4, "ｱｲ": 2, "A全": 3} {
		if got := ChatLineCells(s); got != want {
			t.Errorf("%q: %d cells, want %d", s, got, want)
		}
	}
}
