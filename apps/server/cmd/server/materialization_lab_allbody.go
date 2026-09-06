package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"zutto-pccom/apps/server/internal/hostprogram/materializationdemo"
	"zutto-pccom/apps/server/internal/world"
	"zutto-pccom/apps/server/internal/worldrepo"
)

// materializationAllBodyLab runs the actual materializationdemo Runtime ALLBODY
// wrapper against an isolated MemoryStore clone. This is intentionally one step
// above the repo-level random replay: it exercises the same envelope collection,
// background job, random target ordering, dependency rendering and failure
// accounting used by the terminal command without writing canonical state.
type materializationAllBodyLabState struct {
	mu     sync.Mutex
	jobs   map[string]*materializationAllBodyJob
	active string
	seq    uint64
}

var materializationAllBodyLab = materializationAllBodyLabState{jobs: map[string]*materializationAllBodyJob{}}

type materializationAllBodyJob struct {
	ID         string                         `json:"id"`
	Status     string                         `json:"status"`
	Phone      string                         `json:"phone"`
	Runs       int                            `json:"runs"`
	CreatedAt  time.Time                      `json:"created_at"`
	StartedAt  time.Time                      `json:"started_at,omitempty"`
	FinishedAt time.Time                      `json:"finished_at,omitempty"`
	Progress   materializationLabProgress     `json:"progress"`
	Results    []materializationAllBodyResult `json:"results,omitempty"`
	Summary    materializationAllBodySummary  `json:"summary"`
	Error      string                         `json:"error,omitempty"`
}

type materializationAllBodyResult struct {
	Run            int     `json:"run"`
	DurationMS     int64   `json:"duration_ms"`
	ExpectedBodies int     `json:"expected_bodies"`
	FinalBodies    int     `json:"final_bodies"`
	Complete       bool    `json:"complete"`
	RuntimeState   string  `json:"runtime_state"`
	Failures       int     `json:"failures"`
	StatusText     string  `json:"status_text"`
	MissingPostIDs []int64 `json:"missing_post_ids,omitempty"`
}

type materializationAllBodySummary struct {
	Runs          int     `json:"runs"`
	CompleteRuns  int     `json:"complete_runs"`
	FailedRuns    int     `json:"failed_runs"`
	TotalMissing  int     `json:"total_missing_bodies"`
	SuccessRate   float64 `json:"success_rate"`
	MeanDurationMS float64 `json:"mean_duration_ms"`
}

func (l *materializationLab) allBodyHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		if !l.authorized(r) {
			w.WriteHeader(http.StatusForbidden)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "materialization lab is disabled or unauthorized"})
			return
		}
		switch strings.ToLower(strings.TrimSpace(r.URL.Query().Get("action"))) {
		case "start":
			l.handleAllBodyStart(w, r)
		case "status", "":
			handleAllBodyStatus(w, r)
		case "list":
			handleAllBodyList(w)
		default:
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "action must be start, status, or list"})
		}
	}
}

func (l *materializationLab) handleAllBodyStart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	phone := strings.TrimSpace(r.URL.Query().Get("phone"))
	if phone == "" {
		phone = developmentMaterializationPhone
	}
	runs := queryInt(r, "runs", 3)
	if runs < 1 || runs > 5 {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "runs must be between 1 and 5"})
		return
	}

	l.mu.Lock()
	regularActive := l.active
	l.mu.Unlock()
	materializationRandomLab.mu.Lock()
	randomActive := materializationRandomLab.active
	materializationRandomLab.mu.Unlock()
	materializationAllBodyLab.mu.Lock()
	if regularActive != "" || randomActive != "" || materializationAllBodyLab.active != "" {
		active := materializationAllBodyLab.active
		materializationAllBodyLab.mu.Unlock()
		w.WriteHeader(http.StatusConflict)
		_ = json.NewEncoder(w).Encode(map[string]any{"error": "another materialization lab job is active", "regular_active": regularActive, "random_active": randomActive, "allbody_active": active})
		return
	}
	id := fmt.Sprintf("lab-allbody-%d-%04d", time.Now().UTC().Unix(), atomic.AddUint64(&materializationAllBodyLab.seq, 1)%10000)
	job := &materializationAllBodyJob{ID: id, Status: "queued", Phone: phone, Runs: runs, CreatedAt: time.Now().UTC()}
	materializationAllBodyLab.jobs[id] = job
	materializationAllBodyLab.active = id
	materializationAllBodyLab.mu.Unlock()

	go l.runAllBodyReplay(id)
	_ = json.NewEncoder(w).Encode(job)
}

