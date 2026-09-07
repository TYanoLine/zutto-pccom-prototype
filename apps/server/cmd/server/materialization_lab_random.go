package main

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"zutto-pccom/apps/server/internal/world"
	"zutto-pccom/apps/server/internal/worldrepo"
)

// The random-order lab mirrors ALLBODY's access-order behavior while keeping
// every write inside an isolated MemoryStore clone. It exists to distinguish
// order/dependency effects from transient provider failures without mutating the
// canonical development world.
type materializationRandomLabState struct {
	mu     sync.Mutex
	jobs   map[string]*materializationRandomJob
	active string
	seq    uint64
}

var materializationRandomLab = materializationRandomLabState{jobs: map[string]*materializationRandomJob{}}

type materializationRandomJob struct {
	ID         string                       `json:"id"`
	Status     string                       `json:"status"`
	Phone      string                       `json:"phone"`
	Runs       int                          `json:"runs"`
	TimeoutMS  int                          `json:"timeout_ms"`
	SeedBase   int64                        `json:"seed_base"`
	CreatedAt  time.Time                    `json:"created_at"`
	StartedAt  time.Time                    `json:"started_at,omitempty"`
	FinishedAt time.Time                    `json:"finished_at,omitempty"`
	Progress   materializationLabProgress   `json:"progress"`
	RunOrders  []materializationRandomOrder `json:"run_orders,omitempty"`
	Results    []materializationRandomResult `json:"results,omitempty"`
	Summary    materializationRandomSummary `json:"summary"`
	Error      string                       `json:"error,omitempty"`
}

type materializationRandomOrder struct {
	Run   int     `json:"run"`
	Seed  int64   `json:"seed"`
	Order []int64 `json:"order"`
}

type materializationRandomResult struct {
	Run                  int                       `json:"run"`
	Sequence             int                       `json:"sequence"`
	Seed                 int64                     `json:"seed"`
	PostID               int64                     `json:"post_id"`
	BoardID              string                    `json:"board_id"`
	Author               string                    `json:"author"`
	Subject              string                    `json:"subject"`
	Action               string                    `json:"action"`
	CauseKind            string                    `json:"cause_kind"`
	TargetRenderedBefore bool                      `json:"target_rendered_before"`
	DurationMS           int64                     `json:"duration_ms"`
	Success              bool                      `json:"success"`
	Created              bool                      `json:"created"`
	BodyChars            int                       `json:"body_chars"`
	NewlyRenderedIDs     []int64                   `json:"newly_rendered_ids,omitempty"`
	ErrorClass           string                    `json:"error_class,omitempty"`
	Diagnostic           string                    `json:"diagnostic,omitempty"`
	Usage                worldrepo.GenerationUsage `json:"usage,omitempty"`
}

type materializationRandomSummary struct {
	Attempts               int     `json:"attempts"`
	Successes              int     `json:"successes"`
	Failures               int     `json:"failures"`
	Timeouts               int     `json:"timeouts"`
	DependencyFailures     int     `json:"dependency_failures"`
	EvidenceFailures       int     `json:"evidence_failures"`
	RendererFailures       int     `json:"renderer_failures"`
	OtherFailures          int     `json:"other_failures"`
	NoopAlreadyRendered    int     `json:"noop_already_rendered"`
	NewBodies              int     `json:"new_bodies"`
	MultiBodyDependencyCalls int   `json:"multi_body_dependency_calls"`
	SuccessRate            float64 `json:"success_rate"`
	MeanDurationMS         float64 `json:"mean_duration_ms"`
	P50DurationMS          int64   `json:"p50_duration_ms"`
	P95DurationMS          int64   `json:"p95_duration_ms"`
}

