package main

import (
	"strings"
	"testing"
)

func TestParseBulkFailureCountAcceptsCurrentInlineFormat(t *testing.T) {
	status := "[DEV] REQUESTS       : 24/24 / bodies=21/24 / generated=21 / failures=3\r\n"
	if got := parseBulkFailureCount(status); got != 3 {
		t.Fatalf("failure count=%d, want 3", got)
	}
}

func TestParseBulkFailureCountKeepsLegacyFormat(t *testing.T) {
	status := "[DEV] FAILURES       : 2\r\n"
	if got := parseBulkFailureCount(status); got != 2 {
		t.Fatalf("legacy failure count=%d, want 2", got)
	}
}

func TestCompactLabStatusKeepsFailureDetails(t *testing.T) {
	status := strings.Join([]string{
		"[DEV] BULK STATUS    : COMPLETED",
		"[DEV] REQUESTS       : 2/2 / bodies=1/2 / generated=1 / failures=1",
		"[DEV] LAST FAILURE   : 2:1005 empty-body (error stage=article-detail detail=bad)",
		"[DEV] FAILURE 1      : 2:1005 empty-body (error stage=article-detail detail=bad)",
	}, "\r\n")
	got := compactLabStatus(status)
	for _, want := range []string{"failures=1", "LAST FAILURE", "FAILURE 1", "article-detail"} {
		if !strings.Contains(got, want) {
			t.Fatalf("compact status missing %q: %s", want, got)
		}
	}
}