func handleAllBodyStatus(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.URL.Query().Get("id"))
	materializationAllBodyLab.mu.Lock()
	defer materializationAllBodyLab.mu.Unlock()
	if id == "" {
		if materializationAllBodyLab.active != "" {
			_ = json.NewEncoder(w).Encode(materializationAllBodyLab.jobs[materializationAllBodyLab.active])
			return
		}
		var newest *materializationAllBodyJob
		for _, job := range materializationAllBodyLab.jobs {
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
	job := materializationAllBodyLab.jobs[id]
	if job == nil {
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "job not found"})
		return
	}
	_ = json.NewEncoder(w).Encode(job)
}

func handleAllBodyList(w http.ResponseWriter) {
	materializationAllBodyLab.mu.Lock()
	defer materializationAllBodyLab.mu.Unlock()
	jobs := make([]*materializationAllBodyJob, 0, len(materializationAllBodyLab.jobs))
	for _, job := range materializationAllBodyLab.jobs { jobs = append(jobs, job) }
	sort.SliceStable(jobs, func(i, j int) bool { return jobs[i].CreatedAt.After(jobs[j].CreatedAt) })
	if len(jobs) > 20 { jobs = jobs[:20] }
	_ = json.NewEncoder(w).Encode(map[string]any{"jobs": jobs, "active": materializationAllBodyLab.active})
}

func (l *materializationLab) runAllBodyReplay(id string) {
	materializationAllBodyLab.mu.Lock()
	job := materializationAllBodyLab.jobs[id]
	job.Status = "running"
	job.StartedAt = time.Now().UTC()
	materializationAllBodyLab.mu.Unlock()

	snapshot, selected, err := l.snapshotForWorkerReplay(job.Phone, nil)
	if err != nil { finishAllBodyError(id, err); return }
	selectedIDs := map[int64]bool{}
	for _, p := range selected { selectedIDs[p.ID] = true }
	materializationAllBodyLab.mu.Lock()
	job.Progress.Total = job.Runs
	materializationAllBodyLab.mu.Unlock()

	for run := 1; run <= job.Runs; run++ {
		base := world.NewMemoryStore()
		copySnapshot := snapshot
		copySnapshot.Posts = clonePostsForLab(snapshot.Posts)
		for i := range copySnapshot.Posts {
			if selectedIDs[copySnapshot.Posts[i].ID] { copySnapshot.Posts[i].Body = "" }
		}
		if err := base.RestoreDevelopmentSnapshot(copySnapshot); err != nil { finishAllBodyError(id, fmt.Errorf("restore isolated snapshot: %w", err)); return }
		repo := worldrepo.New(base, l.engine, l.materializer, l.worldDate)
		host, err := repo.HostByPhone(job.Phone)
		if err != nil { finishAllBodyError(id, fmt.Errorf("isolated host lookup: %w", err)); return }
		runtime := materializationdemo.New(host, repo)
		started := time.Now()
		_, _ = runtime.HandleLine("ALLBODY")

		var statusText string
		deadline := time.Now().Add(8 * time.Minute)
		for {
			statusText, _ = runtime.HandleLine("STATUS")
			if terminalBulkStatus(statusText) { break }
			if time.Now().After(deadline) {
				statusText += "\n[LAB] timeout waiting for ALLBODY terminal state"
				break
			}
			time.Sleep(150 * time.Millisecond)
		}
		duration := time.Since(started)
		finalPosts := base.ListPosts(host.ID)
		finalBodies := 0
		missing := make([]int64, 0)
		for _, p := range finalPosts {
			if !selectedIDs[p.ID] { continue }
			if strings.TrimSpace(p.Body) != "" { finalBodies++ } else { missing = append(missing, p.ID) }
		}
		sort.Slice(missing, func(i, j int) bool { return missing[i] < missing[j] })
		state := parseBulkState(statusText)
		failureCount := parseBulkFailureCount(statusText)
		result := materializationAllBodyResult{
			Run: run, DurationMS: duration.Milliseconds(), ExpectedBodies: len(selected), FinalBodies: finalBodies,
			Complete: state == "COMPLETED" && len(missing) == 0 && failureCount == 0,
			RuntimeState: state, Failures: failureCount, StatusText: compactLabStatus(statusText), MissingPostIDs: missing,
		}
		materializationAllBodyLab.mu.Lock()
		job.Results = append(job.Results, result)
		job.Progress.Completed++
		materializationAllBodyLab.mu.Unlock()
	}
	finishAllBodySuccess(id)
}

