package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestDebugMaterializationAuditBaseURL(t *testing.T) {
	for _, tc := range []struct {
		addr string
		want string
	}{
		{addr: ":8080", want: "http://127.0.0.1:8080"},
		{addr: "0.0.0.0:10000", want: "http://127.0.0.1:10000"},
	} {
		got, err := debugMaterializationAuditBaseURL(tc.addr)
		if err != nil {
			t.Fatalf("addr=%q err=%v", tc.addr, err)
		}
		if got != tc.want {
			t.Fatalf("addr=%q got=%q want=%q", tc.addr, got, tc.want)
		}
	}
}

func TestRunDebugMaterializationAuditStartsAndCollectsCompletedJob(t *testing.T) {
	var starts atomic.Int32
	mux := http.NewServeMux()
	mux.HandleFunc("/api/debug/materialization-lab-fresh", func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Query().Get("action") {
		case "start":
			starts.Add(1)
			if r.Method != http.MethodPost {
				t.Fatalf("start method=%s want POST", r.Method)
			}
			for key, want := range map[string]string{
				"phone": debugMaterializationAuditPhone,
				"situation_mode": "title-first",
				"historical_texture": "model-memory",
				"era_gate": "observe-only",
				"board_count": "6",
				"shell_limit": "4",
			} {
				if got := r.URL.Query().Get(key); got != want {
					t.Fatalf("%s=%q want %q", key, got, want)
				}
			}
			_ = json.NewEncoder(w).Encode(materializationFreshJob{ID: "job-1", Status: "queued"})
		case "status":
			if r.URL.Query().Get("id") != "job-1" {
				t.Fatalf("status id=%q", r.URL.Query().Get("id"))
			}
			_ = json.NewEncoder(w).Encode(materializationFreshJob{
				ID: "job-1", Status: "completed", RuntimeState: "COMPLETED", PostCount: 1, BodyCount: 1,
				Articles: []materializationFreshArticle{{ID: 10, BoardID: "4", Subject: "ボスの攻撃が避けられない", Body: "本文"}},
			})
		default:
			http.Error(w, "bad action", http.StatusBadRequest)
		}
	})
	ts := httptest.NewServer(mux)
	defer ts.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	job, err := runDebugMaterializationAudit(ctx, ts.Client(), ts.URL)
	if err != nil {
		t.Fatal(err)
	}
	if starts.Load() != 1 {
		t.Fatalf("start calls=%d want 1", starts.Load())
	}
	if job.Status != "completed" || len(job.Articles) != 1 || !strings.Contains(job.Articles[0].Subject, "ボス") {
		t.Fatalf("unexpected job: %+v", job)
	}
}
