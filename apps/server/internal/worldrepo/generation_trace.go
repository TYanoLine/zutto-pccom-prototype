package worldrepo

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"zutto-pccom/apps/server/internal/llm"
	"zutto-pccom/apps/server/internal/world"
)

// These traces are bounded, process-local diagnostic data. They never enter
// canonical world state, a prompt, or the operational Render content log.
const (
	generationTraceMaxRuns = 12
	generationTraceMaxSteps = 30
	generationTraceMaxPromptRunes = 24000
	generationTraceMaxResultRunes = 36000
)

type GenerationTraceStep struct {
	Stage string `json:"stage"`
	Status string `json:"status"`
	StartedAt time.Time `json:"started_at"`
	FinishedAt *time.Time `json:"finished_at,omitempty"`
	Prompt string `json:"prompt"`
	Result string `json:"result,omitempty"`
	Error string `json:"error,omitempty"`
}

type GenerationTraceRun struct {
	ID string `json:"id"`
	Host string `json:"host"`
	Board string `json:"board"`
	Kind string `json:"kind"`
	PostID int64 `json:"post_id,omitempty"`
	Status string `json:"status"`
	StartedAt time.Time `json:"started_at"`
	FinishedAt *time.Time `json:"finished_at,omitempty"`
	Error string `json:"error,omitempty"`
	Steps []GenerationTraceStep `json:"steps"`
}

type GenerationTraceSnapshot struct {
	Runs []GenerationTraceRun `json:"runs"`
	Running bool `json:"running"`
}

// A mutex guards both concurrent board jobs and polling HTTP readers. The
// snapshot returns deep copies to prevent races or accidental mutation.
type generationTraceStore struct {
	mu sync.Mutex
	next uint64
	runs []*GenerationTraceRun
}

func boundedTraceText(value string, max int) string {
	chars := []rune(value)
	if len(chars) <= max { return value }
	return string(chars[:max]) + "\n[diagnostic trace truncated]"
}

func (r *Repository) SetGenerationTraceEnabled(enabled bool) {
	if r == nil { return }
	if enabled {
		r.generationTrace = &generationTraceStore{}
	} else {
		r.generationTrace = nil
	}
}

func (r *Repository) GenerationTraceSnapshot() GenerationTraceSnapshot {
	snapshot := GenerationTraceSnapshot{Runs: []GenerationTraceRun{}}
	if r == nil || r.generationTrace == nil { return snapshot }
	store := r.generationTrace
	store.mu.Lock()
	defer store.mu.Unlock()
	for i := len(store.runs)-1; i >= 0; i-- {
		run := *store.runs[i]
		run.Steps = append([]GenerationTraceStep(nil), run.Steps...)
		if run.Status == "running" { snapshot.Running = true }
		snapshot.Runs = append(snapshot.Runs, run)
	}
	return snapshot
}

func (r *Repository) beginGenerationTrace(ctx context.Context, host world.Host, board world.Board, kind string, postID int64) (context.Context, func(error)) {
	if r == nil || r.generationTrace == nil || host.ID != hakataGeneratedContentHostID {
		return ctx, func(error) {}
	}
	store := r.generationTrace
	store.mu.Lock()
	store.next++
	run := &GenerationTraceRun{
		ID: fmt.Sprintf("%d-%d", time.Now().UnixMilli(), store.next),
		Host: host.ID, Board: board.ID, Kind: kind, PostID: postID,
		Status: "running", StartedAt: time.Now(), Steps: []GenerationTraceStep{},
	}
	store.runs = append(store.runs, run)
	if len(store.runs) > generationTraceMaxRuns {
		store.runs = append([]*GenerationTraceRun(nil), store.runs[len(store.runs)-generationTraceMaxRuns:]...)
	}
	store.mu.Unlock()

	start := func(stage, prompt string) func(string, error) {
		store.mu.Lock()
		if len(run.Steps) >= generationTraceMaxSteps {
			store.mu.Unlock()
			return func(string, error) {}
		}
		index := len(run.Steps)
		run.Steps = append(run.Steps, GenerationTraceStep{
			Stage: stage, Status: "running", StartedAt: time.Now(),
			Prompt: boundedTraceText(prompt, generationTraceMaxPromptRunes),
		})
		store.mu.Unlock()
		return func(response string, failure error) {
			store.mu.Lock()
			defer store.mu.Unlock()
			step := &run.Steps[index]
			now := time.Now()
			step.FinishedAt = &now
			step.Result = boundedTraceText(response, generationTraceMaxResultRunes)
			if failure != nil {
				step.Status = "failed"
				step.Error = boundedTraceText(failure.Error(), 3000)
			} else {
				step.Status = "completed"
			}
		}
	}

	end := func(failure error) {
		store.mu.Lock()
		defer store.mu.Unlock()
		now := time.Now()
		run.FinishedAt = &now
		run.Status = "completed"
		if failure != nil {
			run.Status = "failed"
			run.Error = boundedTraceText(strings.TrimSpace(failure.Error()), 3000)
		}
	}
	return llm.WithDebugTrace(ctx, start), end
}
