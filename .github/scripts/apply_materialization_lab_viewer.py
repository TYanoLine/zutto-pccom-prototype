from pathlib import Path
import textwrap


def replace_once(path, old, new):
    p = Path(path)
    s = p.read_text()
    if old not in s:
        raise SystemExit(f'missing replacement target in {path}: {old[:100]!r}')
    p.write_text(s.replace(old, new, 1))


def write(path, content):
    p = Path(path)
    p.parent.mkdir(parents=True, exist_ok=True)
    p.write_text(textwrap.dedent(content).lstrip())

# Server wiring.
replace_once(
    'apps/server/cmd/server/materialization_lab.go',
    '\ttoken        string\n\n\tmu     sync.Mutex',
    '\ttoken        string\n\tfreshArchive materializationFreshArchiveStore\n\n\tmu     sync.Mutex',
)

replace_once(
    'apps/server/cmd/server/main.go',
    '\tvar catalogStore *worldcatalog.Store\n\tvar historyStore *historicalkb.Store\n',
    '\tvar catalogStore *worldcatalog.Store\n\tvar historyStore *historicalkb.Store\n\tvar freshArchive *postgresMaterializationFreshArchive\n',
)
replace_once(
    'apps/server/cmd/server/main.go',
    '\t\tif err == nil { historyStore, err = historicalkb.Open(ctx, cfg.DatabaseURL) }\n\t\tif err == nil { err = historyStore.EnsureSchema(ctx) }\n\t\tcancel()\n',
    '\t\tif err == nil { historyStore, err = historicalkb.Open(ctx, cfg.DatabaseURL) }\n\t\tif err == nil { err = historyStore.EnsureSchema(ctx) }\n\t\tif err == nil { freshArchive, err = openPostgresMaterializationFreshArchive(ctx, cfg.DatabaseURL) }\n\t\tcancel()\n',
)
replace_once(
    'apps/server/cmd/server/main.go',
    '\t\tdefer catalogStore.Close()\n\t\tdefer historyStore.Close()\n',
    '\t\tdefer catalogStore.Close()\n\t\tdefer historyStore.Close()\n\t\tdefer freshArchive.Close()\n',
)
replace_once(
    'apps/server/cmd/server/main.go',
    '\tmaterializationLab := newMaterializationLab(store, worldEngine, postMaterializer, cfg.WorldDate, cfg.MaterializationLabToken)\n',
    '\tmaterializationLab := newMaterializationLab(store, worldEngine, postMaterializer, cfg.WorldDate, cfg.MaterializationLabToken)\n\tmaterializationLab.freshArchive = freshArchive\n',
)
replace_once(
    'apps/server/cmd/server/main.go',
    '\tmux.HandleFunc("/api/debug/materialization-lab-fresh", materializationLab.freshHandler())\n',
    '\tmux.HandleFunc("/api/debug/materialization-lab-fresh", materializationLab.freshHandler())\n\tmux.HandleFunc("/api/debug/materialization-lab-fresh-view", materializationLab.freshViewerHandler())\n',
)
replace_once(
    'apps/server/cmd/server/main.go',
    '\"materialization_lab_auth\":\"none-test-only\",\"materialization_lab_daily_runs\":publicLabDailyRuns',
    '\"materialization_lab_auth\":\"none-test-only\",\"materialization_lab_archive\":freshArchive!=nil,\"materialization_lab_daily_runs\":publicLabDailyRuns',
)

