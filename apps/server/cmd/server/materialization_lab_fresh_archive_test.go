package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

type fakeFreshArchive struct{ jobs []*materializationFreshJob }

func (f *fakeFreshArchive) Save(_ context.Context, job *materializationFreshJob) error {
	f.jobs = append(f.jobs, cloneMaterializationFreshJob(job))
	return nil
}
func (f *fakeFreshArchive) Get(_ context.Context, id string) (*materializationFreshJob, error) {
	for _, job := range f.jobs {
		if job.ID == id {
			return cloneMaterializationFreshJob(job), nil
		}
	}
	return nil, errMaterializationFreshArchiveNotFound
}
func (f *fakeFreshArchive) Latest(_ context.Context) (*materializationFreshJob, error) {
	if len(f.jobs) == 0 {
		return nil, errMaterializationFreshArchiveNotFound
	}
	return cloneMaterializationFreshJob(f.jobs[len(f.jobs)-1]), nil
}
func (f *fakeFreshArchive) List(_ context.Context, limit int) ([]materializationFreshArchiveSummary, error) {
	if limit < 1 {
		return nil, errors.New("bad limit")
	}
	out := make([]materializationFreshArchiveSummary, 0, len(f.jobs))
	for i := len(f.jobs) - 1; i >= 0 && len(out) < limit; i-- {
		out = append(out, freshJobSummary(f.jobs[i]))
	}
	return out, nil
}

func TestFreshViewerIsReadOnlyAndReturnsArchivedJob(t *testing.T) {
	finished := time.Date(2026, 9, 8, 1, 2, 3, 0, time.UTC)
	archive := &fakeFreshArchive{jobs: []*materializationFreshJob{{
		ID: "lab-fresh-test", Status: "completed", SituationMode: "batch",
		BoardCount: 6, ShellLimit: 8, PostCount: 37, BodyCount: 37,
		CreatedAt: finished.Add(-time.Minute), FinishedAt: finished,
		Articles: []materializationFreshArticle{{ID: 1001, BoardID: "1", Author: "NEKO", Subject: "テスト", Body: "本文"}},
	}}}
	lab := &materializationLab{freshArchive: archive}

	postReq := httptest.NewRequest(http.MethodPost, "/api/debug/materialization-lab-fresh-view", nil)
	postRes := httptest.NewRecorder()
	lab.freshViewerHandler().ServeHTTP(postRes, postReq)
	if postRes.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST status=%d", postRes.Code)
	}

	getReq := httptest.NewRequest(http.MethodGet, "/api/debug/materialization-lab-fresh-view?id=lab-fresh-test", nil)
	getRes := httptest.NewRecorder()
	lab.freshViewerHandler().ServeHTTP(getRes, getReq)
	if getRes.Code != http.StatusOK {
		t.Fatalf("GET status=%d body=%s", getRes.Code, getRes.Body.String())
	}
	var got materializationFreshJob
	if err := json.NewDecoder(getRes.Body).Decode(&got); err != nil {
		t.Fatal(err)
	}
	if got.ID != "lab-fresh-test" || len(got.Articles) != 1 || got.Articles[0].Body != "本文" {
		t.Fatalf("unexpected archived job: %+v", got)
	}
}

func TestFreshViewerListsArchiveSummariesWithoutArticleBodies(t *testing.T) {
	archive := &fakeFreshArchive{jobs: []*materializationFreshJob{{ID: "one", Status: "completed", CreatedAt: time.Now().Add(-time.Minute), Articles: []materializationFreshArticle{{Body: "secret-body"}}}, {ID: "two", Status: "completed", CreatedAt: time.Now()}}}
	lab := &materializationLab{freshArchive: archive}
	req := httptest.NewRequest(http.MethodGet, "/api/debug/materialization-lab-fresh-view?list=1&limit=10", nil)
	res := httptest.NewRecorder()
	lab.freshViewerHandler().ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("status=%d", res.Code)
	}
	if body := res.Body.String(); len(body) == 0 || contains(body, "secret-body") {
		t.Fatalf("summary leaked article body: %s", body)
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
