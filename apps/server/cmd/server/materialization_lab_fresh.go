package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"zutto-pccom/apps/server/internal/buildinfo"
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
	BodyModel               string    `json:"body_model,omitempty"`
	Action                  string    `json:"action,omitempty"`
	AnchorKey               string    `json:"anchor_key,omitempty"`
	CauseKind               string    `json:"cause_kind,omitempty"`
	DiscourseMode           string    `json:"discourse_mode,omitempty"`
	SourcePostID            int64     `json:"source_post_id,omitempty"`
	RespondsToPostID        int64     `json:"responds_to_post_id,omitempty"`
	SituationKind           string    `json:"situation_kind,omitempty"`
	SituationSummary        string    `json:"situation_summary,omitempty"`
	SituationFacts          []string  `json:"situation_facts,omitempty"`
	TopicTarget             string    `json:"topic_target,omitempty"`
	TopicTargetStatus       string    `json:"topic_target_status,omitempty"`
	SubjectTargetPresent    bool      `json:"subject_target_present"`
	ProducerEventID         string    `json:"producer_event_id,omitempty"`
	ProducerEpisode         string    `json:"producer_episode,omitempty"`
	ProducerReferents       []string  `json:"producer_referents,omitempty"`
	ProducerActorKnowledge  []string  `json:"producer_actor_knowledge,omitempty"`
	ProducerAudienceContext []string  `json:"producer_audience_context,omitempty"`
	ProducerContribution    []string  `json:"producer_contribution,omitempty"`
	ProducerMustNot         []string  `json:"producer_must_not,omitempty"`
}

