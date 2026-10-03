package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestServerBuildInfoFromEnvironment(t *testing.T) {
	t.Setenv("RENDER_GIT_COMMIT", "0123456789abcdef")
	t.Setenv("RENDER_GIT_BRANCH", "main")
	t.Setenv("GIT_COMMIT", "different")
	started := time.Date(2026, 10, 1, 12, 15, 0, 0, time.FixedZone("JST", 9*60*60))
	got := serverBuildInfoFromEnvironment(started)
	if got.Commit != "0123456789abcdef" || got.Branch != "main" {
		t.Fatalf("wrong deploy metadata: %+v", got)
	}
	if got.StartedAt.Format(time.RFC3339) != "2026-10-01T03:15:00Z" {
		t.Fatalf("start time should be UTC: %s", got.StartedAt)
	}
}

func TestServerBuildInfoFallbackIsExplicit(t *testing.T) {
	for _, key := range []string{"RENDER_GIT_COMMIT", "RENDER_GIT_BRANCH", "GIT_COMMIT", "GIT_BRANCH"} {
		t.Setenv(key, "")
	}
	got := serverBuildInfoFromEnvironment(time.Time{})
	if got.Commit != "unknown" || got.Branch != "unknown" {
		t.Fatalf("missing metadata must not claim a revision: %+v", got)
	}
}

func TestServerVersionHandler(t *testing.T) {
	info := serverBuildInfo{Commit: "abcd1234", Branch: "main", StartedAt: time.Date(2026, 10, 1, 3, 0, 0, 0, time.UTC)}
	handler := newServerVersionHandler(info)
	get := httptest.NewRecorder()
	handler.ServeHTTP(get, httptest.NewRequest(http.MethodGet, "/api/version", nil))
	if get.Code != http.StatusOK || get.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("GET code=%d cache=%q", get.Code, get.Header().Get("Cache-Control"))
	}
	var got serverBuildInfo
	if err := json.Unmarshal(get.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got != info {
		t.Fatalf("got %+v, want %+v", got, info)
	}
	post := httptest.NewRecorder()
	handler.ServeHTTP(post, httptest.NewRequest(http.MethodPost, "/api/version", nil))
	if post.Code != http.StatusMethodNotAllowed || post.Header().Get("Allow") != "GET" {
		t.Fatalf("POST code=%d allow=%q", post.Code, post.Header().Get("Allow"))
	}
}
