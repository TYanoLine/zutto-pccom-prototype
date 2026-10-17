package world

import (
	"math"
	"math/rand"
	"testing"
)

func TestReplyCountMonotonicity(t *testing.T) {
	// 段階が高いほど期待値が大きい（固定の乱数源、各段階 2 万回）
	const trials = 20000
	const rate = 1.4
	const active = 100

	means := make([]float64, 5)
	for appeal := 0; appeal <= 4; appeal++ {
		rng := rand.New(rand.NewSource(42))
		sum := 0
		for i := 0; i < trials; i++ {
			sum += ReplyCountFor(rng, rate, appeal, active, MaxThreadReplies)
		}
		means[appeal] = float64(sum) / float64(trials)
	}

	for appeal := 1; appeal <= 4; appeal++ {
		if means[appeal] <= means[appeal-1] {
			t.Fatalf("expected strictly increasing means across appeal levels: appeal %d (%.4f) <= appeal %d (%.4f)",
				appeal, means[appeal], appeal-1, means[appeal-1])
		}
	}
}

func TestReplyCountUpperBounds(t *testing.T) {
	// active - 1 と MaxThreadReplies を超えない
	const trials = 5000
	rng := rand.New(rand.NewSource(12345))

	// Case 1: active = 3 -> active - 1 = 2
	for i := 0; i < trials; i++ {
		got := ReplyCountFor(rng, 3.0, 4, 3, MaxThreadReplies)
		if got > 2 {
			t.Fatalf("expected count <= 2 (active - 1), got %d", got)
		}
	}

	// Case 2: active = 2 -> active - 1 = 1
	for i := 0; i < trials; i++ {
		got := ReplyCountFor(rng, 3.0, 4, 2, MaxThreadReplies)
		if got > 1 {
			t.Fatalf("expected count <= 1 (active - 1), got %d", got)
		}
	}

	// Case 3: active = 1 -> active - 1 = 0
	for i := 0; i < 100; i++ {
		got := ReplyCountFor(rng, 3.0, 4, 1, MaxThreadReplies)
		if got != 0 {
			t.Fatalf("expected count == 0 when active == 1, got %d", got)
		}
	}

	// Case 4: MaxThreadReplies cap
	for i := 0; i < trials; i++ {
		got := ReplyCountFor(rng, 10.0, 4, 1000, 15)
		if got > 15 {
			t.Fatalf("expected count <= custom max 15, got %d", got)
		}
		gotDefaultMax := ReplyCountFor(rng, 10.0, 4, 1000, 0)
		if gotDefaultMax > MaxThreadReplies {
			t.Fatalf("expected count <= MaxThreadReplies (30), got %d", gotDefaultMax)
		}
	}
}

func TestReplyCountBaselineMeanNearRate(t *testing.T) {
	// appeal=2、active が十分大きいとき、平均が rate の近く
	const trials = 50000
	const rate = 1.5
	const active = 1000

	rng := rand.New(rand.NewSource(999))
	sum := 0
	for i := 0; i < trials; i++ {
		sum += ReplyCountFor(rng, rate, 2, active, MaxThreadReplies)
	}
	mean := float64(sum) / float64(trials)
	diff := math.Abs(mean - rate)

	// Tolerance is 0.05 on 50,000 trials
	if diff > 0.05 {
		t.Fatalf("expected mean near rate %.2f, got %.4f (diff %.4f > 0.05)", rate, mean, diff)
	}
}

func TestReplyCountDeterminism(t *testing.T) {
	// 同じ乱数源・同じ入力で同じ結果
	seq1 := make([]int, 100)
	seq2 := make([]int, 100)

	rng1 := rand.New(rand.NewSource(777))
	for i := range seq1 {
		seq1[i] = ReplyCountFor(rng1, 1.2, i%5, 50, MaxThreadReplies)
	}

	rng2 := rand.New(rand.NewSource(777))
	for i := range seq2 {
		seq2[i] = ReplyCountFor(rng2, 1.2, i%5, 50, MaxThreadReplies)
	}

	for i := range seq1 {
		if seq1[i] != seq2[i] {
			t.Fatalf("mismatch at index %d: %d != %d", i, seq1[i], seq2[i])
		}
	}
}

func TestReplyCountTimeFactor(t *testing.T) {
	const trials = 20000
	const rate = 1.2
	const standard = 30

	// Low active (night lull or small turn out)
	rngLow := rand.New(rand.NewSource(333))
	sumLow := 0
	for i := 0; i < trials; i++ {
		sumLow += ReplyCountForWithStandard(rngLow, rate, 2, 5, standard, MaxThreadReplies)
	}
	meanLow := float64(sumLow) / float64(trials)

	// High active (teleho peak)
	rngHigh := rand.New(rand.NewSource(333))
	sumHigh := 0
	for i := 0; i < trials; i++ {
		sumHigh += ReplyCountForWithStandard(rngHigh, rate, 2, 60, standard, MaxThreadReplies)
	}
	meanHigh := float64(sumHigh) / float64(trials)

	if meanHigh <= meanLow {
		t.Fatalf("expected meanHigh (%.4f) > meanLow (%.4f)", meanHigh, meanLow)
	}
}
