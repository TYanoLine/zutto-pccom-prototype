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
// therefore exercises the experimental conversation-view path immediately
// followed by the Article Worker burst, while ordinary runtime paths stay unchanged.
type materializationFreshLabState struct {
	mu     sync.Mutex
	jobs   map[string]*materializationFreshJob
	active string
	seq    uint64
}

var materializationFreshLab = materializationFreshLabState{jobs: map[string]*materializationFreshJob{}}

type materializationFreshArticle struct {
	ID                      int64     `json:"id"`
	BoardID                 string    `json:"board_id"`
	ParentID                int64     `json:"parent_id,omitempty"`
	Author                  string    `json:"author"`
	CreatedAt               time.Time `json:"created_at"`
	Subject                 string    `json:"subject"`
	Body                    string    `json:"body"`
	Action                  string    `json:"action,omitempty"`
	AnchorKey               string    `json:"anchor_key,omitempty"`
	CauseKind               string    `json:"cause_kind,omitempty"`
	DiscourseMode           string    `json:"discourse_mode,omitempty"`
	SourcePostID            int64     `json:"source_post_id,omitempty"`
	RespondsToPostID        int64     `json:"responds_to_post_id,omitempty"`
	SituationKind           string    `json:"situation_kind,omitempty"`
	SituationSummary        string    `json:"situation_summary,omitempty"`
	SituationFacts          []string  `json:"situation_facts,omitempty"`
	ProducerEventID         string    `json:"producer_event_id,omitempty"`
	ProducerEpisode         string    `json:"producer_episode,omitempty"`
	ProducerReferents       []string  `json:"producer_referents,omitempty"`
	ProducerActorKnowledge  []string  `json:"producer_actor_knowledge,omitempty"`
	ProducerAudienceContext []string  `json:"producer_audience_context,omitempty"`
	ProducerContribution    []string  `json:"producer_contribution,omitempty"`
	ProducerMustNot         []string  `json:"producer_must_not,omitempty"`
}

type materializationFreshJob struct {
	ID                 string                        `json:"id"`
	Status             string                        `json:"status"`
	Phone              string                        `json:"phone"`
	CreatedAt          time.Time                     `json:"created_at"`
	StartedAt          time.Time                     `json:"started_at,omitempty"`
	FinishedAt         time.Time                     `json:"finished_at,omitempty"`
	DurationMS         int64                         `json:"duration_ms,omitempty"`
	RuntimeState       string                        `json:"runtime_state,omitempty"`
	PostCount          int                           `json:"post_count,omitempty"`
	BodyCount          int                           `json:"body_count,omitempty"`
	EmptyPostIDs       []int64                       `json:"empty_post_ids,omitempty"`
	Failures           int                           `json:"failures,omitempty"`
	StatusText         string                        `json:"status_text,omitempty"`
	PlanningDiagnostic map[string]string             `json:"planning_diagnostic,omitempty"`
	Usage              string                        `json:"usage,omitempty"`
	Articles           []materializationFreshArticle `json:"articles,omitempty"`
	Error              string                        `json:"error,omitempty"`
}

func (l *materializationLab) freshHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		if !labRequestAllowed(w, r) {
			return
		}
		switch strings.ToLower(strings.TrimSpace(r.URL.Query().Get("action"))) {
		case "start":
			l.handleFreshStart(w, r)
		case "status", "":
			handleFreshStatus(w, r)
		default:
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "action must be start or status"})
		}
	}
}

func (l *materializationLab) handleFreshStart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	phone := strings.TrimSpace(r.URL.Query().Get("phone"))
	if phone == "" {
		phone = developmentMaterializationPhone
	}
	if !publicLabAdmission.start(w, r, phone, 1) {
		return
	}
	materializationFreshLab.mu.Lock()
	id := fmt.Sprintf("lab-fresh-%d-%04d", time.Now().UTC().Unix(), atomic.AddUint64(&materializationFreshLab.seq, 1)%10000)
	job := &materializationFreshJob{ID: id, Status: "queued", Phone: phone, CreatedAt: time.Now().UTC()}
	materializationFreshLab.jobs[id] = job
	materializationFreshLab.active = id
	materializationFreshLab.mu.Unlock()
	// Encode before the worker can mutate the queued job.
	_ = json.NewEncoder(w).Encode(job)
	go l.runFreshAllBody(id)
}

func handleFreshStatus(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.URL.Query().Get("id"))
	materializationFreshLab.mu.Lock()
	defer materializationFreshLab.mu.Unlock()
	if id == "" {
		id = materializationFreshLab.active
	}
	if id == "" {
		var newest *materializationFreshJob
		for _, j := range materializationFreshLab.jobs {
			if newest == nil || j.CreatedAt.After(newest.CreatedAt) {
				newest = j
			}
		}
		if newest == nil {
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "idle"})
			return
		}
		_ = json.NewEncoder(w).Encode(newest)
		return
	}
	job := materializationFreshLab.jobs[id]
	if job == nil {
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "job not found"})
		return
	}
	_ = json.NewEncoder(w).Encode(job)
}

