package worldcatalog

import (
	"fmt"
	"testing"

	"zutto-pccom/apps/server/internal/hostcatalog"
)

func TestGeneratedCentersSatisfyDescriptorInvariants(t *testing.T) {
	for seed := int64(1); seed <= 20; seed++ {
		for generation := 0; generation < 3; generation++ {
			for i := 0; i < generatedCountForTests; i++ {
				c := makeCenter(fmt.Sprintf("HOST %d", i), seed*7919, i, generation)
				if err := c.Descriptor().Validate(hostcatalog.Options{}); err != nil {
					t.Fatalf("seed=%d index=%d generation=%d: %v\n%+v", seed, i, generation, err, c)
				}
			}
		}
	}
}

func TestCenterDescriptorRoundTrip(t *testing.T) {
	c := makeCenter("ROUND TRIP BBS", 123456789, 12, 1)
	if got := CenterFromDescriptor(c.Descriptor()); got != c {
		t.Fatalf("round trip changed center:\n got  %+v\n want %+v", got, c)
	}
	d := c.Descriptor()
	if d.Origin != hostcatalog.OriginGenerated || !d.Listed || d.Key != c.ID {
		t.Fatalf("unexpected descriptor identity: %+v", d)
	}
}

// Generated numbers must never collide with preset numbers; the unique index on
// (world_id, phone_number) would otherwise reject the world at creation time.
func TestGeneratedPhonesAreUniqueAndAvoidPresets(t *testing.T) {
	presets, err := hostcatalog.LoadPresets(hostcatalog.Options{})
	if err != nil {
		t.Fatal(err)
	}
	reserved := map[string]string{}
	for _, p := range presets {
		if p.Phone != "" {
			reserved[p.Phone] = p.Key
		}
	}
	for seed := int64(1); seed <= 20; seed++ {
		names := make([]string, generatedCountForTests)
		for i := range names {
			names[i] = fmt.Sprintf("HOST %d", i)
		}
		seen := map[string]string{}
		for _, c := range makeCenters(names, seed*104729) {
			if key, clash := reserved[c.Phone]; clash {
				t.Fatalf("seed=%d: generated %s collides with preset %s (%s)", seed, c.ID, key, c.Phone)
			}
			if prev, dup := seen[c.Phone]; dup {
				t.Fatalf("seed=%d: %s and %s share %s", seed, prev, c.ID, c.Phone)
			}
			seen[c.Phone] = c.ID
		}
	}
}

// Mirrors generatedCenterCount in cmd/server (which lives in package main).
const generatedCountForTests = 100
