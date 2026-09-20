package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"zutto-pccom/apps/server/internal/buildinfo"
	"zutto-pccom/apps/server/internal/llm"
	"zutto-pccom/apps/server/internal/personapoc"
	"zutto-pccom/apps/server/internal/worldengine"
)

const personaLabProfileBatchSize = 10
const personaLabMaxProfiles = 100
const personaLabFutureFlagThreshold = 0.50

type personaProfileGenerator interface {
	GeneratePersonaProfiles(context.Context, string, []llm.PersonaProfileSeed) (llm.PersonaProfileBatch, error)
}

type personaFutureAdvisor interface {
	AdvisePersonaProfiles(context.Context, worldengine.PersonaProfileAdviceRequest) (worldengine.PersonaProfileAdviceDecision, error)
}

type personaLab struct {
	generator          personaProfileGenerator
	advisor            personaFutureAdvisor
	worldDate          string
	profileGeneration  bool

	mu     sync.Mutex
	jobs   map[string]*personaProfileJob
	active string
	seq    uint64
}

type personaProfileJob struct {
	ID                   string                       `json:"id"`
	Status               string                       `json:"status"`
	Seed                 int64                        `json:"seed"`
	ProfileID            string                       `json:"profile_id"`
	ProfileLabel         string                       `json:"profile_label"`
	AccountCount         int                          `json:"account_count"`
	ProfileCount         int                          `json:"profile_count"`
	PoolSize             int                          `json:"pool_size"`
	BatchSize            int                          `json:"batch_size"`
	IdentityGenerationUS int64                        `json:"identity_generation_us"`
	JevAvailable         bool                         `json:"jev_available"`
	CreatedAt            time.Time                    `json:"created_at"`
	StartedAt            time.Time                    `json:"started_at,omitempty"`
	FinishedAt           time.Time                    `json:"finished_at,omitempty"`
	Progress             personaProfileProgress       `json:"progress"`
	Batches              []personaProfileBatchResult  `json:"batches,omitempty"`
	Results              []personaProfileResult       `json:"results,omitempty"`
	Summary              personaProfileSummary        `json:"summary"`
	Error                string                       `json:"error,omitempty"`
}

type personaProfileProgress struct {
	Completed int `json:"completed"`
	Total     int `json:"total"`
}

type personaProfileBatchResult struct {
	Batch          int    `json:"batch"`
	Count          int    `json:"count"`
	LLMDurationMS  int64  `json:"llm_duration_ms"`
	LLMModel       string `json:"llm_model,omitempty"`
	LLMInputTokens int    `json:"llm_input_tokens"`
	LLMOutputTokens int   `json:"llm_output_tokens"`
	LLMTotalTokens int    `json:"llm_total_tokens"`
	JevDurationMS  int64  `json:"jev_duration_ms"`
	JevModel       string `json:"jev_model,omitempty"`
	JevInputTokens int    `json:"jev_input_tokens"`
	JevError       string `json:"jev_error,omitempty"`
}

type personaProfileResult struct {
	PersonaID                 string  `json:"persona_id"`
	Handle                    string  `json:"handle"`
	DetailTier                string  `json:"detail_tier"`
	Skeleton                  string  `json:"skeleton"`
	Profile                   string  `json:"profile"`
	JevChecked                bool    `json:"jev_checked"`
	FutureProbability         float64 `json:"future_probability"`
	ExternalReviewProbability float64 `json:"external_review_probability"`
	FutureFlag                bool    `json:"future_flag"`
	ExternalReviewFlag        bool    `json:"external_review_flag"`
}

type personaProfileSummary struct {
	LLMCalls               int     `json:"llm_calls"`
	LLMDurationMS          int64   `json:"llm_duration_ms"`
	LLMInputTokens         int     `json:"llm_input_tokens"`
	LLMOutputTokens        int     `json:"llm_output_tokens"`
	LLMTotalTokens         int     `json:"llm_total_tokens"`
	JevCalls               int     `json:"jev_calls"`
	JevDurationMS          int64   `json:"jev_duration_ms"`
	JevInputTokens         int     `json:"jev_input_tokens"`
	JevErrors              int     `json:"jev_errors"`
	FutureFlagged          int     `json:"future_flagged"`
	ExternalReviewFlagged  int     `json:"external_review_flagged"`
	MaxFutureProbability   float64 `json:"max_future_probability"`
	MaxExternalReview      float64 `json:"max_external_review_probability"`
	TotalDurationMS        int64   `json:"total_duration_ms"`
	ProfilesPerSecond      float64 `json:"profiles_per_second"`
}

