package main

import "testing"

func TestNormalizeFreshHistoricalTexture(t *testing.T) {
	cases := []struct {
		in   string
		want string
		ok   bool
	}{
		{"", "sourced", true},
		{"sourced", "sourced", true},
		{"off", "off", true},
		{"model-memory", "model-memory", true},
		{"model-memory-concrete", "model-memory-concrete", true},
		{"1996-08-curated", "1996-08-curated", true},
		{"1996-09", "", false},
	}
	for _, tc := range cases {
		got, ok := normalizeFreshHistoricalTexture(tc.in)
		if got != tc.want || ok != tc.ok {
			t.Fatalf("normalizeFreshHistoricalTexture(%q)=(%q,%v), want (%q,%v)", tc.in, got, ok, tc.want, tc.ok)
		}
	}
}

func TestFreshHistoricalTextureFactsAreBounded(t *testing.T) {
	if got := freshHistoricalTextureFacts("off"); got != nil {
		t.Fatalf("off texture = %#v, want nil", got)
	}
	facts := freshHistoricalTextureFacts("1996-08-curated")
	if len(facts) < 8 || len(facts) > 16 {
		t.Fatalf("curated facts count=%d, want bounded small set", len(facts))
	}
}
