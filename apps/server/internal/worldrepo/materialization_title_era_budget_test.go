package worldrepo

import "testing"

func TestDevelopmentTitleEraResearchAllowanceSharesRunBudgetAcrossBoards(t *testing.T) {
	cases := []struct {
		used, boardsRemaining, want int
	}{
		{0, 4, 4},
		{4, 3, 4},
		{8, 2, 4},
		{12, 1, 4},
		{1, 3, 5}, // unused share from an earlier board flows forward
		{15, 2, 1},
		{16, 1, 0},
		{0, 0, 0},
	}
	for _, tc := range cases {
		if got := developmentTitleEraResearchAllowance(tc.used, tc.boardsRemaining); got != tc.want {
			t.Fatalf("used=%d boards=%d got=%d want=%d", tc.used, tc.boardsRemaining, got, tc.want)
		}
	}
}
