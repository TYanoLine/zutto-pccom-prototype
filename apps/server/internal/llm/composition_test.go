package llm

import "testing"

func TestEnsureExactQuoteInsertsAndRejectsAlteredQuote(t *testing.T) {
	got, err := ensureExactQuote("返事です。", "親記事の本文")
	if err != nil {
		t.Fatal(err)
	}
	if want := ">親記事の本文\r\n\r\n返事です。\r\n"; got != want {
		t.Fatalf("composed quote=%q, want %q", got, want)
	}
	if _, err := ensureExactQuote(">改変された本文\n返事", "親記事の本文"); err == nil {
		t.Fatal("altered quote was accepted")
	}
}

func TestBodyLengthValidationExcludesQuotedLines(t *testing.T) {
	draft := BoardPostDraft{Author: "NORI", Subject: "返信", Body: ">長い親記事の引用です\r\n\r\n１２３４５６７８９０"}
	if err := validateBoardPostDraftWithBodyLimit(draft, 10); err != nil {
		t.Fatalf("quote was counted as new writing: %v", err)
	}
	draft.Body += "１"
	if err := validateBoardPostDraftWithBodyLimit(draft, 10); err == nil {
		t.Fatal("non-quoted text over configured bound was accepted")
	}
}

func TestNormalizeBodyBoundsClampsToSafetyLimit(t *testing.T) {
	minChars, maxChars := normalizeBodyBounds(10000, 0)
	if minChars != 8192 || maxChars != 8192 {
		t.Fatalf("unexpected bounds: %d-%d", minChars, maxChars)
	}
}
