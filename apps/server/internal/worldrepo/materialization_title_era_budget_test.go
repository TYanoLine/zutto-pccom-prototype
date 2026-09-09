package worldrepo

import "testing"

func TestDevelopmentTitleEraResearchAllowanceSharesRunBudgetAcrossBoards(t *testing.T) {
	cases := []struct {
		used, boardsRemaining, want int
	}{
		{0, 4, 3},
		{3, 3, 3},
		{6, 2, 3},
		{9, 1, 3},
		{1, 3, 4}, // unused share from an earlier board flows forward
		{11, 2, 1},
		{12, 1, 0},
		{0, 0, 0},
	}
	for _, tc := range cases {
		if got := developmentTitleEraResearchAllowance(tc.used, tc.boardsRemaining); got != tc.want {
			t.Fatalf("used=%d boards=%d got=%d want=%d", tc.used, tc.boardsRemaining, got, tc.want)
		}
	}
}
