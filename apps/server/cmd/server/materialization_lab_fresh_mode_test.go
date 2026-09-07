package main

import "testing"

func TestNormalizeFreshSituationMode(t *testing.T) {
	tests := []struct {
		in, want string
		ok       bool
	}{
		{"", "facets", true},
		{"facets", "facets", true},
		{" FACETLESS ", "facetless", true},
		{"other", "", false},
	}
	for _, tt := range tests {
		got, ok := normalizeFreshSituationMode(tt.in)
		if got != tt.want || ok != tt.ok {
			t.Fatalf("normalizeFreshSituationMode(%q)=(%q,%v), want (%q,%v)", tt.in, got, ok, tt.want, tt.ok)
		}
	}
}
