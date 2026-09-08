package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"zutto-pccom/apps/server/internal/world"
	"zutto-pccom/apps/server/internal/worldengine"
	"zutto-pccom/apps/server/internal/worldrepo"
)

// materializationLab is a development-only, non-canonical experiment harness.
// It clones the current development host into an isolated MemoryStore and runs
// the real materialization pipeline against the clone. OpenAI credentials stay
// server-side on Render and no experiment write can reach the canonical store.
type materializationLab struct {
	store        debugExportStore
	engine       worldrepo.EvidenceResolver
	materializer worldrepo.Materializer
	worldDate    string
	token        string
	freshArchive materializationFreshArchiveStore

	mu     sync.Mutex
	jobs   map[string]*materializationLabJob
	active string
	seq    uint64
}

type materializationLabJob struct {
	ID          string                           `json:"id"`
	Suite       string                           `json:"suite"`
	Status      string                           `json:"status"`
	Phone       string                           `json:"phone"`
	Runs        int                              `json:"runs"`
	TimeoutMS   int                              `json:"timeout_ms"`
	PostIDs     []int64                          `json:"post_ids,omitempty"`
	CreatedAt   time.Time                        `json:"created_at"`
	StartedAt   time.Time                        `json:"started_at,omitempty"`
	FinishedAt  time.Time                        `json:"finished_at,omitempty"`
	Progress    materializationLabProgress       `json:"progress"`
	Results     []materializationLabWorkerResult `json:"results,omitempty"`
	Summary     materializationLabSummary        `json:"summary"`
	Error       string                           `json:"error,omitempty"`
}

type materializationLabProgress struct {
	Completed int `json:"completed"`
	Total     int `json:"total"`
}

type materializationLabWorkerResult struct {
	Run           int           `json:"run"`
	PostID        int64         `json:"post_id"`
	BoardID       string        `json:"board_id"`
	Author        string        `json:"author"`
	Subject       string        `json:"subject"`
	Action        string        `json:"action"`
	CauseKind     string        `json:"cause_kind"`
	DurationMS    int64         `json:"duration_ms"`
	Success       bool          `json:"success"`
	Created       bool          `json:"created"`
	BodyChars     int           `json:"body_chars"`
	ErrorClass    string        `json:"error_class,omitempty"`
	Diagnostic    string        `json:"diagnostic,omitempty"`
	Usage         worldrepo.GenerationUsage `json:"usage,omitempty"`
}

type materializationLabSummary struct {
	Attempts       int     `json:"attempts"`
	Successes      int     `json:"successes"`
	Failures       int     `json:"failures"`
	Timeouts       int     `json:"timeouts"`
	Dependency     int     `json:"dependency_failures"`
	Evidence       int     `json:"evidence_failures"`
	Renderer       int     `json:"renderer_failures"`
	Other          int     `json:"other_failures"`
	SuccessRate    float64 `json:"success_rate"`
	MeanDurationMS float64 `json:"mean_duration_ms"`
	P50DurationMS  int64   `json:"p50_duration_ms"`
	P95DurationMS  int64   `json:"p95_duration_ms"`
}

func newMaterializationLab(store debugExportStore, engine worldrepo.EvidenceResolver, materializer worldrepo.Materializer, worldDate, token string) *materializationLab {
	return &materializationLab{
		store: store, engine: engine, materializer: materializer, worldDate: worldDate, token: token,
		jobs: map[string]*materializationLabJob{},
	}
}

func (l *materializationLab) handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		if !labRequestAllowed(w, r) { return }
		action := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("action")))
		switch action {
		case "start":
			l.handleStart(w, r)
		case "status", "":
			l.handleStatus(w, r)
		case "list":
			l.handleList(w)
		default:
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "action must be start, status, or list"})
		}
	}
}