func newPersonaLab(generator personaProfileGenerator, advisor personaFutureAdvisor, worldDate string, profileGeneration bool) *personaLab {
	return &personaLab{
		generator: generator, advisor: advisor, worldDate: worldDate, profileGeneration: profileGeneration,
		jobs: map[string]*personaProfileJob{},
	}
}

func (l *personaLab) handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		action := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("action")))
		switch action {
		case "start-profiles":
			l.handleStartProfiles(w, r)
		case "profile-status":
			l.handleProfileStatus(w, r)
		case "", "generate":
			l.handleGenerate(w, r)
		default:
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "action must be generate, start-profiles, or profile-status"})
		}
	}
}

func (l *personaLab) handleGenerate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "GET only"})
		return
	}
	count, seed, profile := personaLabParams(r)
	l.writeGeneration(w, personapoc.Generate(seed, count, profile, true), seed, profile)
}

func (l *personaLab) writeGeneration(w http.ResponseWriter, run personapoc.Result, seed int64, profile string) {
	benchmarks := personapoc.Benchmark(seed+1000003, profile, []int{50, 100, 500, 1000})
	build := buildinfo.Current()
	_ = json.NewEncoder(w).Encode(map[string]any{
		"generated_at": time.Now().UTC(),
		"build_commit": build.Commit,
		"build_branch": build.Branch,
		"profiles": personapoc.Profiles(),
		"run": run,
		"benchmarks": benchmarks,
		"semantics": map[string]any{
			"api_calls": 0,
			"llm_calls": 0,
			"persona_bank": false,
			"host_profile_affects": "membership selection only",
			"canonical": false,
			"profile_generation_available": l.profileGeneration,
			"jev_profile_audit_available": l.advisor != nil,
			"profile_batch_size": personaLabProfileBatchSize,
			"note": "development PoC only; generated people and profiles are not inserted into the persistent world",
		},
	})
}

func personaLabParams(r *http.Request) (int, int64, string) {
	count := 100
	if raw := r.URL.Query().Get("count"); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil {
			count = n
		}
	}
	if count < 1 { count = 1 }
	if count > 2000 { count = 2000 }
	seed := int64(19960826)
	if raw := r.URL.Query().Get("seed"); raw != "" {
		if n, err := strconv.ParseInt(raw, 10, 64); err == nil {
			seed = n
		}
	}
	profile := strings.TrimSpace(r.URL.Query().Get("profile"))
	if profile == "" { profile = "general" }
	return count, seed, profile
}

func (l *personaLab) handleStartProfiles(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		w.WriteHeader(http.StatusMethodNotAllowed)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "POST only"})
		return
	}
	if !l.profileGeneration || l.generator == nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "OpenAI profile generation is not configured"})
		return
	}
	count, seed, profile := personaLabParams(r)
	profileCount := 20
	if raw := strings.TrimSpace(r.URL.Query().Get("profile_count")); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil {
			profileCount = n
		}
	}
	if profileCount < 1 || profileCount > personaLabMaxProfiles {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": fmt.Sprintf("profile_count must be between 1 and %d", personaLabMaxProfiles)})
		return
	}
	if profileCount > count {
		profileCount = count
	}
	if !publicLabAdmission.start(w, r, publicLabPhone, 1) {
		return
	}

	run := personapoc.Generate(seed, count, profile, true)
	targets := selectPersonaProfileTargets(run.Personas, profileCount)
	if len(targets) == 0 {
		publicLabAdmission.finish()
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "no persona targets generated"})
		return
	}

	l.mu.Lock()
	id := fmt.Sprintf("persona-%d-%04d", time.Now().UTC().Unix(), atomic.AddUint64(&l.seq, 1)%10000)
	job := &personaProfileJob{
		ID: id, Status: "queued", Seed: seed, ProfileID: run.Profile.ID, ProfileLabel: run.Profile.Label,
		AccountCount: run.Count, ProfileCount: len(targets), PoolSize: run.PoolSize, BatchSize: personaLabProfileBatchSize,
		IdentityGenerationUS: run.Timing.TotalUS, JevAvailable: l.advisor != nil,
		CreatedAt: time.Now().UTC(), Progress: personaProfileProgress{Total: len(targets)},
	}
	l.jobs[id] = job
	l.active = id
	l.mu.Unlock()

	_ = json.NewEncoder(w).Encode(job)
	go l.runProfiles(id, targets)
}

