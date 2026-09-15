package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var errMaterializationFreshArchiveNotFound = errors.New("materialization fresh archive not found")

type materializationFreshArchiveSummary struct {
	ID                string    `json:"id"`
	Status            string    `json:"status"`
	SituationMode     string    `json:"situation_mode,omitempty"`
	HistoricalTexture string    `json:"historical_texture,omitempty"`
	BoardCount        int       `json:"board_count,omitempty"`
	ShellLimit        int       `json:"shell_limit,omitempty"`
	PostCount         int       `json:"post_count,omitempty"`
	BodyCount         int       `json:"body_count,omitempty"`
	Failures          int       `json:"failures,omitempty"`
	CreatedAt         time.Time `json:"created_at"`
	FinishedAt        time.Time `json:"finished_at,omitempty"`
}

type materializationFreshArchiveStore interface {
	Save(context.Context, *materializationFreshJob) error
	Get(context.Context, string) (*materializationFreshJob, error)
	Latest(context.Context) (*materializationFreshJob, error)
	List(context.Context, int) ([]materializationFreshArchiveSummary, error)
}

type postgresMaterializationFreshArchive struct{ pool *pgxpool.Pool }

func openPostgresMaterializationFreshArchive(ctx context.Context, databaseURL string) (*postgresMaterializationFreshArchive, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	archive := &postgresMaterializationFreshArchive{pool: pool}
	if _, err := pool.Exec(ctx, `
CREATE TABLE IF NOT EXISTS development_materialization_fresh_archives (
  id text PRIMARY KEY,
  status text NOT NULL,
  situation_mode text NOT NULL DEFAULT '',
  historical_texture text NOT NULL DEFAULT '',
  board_count integer NOT NULL DEFAULT 0,
  shell_limit integer NOT NULL DEFAULT 0,
  post_count integer NOT NULL DEFAULT 0,
  body_count integer NOT NULL DEFAULT 0,
  failures integer NOT NULL DEFAULT 0,
  created_at timestamptz NOT NULL,
  finished_at timestamptz NOT NULL,
  payload jsonb NOT NULL
);
ALTER TABLE development_materialization_fresh_archives ADD COLUMN IF NOT EXISTS historical_texture text NOT NULL DEFAULT '';
CREATE INDEX IF NOT EXISTS development_materialization_fresh_archives_created_idx
  ON development_materialization_fresh_archives (created_at DESC);
`); err != nil {
		pool.Close()
		return nil, err
	}
	return archive, nil
}

func (a *postgresMaterializationFreshArchive) Close() {
	if a != nil && a.pool != nil {
		a.pool.Close()
	}
}

func (a *postgresMaterializationFreshArchive) Save(ctx context.Context, job *materializationFreshJob) error {
	if a == nil || a.pool == nil || job == nil {
		return errors.New("materialization fresh archive unavailable")
	}
	payload, err := json.Marshal(job)
	if err != nil {
		return err
	}
	_, err = a.pool.Exec(ctx, `
INSERT INTO development_materialization_fresh_archives
  (id,status,situation_mode,historical_texture,board_count,shell_limit,post_count,body_count,failures,created_at,finished_at,payload)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12::jsonb)
ON CONFLICT (id) DO UPDATE SET
  status=EXCLUDED.status,
  situation_mode=EXCLUDED.situation_mode,
  historical_texture=EXCLUDED.historical_texture,
  board_count=EXCLUDED.board_count,
  shell_limit=EXCLUDED.shell_limit,
  post_count=EXCLUDED.post_count,
  body_count=EXCLUDED.body_count,
  failures=EXCLUDED.failures,
  created_at=EXCLUDED.created_at,
  finished_at=EXCLUDED.finished_at,
  payload=EXCLUDED.payload
`, job.ID, job.Status, job.SituationMode, job.HistoricalTexture, job.BoardCount, job.ShellLimit, job.PostCount, job.BodyCount, job.Failures, job.CreatedAt, job.FinishedAt, payload)
	return err
}

func (a *postgresMaterializationFreshArchive) Get(ctx context.Context, id string) (*materializationFreshJob, error) {
	var payload []byte
	err := a.pool.QueryRow(ctx, `SELECT payload FROM development_materialization_fresh_archives WHERE id=$1`, id).Scan(&payload)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, errMaterializationFreshArchiveNotFound
	}
	if err != nil {
		return nil, err
	}
	var job materializationFreshJob
	if err := json.Unmarshal(payload, &job); err != nil {
		return nil, err
	}
	return &job, nil
}

func (a *postgresMaterializationFreshArchive) Latest(ctx context.Context) (*materializationFreshJob, error) {
	var payload []byte
	err := a.pool.QueryRow(ctx, `SELECT payload FROM development_materialization_fresh_archives ORDER BY created_at DESC LIMIT 1`).Scan(&payload)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, errMaterializationFreshArchiveNotFound
	}
	if err != nil {
		return nil, err
	}
	var job materializationFreshJob
	if err := json.Unmarshal(payload, &job); err != nil {
		return nil, err
	}
	return &job, nil
}

