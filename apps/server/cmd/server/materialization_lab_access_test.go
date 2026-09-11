package main

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestLabAdmissionLimits(t *testing.T) {
	var g labAdmission
	now := time.Date(2026, 9, 7, 10, 0, 0, 0, time.UTC)
	check := func(at time.Time, runs, want int) {
		t.Helper()
		got, retry, _ := g.acquire(at, runs)
		if got != want {
			t.Fatalf("status=%d want=%d", got, want)
		}
		if got == http.StatusTooManyRequests && retry <= 0 {
			t.Fatal("missing retry interval")
		}
	}
	check(now, 5, 0)
	check(now.Add(2*time.Minute), 1, http.StatusConflict)
	g.finish()
	check(now.Add(time.Second), 1, http.StatusTooManyRequests)
	for i := 1; i <= 3; i++ {
		check(now.Add(time.Duration(i)*time.Minute), 5, 0)
		g.finish()
	}
	check(now.Add(4*time.Minute), 1, http.StatusTooManyRequests)
	check(now.Add(24*time.Hour), 1, 0)
	g.block()
	g.finish()
	check(now.Add(48*time.Hour), 1, http.StatusServiceUnavailable)
}

func TestLabAdmissionConcurrentStart(t *testing.T) {
	var g labAdmission
	var wg sync.WaitGroup
	var accepted atomic.Int32
	now := time.Now()
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if status, _, _ := g.acquire(now, 1); status == 0 {
				accepted.Add(1)
			}
		}()
	}
	wg.Wait()
	if accepted.Load() != 1 {
		t.Fatalf("accepted=%d", accepted.Load())
	}
}

func TestPublicLabHandlers(t *testing.T) {
	t.Setenv("MATERIALIZATION_LAB_DISABLED", "")
	// A stale configured token must not prevent access in this test deployment.
	l := newMaterializationLab(nil, nil, nil, "1996-08-26", "unused-old-token")
	handlers := []http.HandlerFunc{l.handler(), l.randomHandler(), l.allBodyHandler(), l.freshHandler()}
	for _, h := range handlers {
		for _, tc := range []struct {
			method, query string
			want          int
		}{
			{"GET", "?action=status", 200},
			{"GET", "?action=start", 405},
			{"POST", "?action=start&phone=0312345678", 400},
			{"DELETE", "?action=status", 405},
		} {
			w := httptest.NewRecorder()
			h(w, httptest.NewRequest(tc.method, "/lab"+tc.query, nil))
			if w.Code != tc.want {
				t.Fatalf("%s %s: %d %s", tc.method, tc.query, w.Code, w.Body.String())
			}
		}
	}
	// All real start handlers must use the same gate before touching the store.
	publicLabAdmission.mu.Lock()
	publicLabAdmission.active = true
	publicLabAdmission.mu.Unlock()
	defer publicLabAdmission.finish()
	for _, h := range handlers {
		w := httptest.NewRecorder()
		h(w, httptest.NewRequest("POST", "/lab?action=start", nil))
		if w.Code != http.StatusConflict {
			t.Fatalf("shared gate: %d %s", w.Code, w.Body.String())
		}
	}
	t.Setenv("MATERIALIZATION_LAB_DISABLED", "1")
	for _, h := range handlers {
		w := httptest.NewRecorder()
		h(w, httptest.NewRequest("GET", "/lab?action=status", nil))
		if w.Code != http.StatusServiceUnavailable {
			t.Fatalf("disabled: %d", w.Code)
		}
	}
}

func TestLabAdmissionRejectsInvalidScopeWithoutQuota(t *testing.T) {
	t.Setenv("MATERIALIZATION_LAB_DISABLED", "")
	var g labAdmission
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/lab?action=start", nil)
	if g.start(w, r, "other-host", 1) || w.Code != 400 {
		t.Fatal("wrong host admitted")
	}
	if g.active || g.runs != 0 {
		t.Fatal("invalid host consumed quota")
	}
	w = httptest.NewRecorder()
	if !g.start(w, r, publicLabPhone, 1) {
		t.Fatalf("tokenless start rejected: %s", w.Body.String())
	}
	g.finish()
}
