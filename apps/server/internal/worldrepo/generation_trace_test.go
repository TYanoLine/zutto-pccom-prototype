package worldrepo

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"

	"zutto-pccom/apps/server/internal/world"
)

func TestGenerationTraceRequiresOptInAndHAKATA(t *testing.T) {
	r := &Repository{}
	hakata := world.Host{ID: hakataGeneratedContentHostID}
	board := world.Board{ID: "20/1"}
	_, finish := r.beginGenerationTrace(context.Background(), hakata, board, "headers", 0)
	finish(nil)
	if len(r.GenerationTraceSnapshot().Runs) != 0 { t.Fatal("trace recorded without enablement") }

	r.SetGenerationTraceEnabled(true)
	_, finish = r.beginGenerationTrace(context.Background(), world.Host{ID:"another-host"}, board, "headers", 0)
	finish(nil)
	if len(r.GenerationTraceSnapshot().Runs) != 0 { t.Fatal("trace leaked from other host") }

	_, finish = r.beginGenerationTrace(context.Background(), hakata, board, "headers", 0)
	active := r.GenerationTraceSnapshot()
	if !active.Running || len(active.Runs) != 1 || active.Runs[0].Status != "running" {
		t.Fatalf("running trace not visible: %+v", active)
	}
	finish(errors.New("invalid structured situation"))
	failed := r.GenerationTraceSnapshot()
	if failed.Running || failed.Runs[0].Status != "failed" || !strings.Contains(failed.Runs[0].Error, "structured") {
		t.Fatalf("failed trace not visible: %+v", failed)
	}

	r.SetGenerationTraceEnabled(false)
	if len(r.GenerationTraceSnapshot().Runs) != 0 { t.Fatal("disabling trace did not clear records") }
}

func TestGenerationTraceConcurrentBoundedSnapshots(t *testing.T) {
	r := &Repository{}
	r.SetGenerationTraceEnabled(true)
	host := world.Host{ID: hakataGeneratedContentHostID}
	board := world.Board{ID:"70/1"}
	var wg sync.WaitGroup
	for i := 0; i < generationTraceMaxRuns+7; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, finish := r.beginGenerationTrace(context.Background(), host, board, "headers", 0)
			_ = r.GenerationTraceSnapshot()
			finish(nil)
		}()
	}
	wg.Wait()
	snapshot := r.GenerationTraceSnapshot()
	if len(snapshot.Runs) != generationTraceMaxRuns || snapshot.Running {
		t.Fatalf("bounded trace snapshot: %+v", snapshot)
	}
	for _, run := range snapshot.Runs {
		if run.Status != "completed" || run.Board != "70/1" { t.Fatalf("unexpected trace: %+v", run) }
	}
}

func TestGenerationTraceTextBounds(t *testing.T) {
	bounded := boundedTraceText(strings.Repeat("あ", 21), 10)
	if len([]rune(bounded)) <= 10 || !strings.Contains(bounded, "truncated") {
		t.Fatalf("expected explicit truncation indicator: %q", bounded)
	}
}
