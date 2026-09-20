package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"zutto-pccom/apps/server/internal/llm"
)

const personaHistoryPocRounds = 3

type personaHistoryGenerator interface {
	GeneratePersonaHistoryPosts(context.Context, string, int, []llm.PersonaHistoryPostInput) (llm.PersonaHistoryPostBatch, error)
	ExtractPersonaHistory(context.Context, string, int, []llm.PersonaHistoryExtractInput) (llm.PersonaHistoryCandidateBatch, error)
}

type personaHistoryLab struct {
	generator personaHistoryGenerator
	worldDate string
	available bool

	mu     sync.Mutex
	jobs   map[string]*personaHistoryJob
	active string
	seq    uint64
}

type personaHistoryJob struct {
	ID         string                       `json:"id"`
	Status     string                       `json:"status"`
	WorldDate  string                       `json:"world_date"`
	Profiles   []llm.PersonaHistoryProfile  `json:"profiles"`
	CreatedAt  time.Time                    `json:"created_at"`
	StartedAt  time.Time                    `json:"started_at,omitempty"`
	FinishedAt time.Time                    `json:"finished_at,omitempty"`
	Progress   personaHistoryProgress       `json:"progress"`
	Rounds     []personaHistoryRound        `json:"rounds,omitempty"`
	History    map[string][]llm.PersonaHistoryEntry `json:"history"`
	Summary    personaHistorySummary        `json:"summary"`
	Error      string                       `json:"error,omitempty"`
}

type personaHistoryProgress struct {
	CompletedSteps int `json:"completed_steps"`
	TotalSteps     int `json:"total_steps"`
}

type personaHistoryRound struct {
	Round        int                                      `json:"round"`
	Situations   []personaHistorySituation                `json:"situations"`
	Posts        []personaHistoryPostResult               `json:"posts"`
	Decisions    []personaHistoryDecision                 `json:"decisions"`
	HistoryAfter map[string][]llm.PersonaHistoryEntry     `json:"history_after"`
	PostUsage    llm.TokenUsage                           `json:"post_usage"`
	ExtractUsage llm.TokenUsage                           `json:"extract_usage"`
}

type personaHistorySituation struct {
	PersonaID string `json:"persona_id"`
	Board     string `json:"board"`
	Situation string `json:"situation"`
}

type personaHistoryPostResult struct {
	ID        string `json:"id"`
	PersonaID string `json:"persona_id"`
	Subject   string `json:"subject"`
	Body      string `json:"body"`
}

type personaHistoryDecision struct {
	Candidate llm.PersonaHistoryCandidate `json:"candidate"`
	Accepted  bool                        `json:"accepted"`
	Reason    string                      `json:"reason"`
}

type personaHistorySummary struct {
	PostCalls       int   `json:"post_calls"`
	ExtractCalls    int   `json:"extract_calls"`
	Accepted        int   `json:"accepted"`
	Rejected        int   `json:"rejected"`
	InputTokens     int   `json:"input_tokens"`
	OutputTokens    int   `json:"output_tokens"`
	TotalTokens     int   `json:"total_tokens"`
	TotalDurationMS int64 `json:"total_duration_ms"`
}

func newPersonaHistoryLab(generator personaHistoryGenerator, worldDate string, available bool) *personaHistoryLab {
	return &personaHistoryLab{
		generator: generator,
		worldDate: worldDate,
		available: available,
		jobs:      map[string]*personaHistoryJob{},
	}
}

func (l *personaHistoryLab) handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		switch strings.ToLower(strings.TrimSpace(r.URL.Query().Get("action"))) {
		case "start":
			l.handleStart(w, r)
		case "status", "":
			l.handleStatus(w, r)
		default:
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "action must be start or status"})
		}
	}
}

func (l *personaHistoryLab) handleStart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		w.WriteHeader(http.StatusMethodNotAllowed)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "POST only"})
		return
	}
	if !l.available || l.generator == nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "OpenAI persona-history generation is not configured"})
		return
	}
	if !publicLabAdmission.start(w, r, publicLabPhone, 1) {
		return
	}

	profiles := personaHistoryPocProfiles()
	now := time.Now().UTC()
	id := fmt.Sprintf("history-%d-%04d", now.Unix(), atomic.AddUint64(&l.seq, 1)%10000)
	job := &personaHistoryJob{
		ID:        id,
		Status:    "queued",
		WorldDate: l.worldDate,
		Profiles:  profiles,
		CreatedAt: now,
		Progress:  personaHistoryProgress{TotalSteps: personaHistoryPocRounds * 2},
		History:   make(map[string][]llm.PersonaHistoryEntry, len(profiles)),
	}
	for _, profile := range profiles {
		job.History[profile.ID] = []llm.PersonaHistoryEntry{}
	}

	l.mu.Lock()
	l.jobs[id] = job
	l.active = id
	l.mu.Unlock()

	_ = json.NewEncoder(w).Encode(job)
	go l.run(id)
}

