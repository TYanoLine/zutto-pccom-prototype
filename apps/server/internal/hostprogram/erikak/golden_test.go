package erikak

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"zutto-pccom/apps/server/internal/world"
)

var updateErikaGolden = flag.Bool("update-erikak-golden", false, "rewrite testdata golden files from the current implementation")

const (
	boardsGoldenPath  = "testdata/boards_golden.json"
	screensGoldenPath = "testdata/screens_golden.txt"
	goldenHostPhone   = "0920000196"
)

type goldenBoard struct {
	Path                 string  `json:"path"`
	Key                  string  `json:"key"`
	Alias                string  `json:"alias,omitempty"`
	Parent               string  `json:"parent,omitempty"`
	Name                 string  `json:"name"`
	Hidden               bool    `json:"hidden,omitempty"`
	Scope                string  `json:"scope,omitempty"`
	RootAuthorPolicy     string  `json:"root_author_policy,omitempty"`
	ActivityWeight       float64 `json:"activity_weight"`
	ReplyRate            float64 `json:"reply_rate"`
	RetainedRootCap      int     `json:"retained_root_cap"`
	VerifiedReferentRate float64 `json:"verified_referent_rate"`
	Unread               bool    `json:"unread"`
}

// goldenBoardTable is the only place this file reads the board definitions, so
// the refactor changes exactly this function.
func goldenBoardTable(t *testing.T) []goldenBoard {
	t.Helper()
	store := world.NewMemoryStore()
	host, err := store.HostByPhone(goldenHostPhone)
	if err != nil {
		t.Fatal(err)
	}
	detail, ok := store.HostDetail(host.ID)
	if !ok || detail.ErikaK == nil {
		t.Fatal("golden host has no Erika-K detail")
	}
	out := make([]goldenBoard, 0, len(detail.ErikaK.Boards))
	for _, n := range boardCatalogFromDetail(detail.ErikaK).Boards {
		out = append(out, goldenBoard{
			Path: n.Path, Key: n.Key, Alias: n.Alias, Parent: n.Parent, Name: n.Name,
			Hidden: n.Hidden, Scope: n.SemanticScope, RootAuthorPolicy: n.RootAuthorPolicy,
			ActivityWeight: n.ActivityWeight, ReplyRate: n.ReplyRate,
			RetainedRootCap: n.RetainedRootCap, VerifiedReferentRate: n.VerifiedReferentRate,
			Unread: n.Unread,
		})
	}
	return out
}

// visible makes control characters readable so the transcript is a stable text file.
func visible(text string) string {
	return strings.NewReplacer("\x1b", "<ESC>", "\r\n", "<CRLF>\n", "\r", "<CR>").Replace(text)
}

// screenTranscript drives scripted sessions through the host program and records
// every input and output. The store has no posts, so no output depends on time
// or randomness.
func screenTranscript(t *testing.T) string {
	t.Helper()
	store := world.NewMemoryStore()
	host, err := store.HostByPhone(goldenHostPhone)
	if err != nil {
		t.Fatal(err)
	}
	var out strings.Builder
	record := func(label, text string) {
		fmt.Fprintf(&out, "===== %s =====\n%s\n", label, visible(text))
	}
	send := func(r *Runtime, input string) {
		text, disconnect := r.HandleLine(input)
		record(fmt.Sprintf("input %q disconnect=%t", input, disconnect), text)
	}

	guest := New(host, store)
	record("welcome", guest.Welcome())
	send(guest, "GUEST")
	send(guest, "1")
	send(guest, "/")
	for _, b := range goldenBoardTable(t) {
		send(guest, "BJ "+b.Path)
		send(guest, "/")
	}
	for _, in := range []string{"MA", "T", "H", "MEMB", "V", "WHO", "9"} {
		send(guest, in)
	}

	member := New(host, store)
	send(member, "MIKI")
	send(member, "dummy")
	return out.String()
}

func firstDiff(got, want string) string {
	g, w := strings.Split(got, "\n"), strings.Split(want, "\n")
	for i := 0; i < len(g) && i < len(w); i++ {
		if g[i] != w[i] {
			return fmt.Sprintf("line %d:\n got:  %q\n want: %q", i+1, g[i], w[i])
		}
	}
	return fmt.Sprintf("lengths differ: got %d lines, want %d lines", len(g), len(w))
}

func TestScreensAndBoardsMatchGolden(t *testing.T) {
	boardsJSON, err := json.MarshalIndent(goldenBoardTable(t), "", " ")
	if err != nil {
		t.Fatal(err)
	}
	gotBoards := string(boardsJSON) + "\n"
	gotScreens := screenTranscript(t)

	if *updateErikaGolden {
		if err := os.MkdirAll(filepath.Dir(boardsGoldenPath), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(boardsGoldenPath, []byte(gotBoards), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(screensGoldenPath, []byte(gotScreens), 0o644); err != nil {
			t.Fatal(err)
		}
		t.Logf("rewrote %s and %s", boardsGoldenPath, screensGoldenPath)
		return
	}

	wantBoards, err := os.ReadFile(boardsGoldenPath)
	if err != nil {
		t.Fatalf("read golden (generate it once with -update-erikak-golden on the unchanged implementation): %v", err)
	}
	if gotBoards != string(wantBoards) {
		t.Errorf("board table differs from the golden: %s", firstDiff(gotBoards, string(wantBoards)))
	}
	wantScreens, err := os.ReadFile(screensGoldenPath)
	if err != nil {
		t.Fatal(err)
	}
	if gotScreens != string(wantScreens) {
		t.Errorf("screen transcript differs from the golden: %s", firstDiff(gotScreens, string(wantScreens)))
	}
}

func TestGoldenBoardTableShape(t *testing.T) {
	boards := goldenBoardTable(t)
	if len(boards) != 29 {
		t.Fatalf("board table has %d entries, want 29", len(boards))
	}

	if boards[0].Path != "1" || boards[len(boards)-1].Path != "80/4" {
		t.Fatalf("unexpected order: first=%q last=%q", boards[0].Path, boards[len(boards)-1].Path)
	}
	unread := 0
	for _, b := range boards {
		if b.Unread {
			unread++
		}
		if b.Path == "10/1" && (b.Key != "1" || b.Parent != "10") {
			t.Fatalf("10/1 key/parent = %q/%q, want 1/10", b.Key, b.Parent)
		}
	}

	if unread != 5 {
		t.Fatalf("%d boards are marked unread, want 5", unread)
	}
}

func TestEmbeddedErikaKDetailFitsScreen(t *testing.T) {
	store := world.NewMemoryStore()
	host, err := store.HostByPhone(goldenHostPhone)
	if err != nil {
		t.Fatal(err)
	}
	detail, ok := store.HostDetail(host.ID)
	if !ok || detail.ErikaK == nil {
		t.Fatal("golden host has no Erika-K detail")
	}
	if err := ValidateDetail(*detail.ErikaK); err != nil {
		t.Fatal(err)
	}
}