func (l *materializationLab) randomHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		if !labRequestAllowed(w, r) { return }
		switch strings.ToLower(strings.TrimSpace(r.URL.Query().Get("action"))) {
		case "start":
			l.handleRandomStart(w, r)
		case "status", "":
			handleRandomStatus(w, r)
		case "list":
			handleRandomList(w)
		default:
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "action must be start, status, or list"})
		}
	}
}

func (l *materializationLab) handleRandomStart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	phone := strings.TrimSpace(r.URL.Query().Get("phone"))
	if phone == "" {
		phone = developmentMaterializationPhone
	}
	runs := queryInt(r, "runs", 3)
	if runs < 1 || runs > 8 {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "runs must be between 1 and 8"})
		return
	}
	timeoutMS := queryInt(r, "timeout_ms", 35000)
	if timeoutMS < 5000 || timeoutMS > 120000 {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "timeout_ms must be between 5000 and 120000"})
		return
	}
	seedBase := int64(queryInt(r, "seed", 19660826))

	if !publicLabAdmission.start(w, r, phone, runs) { return }
	materializationRandomLab.mu.Lock()
	id := fmt.Sprintf("lab-random-%d-%04d", time.Now().UTC().Unix(), atomic.AddUint64(&materializationRandomLab.seq, 1)%10000)
	job := &materializationRandomJob{
		ID: id, Status: "queued", Phone: phone, Runs: runs, TimeoutMS: timeoutMS,
		SeedBase: seedBase, CreatedAt: time.Now().UTC(),
	}
	materializationRandomLab.jobs[id] = job
	materializationRandomLab.active = id
	materializationRandomLab.mu.Unlock()

	// Encode before the worker can mutate the queued job.
	_ = json.NewEncoder(w).Encode(job)
	go l.runRandomReplay(id)
}

func handleRandomStatus(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.URL.Query().Get("id"))
	materializationRandomLab.mu.Lock()
	defer materializationRandomLab.mu.Unlock()
	if id == "" {
		if materializationRandomLab.active != "" {
			_ = json.NewEncoder(w).Encode(materializationRandomLab.jobs[materializationRandomLab.active])
			return
		}
		var newest *materializationRandomJob
		for _, job := range materializationRandomLab.jobs {
			if newest == nil || job.CreatedAt.After(newest.CreatedAt) {
				newest = job
			}
		}
		if newest == nil {
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "idle"})
			return
		}
		_ = json.NewEncoder(w).Encode(newest)
		return
	}
	job := materializationRandomLab.jobs[id]
	if job == nil {
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "job not found"})
		return
	}
	_ = json.NewEncoder(w).Encode(job)
}

func handleRandomList(w http.ResponseWriter) {
	materializationRandomLab.mu.Lock()
	defer materializationRandomLab.mu.Unlock()
	jobs := make([]*materializationRandomJob, 0, len(materializationRandomLab.jobs))
	for _, job := range materializationRandomLab.jobs {
		jobs = append(jobs, job)
	}
	sort.SliceStable(jobs, func(i, j int) bool { return jobs[i].CreatedAt.After(jobs[j].CreatedAt) })
	if len(jobs) > 20 {
		jobs = jobs[:20]
	}
	_ = json.NewEncoder(w).Encode(map[string]any{"jobs": jobs, "active": materializationRandomLab.active})
}

