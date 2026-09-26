package erikak

import (
	"strings"
	"testing"
)

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
	if got := displayCellWidth(strings.Repeat("―", 40)); got != 80 {
		t.Fatalf("horizontal separator width = %d, want 80", got)
	}
}

func TestPadRunesPadsDisplayCellsNotRuneCount(t *testing.T) {
	got := padRunes("博多", 6)
	if width := displayCellWidth(got); width != 6 {
		t.Fatalf("display width = %d, want 6 (%q)", width, got)
	}
}


func TestLoginBannerUsesExactDisplayCells(t *testing.T) {
	r := &Runtime{handle: "GUEST"}
	output := r.finishLogin()
	for _, line := range strings.Split(output, "\r\n") {
		switch {
		case strings.Contains(line, "WELCOME TO HAKATA CANAL NET"),
			strings.Contains(line, "博多から、"),
			strings.Contains(line, "23:00以降"),
			strings.Contains(line, "ERIKA-K"),
			strings.HasPrefix(line, "■■"):
			if got := displayCellWidth(line); got != 80 {
				t.Fatalf("banner line width = %d, want 80: %q", got, line)
			}
		}
	}
}

func TestMainMenuColumnStartsAreStable(t *testing.T) {
	r := &Runtime{}
	lines := strings.Split(r.renderMainMenu(), "\r\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "―") {
			if got := displayCellWidth(line); got != 80 {
				t.Fatalf("menu separator width = %d, want 80", got)
			}
		}
	}

	rows := []struct {
		linePrefix string
		second     string
		third      string
	}{
		{"[1]", "[2]", "[3]"},
		{"[4]", "[5]", "[6]"},
		{"[7]", "[9]", "[0]"},
		{"[A]", "[ASET]", "[MA]"},
		{"[T]", "[V]", "[H]"},
	}
	for _, row := range rows {
		var line string
		for _, candidate := range lines {
			if strings.HasPrefix(candidate, row.linePrefix) {
				line = candidate
				break
			}
		}
		if line == "" {
			t.Fatalf("menu row %q not found", row.linePrefix)
		}
		secondByte := strings.Index(line, row.second)
		thirdByte := strings.Index(line, row.third)
		if secondByte < 0 || thirdByte < 0 {
			t.Fatalf("menu markers missing from %q", line)
		}
		if got := displayCellWidth(line[:secondByte]); got != 22 {
			t.Fatalf("%s starts at cell %d, want 22: %q", row.second, got, line)
		}
		if got := displayCellWidth(line[:thirdByte]); got != 44 {
			t.Fatalf("%s starts at cell %d, want 44: %q", row.third, got, line)
		}
	}
}
