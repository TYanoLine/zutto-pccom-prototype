package llm

import (
	"errors"
	"strings"
)

func normalizeBodyBounds(minChars, maxChars int) (int, int) {
	if minChars < 1 {
		minChars = 1
	}
	if minChars > 8192 {
		minChars = 8192
	}
	if maxChars < minChars {
		maxChars = 700
	}
	if maxChars > 8192 {
		maxChars = 8192
	}
	if maxChars < minChars {
		maxChars = minChars
	}
	return minChars, maxChars
}

func outputTokenBudget(maxBodyChars int) int {
	switch {
	case maxBodyChars > 2560:
		return 4000
	case maxBodyChars > 1280:
		return 2400
	default:
		return 1200
	}
}

func ensureExactQuote(body, quote string) (string, error) {
	body = strings.TrimSpace(strings.ReplaceAll(strings.ReplaceAll(body, "\r\n", "\n"), "\r", "\n"))
	quote = strings.TrimSpace(strings.ReplaceAll(strings.ReplaceAll(quote, "\r\n", "\n"), "\r", "\n"))
	if quote == "" {
		return body, nil
	}
	expected := ">" + quote
	for _, line := range strings.Split(body, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, ">") {
			if trimmed != expected {
				return "", errors.New("quote was altered")
			}
			return normalizeCRLF(body), nil
		}
	}
	if body == "" {
		return expected + "\r\n", nil
	}
	return expected + "\r\n\r\n" + normalizeCRLF(body), nil
}

func EnsureExactQuote(body, quote string) (string, error) { return ensureExactQuote(body, quote) }

func bodyWithoutQuotes(body string) string {
	lines := strings.Split(strings.ReplaceAll(strings.ReplaceAll(body, "\r\n", "\n"), "\r", "\n"), "\n")
	kept := make([]string, 0, len(lines))
	for _, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), ">") {
			continue
		}
		kept = append(kept, line)
	}
	return strings.TrimSpace(strings.Join(kept, "\n"))
}