func (l *materializationLab) runRandomReplay(id string) {
	defer publicLabAdmission.finish()
	materializationRandomLab.mu.Lock()
	job := materializationRandomLab.jobs[id]
	job.Status = "running"
	job.StartedAt = time.Now().UTC()
	materializationRandomLab.mu.Unlock()

	snapshot, selected, err := l.snapshotForWorkerReplay(job.Phone, nil)
	if err != nil {
		finishRandomError(id, err)
		return
	}
	baseOrder := allBodyBaseOrder(snapshot, selected)
	if len(baseOrder) != len(selected) {
		finishRandomError(id, fmt.Errorf("ALLBODY base order selected=%d ordered=%d", len(selected), len(baseOrder)))
		return
	}
	selectedIDs := map[int64]bool{}
	for _, p := range selected {
		selectedIDs[p.ID] = true
	}
	materializationRandomLab.mu.Lock()
	job.Progress.Total = len(baseOrder) * job.Runs
	materializationRandomLab.mu.Unlock()

	for run := 1; run <= job.Runs; run++ {
		seed := job.SeedBase + int64(run-1)
		order := shuffledPosts(baseOrder, seed)
		orderIDs := make([]int64, len(order))
		for i := range order {
			orderIDs[i] = order[i].ID
		}
		materializationRandomLab.mu.Lock()
		job.RunOrders = append(job.RunOrders, materializationRandomOrder{Run: run, Seed: seed, Order: orderIDs})
		materializationRandomLab.mu.Unlock()

		base := world.NewMemoryStore()
		copySnapshot := snapshot
		copySnapshot.Posts = clonePostsForLab(snapshot.Posts)
		for i := range copySnapshot.Posts {
			if selectedIDs[copySnapshot.Posts[i].ID] {
				copySnapshot.Posts[i].Body = ""
			}
		}
		if err := base.RestoreDevelopmentSnapshot(copySnapshot); err != nil {
			finishRandomError(id, fmt.Errorf("restore isolated snapshot: %w", err))
			return
		}
		repo := worldrepo.New(base, l.engine, l.materializer, l.worldDate)
		host, err := repo.HostByPhone(job.Phone)
		if err != nil {
			finishRandomError(id, fmt.Errorf("isolated host lookup: %w", err))
			return
		}
		boards := map[string]world.Board{}
		for _, board := range base.ListBoards(host.ID) {
			boards[board.ID] = board
		}

		for sequence, target := range order {
			board, ok := boards[target.BoardID]
			if !ok {
				appendRandomResult(id, materializationRandomResult{
					Run: run, Sequence: sequence + 1, Seed: seed, PostID: target.ID, BoardID: target.BoardID,
					Author: target.Author, Subject: target.Subject, Action: target.Intent.Action, CauseKind: target.Intent.CauseKind,
					ErrorClass: "board_missing", Diagnostic: "board not found in isolated snapshot",
				})
				continue
			}
			before := renderedSelectedIDs(base.ListPosts(host.ID), selectedIDs)
			targetRenderedBefore := before[target.ID]
			started := time.Now()
			post, found, created, diagnostic := repo.MaterializationArticleWithDebugTimeout(host, board, target.ID, time.Duration(job.TimeoutMS)*time.Millisecond)
			duration := time.Since(started)
			after := renderedSelectedIDs(base.ListPosts(host.ID), selectedIDs)
			newIDs := newlyRenderedIDs(before, after)
			usage, _ := repo.MaterializationGenerationUsage(target.ID)
			success := found && strings.TrimSpace(post.Body) != "" && !strings.Contains(diagnostic, "error stage=")
			result := materializationRandomResult{
				Run: run, Sequence: sequence + 1, Seed: seed, PostID: target.ID, BoardID: target.BoardID,
				Author: target.Author, Subject: target.Subject, Action: target.Intent.Action, CauseKind: target.Intent.CauseKind,
				TargetRenderedBefore: targetRenderedBefore, DurationMS: duration.Milliseconds(), Success: success,
				Created: created, BodyChars: len([]rune(post.Body)), NewlyRenderedIDs: newIDs,
				Diagnostic: diagnostic, Usage: usage,
			}
			if !success {
				result.ErrorClass = classifyLabDiagnostic(diagnostic)
			}
			appendRandomResult(id, result)
		}
	}
	finishRandomSuccess(id)
}

