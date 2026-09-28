package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	debugMaterializationAuditPhone        = "0450000196"
	debugMaterializationAuditPollInterval = 2 * time.Second
	debugMaterializationAuditTimeout      = 12 * time.Minute
)

func startDebugMaterializationAudit(addr string) {
	baseURL, err := debugMaterializationAuditBaseURL(addr)
	if err != nil {
		log.Printf("DEBUG materialization audit skipped: %v", err)
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), debugMaterializationAuditTimeout)
		defer cancel()
		client := &http.Client{Timeout: 20 * time.Second}
		if err := waitForDebugAuditServer(ctx, client, baseURL); err != nil {
			log.Printf("DEBUG materialization audit startup failed: %v", err)
			return
		}
		job, err := runDebugMaterializationAudit(ctx, client, baseURL)
		if err != nil {
			log.Printf("DEBUG materialization audit failed: %v", err)
			return
		}
		log.Printf("DEBUG materialization audit completed: id=%s commit=%s status=%s duration_ms=%d posts=%d bodies=%d failures=%d empty=%v usage=%q",
			job.ID, job.BuildCommit, job.Status, job.DurationMS, job.PostCount, job.BodyCount, job.Failures, job.EmptyPostIDs, job.Usage)
		for _, article := range job.Articles {
			if article.BoardID != "4" {
				continue
			}
			payload, marshalErr := json.Marshal(map[string]any{
				"id": article.ID,
				"parent_id": article.ParentID,
				"author": article.Author,
				"created_at": article.CreatedAt,
				"subject": article.Subject,
				"body": article.Body,
				"discourse_mode": article.DiscourseMode,
				"situation_summary": article.SituationSummary,
				"situation_facts": article.SituationFacts,
			})
			if marshalErr != nil {
				log.Printf("DEBUG materialization audit GAME article encode failed: id=%d err=%v", article.ID, marshalErr)
				continue
			}
			log.Printf("DEBUG materialization audit GAME article: %s", payload)
		}
	}()
}

func debugMaterializationAuditBaseURL(addr string) (string, error) {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return "", fmt.Errorf("empty server address")
	}
	_, port, err := net.SplitHostPort(addr)
	if err != nil {
		return "", fmt.Errorf("parse server address %q: %w", addr, err)
	}
	if port == "" {
		return "", fmt.Errorf("server address %q has no port", addr)
	}
	return "http://127.0.0.1:" + port, nil
}

func waitForDebugAuditServer(ctx context.Context, client *http.Client, baseURL string) error {
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	for {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/health", nil)
		if err != nil {
			return err
		}
		resp, err := client.Do(req)
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				return nil
			}
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

func runDebugMaterializationAudit(ctx context.Context, client *http.Client, baseURL string) (*materializationFreshJob, error) {
	startURL, err := url.Parse(baseURL + "/api/debug/materialization-lab-fresh")
	if err != nil {
		return nil, err
	}
	query := startURL.Query()
	query.Set("action", "start")
	query.Set("phone", debugMaterializationAuditPhone)
	query.Set("situation_mode", "title-first")
	query.Set("historical_texture", "model-memory")
	query.Set("era_gate", "observe-only")
	query.Set("board_count", "4")
	query.Set("shell_limit", "3")
	startURL.RawQuery = query.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, startURL.String(), nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var payload map[string]any
		_ = json.NewDecoder(resp.Body).Decode(&payload)
		return nil, fmt.Errorf("start lab status=%d payload=%v", resp.StatusCode, payload)
	}
	var started materializationFreshJob
	if err := json.NewDecoder(resp.Body).Decode(&started); err != nil {
		return nil, fmt.Errorf("decode start response: %w", err)
	}
	if strings.TrimSpace(started.ID) == "" {
		return nil, fmt.Errorf("start response did not include a job id")
	}
	log.Printf("DEBUG materialization audit started: id=%s", started.ID)

	statusURL, err := url.Parse(baseURL + "/api/debug/materialization-lab-fresh")
	if err != nil {
		return nil, err
	}
	statusQuery := statusURL.Query()
	statusQuery.Set("action", "status")
	statusQuery.Set("id", started.ID)
	statusURL.RawQuery = statusQuery.Encode()

	ticker := time.NewTicker(debugMaterializationAuditPollInterval)
	defer ticker.Stop()
	for {
		statusReq, err := http.NewRequestWithContext(ctx, http.MethodGet, statusURL.String(), nil)
		if err != nil {
			return nil, err
		}
		statusResp, err := client.Do(statusReq)
		if err != nil {
			return nil, err
		}
		var job materializationFreshJob
		decodeErr := json.NewDecoder(statusResp.Body).Decode(&job)
		_ = statusResp.Body.Close()
		if decodeErr != nil {
			return nil, fmt.Errorf("decode status response: %w", decodeErr)
		}
		if statusResp.StatusCode < 200 || statusResp.StatusCode >= 300 {
			return nil, fmt.Errorf("status lab status=%d error=%s", statusResp.StatusCode, job.Error)
		}
		switch job.Status {
		case "completed":
			return &job, nil
		case "failed":
			return &job, fmt.Errorf("lab job %s failed: %s", job.ID, job.Error)
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
		}
	}
}