func (l *materializationLab) handleStart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	suite := strings.TrimSpace(r.URL.Query().Get("suite"))
	if suite == "" {
		suite = "worker-replay"
	}
	if suite != "worker-replay" {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "currently supported suite: worker-replay"})
		return
	}
	phone := strings.TrimSpace(r.URL.Query().Get("phone"))
	if phone == "" {
		phone = developmentMaterializationPhone
	}
	runs := queryInt(r, "runs", 1)
	if runs < 1 || runs > 5 {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "runs must be between 1 and 5"})
		return
	}
	timeoutMS := queryInt(r, "timeout_ms", 35000)
	if timeoutMS < 5000 || timeoutMS > 120000 {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "timeout_ms must be between 5000 and 120000"})
		return
	}
	postIDs, err := parsePostIDs(r.URL.Query().Get("post_ids"))
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	if len(postIDs) > 15 {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "post_ids may contain at most 15 ids"})
		return
	}

	if !publicLabAdmission.start(w, r, phone, runs) { return }
	l.mu.Lock()
	id := fmt.Sprintf("lab-%d-%04d", time.Now().UTC().Unix(), atomic.AddUint64(&l.seq, 1)%10000)
	job := &materializationLabJob{
		ID: id, Suite: suite, Status: "queued", Phone: phone, Runs: runs, TimeoutMS: timeoutMS,
		PostIDs: append([]int64(nil), postIDs...), CreatedAt: time.Now().UTC(),
	}
	l.jobs[id] = job
	l.active = id
	l.mu.Unlock()

	// Encode before the worker can mutate the queued job.
	_ = json.NewEncoder(w).Encode(job)
	go l.runWorkerReplay(id)
}

func (l *materializationLab) handleStatus(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.URL.Query().Get("id"))
	l.mu.Lock()
	defer l.mu.Unlock()
	if id == "" {
		if l.active != "" {
			_ = json.NewEncoder(w).Encode(l.jobs[l.active])
			return
		}
		var newest *materializationLabJob
		for _, job := range l.jobs {
			if newest == nil || job.CreatedAt.After(newest.CreatedAt) {
				newest = job
			}
		}
		if newest == nil {
			_ = json.NewEncoder(w).Encode(map[string]any{"status": "idle"})
			return
		}
		_ = json.NewEncoder(w).Encode(newest)
		return
	}
	job := l.jobs[id]
	if job == nil {
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "job not found"})
		return
	}
	_ = json.NewEncoder(w).Encode(job)
}

func (l *materializationLab) handleList(w http.ResponseWriter) {
	l.mu.Lock()
	defer l.mu.Unlock()
	jobs := make([]*materializationLabJob, 0, len(l.jobs))
	for _, job := range l.jobs {
		jobs = append(jobs, job)
	}
	sort.SliceStable(jobs, func(i, j int) bool { return jobs[i].CreatedAt.After(jobs[j].CreatedAt) })
	if len(jobs) > 20 {
		jobs = jobs[:20]
	}
	_ = json.NewEncoder(w).Encode(map[string]any{"jobs": jobs, "active": l.active})
}

