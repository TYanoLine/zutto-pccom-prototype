package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"zutto-pccom/apps/server/internal/world"
)

func TestDebugExportDoesNotMaterializeDemoHost(t *testing.T) {
	t.Setenv("ZUTTO_BUILD_COMMIT", "fedcba9876543210")
	t.Setenv("ZUTTO_BUILD_BRANCH", "debug-test")
	store := world.NewMemoryStore()
	before := store.ListPosts("materialize-demo")
	if len(before) != 0 {
		t.Fatalf("fixture unexpectedly has %d posts", len(before))
	}

	req := httptest.NewRequest(http.MethodGet, "/api/debug/export?phone=0450000196&full=1", nil)
	rr := httptest.NewRecorder()
	newDebugExportHandler(store).ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	if got := rr.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("Cache-Control=%q, want no-store", got)
	}

	var out debugRuntimeExport
	if err := json.NewDecoder(rr.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if !out.ReadOnly || out.Scope != "current-server-memory" {
		t.Fatalf("unexpected export metadata: %+v", out)
	}
	if out.Build.Commit != "fedcba9876543210" || out.Build.ShortCommit != "fedcba987654" || out.Build.Branch != "debug-test" {
		t.Fatalf("unexpected build metadata: %+v", out.Build)
	}
	if out.Host.ID != "materialize-demo" || out.Host.Phone != "0450000196" {
		t.Fatalf("unexpected host: %+v", out.Host)
	}
	if out.Counts.TotalHostPosts != 0 || len(out.Posts) != 0 {
		t.Fatalf("export unexpectedly materialized posts: counts=%+v posts=%+v", out.Counts, out.Posts)
	}
	if after := store.ListPosts("materialize-demo"); len(after) != 0 {
		t.Fatalf("debug export mutated store: %d posts after export", len(after))
	}
}

func TestDebugExportFiltersBoardAndBodies(t *testing.T) {
	store := world.NewMemoryStore()
	host, err := store.HostByPhone("0920000196")
	if err != nil {
		t.Fatal(err)
	}
	all := store.ListPosts(host.ID)
	if len(all) == 0 {
		t.Fatal("expected seeded posts")
	}

	req := httptest.NewRequest(http.MethodGet, "/api/debug/export?phone=0920000196&board=60/1", nil)
	rr := httptest.NewRecorder()
	newDebugExportHandler(store).ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	var summary debugRuntimeExport
	if err := json.NewDecoder(rr.Body).Decode(&summary); err != nil {
		t.Fatal(err)
	}
	if summary.BodiesIncluded {
		t.Fatal("summary export should omit bodies")
	}
	if summary.Counts.TotalHostPosts != len(all) || summary.Counts.ReturnedPosts == 0 {
		t.Fatalf("unexpected counts: %+v", summary.Counts)
	}
	for _, post := range summary.Posts {
		if post.BoardID != "60/1" {
			t.Fatalf("unexpected board %q", post.BoardID)
		}
		if post.Body != "" {
			t.Fatalf("summary leaked body for post %d", post.ID)
		}
	}

	fullReq := httptest.NewRequest(http.MethodGet, "/api/debug/export?phone=0920000196&board=60/1&full=true", nil)
	fullRR := httptest.NewRecorder()
	newDebugExportHandler(store).ServeHTTP(fullRR, fullReq)
	var full debugRuntimeExport
	if err := json.NewDecoder(fullRR.Body).Decode(&full); err != nil {
		t.Fatal(err)
	}
	if !full.BodiesIncluded || len(full.Posts) == 0 || strings.TrimSpace(full.Posts[0].Body) == "" {
		t.Fatalf("full export did not include body: %+v", full)
	}
}

func TestDebugExportRequiresPhoneAndGET(t *testing.T) {
	store := world.NewMemoryStore()

	missing := httptest.NewRecorder()
	newDebugExportHandler(store).ServeHTTP(missing, httptest.NewRequest(http.MethodGet, "/api/debug/export", nil))
	if missing.Code != http.StatusBadRequest {
		t.Fatalf("missing phone status=%d", missing.Code)
	}

	post := httptest.NewRecorder()
	newDebugExportHandler(store).ServeHTTP(post, httptest.NewRequest(http.MethodPost, "/api/debug/export?phone=0450000196", nil))
	if post.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST status=%d", post.Code)
	}
}
