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

func TestGenerationTraceHandlerRequiresTokenAndGet(t *testing.T) {
	source := &testTraceSource{}
	handler := newHakataTraceHandler("operator-test-key", true, source)
	for _, tc := range []struct { method, token string; status int }{
		{"GET", "", http.StatusForbidden},
		{"GET", "incorrect", http.StatusForbidden},
		{"POST", "operator-test-key", http.StatusMethodNotAllowed},
		{"GET", "operator-test-key", http.StatusOK},
	} {
		request := httptest.NewRequest(tc.method, "/api/debug/bbs/generation-trace", nil)
		if tc.token != "" { request.Header.Set("X-Zutto-Debug-Token", tc.token) }
		response := httptest.NewRecorder()
		handler(response, request)
		if response.Code != tc.status { t.Fatalf("%s token=%q status=%d wanted=%d", tc.method, tc.token, response.Code, tc.status) }
		if response.Header().Get("Cache-Control") != "no-store" { t.Fatal("trace response must be uncached") }
		if tc.status == http.StatusOK {
			var snapshot worldrepo.GenerationTraceSnapshot
			if err := json.Unmarshal(response.Body.Bytes(), &snapshot); err != nil || len(snapshot.Runs) != 1 {
				t.Fatalf("invalid trace snapshot: %v, %+v", err, snapshot)
			}
		}
	}
	if source.calls != 1 { t.Fatalf("unauthorized requests accessed traces: %d", source.calls) }
}

func TestGenerationTraceHandlerDisabledWithoutSecret(t *testing.T) {
	for _, cfg := range []struct{ token string; enabled bool }{
		{token:"", enabled:true}, {token:"test-key",enabled:false},
	} {
		source := &testTraceSource{}
		request := httptest.NewRequest("GET", "/api/debug/bbs/generation-trace", nil)
		request.Header.Set("X-Zutto-Debug-Token", "test-key")
		response := httptest.NewRecorder()
		newHakataTraceHandler(cfg.token, cfg.enabled, source)(response, request)
		if response.Code != http.StatusForbidden || source.calls != 0 {
			t.Fatalf("disabled trace exposed data: status=%d calls=%d", response.Code, source.calls)
		}
	}
}