func (l *materializationLab) runWorkerReplay(id string) {
	defer publicLabAdmission.finish()
	l.mu.Lock()
	job := l.jobs[id]
	job.Status = "running"
	job.StartedAt = time.Now().UTC()
	l.mu.Unlock()

	snapshot, selected, err := l.snapshotForWorkerReplay(job.Phone, job.PostIDs)
	if err != nil {
		l.finishError(id, err)
		return
	}
	total := len(selected) * job.Runs
	l.mu.Lock()
	job.Progress.Total = total
	l.mu.Unlock()

	for run := 1; run <= job.Runs; run++ {
		base := world.NewMemoryStore()
		copySnapshot := snapshot
		copySnapshot.Posts = clonePostsForLab(snapshot.Posts)
		for i := range copySnapshot.Posts {
			for _, target := range selected {
				if copySnapshot.Posts[i].ID == target.ID {
					copySnapshot.Posts[i].Body = ""
					break
				}
			}
		}
		if err := base.RestoreDevelopmentSnapshot(copySnapshot); err != nil {
			l.finishError(id, fmt.Errorf("restore isolated snapshot: %w", err))
			return
		}
		repo := worldrepo.New(base, l.engine, l.materializer, l.worldDate)
		host, err := repo.HostByPhone(job.Phone)
		if err != nil {
			l.finishError(id, fmt.Errorf("isolated host lookup: %w", err))
			return
		}
		boards := map[string]world.Board{}
		for _, board := range base.ListBoards(host.ID) {
			boards[board.ID] = board
		}

		for _, target := range selected {
			board, ok := boards[target.BoardID]
			if !ok {
				l.appendResult(id, materializationLabWorkerResult{Run: run, PostID: target.ID, BoardID: target.BoardID, Author: target.Author, Subject: target.Subject, Action: target.Intent.Action, CauseKind: target.Intent.CauseKind, ErrorClass: "board_missing", Diagnostic: "board not found in isolated snapshot"})
				continue
			}
			started := time.Now()
			post, found, created, diagnostic := repo.MaterializationArticleWithDebugTimeout(host, board, target.ID, time.Duration(job.TimeoutMS)*time.Millisecond)
			duration := time.Since(started)
			usage, _ := repo.MaterializationGenerationUsage(target.ID)
			success := found && post.Body != "" && !strings.Contains(diagnostic, "error stage=")
			result := materializationLabWorkerResult{
				Run: run, PostID: target.ID, BoardID: target.BoardID, Author: target.Author, Subject: target.Subject,
				Action: target.Intent.Action, CauseKind: target.Intent.CauseKind,
				DurationMS: duration.Milliseconds(), Success: success, Created: created, BodyChars: len([]rune(post.Body)),
				Diagnostic: diagnostic, Usage: usage,
			}
			if !success {
				result.ErrorClass = classifyLabDiagnostic(diagnostic)
			}
			l.appendResult(id, result)
		}
	}
	l.finishSuccess(id)
}

func (l *materializationLab) snapshotForWorkerReplay(phone string, requested []int64) (world.DevelopmentHostSnapshot, []world.Post, error) {
	host, err := l.store.HostByPhone(phone)
	if err != nil {
		return world.DevelopmentHostSnapshot{}, nil, err
	}
	boards := append([]world.Board(nil), l.store.ListBoards(host.ID)...)
	posts := clonePostsForLab(l.store.ListPosts(host.ID))
	personas := append([]world.Persona(nil), l.store.ListHostPersonas(host.ID)...)
	facts := map[string][]world.PersonaFact{}
	memberships := make([]string, 0, len(personas))
	for _, p := range personas {
		memberships = append(memberships, p.ID)
		if pf := l.store.ListPersonaFacts(p.ID); len(pf) > 0 {
			facts[p.ID] = append([]world.PersonaFact(nil), pf...)
		}
	}
	maxID := int64(0)
	for _, p := range posts {
		if p.ID > maxID {
			maxID = p.ID
		}
	}
	snapshot := world.DevelopmentHostSnapshot{
		SchemaVersion: world.DevelopmentHostSnapshotSchemaVersion,
		Host: host, Boards: boards, Posts: posts, Personas: personas, PersonaFacts: facts,
		Memberships: memberships, NextPostID: maxID,
	}
	selected := make([]world.Post, 0, len(posts))
	wanted := map[int64]bool{}
	for _, id := range requested {
		wanted[id] = true
	}
	for _, p := range posts {
		if strings.TrimSpace(p.Intent.ProducerEventID) == "" {
			continue
		}
		if len(wanted) > 0 && !wanted[p.ID] {
			continue
		}
		selected = append(selected, p)
	}
	if len(selected) == 0 {
		return world.DevelopmentHostSnapshot{}, nil, fmt.Errorf("no matching producer posts in current canonical snapshot")
	}
	sort.SliceStable(selected, func(i, j int) bool {
		if selected[i].CreatedAt.Equal(selected[j].CreatedAt) {
			return selected[i].ID < selected[j].ID
		}
		return selected[i].CreatedAt.Before(selected[j].CreatedAt)
	})
	return snapshot, selected, nil
}