func selectPersonaProfileTargets(personas []personapoc.Identity, count int) []personapoc.Identity {
	out := append([]personapoc.Identity(nil), personas...)
	tierOrder := func(t string) int {
		switch t {
		case "core-candidate": return 0
		case "active": return 1
		default: return 2
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		ti, tj := tierOrder(out[i].DetailTier), tierOrder(out[j].DetailTier)
		if ti != tj { return ti < tj }
		si := out[i].VisitDaysPerWeek * (.35 + out[i].WriteBias + .45*out[i].ReplyBias)
		sj := out[j].VisitDaysPerWeek * (.35 + out[j].WriteBias + .45*out[j].ReplyBias)
		if si != sj { return si > sj }
		return out[i].ID < out[j].ID
	})
	if count < len(out) {
		out = out[:count]
	}
	return out
}

func personaProfileSeed(p personapoc.Identity) llm.PersonaProfileSeed {
	return llm.PersonaProfileSeed{
		ID: p.ID, Handle: p.Handle, Age: p.Age, Gender: p.Gender, Occupation: p.Occupation,
		ActivityClass: p.ActivityClass, VisitDaysPerWeek: p.VisitDaysPerWeek, LurkerBias: p.LurkerBias,
		WriteBias: p.WriteBias, ReplyBias: p.ReplyBias, ThreadStartBias: p.ThreadStartBias,
		TopInterests: append([]string(nil), p.TopInterests...), StyleTags: append([]string(nil), p.StyleTags...),
		ConnectWindow: p.ConnectWindow, Quirk: p.Quirk, DetailTier: p.DetailTier,
	}
}