# Fresh job includes board labels and archive diagnostics.
replace_once(
    'apps/server/cmd/server/materialization_lab_fresh.go',
    '\tShellLimit          int                           `json:"shell_limit,omitempty"`\n\tSituationDiagnostic string',
    '\tShellLimit          int                           `json:"shell_limit,omitempty"`\n\tBoards              []world.Board                 `json:"boards,omitempty"`\n\tArchiveError        string                        `json:"archive_error,omitempty"`\n\tSituationDiagnostic string',
)
replace_once(
    'apps/server/cmd/server/materialization_lab_fresh.go',
    'job := &materializationFreshJob{ID: id, Status: "queued", Phone: phone, SituationMode: situationMode, BoardCount: boardCount, ShellLimit: shellLimit, CreatedAt: time.Now().UTC()}',
    'job := &materializationFreshJob{ID: id, Status: "queued", Phone: phone, SituationMode: situationMode, BoardCount: boardCount, ShellLimit: shellLimit, Boards: freshScaleBoards(boardCount), CreatedAt: time.Now().UTC()}',
)
replace_once(
    'apps/server/cmd/server/materialization_lab_fresh.go',
    '\tif materializationFreshLab.active == id {\n\t\tmaterializationFreshLab.active = ""\n\t}\n\tmaterializationFreshLab.mu.Unlock()\n}\n\nfunc collectMaterializationFreshArticles',
    '\tif materializationFreshLab.active == id {\n\t\tmaterializationFreshLab.active = ""\n\t}\n\tarchiveJob := cloneMaterializationFreshJob(job)\n\tmaterializationFreshLab.mu.Unlock()\n\tl.archiveFreshCompletedJob(archiveJob)\n}\n\nfunc collectMaterializationFreshArticles',
)

write('apps/server/cmd/server/materialization_lab_fresh_archive.go', r'''
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
    ID            string    `json:"id"`
    Status        string    `json:"status"`
    SituationMode string    `json:"situation_mode,omitempty"`
    BoardCount    int       `json:"board_count,omitempty"`
    ShellLimit    int       `json:"shell_limit,omitempty"`
    PostCount     int       `json:"post_count,omitempty"`
    BodyCount     int       `json:"body_count,omitempty"`
    Failures      int       `json:"failures,omitempty"`
    CreatedAt     time.Time `json:"created_at"`
    FinishedAt    time.Time `json:"finished_at,omitempty"`
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
  board_count integer NOT NULL DEFAULT 0,
  shell_limit integer NOT NULL DEFAULT 0,
  post_count integer NOT NULL DEFAULT 0,
  body_count integer NOT NULL DEFAULT 0,
  failures integer NOT NULL DEFAULT 0,
  created_at timestamptz NOT NULL,
  finished_at timestamptz NOT NULL,
  payload jsonb NOT NULL
);
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
  (id,status,situation_mode,board_count,shell_limit,post_count,body_count,failures,created_at,finished_at,payload)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11::jsonb)
ON CONFLICT (id) DO UPDATE SET
  status=EXCLUDED.status,
  situation_mode=EXCLUDED.situation_mode,
  board_count=EXCLUDED.board_count,
  shell_limit=EXCLUDED.shell_limit,
  post_count=EXCLUDED.post_count,
  body_count=EXCLUDED.body_count,
  failures=EXCLUDED.failures,
  created_at=EXCLUDED.created_at,
  finished_at=EXCLUDED.finished_at,
  payload=EXCLUDED.payload
`, job.ID, job.Status, job.SituationMode, job.BoardCount, job.ShellLimit, job.PostCount, job.BodyCount, job.Failures, job.CreatedAt, job.FinishedAt, payload)
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
SELECT id,status,situation_mode,board_count,shell_limit,post_count,body_count,failures,created_at,finished_at
FROM development_materialization_fresh_archives
ORDER BY created_at DESC LIMIT $1`, limit)
    if err != nil {
        return nil, err
    }
    defer rows.Close()
    out := make([]materializationFreshArchiveSummary, 0, limit)
    for rows.Next() {
        var item materializationFreshArchiveSummary
        if err := rows.Scan(&item.ID, &item.Status, &item.SituationMode, &item.BoardCount, &item.ShellLimit, &item.PostCount, &item.BodyCount, &item.Failures, &item.CreatedAt, &item.FinishedAt); err != nil {
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
        ID: job.ID, Status: job.Status, SituationMode: job.SituationMode,
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
''')