func (a *postgresMaterializationFreshArchive) List(ctx context.Context, limit int) ([]materializationFreshArchiveSummary, error) {
	if limit < 1 {
		limit = 1
	}
	if limit > 50 {
		limit = 50
	}
	rows, err := a.pool.Query(ctx, `
SELECT id,status,situation_mode,historical_texture,board_count,shell_limit,post_count,body_count,failures,created_at,finished_at
FROM development_materialization_fresh_archives
ORDER BY created_at DESC LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]materializationFreshArchiveSummary, 0, limit)
	for rows.Next() {
		var item materializationFreshArchiveSummary
		if err := rows.Scan(&item.ID, &item.Status, &item.SituationMode, &item.HistoricalTexture, &item.BoardCount, &item.ShellLimit, &item.PostCount, &item.BodyCount, &item.Failures, &item.CreatedAt, &item.FinishedAt); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func cloneMaterializationFreshJob(job *materializationFreshJob) *materializationFreshJob {
	if job == nil {
		return nil
	}
	payload, err := json.Marshal(job)
	if err != nil {
		return nil
	}
	var clone materializationFreshJob
	if err := json.Unmarshal(payload, &clone); err != nil {
		return nil
	}
	return &clone
}

func (l *materializationLab) archiveFreshCompletedJob(job *materializationFreshJob) {
	if l == nil || l.freshArchive == nil || job == nil || job.Status != "completed" {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := l.freshArchive.Save(ctx, job); err != nil {
		log.Printf("archive fresh materialization job %s: %v", job.ID, err)
		materializationFreshLab.mu.Lock()
		if current := materializationFreshLab.jobs[job.ID]; current != nil {
			current.ArchiveError = err.Error()
		}
		materializationFreshLab.mu.Unlock()
	}
}

func freshJobSummary(job *materializationFreshJob) materializationFreshArchiveSummary {
	if job == nil {
		return materializationFreshArchiveSummary{}
	}
	return materializationFreshArchiveSummary{
		ID: job.ID, Status: job.Status, SituationMode: job.SituationMode, HistoricalTexture: job.HistoricalTexture,
		BoardCount: job.BoardCount, ShellLimit: job.ShellLimit,
		PostCount: job.PostCount, BodyCount: job.BodyCount, Failures: job.Failures,
		CreatedAt: job.CreatedAt, FinishedAt: job.FinishedAt,
	}
}

func memoryFreshJob(id string) *materializationFreshJob {
	materializationFreshLab.mu.Lock()
	defer materializationFreshLab.mu.Unlock()
	if id != "" {
		return cloneMaterializationFreshJob(materializationFreshLab.jobs[id])
	}
	var newest *materializationFreshJob
	for _, job := range materializationFreshLab.jobs {
		if job.Status != "completed" {
			continue
		}
		if newest == nil || job.CreatedAt.After(newest.CreatedAt) {
			newest = job
		}
	}
	return cloneMaterializationFreshJob(newest)
}

func memoryFreshSummaries(limit int) []materializationFreshArchiveSummary {
	materializationFreshLab.mu.Lock()
	defer materializationFreshLab.mu.Unlock()
	jobs := make([]*materializationFreshJob, 0, len(materializationFreshLab.jobs))
	for _, job := range materializationFreshLab.jobs {
		if job.Status == "completed" {
			jobs = append(jobs, cloneMaterializationFreshJob(job))
		}
	}
	sort.SliceStable(jobs, func(i, j int) bool { return jobs[i].CreatedAt.After(jobs[j].CreatedAt) })
	if limit > len(jobs) {
		limit = len(jobs)
	}
	out := make([]materializationFreshArchiveSummary, 0, limit)
	for _, job := range jobs[:limit] {
		out = append(out, freshJobSummary(job))
	}
	return out
}

// freshViewerHandler is deliberately read-only. It never invokes the world repository,
// generation endpoints, reset logic, or an LLM. Completed fresh jobs are copied out of
// the experimental runtime and served from the archive (with memory fallback).
func (l *materializationLab) freshViewerHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "GET only"})
			return
		}
		if strings.EqualFold(strings.TrimSpace(r.URL.Query().Get("list")), "1") || strings.EqualFold(strings.TrimSpace(r.URL.Query().Get("list")), "true") {
			limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
			if limit < 1 {
				limit = 20
			}
			if limit > 50 {
				limit = 50
			}
			if l.freshArchive != nil {
				ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
				items, err := l.freshArchive.List(ctx, limit)
				cancel()
				if err == nil {
					_ = json.NewEncoder(w).Encode(map[string]any{"jobs": items, "source": "archive"})
					return
				}
				log.Printf("list fresh materialization archive: %v", err)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"jobs": memoryFreshSummaries(limit), "source": "memory"})
			return
		}

		id := strings.TrimSpace(r.URL.Query().Get("id"))
		if l.freshArchive != nil {
			ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
			var job *materializationFreshJob
			var err error
			if id == "" {
				job, err = l.freshArchive.Latest(ctx)
			} else {
				job, err = l.freshArchive.Get(ctx, id)
			}
			cancel()
			if err == nil && job != nil {
				_ = json.NewEncoder(w).Encode(job)
				return
			}
			if err != nil && !errors.Is(err, errMaterializationFreshArchiveNotFound) {
				log.Printf("read fresh materialization archive: %v", err)
			}
		}
		if job := memoryFreshJob(id); job != nil {
			_ = json.NewEncoder(w).Encode(job)
			return
		}
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "completed fresh lab job not found"})
	}
}