func clonePostsForLab(posts []world.Post) []world.Post {
	out := make([]world.Post, len(posts))
	for i, p := range posts {
		out[i] = p
		out[i].Intent.Claims = append([]string(nil), p.Intent.Claims...)
		out[i].Intent.RespondsToClaims = append([]string(nil), p.Intent.RespondsToClaims...)
		out[i].Intent.ProducerReferents = append([]string(nil), p.Intent.ProducerReferents...)
		out[i].Intent.ProducerActorKnowledge = append([]string(nil), p.Intent.ProducerActorKnowledge...)
		out[i].Intent.ProducerAudienceContext = append([]string(nil), p.Intent.ProducerAudienceContext...)
		out[i].Intent.ProducerContribution = append([]string(nil), p.Intent.ProducerContribution...)
		out[i].Intent.ProducerMustNot = append([]string(nil), p.Intent.ProducerMustNot...)
		out[i].Intent.RenderContext = ""
	}
	return out
}

func (l *materializationLab) appendResult(id string, result materializationLabWorkerResult) {
	l.mu.Lock()
	defer l.mu.Unlock()
	job := l.jobs[id]
	job.Results = append(job.Results, result)
	job.Progress.Completed++
}

func (l *materializationLab) finishSuccess(id string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	job := l.jobs[id]
	job.Status = "completed"
	job.FinishedAt = time.Now().UTC()
	job.Summary = summarizeLabResults(job.Results)
	if l.active == id {
		l.active = ""
	}
}

func (l *materializationLab) finishError(id string, err error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	job := l.jobs[id]
	job.Status = "failed"
	job.FinishedAt = time.Now().UTC()
	job.Error = err.Error()
	job.Summary = summarizeLabResults(job.Results)
	if l.active == id {
		l.active = ""
	}
}

func summarizeLabResults(results []materializationLabWorkerResult) materializationLabSummary {
	s := materializationLabSummary{Attempts: len(results)}
	durations := make([]int64, 0, len(results))
	var durationTotal int64
	for _, r := range results {
		durations = append(durations, r.DurationMS)
		durationTotal += r.DurationMS
		if r.Success {
			s.Successes++
			continue
		}
		s.Failures++
		switch r.ErrorClass {
		case "timeout":
			s.Timeouts++
		case "dependency":
			s.Dependency++
		case "evidence":
			s.Evidence++
		case "renderer":
			s.Renderer++
		default:
			s.Other++
		}
	}
	if s.Attempts > 0 {
		s.SuccessRate = float64(s.Successes) / float64(s.Attempts)
		s.MeanDurationMS = float64(durationTotal) / float64(s.Attempts)
		sort.Slice(durations, func(i, j int) bool { return durations[i] < durations[j] })
		s.P50DurationMS = percentileDuration(durations, .50)
		s.P95DurationMS = percentileDuration(durations, .95)
	}
	return s
}

func percentileDuration(sorted []int64, p float64) int64 {
	if len(sorted) == 0 {
		return 0
	}
	idx := int(float64(len(sorted)-1)*p + .5)
	if idx < 0 {
		idx = 0
	}
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return sorted[idx]
}

func classifyLabDiagnostic(d string) string {
	v := strings.ToLower(d)
	if strings.Contains(v, "context deadline exceeded") || strings.Contains(v, "timeout") {
		return "timeout"
	}
	if strings.Contains(v, "error stage=dependency") {
		return "dependency"
	}
	if strings.Contains(v, "error stage=evidence") {
		return "evidence"
	}
	if strings.Contains(v, "error stage=renderer") {
		return "renderer"
	}
	return "other"
}

func queryInt(r *http.Request, key string, fallback int) int {
	v := strings.TrimSpace(r.URL.Query().Get(key))
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}

func parsePostIDs(raw string) ([]int64, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	parts := strings.Split(raw, ",")
	out := make([]int64, 0, len(parts))
	seen := map[int64]bool{}
	for _, part := range parts {
		id, err := strconv.ParseInt(strings.TrimSpace(part), 10, 64)
		if err != nil || id <= 0 {
			return nil, fmt.Errorf("invalid post id %q", part)
		}
		if !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	return out, nil
}

var _ worldrepo.EvidenceResolver = worldengine.Engine{}