write('apps/server/cmd/server/materialization_lab_fresh_archive_test.go', r'''
package main

import (
    "context"
    "encoding/json"
    "errors"
    "net/http"
    "net/http/httptest"
    "testing"
    "time"
)

type fakeFreshArchive struct{ jobs []*materializationFreshJob }

func (f *fakeFreshArchive) Save(_ context.Context, job *materializationFreshJob) error {
    f.jobs = append(f.jobs, cloneMaterializationFreshJob(job))
    return nil
}
func (f *fakeFreshArchive) Get(_ context.Context, id string) (*materializationFreshJob, error) {
    for _, job := range f.jobs {
        if job.ID == id {
            return cloneMaterializationFreshJob(job), nil
        }
    }
    return nil, errMaterializationFreshArchiveNotFound
}
func (f *fakeFreshArchive) Latest(_ context.Context) (*materializationFreshJob, error) {
    if len(f.jobs) == 0 {
        return nil, errMaterializationFreshArchiveNotFound
    }
    return cloneMaterializationFreshJob(f.jobs[len(f.jobs)-1]), nil
}
func (f *fakeFreshArchive) List(_ context.Context, limit int) ([]materializationFreshArchiveSummary, error) {
    if limit < 1 { return nil, errors.New("bad limit") }
    out := make([]materializationFreshArchiveSummary, 0, len(f.jobs))
    for i := len(f.jobs)-1; i >= 0 && len(out) < limit; i-- {
        out = append(out, freshJobSummary(f.jobs[i]))
    }
    return out, nil
}

func TestFreshViewerIsReadOnlyAndReturnsArchivedJob(t *testing.T) {
    finished := time.Date(2026, 9, 8, 1, 2, 3, 0, time.UTC)
    archive := &fakeFreshArchive{jobs: []*materializationFreshJob{{
        ID: "lab-fresh-test", Status: "completed", SituationMode: "batch",
        BoardCount: 6, ShellLimit: 8, PostCount: 37, BodyCount: 37,
        CreatedAt: finished.Add(-time.Minute), FinishedAt: finished,
        Articles: []materializationFreshArticle{{ID: 1001, BoardID: "1", Author: "NEKO", Subject: "テスト", Body: "本文"}},
    }}}
    lab := &materializationLab{freshArchive: archive}

    postReq := httptest.NewRequest(http.MethodPost, "/api/debug/materialization-lab-fresh-view", nil)
    postRes := httptest.NewRecorder()
    lab.freshViewerHandler().ServeHTTP(postRes, postReq)
    if postRes.Code != http.StatusMethodNotAllowed { t.Fatalf("POST status=%d", postRes.Code) }

    getReq := httptest.NewRequest(http.MethodGet, "/api/debug/materialization-lab-fresh-view?id=lab-fresh-test", nil)
    getRes := httptest.NewRecorder()
    lab.freshViewerHandler().ServeHTTP(getRes, getReq)
    if getRes.Code != http.StatusOK { t.Fatalf("GET status=%d body=%s", getRes.Code, getRes.Body.String()) }
    var got materializationFreshJob
    if err := json.NewDecoder(getRes.Body).Decode(&got); err != nil { t.Fatal(err) }
    if got.ID != "lab-fresh-test" || len(got.Articles) != 1 || got.Articles[0].Body != "本文" {
        t.Fatalf("unexpected archived job: %+v", got)
    }
}

func TestFreshViewerListsArchiveSummariesWithoutArticleBodies(t *testing.T) {
    archive := &fakeFreshArchive{jobs: []*materializationFreshJob{{ID:"one",Status:"completed",CreatedAt:time.Now().Add(-time.Minute),Articles:[]materializationFreshArticle{{Body:"secret-body"}}},{ID:"two",Status:"completed",CreatedAt:time.Now()}}}
    lab := &materializationLab{freshArchive: archive}
    req := httptest.NewRequest(http.MethodGet, "/api/debug/materialization-lab-fresh-view?list=1&limit=10", nil)
    res := httptest.NewRecorder()
    lab.freshViewerHandler().ServeHTTP(res, req)
    if res.Code != http.StatusOK { t.Fatalf("status=%d", res.Code) }
    if body := res.Body.String(); len(body) == 0 || contains(body, "secret-body") { t.Fatalf("summary leaked article body: %s", body) }
}

func contains(s, sub string) bool {
    for i := 0; i+len(sub) <= len(s); i++ { if s[i:i+len(sub)] == sub { return true } }
    return false
}
''')

