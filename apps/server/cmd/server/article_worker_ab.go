package main

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"zutto-pccom/apps/server/internal/llm"
	"zutto-pccom/apps/server/internal/world"
	"zutto-pccom/apps/server/internal/worldrepo"
)

type articleWorkerABArm struct {
	Model     string `json:"model"`
	Body      string `json:"body,omitempty"`
	Subject   string `json:"subject,omitempty"`
	LatencyMS int64  `json:"latency_ms"`
	Error     string `json:"error,omitempty"`
}

func newArticleWorkerABHandler(repo *worldrepo.Repository, base worldrepo.LLMMaterializer, gemini llm.BoardPostRenderer, geminiConfigured bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if !geminiConfigured {
			w.WriteHeader(http.StatusServiceUnavailable)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": "GEMINI_API_KEY is not configured"})
			return
		}
		phone := strings.TrimSpace(r.URL.Query().Get("phone"))
		if phone == "" {
			phone = "0450000196"
		}
		boardID := strings.TrimSpace(r.URL.Query().Get("board"))
		postID, err := strconv.ParseInt(strings.TrimSpace(r.URL.Query().Get("post")), 10, 64)
		if err != nil || boardID == "" {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": "board and numeric post are required"})
			return
		}
		host, err := repo.HostByPhone(phone)
		if err != nil {
			w.WriteHeader(http.StatusNotFound)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": err.Error()})
			return
		}
		boards, _ := repo.MaterializationBoards(host)
		var board world.Board
		found := false
		for _, b := range boards {
			if b.ID == boardID {
				board = b
				found = true
				break
			}
		}
		if !found {
			w.WriteHeader(http.StatusNotFound)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": "board not found"})
			return
		}
		req, decision, selected, err := repo.MaterializationArticleWorkerABInput(host, board, postID)
		if err != nil {
			w.WriteHeader(http.StatusBadGateway)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": err.Error()})
			return
		}

		openAI := base
		geminiMat := base
		geminiMat.Renderer = gemini
		type armResult struct {
			name string
			arm  articleWorkerABArm
		}
		ch := make(chan armResult, 2)
		run := func(name string, mat worldrepo.LLMMaterializer) {
			start := time.Now()
			ctx, cancel := context.WithTimeout(r.Context(), 70*time.Second)
			defer cancel()
			posts, usage, err := mat.GenerateBoardPostsWithUsage(ctx, req, decision)
			arm := articleWorkerABArm{Model: usage.Model, LatencyMS: time.Since(start).Milliseconds()}
			if err != nil {
				arm.Error = err.Error()
			} else if len(posts) == 0 {
				arm.Error = "no post returned"
			} else {
				arm.Subject = posts[0].Subject
				arm.Body = posts[0].Body
			}
			ch <- armResult{name: name, arm: arm}
		}
		var wg sync.WaitGroup
		wg.Add(2)
		go func() { defer wg.Done(); run("luna", openAI) }()
		go func() { defer wg.Done(); run("gemini", geminiMat) }()
		go func() { wg.Wait(); close(ch) }()
		arms := map[string]articleWorkerABArm{}
		for result := range ch {
			arms[result.name] = result.arm
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"canonical": map[string]any{"post": selected.ID, "board": board.Name, "author": selected.Author, "subject": selected.Subject, "worker_intent": worldrepo.DebugIntentSummary(req.Intent)},
			"luna":      arms["luna"], "gemini": arms["gemini"], "persisted": false,
		})
	}
}
