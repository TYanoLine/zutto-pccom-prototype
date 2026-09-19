package worldrepo

import (
	"math"
	"testing"
)

func TestDemoBlendWriteProbabilityIsConservative(t *testing.T) {
	for _, tc := range []struct {
		name    string
		base    float64
		advised float64
		want    float64
	}{
		{name: "blend", base: .20, advised: .80, want: .44},
		{name: "high advice clamp", base: .20, advised: 2, want: .52},
		{name: "low advice clamp", base: .20, advised: -1, want: .12},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := demoBlendWriteProbability(tc.base, tc.advised); math.Abs(got-tc.want) > 1e-9 {
				t.Fatalf("blend=%v, want %v", got, tc.want)
			}
		})
	}
}

func TestDemoBehaviorBlendsKeepLocalModelInControl(t *testing.T) {
	if got := demoBlendActivityProbability(.20, .80); math.Abs(got-.38) > 1e-9 {
		t.Fatalf("activity blend=%v, want .38", got)
	}
	if got := demoBlendReplyProbability(.20, .80); math.Abs(got-.44) > 1e-9 {
		t.Fatalf("reply blend=%v, want .44", got)
	}
}

func TestStabilizeJevProbabilityBucketsProviderJitter(t *testing.T) {
	for _, tc := range []struct {
		in   float64
		want float64
	}{
		{in: .39, want: .40},
		{in: .40, want: .40},
		{in: .41, want: .40},
		{in: .64, want: .65},
		{in: -.20, want: 0},
		{in: 1.20, want: 1},
	} {
		if got := stabilizeJevProbability(tc.in); math.Abs(got-tc.want) > 1e-9 {
			t.Fatalf("stabilize(%v)=%v, want %v", tc.in, got, tc.want)
		}
	}
}