func (l *personaLab) runProfiles(id string, targets []personapoc.Identity) {
	defer publicLabAdmission.finish()
	l.mu.Lock()
	job := l.jobs[id]
	job.Status = "running"
	job.StartedAt = time.Now().UTC()
	started := job.StartedAt
	l.mu.Unlock()

	for offset := 0; offset < len(targets); offset += personaLabProfileBatchSize {
		end := offset + personaLabProfileBatchSize
		if end > len(targets) { end = len(targets) }
		batch := targets[offset:end]
		seeds := make([]llm.PersonaProfileSeed, 0, len(batch))
		for _, p := range batch { seeds = append(seeds, personaProfileSeed(p)) }

		ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
		llmStarted := time.Now()
		draft, err := l.generator.GeneratePersonaProfiles(ctx, l.worldDate, seeds)
		llmDuration := time.Since(llmStarted)
		cancel()
		if err != nil {
			l.finishProfileError(id, fmt.Errorf("profile batch %d: %w", offset/personaLabProfileBatchSize+1, err))
			return
		}
		profileByID := make(map[string]string, len(draft.Profiles))
		auditItems := make([]worldengine.PersonaProfileAdviceItem, 0, len(draft.Profiles))
		for _, p := range draft.Profiles {
			profileByID[p.ID] = p.Profile
			auditItems = append(auditItems, worldengine.PersonaProfileAdviceItem{ID: p.ID, Profile: p.Profile})
		}

		var advice worldengine.PersonaProfileAdviceDecision
		var jevErr error
		var jevDuration time.Duration
		if l.advisor != nil {
			jevCtx, jevCancel := context.WithTimeout(context.Background(), 12*time.Second)
			jevStarted := time.Now()
			advice, jevErr = l.advisor.AdvisePersonaProfiles(jevCtx, worldengine.PersonaProfileAdviceRequest{WorldDate: l.worldDate, Profiles: auditItems})
			jevDuration = time.Since(jevStarted)
			jevCancel()
		}

		batchResult := personaProfileBatchResult{
			Batch: offset/personaLabProfileBatchSize + 1, Count: len(batch),
			LLMDurationMS: llmDuration.Milliseconds(), LLMModel: draft.Usage.Model,
			LLMInputTokens: draft.Usage.InputTokens, LLMOutputTokens: draft.Usage.OutputTokens, LLMTotalTokens: draft.Usage.TotalTokens,
			JevDurationMS: jevDuration.Milliseconds(), JevModel: advice.Model, JevInputTokens: advice.InputTokens,
		}
		if jevErr != nil { batchResult.JevError = jevErr.Error() }

		results := make([]personaProfileResult, 0, len(batch))
		for _, p := range batch {
			result := personaProfileResult{
				PersonaID: p.ID, Handle: p.Handle, DetailTier: p.DetailTier, Skeleton: p.ProfileSummary,
				Profile: profileByID[p.ID],
			}
			if l.advisor != nil && jevErr == nil {
				result.JevChecked = true
				prob := advice.Profiles[p.ID]
				result.FutureProbability = prob.FutureInformation
				result.ExternalReviewProbability = prob.ExternalReview
				result.FutureFlag = prob.FutureInformation >= personaLabFutureFlagThreshold
				result.ExternalReviewFlag = prob.ExternalReview >= personaLabFutureFlagThreshold
			}
			results = append(results, result)
		}

		l.mu.Lock()
		job = l.jobs[id]
		job.Batches = append(job.Batches, batchResult)
		job.Results = append(job.Results, results...)
		job.Progress.Completed += len(results)
		job.Summary.LLMCalls++
		job.Summary.LLMDurationMS += batchResult.LLMDurationMS
		job.Summary.LLMInputTokens += batchResult.LLMInputTokens
		job.Summary.LLMOutputTokens += batchResult.LLMOutputTokens
		job.Summary.LLMTotalTokens += batchResult.LLMTotalTokens
		if l.advisor != nil {
			job.Summary.JevCalls++
			job.Summary.JevDurationMS += batchResult.JevDurationMS
			job.Summary.JevInputTokens += batchResult.JevInputTokens
			if jevErr != nil { job.Summary.JevErrors++ }
		}
		for _, result := range results {
			if result.FutureFlag { job.Summary.FutureFlagged++ }
			if result.ExternalReviewFlag { job.Summary.ExternalReviewFlagged++ }
			if result.JevChecked && result.FutureProbability > job.Summary.MaxFutureProbability {
				job.Summary.MaxFutureProbability = result.FutureProbability
			}
			if result.JevChecked && result.ExternalReviewProbability > job.Summary.MaxExternalReview {
				job.Summary.MaxExternalReview = result.ExternalReviewProbability
			}
		}
		job.Summary.TotalDurationMS = time.Since(started).Milliseconds()
		l.mu.Unlock()
	}

	l.mu.Lock()
	job = l.jobs[id]
	job.Status = "completed"
	job.FinishedAt = time.Now().UTC()
	job.Summary.TotalDurationMS = job.FinishedAt.Sub(started).Milliseconds()
	if job.Summary.TotalDurationMS > 0 {
		job.Summary.ProfilesPerSecond = float64(job.ProfileCount) / (float64(job.Summary.TotalDurationMS) / 1000)
	}
	if l.active == id { l.active = "" }
	l.mu.Unlock()
}

func (l *personaLab) finishProfileError(id string, err error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	job := l.jobs[id]
	if job == nil { return }
	job.Status = "failed"
	job.Error = err.Error()
	job.FinishedAt = time.Now().UTC()
	if !job.StartedAt.IsZero() {
		job.Summary.TotalDurationMS = job.FinishedAt.Sub(job.StartedAt).Milliseconds()
	}
	if l.active == id { l.active = "" }
}

func (l *personaLab) handleProfileStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "GET only"})
		return
	}
	id := strings.TrimSpace(r.URL.Query().Get("id"))
	l.mu.Lock()
	defer l.mu.Unlock()
	if id == "" {
		if l.active != "" {
			_ = json.NewEncoder(w).Encode(l.jobs[l.active])
			return
		}
		var newest *personaProfileJob
		for _, job := range l.jobs {
			if newest == nil || job.CreatedAt.After(newest.CreatedAt) { newest = job }
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