func (l *personaHistoryLab) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET")
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
		var newest *personaHistoryJob
		for _, job := range l.jobs {
			if newest == nil || job.CreatedAt.After(newest.CreatedAt) {
				newest = job
			}
		}
		if newest == nil {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status": "idle",
				"available": l.available,
				"profiles": personaHistoryPocProfiles(),
				"rounds": personaHistoryPocRounds,
			})
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

func (l *personaHistoryLab) run(id string) {
	defer publicLabAdmission.finish()

	l.mu.Lock()
	job := l.jobs[id]
	job.Status = "running"
	job.StartedAt = time.Now().UTC()
	started := job.StartedAt
	profiles := append([]llm.PersonaHistoryProfile(nil), job.Profiles...)
	history := clonePersonaHistory(job.History)
	l.mu.Unlock()

	for round := 1; round <= personaHistoryPocRounds; round++ {
		situations := personaHistoryPocSituations(round)
		inputs := make([]llm.PersonaHistoryPostInput, 0, len(profiles))
		for _, profile := range profiles {
			situation := personaHistorySituationFor(situations, profile.ID)
			inputs = append(inputs, llm.PersonaHistoryPostInput{
				Profile: profile,
				History: append([]llm.PersonaHistoryEntry(nil), history[profile.ID]...),
				Board: situation.Board,
				Situation: situation.Situation,
			})
		}

		postCtx, postCancel := context.WithTimeout(context.Background(), 90*time.Second)
		postBatch, err := l.generator.GeneratePersonaHistoryPosts(postCtx, l.worldDate, round, inputs)
		postCancel()
		if err != nil {
			l.fail(id, fmt.Errorf("round %d post generation: %w", round, err), started)
			return
		}

		postResults := make([]personaHistoryPostResult, 0, len(postBatch.Posts))
		extractInputs := make([]llm.PersonaHistoryExtractInput, 0, len(postBatch.Posts))
		profileByID := make(map[string]llm.PersonaHistoryProfile, len(profiles))
		for _, profile := range profiles {
			profileByID[profile.ID] = profile
		}
		for i, post := range postBatch.Posts {
			postID := fmt.Sprintf("R%d-P%02d", round, i+1)
			postResults = append(postResults, personaHistoryPostResult{
				ID: postID, PersonaID: post.PersonaID, Subject: post.Subject, Body: post.Body,
			})
			extractInputs = append(extractInputs, llm.PersonaHistoryExtractInput{
				Profile: profileByID[post.PersonaID],
				History: append([]llm.PersonaHistoryEntry(nil), history[post.PersonaID]...),
				Post: post,
			})
		}

		l.mu.Lock()
		job = l.jobs[id]
		job.Progress.CompletedSteps++
		job.Summary.PostCalls++
		addPersonaHistoryUsage(&job.Summary, postBatch.Usage)
		l.mu.Unlock()

		extractCtx, extractCancel := context.WithTimeout(context.Background(), 90*time.Second)
		candidateBatch, err := l.generator.ExtractPersonaHistory(extractCtx, l.worldDate, round, extractInputs)
		extractCancel()
		if err != nil {
			l.fail(id, fmt.Errorf("round %d history extraction: %w", round, err), started)
			return
		}

		postByPersona := make(map[string]personaHistoryPostResult, len(postResults))
		for _, post := range postResults {
			postByPersona[post.PersonaID] = post
		}
		decisions := make([]personaHistoryDecision, 0, len(candidateBatch.Candidates))
		for _, candidate := range candidateBatch.Candidates {
			post, ok := postByPersona[candidate.PersonaID]
			if !ok {
				decisions = append(decisions, personaHistoryDecision{Candidate: candidate, Accepted: false, Reason: "投稿が見つからない"})
				continue
			}
			accepted, reason := applyPersonaHistoryCandidate(history, round, post.ID, post.Body, candidate)
			decisions = append(decisions, personaHistoryDecision{Candidate: candidate, Accepted: accepted, Reason: reason})
		}

		roundResult := personaHistoryRound{
			Round: round,
			Situations: situations,
			Posts: postResults,
			Decisions: decisions,
			HistoryAfter: clonePersonaHistory(history),
			PostUsage: postBatch.Usage,
			ExtractUsage: candidateBatch.Usage,
		}

		l.mu.Lock()
		job = l.jobs[id]
		job.Rounds = append(job.Rounds, roundResult)
		job.History = clonePersonaHistory(history)
		job.Progress.CompletedSteps++
		job.Summary.ExtractCalls++
		addPersonaHistoryUsage(&job.Summary, candidateBatch.Usage)
		for _, decision := range decisions {
			if decision.Accepted {
				job.Summary.Accepted++
			} else {
				job.Summary.Rejected++
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
	if l.active == id {
		l.active = ""
	}
	l.mu.Unlock()
}

func addPersonaHistoryUsage(summary *personaHistorySummary, usage llm.TokenUsage) {
	summary.InputTokens += usage.InputTokens
	summary.OutputTokens += usage.OutputTokens
	summary.TotalTokens += usage.TotalTokens
}

func (l *personaHistoryLab) fail(id string, err error, started time.Time) {
	l.mu.Lock()
	defer l.mu.Unlock()
	job := l.jobs[id]
	if job == nil {
		return
	}
	job.Status = "failed"
	job.Error = err.Error()
	job.FinishedAt = time.Now().UTC()
	job.Summary.TotalDurationMS = job.FinishedAt.Sub(started).Milliseconds()
	if l.active == id {
		l.active = ""
	}
}

func personaHistoryPocProfiles() []llm.PersonaHistoryProfile {
	return []llm.PersonaHistoryProfile{
		{ID: "P01", Handle: "RYO-5", Age: 29, Occupation: "会社員", Interests: []string{"パソコン通信", "ゲーム"}, Activity: "週に数回。返信はするが新規発言は多くない"},
		{ID: "P02", Handle: "EMI-2", Age: 20, Occupation: "大学生", Interests: []string{"ゲーム", "音楽"}, Activity: "夜の接続が中心。雑談板も読む"},
		{ID: "P03", Handle: "NOBU", Age: 34, Occupation: "技術職", Interests: []string{"ソフトウェア", "モデム"}, Activity: "技術系の質問には比較的よく返信する"},
		{ID: "P04", Handle: "みき", Age: 24, Occupation: "販売・サービス業", Interests: []string{"音楽", "地域の話題"}, Activity: "雑談中心。短い発言も多い"},
		{ID: "P05", Handle: "TAKA", Age: 18, Occupation: "専門学校生", Interests: []string{"ゲーム", "ソフトウェア"}, Activity: "ROM寄りだが、分かる話題には時々書く"},
	}
}

func personaHistoryPocSituations(round int) []personaHistorySituation {
	switch round {
	case 1:
		return []personaHistorySituation{
			{PersonaID: "P01", Board: "通信雑談", Situation: "最近の接続や通信まわりで経験した小さな出来事を一つ書く。新しい個人設定を無理に作らない。"},
			{PersonaID: "P02", Board: "ゲーム雑談", Situation: "最近遊んでいるゲームや遊び方について、雑談として一つ書く。"},
			{PersonaID: "P03", Board: "通信Q&A", Situation: "モデム設定で困っている会員への返信。自分の経験が本当に役立つ範囲だけ書く。"},
			{PersonaID: "P04", Board: "音楽雑談", Situation: "最近よく聴く音楽や聴き方について軽く雑談する。"},
			{PersonaID: "P05", Board: "ソフト雑談", Situation: "ソフトやファイルを扱っていて気づいたことを短く書く。"},
		}
	case 2:
		return []personaHistorySituation{
			{PersonaID: "P01", Board: "雑談", Situation: "平日のパソコン通信の使い方について自然な雑談が続いている。参加するなら自分の実情を書いてよい。"},
			{PersonaID: "P02", Board: "雑談", Situation: "学校生活と趣味の時間の使い方について話題が出ている。無理のない範囲で返信する。"},
			{PersonaID: "P03", Board: "ソフトQ&A", Situation: "初心者が設定を一度に変えて混乱したという相談。自分なりの助言を書く。"},
			{PersonaID: "P04", Board: "地域雑談", Situation: "休みの日に近所でどう過ごすか、という軽い地域雑談。"},
			{PersonaID: "P05", Board: "ゲーム雑談", Situation: "ゲームを途中でやめたり続けたりする基準について雑談している。"},
		}
	default:
		return []personaHistorySituation{
			{PersonaID: "P01", Board: "通信雑談", Situation: "別の会員が通信トラブルの失敗談を書いた。共感する、補足する、あるいは特に言うことがなければ短く反応する。"},
			{PersonaID: "P02", Board: "ゲーム雑談", Situation: "他の会員が長時間遊びすぎたという話を書いた。自分の経験や考えがあれば返信する。"},
			{PersonaID: "P03", Board: "通信Q&A", Situation: "以前の助言に対して『直りました』という報告が来た。返事をする。"},
			{PersonaID: "P04", Board: "音楽雑談", Situation: "他の会員が『同じ曲を何度も聴く』という話をしている。自分の聴き方と比べて返事をしてよい。"},
			{PersonaID: "P05", Board: "ソフト雑談", Situation: "他の会員が操作ミスをしてしまったという話題。何か言うなら自分らしい範囲で返信する。"},
		}
	}
}

func personaHistorySituationFor(values []personaHistorySituation, personaID string) personaHistorySituation {
	for _, value := range values {
		if value.PersonaID == personaID {
			return value
		}
	}
	return personaHistorySituation{PersonaID: personaID, Board: "雑談", Situation: "自然に一言書く。"}
}

func applyPersonaHistoryCandidate(history map[string][]llm.PersonaHistoryEntry, round int, postID, body string, candidate llm.PersonaHistoryCandidate) (bool, string) {
	if !validPersonaHistoryKind(candidate.Kind) {
		return false, "未対応のkind"
	}
	if !validPersonaHistoryKey(candidate.Key) {
		return false, "key形式が不正"
	}
	evidence := strings.TrimSpace(candidate.Evidence)
	if evidence == "" || !strings.Contains(body, evidence) {
		return false, "evidenceが投稿本文の完全一致抜粋ではない"
	}
	if len([]rune(evidence)) > 80 {
		return false, "evidenceが長すぎる"
	}
	if candidate.Confidence < 0 || candidate.Confidence > 1 {
		return false, "confidenceが範囲外"
	}
	if candidate.Kind == "observed_behavior" && candidate.Confidence > 0.65 {
		candidate.Confidence = 0.65
	}

	entries := history[candidate.PersonaID]
	for i := range entries {
		entry := &entries[i]
		if entry.Kind == candidate.Kind && entry.Key == candidate.Key && entry.Value == candidate.Value {
			entry.LastRound = round
			entry.Observations++
			if !containsPersonaHistoryEvidence(entry.Evidence, evidence) {
				entry.Evidence = append(entry.Evidence, fmt.Sprintf("%s: %s", postID, evidence))
			}
			if candidate.Confidence > entry.Confidence {
				entry.Confidence = candidate.Confidence
			}
			history[candidate.PersonaID] = entries
			return true, "既存historyを補強"
		}
		if entry.Kind == "self_fact" && candidate.Kind == "self_fact" && entry.Key == candidate.Key && entry.Value != candidate.Value {
			return false, "既存self_factと矛盾"
		}
	}

	entries = append(entries, llm.PersonaHistoryEntry{
		Kind: candidate.Kind,
		Key: candidate.Key,
		Value: candidate.Value,
		Confidence: candidate.Confidence,
		FirstRound: round,
		LastRound: round,
		Observations: 1,
		Evidence: []string{fmt.Sprintf("%s: %s", postID, evidence)},
	})
	history[candidate.PersonaID] = entries
	return true, "新しいhistoryとして採用"
}

func validPersonaHistoryKind(value string) bool {
	switch value {
	case "self_fact", "preference", "experience", "temporary_state", "observed_behavior":
		return true
	default:
		return false
	}
}

func validPersonaHistoryKey(value string) bool {
	if value == "" || len(value) > 80 {
		return false
	}
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= '0' && r <= '9':
		case r == '.', r == '_', r == '-':
		default:
			return false
		}
	}
	return true
}

func containsPersonaHistoryEvidence(values []string, evidence string) bool {
	for _, value := range values {
		if strings.HasSuffix(value, ": "+evidence) || value == evidence {
			return true
		}
	}
	return false
}

func clonePersonaHistory(in map[string][]llm.PersonaHistoryEntry) map[string][]llm.PersonaHistoryEntry {
	out := make(map[string][]llm.PersonaHistoryEntry, len(in))
	for id, entries := range in {
		cloned := make([]llm.PersonaHistoryEntry, len(entries))
		for i, entry := range entries {
			cloned[i] = entry
			cloned[i].Evidence = append([]string(nil), entry.Evidence...)
		}
		out[id] = cloned
	}
	return out
}
