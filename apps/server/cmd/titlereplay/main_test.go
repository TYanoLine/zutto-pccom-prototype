package main

import (
	"context"
	"testing"
)

func TestReplayPlumbing(t *testing.T) {
	rows, err := readRows("../../../../specs/009-title-voice/baseline/root-titles-2026-10-02_06.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 179 {
		t.Fatalf("rows = %d", len(rows))
	}
	out, err := replay(context.Background(), fakePlanner{}, rows, 1, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 179 {
		t.Fatalf("replayed %d rows, want 179", len(out))
	}
	for _, r := range out {
		if r.Run != 1 || r.Subject == "" || r.Board == "" {
			t.Fatalf("bad row %+v", r)
		}
	}
}
