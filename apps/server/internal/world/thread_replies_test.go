package world

import (
	"math"
	"testing"
)

func TestThreadReplyCountIsDeterministic(t *testing.T) {
	for i := 1; i <= 50; i++ {
		if ThreadReplyCount("h", "b", i, 1.3) != ThreadReplyCount("h", "b", i, 1.3) {
			t.Fatalf("ordinal %d not deterministic", i)
		}
	}
	if ThreadReplyCount("h", "b", 1, 0) != 0 || ThreadReplyCount("h", "b", 0, 1.3) != 0 {
		t.Fatal("zero rate / invalid ordinal must yield no replies")
	}
}

func TestThreadReplyCountShapeFollowsRate(t *testing.T) {
	for _, rate := range []float64{.2, 1.1, 2.3} {
		const n = 20000
		sum, silent, max := 0, 0, 0
		for i := 1; i <= n; i++ {
			c := ThreadReplyCount("hakata", "board", i, rate)
			if c < 0 || c > MaxThreadReplies {
				t.Fatalf("rate %.2f ordinal %d count %d out of range", rate, i, c)
			}
			sum += c
			if c == 0 {
				silent++
			}
			if c > max {
				max = c
			}
		}
		mean := float64(sum) / n
		if math.Abs(mean-rate) > .25*rate+.05 {
			t.Fatalf("rate %.2f: mean replies per root %.3f drifted", rate, mean)
		}
		if rate >= 1 && (float64(silent)/n < .15 || max < 8) {
			t.Fatalf("rate %.2f: silent share %.2f max %d; want unanswered roots and a long tail", rate, float64(silent)/n, max)
		}
	}
}
