package llm

import "testing"

func TestBBSWorldSituationMaxTokensLeavesRoomForTypedBatch(t *testing.T) {
	cases := []struct {
		events int
		min    int
	}{
		{1, 2400},
		{8, 4000},
		{12, 5600},
	}
	for _, tc := range cases {
		got := bbsWorldSituationMaxTokens(tc.events)
		if got < tc.min {
			t.Fatalf("events=%d: got max tokens %d, want >= %d", tc.events, got, tc.min)
		}
		if got > 10000 {
			t.Fatalf("events=%d: got max tokens %d, want <= 10000", tc.events, got)
		}
	}
}

func TestBBSWorldSituationMaxTokensCapsLargeBatch(t *testing.T) {
	if got := bbsWorldSituationMaxTokens(100); got != 10000 {
		t.Fatalf("got %d, want 10000", got)
	}
}
