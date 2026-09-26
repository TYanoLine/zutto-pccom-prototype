package erikak

import "testing"

func TestDisplayCellWidthJapaneseTerminal(t *testing.T) {
	cases := map[string]int{
		" ":  1,
		"　": 2,
		"■":  2,
		"□":  2,
		"★":  2,
		"→":  2,
		"―":  2,
		"①":  2,
		"ｱ":  1,
	}
	for value, want := range cases {
		if got := displayCellWidth(value); got != want {
			t.Fatalf("displayCellWidth(%q) = %d, want %d", value, got, want)
		}
	}
}

func TestPeriodSeparatorsOccupyEightyCells(t *testing.T) {
	if got := displayCellWidth("■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■"); got != 80 {
		t.Fatalf("black-square separator width = %d, want 80", got)
	}
	if got := displayCellWidth("――――――――――――――――――――――――――――――――――――――"); got != 80 {
		t.Fatalf("horizontal separator width = %d, want 80", got)
	}
}

func TestPadRunesPadsDisplayCellsNotRuneCount(t *testing.T) {
	got := padRunes("博多", 6)
	if width := displayCellWidth(got); width != 6 {
		t.Fatalf("display width = %d, want 6 (%q)", width, got)
	}
}
