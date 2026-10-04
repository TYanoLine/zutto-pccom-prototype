package main

import (
    "encoding/json"
    "net/http"
    "net/http/httptest"
    "testing"

    "zutto-pccom/apps/server/internal/worldrepo"
)

type testTraceSource struct { calls int }

func (s *testTraceSource) GenerationTraceSnapshot() worldrepo.GenerationTraceSnapshot {
    s.calls++
    return worldrepo.GenerationTraceSnapshot{Runs: []worldrepo.GenerationTraceRun{{ID:"test", Status:"completed"}}}
}

func TestGenerationTraceHandlerIsReadOnlyAndNeedsNoToken(t *testing.T) {
    source := &testTraceSource{}
    handler := newGenerationTraceHandler(true, source)
    for _, tc := range []struct { method, token string; status int }{
        {"GET", "", http.StatusOK},
        {"GET", "incorrect", http.StatusOK},
        {"POST", "", http.StatusMethodNotAllowed},
    } {
        request := httptest.NewRequest(tc.method, "/api/debug/bbs/generation-trace", nil)
        if tc.token != "" { request.Header.Set("X-Zutto-Debug-Token", tc.token) }
        response := httptest.NewRecorder()
        handler(response, request)
        if response.Code != tc.status { t.Fatalf("%s token=%q status=%d wanted=%d", tc.method, tc.token, response.Code, tc.status) }
        if response.Header().Get("Cache-Control") != "no-store" { t.Fatal("trace response must be uncached") }
        if response.Header().Get("X-Robots-Tag") != "noindex, nofollow, noarchive" { t.Fatal("trace response should discourage indexing") }
        if tc.status == http.StatusOK {
            var snapshot worldrepo.GenerationTraceSnapshot
            if err := json.Unmarshal(response.Body.Bytes(), &snapshot); err != nil || len(snapshot.Runs) != 1 {
                t.Fatalf("invalid trace snapshot: %v, %+v", err, snapshot)
            }
        }
    }
    if source.calls != 2 { t.Fatalf("read-only trace requests must not trigger generation: calls=%d", source.calls) }
}

func TestGenerationTraceHandlerDisabledIgnoresToken(t *testing.T) {
    source := &testTraceSource{}
    request := httptest.NewRequest(http.MethodGet, "/api/debug/bbs/generation-trace", nil)
    request.Header.Set("X-Zutto-Debug-Token", "legacy-debug-key")
    response := httptest.NewRecorder()
    newGenerationTraceHandler(false, source)(response, request)
    if response.Code != http.StatusForbidden || source.calls != 0 {
        t.Fatalf("disabled trace exposed data: status=%d calls=%d", response.Code, source.calls)
    }
}