func allBodyBaseOrder(snapshot world.DevelopmentHostSnapshot, selected []world.Post) []world.Post {
	wanted := map[int64]world.Post{}
	for _, p := range selected {
		wanted[p.ID] = p
	}
	out := make([]world.Post, 0, len(selected))
	for _, board := range snapshot.Boards {
		for _, p := range snapshot.Posts {
			if p.BoardID != board.ID {
				continue
			}
			if selectedPost, ok := wanted[p.ID]; ok {
				out = append(out, selectedPost)
			}
		}
	}
	return out
}

func shuffledPosts(in []world.Post, seed int64) []world.Post {
	out := append([]world.Post(nil), in...)
	rng := rand.New(rand.NewSource(seed))
	rng.Shuffle(len(out), func(i, j int) { out[i], out[j] = out[j], out[i] })
	return out
}

func renderedSelectedIDs(posts []world.Post, selected map[int64]bool) map[int64]bool {
	out := map[int64]bool{}
	for _, p := range posts {
		if selected[p.ID] && strings.TrimSpace(p.Body) != "" {
			out[p.ID] = true
		}
	}
	return out
}

func newlyRenderedIDs(before, after map[int64]bool) []int64 {
	out := make([]int64, 0)
	for id := range after {
		if !before[id] {
			out = append(out, id)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

func appendRandomResult(id string, result materializationRandomResult) {
	materializationRandomLab.mu.Lock()
	defer materializationRandomLab.mu.Unlock()
	job := materializationRandomLab.jobs[id]
	if job == nil {
		return
	}
	job.Results = append(job.Results, result)
	job.Progress.Completed++
}

func finishRandomError(id string, err error) {
	materializationRandomLab.mu.Lock()
	defer materializationRandomLab.mu.Unlock()
	job := materializationRandomLab.jobs[id]
	if job == nil {
		return
	}
	job.Status = "failed"
	job.Error = err.Error()
	job.FinishedAt = time.Now().UTC()
	if materializationRandomLab.active == id {
		materializationRandomLab.active = ""
	}
}

func finishRandomSuccess(id string) {
	materializationRandomLab.mu.Lock()
	defer materializationRandomLab.mu.Unlock()
	job := materializationRandomLab.jobs[id]
	if job == nil {
		return
	}
	job.Summary = summarizeRandomResults(job.Results)
	job.Status = "completed"
	job.FinishedAt = time.Now().UTC()
	if materializationRandomLab.active == id {
		materializationRandomLab.active = ""
	}
}

func summarizeRandomResults(results []materializationRandomResult) materializationRandomSummary {
	var summary materializationRandomSummary
	durations := make([]int64, 0, len(results))
	var durationTotal int64
	for _, result := range results {
		summary.Attempts++
		durations = append(durations, result.DurationMS)
		durationTotal += result.DurationMS
		if result.TargetRenderedBefore {
			summary.NoopAlreadyRendered++
		}
		summary.NewBodies += len(result.NewlyRenderedIDs)
		if len(result.NewlyRenderedIDs) > 1 {
			summary.MultiBodyDependencyCalls++
		}
		if result.Success {
			summary.Successes++
			continue
		}
		summary.Failures++
		switch result.ErrorClass {
		case "timeout":
			summary.Timeouts++
		case "dependency":
			summary.DependencyFailures++
		case "evidence":
			summary.EvidenceFailures++
		case "renderer":
			summary.RendererFailures++
		default:
			summary.OtherFailures++
		}
	}
	if summary.Attempts > 0 {
		summary.SuccessRate = float64(summary.Successes) / float64(summary.Attempts)
		summary.MeanDurationMS = float64(durationTotal) / float64(summary.Attempts)
	}
	if len(durations) > 0 {
		sort.Slice(durations, func(i, j int) bool { return durations[i] < durations[j] })
		summary.P50DurationMS = durations[(len(durations)-1)/2]
		p95 := (len(durations)*95 + 99) / 100
		if p95 < 1 {
			p95 = 1
		}
		if p95 > len(durations) {
			p95 = len(durations)
		}
		summary.P95DurationMS = durations[p95-1]
	}
	return summary
}
