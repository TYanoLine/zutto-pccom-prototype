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
		{name: "blend", base: .20, advised: .80, want: .41},
		{name: "high advice clamp", base: .20, advised: 2, want: .48},
		{name: "low advice clamp", base: .20, advised: -1, want: .13},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := demoBlendWriteProbability(tc.base, tc.advised); math.Abs(got-tc.want) > 1e-9 {
				t.Fatalf("blend=%v, want %v", got, tc.want)
			}
		})
	}
}
