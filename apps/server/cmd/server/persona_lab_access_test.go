package main

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestPersonaLabAdmissionLimits(t *testing.T) {
	var g labAdmission
	now := time.Date(2026, 9, 7, 10, 0, 0, 0, time.UTC)
	check := func(at time.Time, runs, want int) {
		t.Helper()
		got, retry, _ := g.acquire(at, runs)
		if got != want { t.Fatalf("status=%d want=%d", got, want) }
		if got == http.StatusTooManyRequests && retry <= 0 { t.Fatal("missing retry interval") }
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

func TestPersonaLabAdmissionConcurrentStart(t *testing.T) {
	var g labAdmission
	var wg sync.WaitGroup
	var accepted atomic.Int32
	now := time.Now()
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if status, _, _ := g.acquire(now, 1); status == 0 { accepted.Add(1) }
		}()
	}
	wg.Wait()
	if accepted.Load() != 1 { t.Fatalf("accepted=%d", accepted.Load()) }
}

func TestPersonaLabAdmissionRejectsInvalidScopeWithoutQuota(t *testing.T) {
	t.Setenv("PERSONA_LAB_DISABLED", "")
	var g labAdmission
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/lab?action=start", nil)
	if g.start(w, r, "other-host", 1) || w.Code != 400 { t.Fatal("wrong host admitted") }
	if g.active || g.runs != 0 { t.Fatal("invalid host consumed quota") }
	w = httptest.NewRecorder()
	if !g.start(w, r, publicLabPhone, 1) { t.Fatalf("tokenless start rejected: %s", w.Body.String()) }
	g.finish()
}