# Strictly read-only Vercel proxy.
write('apps/web/api/materialization-lab-viewer.js', r'''
const BACKEND_BASE = 'https://zutto-pccom-prototype.onrender.com';

export default async function handler(req, res) {
  res.setHeader('Cache-Control', 'no-store');
  res.setHeader('Content-Type', 'application/json; charset=utf-8');
  if (req.method !== 'GET') {
    res.status(405).json({ error: 'GET only' });
    return;
  }
  const incoming = new URL(req.url || '/api/materialization-lab-viewer', 'https://materialization-lab-viewer.local');
  const upstream = new URL('/api/debug/materialization-lab-fresh-view', BACKEND_BASE);
  for (const key of ['id', 'list', 'limit']) {
    const value = incoming.searchParams.get(key);
    if (value !== null) upstream.searchParams.set(key, value);
  }
  const controller = new AbortController();
  const timeout = setTimeout(() => controller.abort(), 30000);
  try {
    const response = await fetch(upstream, { method: 'GET', headers: { Accept: 'application/json' }, signal: controller.signal, cache: 'no-store' });
    const body = await response.text();
    res.status(response.status).send(body);
  } catch (error) {
    const message = error instanceof Error ? error.message : String(error);
    res.status(502).json({ error: 'materialization lab viewer upstream failed', detail: message });
  } finally {
    clearTimeout(timeout);
  }
}
''')

