package main

import (
	"encoding/json"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Test deployment only. These process-local limits are not a billing cap.
// The two independent persona experiments share this process-local rate limit.
const publicLabPhone = "0450000196"
const publicLabDailyRuns = 20
const publicLabInterval = time.Minute

type labAdmission struct {
	mu sync.Mutex
	active bool
	blocked bool
	lastStart time.Time
	day string
	runs int
}

var publicLabAdmission labAdmission

func personaLabEnabled() bool { return os.Getenv("PERSONA_LAB_DISABLED") != "1" }

func labRequestAllowed(w http.ResponseWriter, r *http.Request) bool {
	if !personaLabEnabled() {
		labAccessError(w, http.StatusServiceUnavailable, "persona experiment disabled")
		return false
	}
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		w.Header().Set("Allow", "GET, POST")
		labAccessError(w, http.StatusMethodNotAllowed, "GET or POST only")
		return false
	}
	if strings.EqualFold(strings.TrimSpace(r.URL.Query().Get("action")), "start") && r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		labAccessError(w, http.StatusMethodNotAllowed, "start requires POST")
		return false
	}
	if phone := strings.TrimSpace(r.URL.Query().Get("phone")); phone != "" && phone != publicLabPhone {
		labAccessError(w, http.StatusBadRequest, "only development host 0450000196 is allowed")
		return false
	}
	return true
}

func labAccessError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": message})
}

func (g *labAdmission) start(w http.ResponseWriter, r *http.Request, phone string, runs int) bool {
	if !labRequestAllowed(w, r) { return false }
	if phone != publicLabPhone {
		labAccessError(w, http.StatusBadRequest, "only development host 0450000196 is allowed")
		return false
	}
	status, retry, message := g.acquire(time.Now(), runs)
	if status != 0 {
		if retry > 0 { w.Header().Set("Retry-After", strconv.Itoa(retry)) }
		labAccessError(w, status, message)
		return false
	}
	return true
}

func (g *labAdmission) acquire(now time.Time, runs int) (int, int, string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.blocked { return http.StatusServiceUnavailable, 0, "lab blocked after runtime timeout; operator restart required" }
	if g.active { return http.StatusConflict, 0, "another persona experiment job is active" }
	if runs < 1 || runs > publicLabDailyRuns { return http.StatusBadRequest, 0, "invalid run count" }
	now = now.UTC()
	day := now.Format("2006-01-02")
	if day != g.day { g.day, g.runs = day, 0 }
	if g.runs + runs > publicLabDailyRuns {
		next := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, time.UTC)
		return http.StatusTooManyRequests, int(next.Sub(now).Seconds())+1, "daily lab run limit reached"
	}
	if remaining := publicLabInterval - now.Sub(g.lastStart); remaining > 0 {
		return http.StatusTooManyRequests, int(remaining.Seconds())+1, "wait between lab starts"
	}
	g.active, g.lastStart = true, now
	g.runs += runs
	return 0, 0, ""
}

func (g *labAdmission) finish() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.active = false
}

// A runtime may still be running after its observer times out. Fail closed.
func (g *labAdmission) block() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.blocked = true
}
