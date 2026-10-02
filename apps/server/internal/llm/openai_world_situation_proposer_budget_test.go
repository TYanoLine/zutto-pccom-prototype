package llm

import "testing"

func TestBBSWorldSituationMaxTokensLeavesRoomForTypedBatch(t *testing.T) {
	cases := []struct {
		events int
		want   int
	}{
		{0, 1200},
		{1, 4000},
		{2, 4000},
		{3, 6000},
		{4, 8000},
		{5, 10000},
		{6, 12000},
		{8, 12000},
		{12, 12000},
	}
	for _, tc := range cases {
		if got := bbsWorldSituationMaxTokens(tc.events); got != tc.want {
			t.Fatalf("events=%d: got max tokens %d, want %d", tc.events, got, tc.want)
		}
	}
}

func TestBBSWorldSituationMaxTokensCapsLargeBatch(t *testing.T) {
	if got := bbsWorldSituationMaxTokens(100); got != 12000 {
		t.Fatalf("got %d, want 12000", got)
	}
}