func (l *materializationLab) runFreshAllBody(id string) {
	defer publicLabAdmission.finish()
	materializationFreshLab.mu.Lock()
	job := materializationFreshLab.jobs[id]
	job.Status = "running"
	job.StartedAt = time.Now().UTC()
	materializationFreshLab.mu.Unlock()

	snapshot, _, err := l.snapshotForWorkerReplay(job.Phone, nil)
	if err != nil {
		finishFreshError(id, err)
		return
	}
	// Match RESET semantics for the semantic world: keep host/personas/boards and
	// monotonically increasing post id, but remove all posts and delayed facts.
	snapshot.Posts = nil
	snapshot.PersonaFacts = map[string][]world.PersonaFact{}
	base := world.NewMemoryStore()
	if err := base.RestoreDevelopmentSnapshot(snapshot); err != nil {
		finishFreshError(id, fmt.Errorf("restore fresh isolated snapshot: %w", err))
		return
	}
	repo := worldrepo.New(base, l.engine, l.materializer, l.worldDate)
	repo.EnableDevelopmentConversationViewPoC()
	host, err := repo.HostByPhone(job.Phone)
	if err != nil {
		finishFreshError(id, err)
		return
	}
	runtime := materializationdemo.New(host, repo)
	started := time.Now()
	_, _ = runtime.HandleLine("ALLBODY")
	var statusText string
	deadline := time.Now().Add(10 * time.Minute)
	for {
		statusText, _ = runtime.HandleLine("STATUS")
		if terminalBulkStatus(statusText) {
			break
		}
		if time.Now().After(deadline) {
			publicLabAdmission.block()
			finishFreshError(id, fmt.Errorf("fresh ALLBODY timeout; lab blocked until server restart"))
			return
		}
		time.Sleep(250 * time.Millisecond)
	}

	posts := base.ListPosts(host.ID)
	bodies := 0
	empty := make([]int64, 0)
	for _, p := range posts {
		if strings.TrimSpace(p.Body) != "" {
			bodies++
		} else {
			empty = append(empty, p.ID)
		}
	}
	diag := map[string]string{}
	for _, b := range base.ListBoards(host.ID) {
		if d := strings.TrimSpace(repo.MaterializationPlanningDiagnostic(host.ID, b.ID)); d != "" {
			diag[b.ID] = d
		}
	}
	articles := collectMaterializationFreshArticles(posts)

	materializationFreshLab.mu.Lock()
	job = materializationFreshLab.jobs[id]
	job.DurationMS = time.Since(started).Milliseconds()
	job.RuntimeState = parseBulkState(statusText)
	job.PostCount = len(posts)
	job.BodyCount = bodies
	job.EmptyPostIDs = empty
	job.Failures = parseBulkFailureCount(statusText)
	job.StatusText = compactLabStatus(statusText)
	job.PlanningDiagnostic = diag
	job.Usage = repo.MaterializationUsageTotalText()
	job.Articles = articles
	job.Status = "completed"
	job.FinishedAt = time.Now().UTC()
	if materializationFreshLab.active == id {
		materializationFreshLab.active = ""
	}
	materializationFreshLab.mu.Unlock()
}

func collectMaterializationFreshArticles(posts []world.Post) []materializationFreshArticle {
	out := make([]materializationFreshArticle, 0, len(posts))
	for _, p := range posts {
		out = append(out, materializationFreshArticle{
			ID:                      p.ID,
			BoardID:                 p.BoardID,
			ParentID:                p.ParentID,
			Author:                  p.Author,
			CreatedAt:               p.CreatedAt,
			Subject:                 p.Subject,
			Body:                    p.Body,
			Action:                  p.Intent.Action,
			AnchorKey:               p.Intent.AnchorKey,
			CauseKind:               p.Intent.CauseKind,
			DiscourseMode:           p.Intent.DiscourseMode,
			SourcePostID:            p.Intent.SourcePostID,
			RespondsToPostID:        p.Intent.RespondsToPostID,
			SituationKind:           p.Intent.SituationKind,
			SituationSummary:        p.Intent.SituationSummary,
			SituationFacts:          append([]string(nil), p.Intent.SituationFacts...),
			ProducerEventID:         p.Intent.ProducerEventID,
			ProducerEpisode:         p.Intent.ProducerEpisode,
			ProducerReferents:       append([]string(nil), p.Intent.ProducerReferents...),
			ProducerActorKnowledge:  append([]string(nil), p.Intent.ProducerActorKnowledge...),
			ProducerAudienceContext: append([]string(nil), p.Intent.ProducerAudienceContext...),
			ProducerContribution:    append([]string(nil), p.Intent.ProducerContribution...),
			ProducerMustNot:         append([]string(nil), p.Intent.ProducerMustNot...),
		})
	}
	return out
}

func finishFreshError(id string, err error) {
	materializationFreshLab.mu.Lock()
	defer materializationFreshLab.mu.Unlock()
	job := materializationFreshLab.jobs[id]
	if job == nil {
		return
	}
	job.Status = "failed"
	job.Error = err.Error()
	job.FinishedAt = time.Now().UTC()
	if materializationFreshLab.active == id {
		materializationFreshLab.active = ""
	}
}
