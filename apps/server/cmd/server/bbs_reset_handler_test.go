package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"zutto-pccom/apps/server/internal/hostcatalog"
	"zutto-pccom/apps/server/internal/world"
)

type testResetStore struct {
	hosts           map[string]world.Host
	running         bool
	prepareCalls    int
	resetGenCalls   int
	removedPosts    int
	keptPosts       int
	operationResult bool
}

func (s *testResetStore) HostByPhone(phone string) (world.Host, error) {
	if h, ok := s.hosts[phone]; ok {
		return h, nil
	}
	return world.Host{}, errors.New("host not found")
}

func (s *testResetStore) MaterializationObservationRunning(hostID string) bool {
	return s.running
}

func (s *testResetStore) PrepareDebugBBSConnection(host world.Host) (int, int, bool) {
	s.prepareCalls++
	return s.removedPosts, s.keptPosts, s.operationResult
}

func (s *testResetStore) ResetBBSGeneratedArticles(host world.Host) (int, int, bool) {
	s.resetGenCalls++
	return s.removedPosts, s.keptPosts, s.operationResult
}

func TestBBSResetHandlerRequiresPost(t *testing.T) {
	base := world.NewMemoryStore()
	mock := &testResetStore{}
	handler := newBBSResetHandler("", base, mock)

	req := httptest.NewRequest(http.MethodGet, "/api/debug/bbs/reset", nil)
	rec := httptest.NewRecorder()
	handler(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405 Method Not Allowed, got %d", rec.Code)
	}
}

func TestBBSResetHandlerTokenValidation(t *testing.T) {
	base := world.NewMemoryStore()
	mock := &testResetStore{
		hosts: map[string]world.Host{
			"0920000196": {ID: "hakata-canal-net", Phone: "0920000196", Debug: hostcatalog.DebugFlags{HTTPEndpoints: true}},
		},
		operationResult: true,
	}

	handler := newBBSResetHandler("secret-token", base, mock)

	// Missing / wrong token: 403
	req := httptest.NewRequest(http.MethodPost, "/api/debug/bbs/reset?phone=0920000196", nil)
	rec := httptest.NewRecorder()
	handler(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden with missing token, got %d", rec.Code)
	}

	// Correct token: 200
	req = httptest.NewRequest(http.MethodPost, "/api/debug/bbs/reset?phone=0920000196", nil)
	req.Header.Set("X-Zutto-Debug-Token", "secret-token")
	rec = httptest.NewRecorder()
	handler(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK with correct token, got %d", rec.Code)
	}
}

func TestBBSResetHandlerModesAndDefaultPhone(t *testing.T) {
	base := world.NewMemoryStore()
	mock := &testResetStore{
		hosts: map[string]world.Host{
			"0920000196": {ID: "hakata-canal-net", Phone: "0920000196", Debug: hostcatalog.DebugFlags{HTTPEndpoints: true}},
		},
		removedPosts:    7,
		keptPosts:       3,
		operationResult: true,
	}

	handler := newBBSResetHandler("", base, mock)

	// 1. Default phone (omitted) + default mode (generated)
	req := httptest.NewRequest(http.MethodPost, "/api/debug/bbs/reset", nil)
	rec := httptest.NewRecorder()
	handler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", rec.Code, rec.Body.String())
	}
	if mock.resetGenCalls != 1 || mock.prepareCalls != 0 {
		t.Fatalf("expected 1 ResetBBSGeneratedArticles call, got resetGen=%d prepare=%d", mock.resetGenCalls, mock.prepareCalls)
	}

	var res map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
		t.Fatal(err)
	}
	if res["mode"] != "generated" || res["removed_posts"] != float64(7) {
		t.Fatalf("unexpected response: %+v", res)
	}

	// 2. Mode = all
	req = httptest.NewRequest(http.MethodPost, "/api/debug/bbs/reset?phone=0920000196&mode=all", nil)
	rec = httptest.NewRecorder()
	handler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", rec.Code, rec.Body.String())
	}
	if mock.prepareCalls != 1 {
		t.Fatalf("expected 1 PrepareDebugBBSConnection call for mode=all, got %d", mock.prepareCalls)
	}
}