func terminalBulkStatus(s string) bool {
	return strings.Contains(s, "BULK STATUS    : COMPLETED") || strings.Contains(s, "BULK STATUS    : FAILED") || strings.Contains(s, "BULK STATUS    : CANCELLED")
}

func parseBulkState(s string) string {
	for _, state := range []string{"COMPLETED", "FAILED", "CANCELLED", "CANCELLING", "BODIES", "ENVELOPES", "STARTING"} {
		if strings.Contains(s, "BULK STATUS    : "+state) { return state }
	}
	return "UNKNOWN"
}

func parseBulkFailureCount(s string) int {
	marker := "[DEV] FAILURES       : "
	idx := strings.Index(s, marker)
	if idx < 0 { return 0 }
	var n int
	_, _ = fmt.Sscanf(s[idx+len(marker):], "%d", &n)
	return n
}

func compactLabStatus(s string) string {
	s = strings.ReplaceAll(s, "\r", "")
	lines := strings.Split(s, "\n")
	out := make([]string, 0, 12)
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || line == "DEV>" { continue }
		if strings.Contains(line, "BULK STATUS") || strings.Contains(line, "BOARDS") || strings.Contains(line, "BODIES") || strings.Contains(line, "REQUESTS") || strings.Contains(line, "FAILURES") || strings.Contains(line, "EMPTY") || strings.Contains(line, "CURRENT") {
			out = append(out, line)
		}
	}
	return strings.Join(out, " | ")
}

func finishAllBodyError(id string, err error) {
	materializationAllBodyLab.mu.Lock()
	defer materializationAllBodyLab.mu.Unlock()
	job := materializationAllBodyLab.jobs[id]
	if job == nil { return }
	job.Status = "failed"
	job.Error = err.Error()
	job.FinishedAt = time.Now().UTC()
	if materializationAllBodyLab.active == id { materializationAllBodyLab.active = "" }
}

func finishAllBodySuccess(id string) {
	materializationAllBodyLab.mu.Lock()
	defer materializationAllBodyLab.mu.Unlock()
	job := materializationAllBodyLab.jobs[id]
	if job == nil { return }
	var totalDuration int64
	for _, result := range job.Results {
		totalDuration += result.DurationMS
		job.Summary.Runs++
		job.Summary.TotalMissing += len(result.MissingPostIDs)
		if result.Complete { job.Summary.CompleteRuns++ } else { job.Summary.FailedRuns++ }
	}
	if job.Summary.Runs > 0 {
		job.Summary.SuccessRate = float64(job.Summary.CompleteRuns) / float64(job.Summary.Runs)
		job.Summary.MeanDurationMS = float64(totalDuration) / float64(job.Summary.Runs)
	}
	job.Status = "completed"
	job.FinishedAt = time.Now().UTC()
	if materializationAllBodyLab.active == id { materializationAllBodyLab.active = "" }
}
