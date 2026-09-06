package worldrepo

import "testing"

func TestLimitDevelopmentShellsForProducerKeepsChronologicalPrefixAndStats(t *testing.T) {
	shells := []developmentTimelineShell{
		{index: 1, action: "thread_start"},
		{index: 2, action: "reply", parentIndex: 1, sourceIndex: 1},
		{index: 3, action: "thread_start"},
		{index: 4, action: "reply", parentIndex: 3, sourceIndex: 3},
		{index: 5, action: "reply", parentIndex: 3, sourceIndex: 4},
		{index: 6, action: "thread_start"},
		{index: 7, action: "reply", parentIndex: 6, sourceIndex: 6},
	}
	stats := developmentSelectionStats{Visits: 20, Posts: len(shells), ROM: 13, Roots: 3, Replies: 4}

	limited, got := limitDevelopmentShellsForProducer(shells, stats)
	if len(limited) != developmentWorldWindowPoCMaxShellsPerBoard {
		t.Fatalf("limited shells=%d want=%d", len(limited), developmentWorldWindowPoCMaxShellsPerBoard)
	}
	for i, shell := range limited {
		if shell.index != i+1 {
			t.Fatalf("shell %d index=%d want=%d", i, shell.index, i+1)
		}
	}
	if got.Posts != 5 || got.Roots != 2 || got.Replies != 3 || got.ROM != 15 {
		t.Fatalf("unexpected limited stats: %+v", got)
	}
}

func TestLimitDevelopmentShellsForProducerLeavesSmallSetUntouched(t *testing.T) {
	shells := []developmentTimelineShell{{index: 1, action: "thread_start"}, {index: 2, action: "reply"}}
	stats := developmentSelectionStats{Visits: 4, Posts: 2, ROM: 2, Roots: 1, Replies: 1}

	limited, got := limitDevelopmentShellsForProducer(shells, stats)
	if len(limited) != 2 || limited[0].index != 1 || limited[1].index != 2 {
		t.Fatalf("small shell set changed: %+v", limited)
	}
	if got != stats {
		t.Fatalf("small-set stats changed: got=%+v want=%+v", got, stats)
	}
}
