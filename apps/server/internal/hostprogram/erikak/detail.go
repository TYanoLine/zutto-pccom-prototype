package erikak

import (
	"strings"
	"unicode/utf8"

	"zutto-pccom/apps/server/internal/hostcatalog"
	"zutto-pccom/apps/server/internal/world"
)

func detailFor(host world.Host, store world.Store) *hostcatalog.ErikaKDetail {
	if details, ok := store.(world.HostDetailStore); ok {
		detail, found := details.HostDetail(host.ID)
		if found && detail.ErikaK != nil {
			return detail.ErikaK
		}
	}
	return nil
}

func ValidateDetail(detail *hostcatalog.ErikaKDetail) error {
	if detail == nil {
		return nil
	}
	for _, line := range detail.Login.StationMessage {
		if displayWidth(line) > 74 {
			return &widthError{line: line}
		}
	}
	return nil
}

type widthError struct{ line string }

func (e *widthError) Error() string { return "station message exceeds 74 cells: " + e.line }

func displayWidth(s string) int {
	width := 0
	for len(s) > 0 {
		r, n := utf8.DecodeRuneInString(s)
		if r < 0x80 {
			width++
		} else {
			width += 2
		}
		s = s[n:]
	}
	return width
}

func fullWidthASCII(s string) string {
	return strings.Map(func(r rune) rune {
		if r >= 0x21 && r <= 0x7e {
			return r + 0xfee0
		}
		return r
	}, s)
}
