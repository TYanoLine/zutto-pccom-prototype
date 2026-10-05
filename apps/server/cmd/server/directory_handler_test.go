package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"zutto-pccom/apps/server/internal/world"
)

type fakeDirectory []world.DirectoryEntry

func (d fakeDirectory) ListedHosts() []world.DirectoryEntry { return d }

func TestDirectoryHandlerServesTheSampleStationExactly(t *testing.T) {
	rec := httptest.NewRecorder()
	newDirectoryHandler(world.NewMemoryStore())(rec, httptest.NewRequest(http.MethodGet, "/api/directory", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Fatalf("content type = %q", ct)
	}
	if cc := rec.Header().Get("Cache-Control"); cc != "no-cache" {
		t.Fatalf("cache control = %q", cc)
	}
	want := `{"centers":[{"id":"hakata-canal-net","name":"HAKATA CANAL NET","software":"絵理香K版","phone":"0920000196","dialMode":"tone","maxBaud":14400}]}`
	if got := strings.TrimSpace(rec.Body.String()); got != want {
		t.Fatalf("body =\n%s\nwant\n%s", got, want)
	}
}

func TestDirectoryHandlerNeverReturnsNull(t *testing.T) {
	rec := httptest.NewRecorder()
	newDirectoryHandler(fakeDirectory(nil))(rec, httptest.NewRequest(http.MethodGet, "/api/directory", nil))
	if got := strings.TrimSpace(rec.Body.String()); got != `{"centers":[]}` {
		t.Fatalf("body = %s", got)
	}
}

func TestDirectoryHandlerOmitsAnEmptySoftwareLabel(t *testing.T) {
	rec := httptest.NewRecorder()
	dir := fakeDirectory{{ID: "x", Name: "X", Phone: "0312345678", DialMode: "pulse", MaxBaud: 9600}}
	newDirectoryHandler(dir)(rec, httptest.NewRequest(http.MethodGet, "/api/directory", nil))
	var payload struct {
		Centers []map[string]any `json:"centers"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if _, present := payload.Centers[0]["software"]; present {
		t.Fatalf("an empty software label must be omitted: %v", payload.Centers[0])
	}
}

func TestDirectoryHandlerMethods(t *testing.T) {
	handler := newDirectoryHandler(world.NewMemoryStore())

	head := httptest.NewRecorder()
	handler(head, httptest.NewRequest(http.MethodHead, "/api/directory", nil))
	if head.Code != http.StatusOK || head.Body.Len() != 0 {
		t.Fatalf("HEAD = %d with %d body bytes", head.Code, head.Body.Len())
	}

	post := httptest.NewRecorder()
	handler(post, httptest.NewRequest(http.MethodPost, "/api/directory", nil))
	if post.Code != http.StatusMethodNotAllowed || post.Header().Get("Allow") != "GET, HEAD" {
		t.Fatalf("POST = %d, Allow = %q", post.Code, post.Header().Get("Allow"))
	}
}