write('apps/web/src/poc/MaterializationLabViewerPage.tsx', r'''
import { useEffect, useMemo, useState } from 'react';

type Board = { id: string; name: string };
type Article = {
  id: number; board_id: string; parent_id?: number; author: string; created_at: string;
  subject: string; body: string; action?: string; anchor_key?: string; cause_kind?: string;
  discourse_mode?: string; source_post_id?: number; responds_to_post_id?: number;
  situation_kind?: string; situation_summary?: string; situation_facts?: string[];
};
type Job = {
  id: string; status: string; situation_mode?: string; board_count?: number; shell_limit?: number;
  boards?: Board[]; created_at: string; finished_at?: string; duration_ms?: number; post_count?: number;
  body_count?: number; failures?: number; usage?: string; situation_diagnostic?: string; articles?: Article[];
};
type JobSummary = Pick<Job, 'id'|'status'|'situation_mode'|'board_count'|'shell_limit'|'created_at'|'finished_at'|'post_count'|'body_count'|'failures'>;

const fallbackBoards: Record<string,string> = {
  '1':'フリートーク','2':'パソコン通信・モデム','3':'地域の話題','4':'ゲーム','5':'音楽','6':'ソフトウェア'
};
const jst = new Intl.DateTimeFormat('ja-JP', { timeZone:'Asia/Tokyo', month:'2-digit', day:'2-digit', hour:'2-digit', minute:'2-digit', hour12:false });

function fmt(ts?: string) { return ts ? jst.format(new Date(ts)) : '-'; }
function threadPosts(root: Article, articles: Article[]) {
  const out = [root];
  const seen = new Set<number>([root.id]);
  let changed = true;
  while (changed) {
    changed = false;
    for (const post of articles) {
      if (seen.has(post.id) || !post.parent_id || !seen.has(post.parent_id)) continue;
      seen.add(post.id); out.push(post); changed = true;
    }
  }
  return out.sort((a,b) => new Date(a.created_at).getTime()-new Date(b.created_at).getTime() || a.id-b.id);
}

export default function MaterializationLabViewerPage() {
  const [summaries,setSummaries] = useState<JobSummary[]>([]);
  const [job,setJob] = useState<Job|null>(null);
  const [boardID,setBoardID] = useState('');
  const [threadID,setThreadID] = useState<number|undefined>();
  const [debug,setDebug] = useState(false);
  const [error,setError] = useState('');
  const [loading,setLoading] = useState(true);

  async function loadJob(id?: string) {
    setLoading(true); setError('');
    try {
      const qs = id ? `?id=${encodeURIComponent(id)}` : '';
      const res = await fetch(`/api/materialization-lab-viewer${qs}`, { cache:'no-store' });
      if (!res.ok) throw new Error(`HTTP ${res.status}`);
      const next: Job = await res.json();
      setJob(next);
      if (id) history.replaceState(null,'',`/poc/materialization-lab-viewer?job=${encodeURIComponent(id)}`);
      const firstBoard = next.boards?.[0]?.id || next.articles?.[0]?.board_id || '1';
      setBoardID(firstBoard); setThreadID(undefined);
    } catch (e) { setError(e instanceof Error ? e.message : String(e)); setJob(null); }
    finally { setLoading(false); }
  }

  useEffect(() => {
    (async () => {
      try {
        const res = await fetch('/api/materialization-lab-viewer?list=1&limit=30', { cache:'no-store' });
        if (res.ok) { const data = await res.json(); setSummaries(data.jobs || []); }
      } catch { /* full job fetch below gives the useful error */ }
      const requested = new URLSearchParams(location.search).get('job') || undefined;
      await loadJob(requested);
    })();
  }, []);

  const articles = job?.articles || [];
  const boards = useMemo(() => {
    if (job?.boards?.length) return job.boards;
    const ids = Array.from(new Set(articles.map(a=>a.board_id))).sort();
    return ids.map(id => ({ id, name:fallbackBoards[id] || `BOARD ${id}` }));
  }, [job,articles]);
  const boardArticles = articles.filter(a=>a.board_id===boardID);
  const roots = boardArticles.filter(a=>!a.parent_id).sort((a,b)=>new Date(a.created_at).getTime()-new Date(b.created_at).getTime());
  const selectedRoot = roots.find(r=>r.id===threadID) || roots[0];
  const selectedPosts = selectedRoot ? threadPosts(selectedRoot, boardArticles) : [];

  useEffect(() => { if (selectedRoot && threadID !== selectedRoot.id) setThreadID(selectedRoot.id); }, [boardID, job?.id, selectedRoot?.id]);

  return <div className="labviewer">
    <style>{css}</style>
    <header>
      <div><div className="eyebrow">DEVELOPMENT MATERIALIZATION LAB</div><h1>生成BBS 評価ビュー</h1></div>
      <div className="readonly">READ ONLY</div>
    </header>
    <div className="note">実験用のisolated fresh worldを閲覧しています。ここから書込・返信・生成・RESETはできません。</div>

    <section className="runbar">
      <label>実験run
        <select value={job?.id || ''} onChange={e=>loadJob(e.target.value)}>
          {job && !summaries.some(s=>s.id===job.id) && <option value={job.id}>{job.id}</option>}
          {summaries.map(s=><option key={s.id} value={s.id}>{fmt(s.finished_at || s.created_at)} · {s.situation_mode || '-'} · {s.post_count || 0}件 · {s.id}</option>)}
        </select>
      </label>
      <button onClick={()=>loadJob(job?.id)} disabled={loading}>再読込</button>
      <label className="debug"><input type="checkbox" checked={debug} onChange={e=>setDebug(e.target.checked)}/> 内部Situationを表示</label>
    </section>

    {error && <div className="error">読み込み失敗: {error}</div>}
    {loading && <div className="loading">読み込み中...</div>}
    {job && <>
      <section className="metrics">
        <span>MODE <b>{job.situation_mode || '-'}</b></span><span>BOARDS <b>{job.board_count || boards.length}</b></span>
        <span>POSTS <b>{job.post_count || articles.length}</b></span><span>BODIES <b>{job.body_count || 0}</b></span>
        <span>FAIL <b>{job.failures || 0}</b></span><span>TIME <b>{job.duration_ms ? (job.duration_ms/1000).toFixed(1)+'s' : '-'}</b></span>
      </section>

      <nav className="boards">
        {boards.map(b => <button key={b.id} className={b.id===boardID?'active':''} onClick={()=>{setBoardID(b.id);setThreadID(undefined)}}>
          <small>{b.id}</small>{b.name}<em>{articles.filter(a=>a.board_id===b.id).length}</em>
        </button>)}
      </nav>

      <main>
        <aside className="threads">
          <div className="paneTitle">スレッド一覧</div>
          {roots.length===0 && <div className="empty">記事なし</div>}
          {roots.map(root => {
            const count = threadPosts(root, boardArticles).length;
            return <button key={root.id} className={root.id===selectedRoot?.id?'selected':''} onClick={()=>setThreadID(root.id)}>
              <strong>{root.subject}</strong><span>{root.author} · {fmt(root.created_at)}{count>1?` · ${count} posts`:''}</span>
            </button>;
          })}
        </aside>
        <section className="conversation">
          <div className="paneTitle">{selectedRoot?.subject || '本文'}</div>
          {selectedPosts.map((post,i)=><article key={post.id}>
            <div className="postHead"><b>{post.author}</b><span>#{post.id} · {fmt(post.created_at)}</span></div>
            {i>0 && <div className="subject">{post.subject}</div>}
            <div className="body">{post.body}</div>
            {debug && <details open className="worldDebug"><summary>WORLD / SITUATION</summary>
              <dl><dt>action</dt><dd>{post.action || '-'}</dd><dt>anchor</dt><dd>{post.anchor_key || '-'}</dd><dt>cause</dt><dd>{post.cause_kind || '-'}</dd><dt>discourse</dt><dd>{post.discourse_mode || '-'}</dd><dt>source</dt><dd>{post.source_post_id || '-'}</dd><dt>situation</dt><dd>{post.situation_kind || '-'}</dd></dl>
              {post.situation_summary && <p>{post.situation_summary}</p>}
              {!!post.situation_facts?.length && <ul>{post.situation_facts.map((f,n)=><li key={n}>{f}</li>)}</ul>}
            </details>}
          </article>)}
        </section>
      </main>
      <footer>JOB {job.id} · finished {fmt(job.finished_at)} · archived experimental output; not canonical BBS world</footer>
    </>}
  </div>;
}

const css = `
:root{background:#07100c;color:#d8f6df;font-family:ui-monospace,SFMono-Regular,Menlo,Consolas,"Noto Sans Mono CJK JP",monospace}*{box-sizing:border-box}body{margin:0;background:#07100c}.labviewer{min-height:100vh;padding:22px;max-width:1500px;margin:auto}header{display:flex;align-items:center;justify-content:space-between;border-bottom:1px solid #335844;padding-bottom:12px}.eyebrow{font-size:11px;letter-spacing:.18em;color:#70a981}h1{font-size:25px;margin:5px 0 0;font-weight:600}.readonly{border:1px solid #72c38b;color:#9ff6b8;padding:7px 10px;font-size:12px}.note{color:#92ad9b;font-size:12px;padding:10px 0}.runbar{display:flex;gap:10px;align-items:end;flex-wrap:wrap;background:#0c1711;border:1px solid #263b2d;padding:10px}.runbar label{font-size:11px;color:#8fad98}.runbar select{display:block;min-width:430px;max-width:70vw;margin-top:4px;background:#07100c;color:#d8f6df;border:1px solid #3d634b;padding:8px}.runbar button,.boards button,.threads button{font:inherit}.runbar>button{background:#13241a;color:#c7edcf;border:1px solid #42644c;padding:8px 12px}.debug{margin-left:auto;display:flex!important;gap:7px;align-items:center;padding-bottom:7px}.metrics{display:flex;gap:18px;flex-wrap:wrap;padding:10px 2px;font-size:11px;color:#779183}.metrics b{color:#dbf6e1;font-size:13px}.boards{display:flex;gap:5px;flex-wrap:wrap;border-bottom:1px solid #35513d;padding:3px 0 9px}.boards button{background:#0b150f;color:#9abb9f;border:1px solid #294333;padding:8px 12px;cursor:pointer}.boards button.active{background:#183121;color:#e1ffe8;border-color:#5b946d}.boards small{color:#5c8168;margin-right:6px}.boards em{font-style:normal;color:#6f9d7c;margin-left:8px}main{display:grid;grid-template-columns:minmax(270px,34%) 1fr;gap:12px;margin-top:12px;min-height:60vh}.threads,.conversation{border:1px solid #2c4635;background:#09130d}.paneTitle{padding:8px 10px;border-bottom:1px solid #2c4635;color:#8ebc9a;font-size:12px;letter-spacing:.08em}.threads button{width:100%;display:block;text-align:left;background:transparent;color:#c4dfca;border:0;border-bottom:1px solid #18291e;padding:11px;cursor:pointer}.threads button.selected{background:#14271a;border-left:3px solid #6bc184}.threads strong{display:block;font-size:13px;font-weight:500}.threads span{display:block;margin-top:5px;font-size:10px;color:#708b78}.conversation article{padding:16px 18px;border-bottom:1px dashed #294233}.postHead{display:flex;justify-content:space-between;gap:10px;color:#9ee3ae;font-size:13px}.postHead span{font-size:10px;color:#718d79}.subject{font-size:11px;color:#7ea488;margin-top:6px}.body{white-space:pre-wrap;line-height:1.75;margin-top:12px;color:#e1f6e5;font-family:inherit;font-size:14px}.worldDebug{margin-top:14px;background:#050b07;border:1px solid #273b2e;padding:8px;color:#8fab96;font-size:10px}.worldDebug summary{cursor:pointer;color:#72ab80}.worldDebug dl{display:grid;grid-template-columns:80px 1fr;gap:3px 8px}.worldDebug dt{color:#577762}.worldDebug dd{margin:0}.worldDebug p,.worldDebug ul{line-height:1.5}.error{margin:14px 0;padding:12px;border:1px solid #8d4949;background:#2b1212;color:#ffc6c6}.loading,.empty{padding:18px;color:#789080}footer{font-size:10px;color:#536a59;padding:14px 2px}@media(max-width:800px){.labviewer{padding:12px}.runbar select{min-width:0;width:80vw}.debug{margin-left:0}main{grid-template-columns:1fr}.threads{max-height:34vh;overflow:auto}.conversation{min-height:40vh}}
`;
''')