type materializationFreshJob struct {
	TitleCandidates     []worldrepo.DevelopmentTitleCandidate `json:"title_candidates,omitempty"`
	TitleFirstTiming    worldrepo.DevelopmentTitleFirstTiming `json:"title_first_timing,omitempty"`
	BuildCommit         string                                `json:"build_commit,omitempty"`
	ID                  string                                `json:"id"`
	Status              string                                `json:"status"`
	Phone               string                                `json:"phone"`
	SituationMode       string                                `json:"situation_mode,omitempty"`
	HistoricalTexture   string                                `json:"historical_texture,omitempty"`
	EraGate             string                                `json:"era_gate,omitempty"`
	BoardCount          int                                   `json:"board_count,omitempty"`
	ShellLimit          int                                   `json:"shell_limit,omitempty"`
	Boards              []world.Board                         `json:"boards,omitempty"`
	ArchiveError        string                                `json:"archive_error,omitempty"`
	SituationDiagnostic string                                `json:"situation_diagnostic,omitempty"`
	CreatedAt           time.Time                             `json:"created_at"`
	StartedAt           time.Time                             `json:"started_at,omitempty"`
	FinishedAt          time.Time                             `json:"finished_at,omitempty"`
	DurationMS          int64                                 `json:"duration_ms,omitempty"`
	RuntimeState        string                                `json:"runtime_state,omitempty"`
	PostCount           int                                   `json:"post_count,omitempty"`
	BodyCount           int                                   `json:"body_count,omitempty"`
	EmptyPostIDs        []int64                               `json:"empty_post_ids,omitempty"`
	Failures            int                                   `json:"failures,omitempty"`
	StatusText          string                                `json:"status_text,omitempty"`
	PlanningDiagnostic  map[string]string                     `json:"planning_diagnostic,omitempty"`
	Usage               string                                `json:"usage,omitempty"`
	Articles            []materializationFreshArticle         `json:"articles,omitempty"`
	Error               string                                `json:"error,omitempty"`
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

func normalizeFreshSituationMode(raw string) (string, bool) {
	mode := strings.ToLower(strings.TrimSpace(raw))
	switch mode {
	case "", "facets":
		return "facets", true
	case "facetless":
		return "facetless", true
	case "batch":
		return "batch", true
	case "title-first":
		return "title-first", true
	case "topic-first":
		return "topic-first", true
	default:
		return "", false
	}
}

func normalizeFreshHistoricalTexture(raw string) (string, bool) {
	mode := strings.ToLower(strings.TrimSpace(raw))
	switch mode {
	case "", "sourced":
		return "sourced", true
	case "off":
		return "off", true
	case "model-memory":
		return "model-memory", true
	case "model-memory-concrete":
		return "model-memory-concrete", true
	case "search-grounded":
		return "search-grounded", true
	case "1996-08-curated":
		return mode, true
	default:
		return "", false
	}
}

func normalizeFreshEraGate(raw string) (string, bool) {
	mode := strings.ToLower(strings.TrimSpace(raw))
	switch mode {
	case "", "strict":
		return "strict", true
	case "observe-only":
		return "observe-only", true
	default:
		return "", false
	}
}

func freshIntParam(raw string, fallback, min, max int) (int, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return fallback, true
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value < min || value > max {
		return 0, false
	}
	return value, true
}

func freshScaleBoards(count int) []world.Board {
	catalog := []world.Board{
		{ID: "1", Name: "フリートーク"},
		{ID: "2", Name: "パソコン通信・モデム"},
		{ID: "3", Name: "地域の話題"},
		{ID: "4", Name: "ゲーム"},
		{ID: "5", Name: "音楽"},
		{ID: "6", Name: "ソフトウェア"},
	}
	if count < 3 {
		count = 3
	}
	if count > len(catalog) {
		count = len(catalog)
	}
	return append([]world.Board(nil), catalog[:count]...)
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
	situationMode, ok := normalizeFreshSituationMode(r.URL.Query().Get("situation_mode"))
	if !ok {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "situation_mode must be facets, facetless, batch, topic-first or title-first"})
		return
	}
	textureParam := r.URL.Query().Get("historical_texture")
	if situationMode == "title-first" && strings.TrimSpace(textureParam) == "" {
		textureParam = "model-memory"
	}
	historicalTexture, ok := normalizeFreshHistoricalTexture(textureParam)
	if !ok {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "historical_texture must be sourced, off, model-memory, model-memory-concrete, search-grounded or 1996-08-curated"})
		return
	}
	if historicalTexture == "search-grounded" && situationMode != "batch" && situationMode != "topic-first" {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "search-grounded requires situation_mode=batch"})
		return
	}
	if situationMode == "topic-first" && historicalTexture != "search-grounded" {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "topic-first requires historical_texture=search-grounded"})
		return
	}
	if situationMode == "title-first" && historicalTexture != "model-memory" && historicalTexture != "model-memory-concrete" {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "title-first requires historical_texture=model-memory or model-memory-concrete"})
		return
	}
	eraGate, ok := normalizeFreshEraGate(r.URL.Query().Get("era_gate"))
	if !ok {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "era_gate must be strict or observe-only"})
		return
	}
	if eraGate == "observe-only" && situationMode != "title-first" {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "era_gate=observe-only requires situation_mode=title-first"})
		return
	}
	boardCount, ok := freshIntParam(r.URL.Query().Get("board_count"), 3, 3, 6)
	if !ok {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "board_count must be 3..6"})
		return
	}
	shellLimit, ok := freshIntParam(r.URL.Query().Get("shell_limit"), 5, 1, 10)
	if !ok {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "shell_limit must be 1..10"})
		return
	}
	if !publicLabAdmission.start(w, r, phone, 1) {
		return
	}
	materializationFreshLab.mu.Lock()
	id := fmt.Sprintf("lab-fresh-%d-%04d", time.Now().UTC().Unix(), atomic.AddUint64(&materializationFreshLab.seq, 1)%10000)
	job := &materializationFreshJob{ID: id, Status: "queued", Phone: phone, SituationMode: situationMode, HistoricalTexture: historicalTexture, EraGate: eraGate, BoardCount: boardCount, ShellLimit: shellLimit, Boards: freshScaleBoards(boardCount), CreatedAt: time.Now().UTC()}
	job.BuildCommit = buildinfo.Current().Commit
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

	snapshot, err := l.snapshotForFreshWorld(job.Phone)
	if err != nil {
		finishFreshError(id, err)
		return
	}
	// Match RESET semantics for the semantic world: keep host/personas/boards and
	// monotonically increasing post id, but remove all posts and delayed facts.
	priorPosts := append([]world.Post(nil), snapshot.Posts...)
	snapshot.Posts = nil
	if job.SituationMode != "title-first" {
		snapshot.PersonaFacts = map[string][]world.PersonaFact{}
	}
	if snapshot.Host.SoftwareID == "materialization-demo" {
		snapshot.Boards = freshScaleBoards(job.BoardCount)
	}
	base := world.NewMemoryStore()
	if err := base.RestoreDevelopmentSnapshot(snapshot); err != nil {
		finishFreshError(id, fmt.Errorf("restore fresh isolated snapshot: %w", err))
		return
	}
	labMaterializer := l.materializer
	configure := func(m worldrepo.LLMMaterializer) worldrepo.LLMMaterializer {
		// The Lab keeps its independently selected historical prompting modes.
		m.ProductionMinimalHistoricalPrompt = false
		m.CuratedHistoricalReferences = job.HistoricalTexture == "sourced"
		m.ModelHistoricalMemory = job.HistoricalTexture == "model-memory" || job.HistoricalTexture == "model-memory-concrete"
		m.PreferConcreteHistoricalNames = job.HistoricalTexture == "model-memory-concrete"
		m.SearchGroundedHistoricalReferences = job.HistoricalTexture == "search-grounded"
		m.HistoricalReferencesEnabled = false
		m.HistoricalTexture = freshHistoricalTextureFacts(job.HistoricalTexture)
		return m
	}
	switch m := l.materializer.(type) {
	case worldrepo.LLMMaterializer:
		labMaterializer = configure(m)
	case *worldrepo.LLMMaterializer:
		clone := configure(*m)
		labMaterializer = &clone
	default:
		if job.HistoricalTexture != "off" {
			finishFreshError(id, fmt.Errorf("historical texture requires LLMMaterializer, got %T", l.materializer))
			return
		}
	}
	if job.EraGate == "observe-only" {
		labMaterializer, err = withTitleEraObserveOnly(labMaterializer)
		if err != nil {
			finishFreshError(id, err)
			return
		}
	}
	repo := worldrepo.New(base, l.engine, labMaterializer, l.worldDate)
	repo.SetArticleDetailPlanner(l.detailPlanner)
	repo.EnableDevelopmentConversationViewPoC()
	repo.SetDevelopmentConversationShellLimit(job.ShellLimit)
	if job.SituationMode == "title-first" {
		repo.EnableDevelopmentTitleFirstPoC(priorPosts)
	}
	if job.SituationMode == "facetless" {
		repo.EnableDevelopmentFacetlessSituationPoC()
	}
	if job.SituationMode == "batch" {
		repo.EnableDevelopmentBatchSituationPoC()
	}
	if job.SituationMode == "topic-first" {
		repo.EnableDevelopmentTopicFirstPoC()
	}
	if job.HistoricalTexture == "search-grounded" {
		repo.EnableDevelopmentSearchGroundingPoC()
	}
	host, err := repo.HostByPhone(job.Phone)
	if err != nil {
		finishFreshError(id, err)
		return
	}
	runtime := materializationdemo.New(host, repo)
	started := time.Now()
	_, _ = runtime.HandleLine("ALLBODY")
	var statusText string
	deadlineMinutes := 10
	if job.BoardCount > 3 || job.ShellLimit > 5 {
		deadlineMinutes = 15
	}
	deadline := time.Now().Add(time.Duration(deadlineMinutes) * time.Minute)
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
	for i := range articles {
		usage, _ := repo.MaterializationGenerationUsage(articles[i].ID)
		articles[i].BodyModel = usage.Model
	}

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
	job.SituationDiagnostic = repo.DevelopmentBatchSituationDiagnostic(host.ID)
	job.TitleCandidates = repo.DevelopmentTitleCandidates()
	if job.SituationMode == "title-first" {
		job.TitleFirstTiming = repo.DevelopmentTitleFirstTiming()
		failedBoards := map[string]bool{}
		for _, c := range job.TitleCandidates {
			if c.Status == "unreviewed" {
				failedBoards[c.BoardID] = true
			}
		}
		job.Failures += len(failedBoards)
		if len(failedBoards) > 0 {
			job.Error = "一部の板で候補検査が失敗しました。候補一覧の理由を確認してください。"
		}
	}
	job.Articles = articles
	job.Status = "completed"
	job.FinishedAt = time.Now().UTC()
	if materializationFreshLab.active == id {
		materializationFreshLab.active = ""
	}
	archiveJob := cloneMaterializationFreshJob(job)
	materializationFreshLab.mu.Unlock()
	l.archiveFreshCompletedJob(archiveJob)
}

func collectMaterializationFreshArticles(posts []world.Post) []materializationFreshArticle {
	out := make([]materializationFreshArticle, 0, len(posts))
	for _, p := range posts {
		target, targetStatus := "", ""
		for _, fact := range p.Intent.SituationFacts {
			if strings.HasPrefix(fact, "topic_target=") {
				target = strings.TrimPrefix(fact, "topic_target=")
			}
			if strings.HasPrefix(fact, "topic_target_status=") {
				targetStatus = strings.TrimPrefix(fact, "topic_target_status=")
			}
		}
		out = append(out, materializationFreshArticle{
			TopicTarget:             target,
			TopicTargetStatus:       targetStatus,
			SubjectTargetPresent:    worldrepo.TopicTargetInSubject(p.Subject, target),
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
