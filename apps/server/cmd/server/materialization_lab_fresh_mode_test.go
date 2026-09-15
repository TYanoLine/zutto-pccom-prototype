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
		{" batch ", "batch", true},
		{" TOPIC-FIRST ", "topic-first", true},
		{" TITLE-FIRST ", "title-first", true},
		{"other", "", false},
	}
	for _, tt := range tests {
		got, ok := normalizeFreshSituationMode(tt.in)
		if got != tt.want || ok != tt.ok {
			t.Fatalf("normalizeFreshSituationMode(%q)=(%q,%v), want (%q,%v)", tt.in, got, ok, tt.want, tt.ok)
		}
	}
}

func TestFreshIntParamAndScaleBoards(t *testing.T) {
	if got, ok := freshIntParam("", 5, 1, 10); !ok || got != 5 {
		t.Fatalf("default=(%d,%v)", got, ok)
	}
	if _, ok := freshIntParam("11", 5, 1, 10); ok {
		t.Fatal("out-of-range value should fail")
	}
	boards := freshScaleBoards(6)
	if len(boards) != 6 || boards[3].Name != "ゲーム" || boards[5].Name != "ソフトウェア" {
		t.Fatalf("unexpected boards: %#v", boards)
	}
}
