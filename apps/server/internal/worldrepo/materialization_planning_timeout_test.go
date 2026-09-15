package worldrepo

import (
	"testing"
	"time"
)

func TestDevelopmentPlanningTimeoutScalesByBatchCount(t *testing.T) {
	cases := []struct {
		shells int
		want   time.Duration
	}{
		{shells: 0, want: 105 * time.Second},
		{shells: 1, want: 105 * time.Second},
		{shells: developmentPlanningBatchSize, want: 105 * time.Second},
		{shells: developmentPlanningBatchSize + 1, want: 195 * time.Second},
		{shells: developmentPlanningBatchSize * 2, want: 195 * time.Second},
		{shells: developmentPlanningBatchSize*2 + 1, want: 285 * time.Second},
	}
	for _, tc := range cases {
		if got := developmentPlanningTimeout(tc.shells); got != tc.want {
			t.Fatalf("shells=%d timeout=%s want=%s", tc.shells, got, tc.want)
		}
	}
}
