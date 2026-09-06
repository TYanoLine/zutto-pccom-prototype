package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"zutto-pccom/apps/server/internal/hostprogram/materializationdemo"
	"zutto-pccom/apps/server/internal/world"
	"zutto-pccom/apps/server/internal/worldrepo"
)

// fresh ALLBODY lab reproduces RESET -> ALLBODY in an isolated MemoryStore. It
// therefore includes the host-wide Producer call immediately followed by the
// Article Worker burst, which the existing replay labs deliberately skip.
type materializationFreshLabState struct {
	mu sync.Mutex
	jobs map[string]*materializationFreshJob
	active string
	seq uint64
}

var materializationFreshLab = materializationFreshLabState{jobs: map[string]*materializationFreshJob{}}

type materializationFreshJob struct {
	ID string `json:"id"`
	Status string `json:"status"`
	Phone string `json:"phone"`
	CreatedAt time.Time `json:"created_at"`
	StartedAt time.Time `json:"started_at,omitempty"`
	FinishedAt time.Time `json:"finished_at,omitempty"`
	DurationMS int64 `json:"duration_ms,omitempty"`
	RuntimeState string `json:"runtime_state,omitempty"`
	PostCount int `json:"post_count,omitempty"`
	BodyCount int `json:"body_count,omitempty"`
	EmptyPostIDs []int64 `json:"empty_post_ids,omitempty"`
	Failures int `json:"failures,omitempty"`
	StatusText string `json:"status_text,omitempty"`
	PlanningDiagnostic map[string]string `json:"planning_diagnostic,omitempty"`
	Usage string `json:"usage,omitempty"`
	Error string `json:"error,omitempty"`
}

func (l *materializationLab) freshHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		if !l.authorized(r) {
			w.WriteHeader(http.StatusForbidden)
			_ = json.NewEncoder(w).Encode(map[string]string{"error":"materialization lab is disabled or unauthorized"})
			return
		}
		switch strings.ToLower(strings.TrimSpace(r.URL.Query().Get("action"))) {
		case "start": l.handleFreshStart(w, r)
		case "status", "": handleFreshStatus(w, r)
		default:
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{"error":"action must be start or status"})
		}
	}
}

func (l *materializationLab) handleFreshStart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodPost { w.WriteHeader(http.StatusMethodNotAllowed); return }
	phone := strings.TrimSpace(r.URL.Query().Get("phone")); if phone == "" { phone = developmentMaterializationPhone }
	l.mu.Lock(); regular := l.active; l.mu.Unlock()
	materializationRandomLab.mu.Lock(); random := materializationRandomLab.active; materializationRandomLab.mu.Unlock()
	materializationAllBodyLab.mu.Lock(); allbody := materializationAllBodyLab.active; materializationAllBodyLab.mu.Unlock()
	materializationFreshLab.mu.Lock()
	if regular != "" || random != "" || allbody != "" || materializationFreshLab.active != "" {
		active := materializationFreshLab.active
		materializationFreshLab.mu.Unlock()
		w.WriteHeader(http.StatusConflict)
		_ = json.NewEncoder(w).Encode(map[string]any{"error":"another materialization lab job is active","regular":regular,"random":random,"allbody":allbody,"fresh":active})
		return
	}
	id := fmt.Sprintf("lab-fresh-%d-%04d", time.Now().UTC().Unix(), atomic.AddUint64(&materializationFreshLab.seq,1)%10000)
	job := &materializationFreshJob{ID:id, Status:"queued", Phone:phone, CreatedAt:time.Now().UTC()}
	materializationFreshLab.jobs[id]=job; materializationFreshLab.active=id
	materializationFreshLab.mu.Unlock()
	go l.runFreshAllBody(id)
	_ = json.NewEncoder(w).Encode(job)
}

func handleFreshStatus(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.URL.Query().Get("id"))
	materializationFreshLab.mu.Lock(); defer materializationFreshLab.mu.Unlock()
	if id == "" { id = materializationFreshLab.active }
	if id == "" {
		var newest *materializationFreshJob
		for _, j := range materializationFreshLab.jobs { if newest == nil || j.CreatedAt.After(newest.CreatedAt) { newest=j } }
		if newest == nil { _=json.NewEncoder(w).Encode(map[string]string{"status":"idle"}); return }
		_=json.NewEncoder(w).Encode(newest); return
	}
	job := materializationFreshLab.jobs[id]
	if job == nil { w.WriteHeader(http.StatusNotFound); _=json.NewEncoder(w).Encode(map[string]string{"error":"job not found"}); return }
	_=json.NewEncoder(w).Encode(job)
}

func (l *materializationLab) runFreshAllBody(id string) {
	materializationFreshLab.mu.Lock(); job:=materializationFreshLab.jobs[id]; job.Status="running"; job.StartedAt=time.Now().UTC(); materializationFreshLab.mu.Unlock()
	snapshot, _, err := l.snapshotForWorkerReplay(job.Phone, nil)
	if err != nil { finishFreshError(id,err); return }
	// Match RESET semantics for the semantic world: keep host/personas/boards and
	// monotonically increasing post id, but remove all posts and delayed facts.
	snapshot.Posts = nil
	snapshot.PersonaFacts = map[string][]world.PersonaFact{}
	base := world.NewMemoryStore()
	if err:=base.RestoreDevelopmentSnapshot(snapshot); err!=nil { finishFreshError(id,fmt.Errorf("restore fresh isolated snapshot: %w",err)); return }
	repo:=worldrepo.New(base,l.engine,l.materializer,l.worldDate)
	host,err:=repo.HostByPhone(job.Phone); if err!=nil { finishFreshError(id,err); return }
	runtime:=materializationdemo.New(host,repo)
	started:=time.Now(); _,_=runtime.HandleLine("ALLBODY")
	var statusText string
	deadline:=time.Now().Add(10*time.Minute)
	for {
		statusText,_=runtime.HandleLine("STATUS")
		if terminalBulkStatus(statusText) { break }
		if time.Now().After(deadline) { statusText += "\n[LAB] timeout waiting for fresh ALLBODY"; break }
		time.Sleep(250*time.Millisecond)
	}
	posts:=base.ListPosts(host.ID); bodies:=0; empty:=make([]int64,0)
	for _,p:=range posts { if strings.TrimSpace(p.Body)!="" { bodies++ } else { empty=append(empty,p.ID) } }
	diag:=map[string]string{}
	for _,b:=range base.ListBoards(host.ID) { if d:=strings.TrimSpace(repo.MaterializationPlanningDiagnostic(host.ID,b.ID)); d!="" { diag[b.ID]=d } }
	materializationFreshLab.mu.Lock(); job=materializationFreshLab.jobs[id]
	job.DurationMS=time.Since(started).Milliseconds(); job.RuntimeState=parseBulkState(statusText); job.PostCount=len(posts); job.BodyCount=bodies; job.EmptyPostIDs=empty; job.Failures=parseBulkFailureCount(statusText); job.StatusText=compactLabStatus(statusText); job.PlanningDiagnostic=diag; job.Usage=repo.MaterializationUsageTotalText(); job.Status="completed"; job.FinishedAt=time.Now().UTC(); if materializationFreshLab.active==id { materializationFreshLab.active="" }; materializationFreshLab.mu.Unlock()
}

func finishFreshError(id string, err error) {
	materializationFreshLab.mu.Lock(); defer materializationFreshLab.mu.Unlock()
	job:=materializationFreshLab.jobs[id]; if job==nil{return}; job.Status="failed"; job.Error=err.Error(); job.FinishedAt=time.Now().UTC(); if materializationFreshLab.active==id { materializationFreshLab.active="" }
}