replace_once(
    'apps/web/src/main.tsx',
    "import ImageArtifactPocPage from './poc/ImageArtifactPocPage';\n",
    "import ImageArtifactPocPage from './poc/ImageArtifactPocPage';\nimport MaterializationLabViewerPage from './poc/MaterializationLabViewerPage';\n",
)
replace_once(
    'apps/web/src/main.tsx',
    "    : path === '/poc/image-artifact'\n      ? ImageArtifactPocPage\n      : Home;",
    "    : path === '/poc/image-artifact'\n      ? ImageArtifactPocPage\n      : path === '/poc/materialization-lab-viewer'\n        ? MaterializationLabViewerPage\n        : Home;",
)
replace_once(
    'apps/web/vercel.json',
    '    { "source": "/poc/image-artifact", "destination": "/" }\n',
    '    { "source": "/poc/image-artifact", "destination": "/" },\n    { "source": "/poc/materialization-lab-viewer", "destination": "/" }\n',
)

# Documentation.
p = Path('docs/MATERIALIZATION_LAB.md')
s = p.read_text()
append = r'''

## Read-only generated BBS viewer

Completed `materialization-lab-fresh` jobs are archived separately from the canonical BBS world when `DATABASE_URL` is configured. The archive stores the completed job JSON (including generated article bodies) in `development_materialization_fresh_archives`; it is **development experiment evidence**, not world state, and is never restored into the normal runtime.

Read-only API:

- `GET /api/debug/materialization-lab-fresh-view` — newest archived completed fresh job
- `GET /api/debug/materialization-lab-fresh-view?id=<job-id>` — one archived job
- `GET /api/debug/materialization-lab-fresh-view?list=1&limit=20` — summaries only
- non-GET methods return `405`; this handler never starts generation, RESETs state, observes boards, or invokes an LLM.

Vercel exposes a strict GET-only proxy at `/api/materialization-lab-viewer` and an evaluator UI at `/poc/materialization-lab-viewer`. The UI defaults to normal BBS reading (board → thread → article). World/Situation diagnostics are hidden unless the evaluator explicitly enables them.
'''
if '## Read-only generated BBS viewer' not in s:
    p.write_text(s.rstrip()+textwrap.dedent(append)+'\n')

write('docs/MATERIALIZATION_LAB_VIEWER.md', r'''
# Materialization Lab read-only viewer

The evaluator at `/poc/materialization-lab-viewer` exists so humans can judge the actual prose and conversation produced by isolated fresh-Lab runs without copying those posts into the saved demo BBS.

## Boundary

- The viewer is read-only. Its server endpoint accepts GET only.
- Reading an archive never calls `WorldRepository`, an observation gate, RESET, the Situation Proposer, an Article Worker, or any other LLM path.
- Completed fresh jobs are archived in PostgreSQL separately from canonical world snapshots.
- Archive rows are experiment evidence and must never be interpreted as canonical world history.
- Normal runtime does not read these rows.

## UX

The page shows recent runs, boards, thread subjects and full generated bodies. Generation metadata (`anchor_key`, cause, discourse mode, Situation summary/facts and source IDs) is hidden by default so prose can first be judged as ordinary BBS conversation. A developer can opt in to the diagnostic layer for causal review.

A job can be deep-linked with `?job=<lab-job-id>` so the same generation result can be reviewed after a server deploy/restart.
''')
