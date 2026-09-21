package erikak

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"zutto-pccom/apps/server/internal/world"
)

type noWaitObservationStore struct {
	*world.MemoryStore
	begun []world.Board
}

func (s *noWaitObservationStore) BeginHostObservation(_ world.Host, boards []world.Board) {
	s.begun = append(s.begun, boards...)
}

func (s *noWaitObservationStore) WaitForBoardHeaders(context.Context, world.Host, world.Board) ([]world.Post, error) {
	panic("board/index navigation must not wait for observation")
}

func (s *noWaitObservationStore) WaitForArticleBody(context.Context, world.Host, world.Board, int64) (world.Post, bool, error) {
	panic("article body wait is not expected in an index-navigation test")
}


func sampleRuntime(t *testing.T) (*Runtime, *world.MemoryStore) {
	t.Helper()
	store := world.NewMemoryStore()
	host, err := store.HostByPhone("0920000196")
	if err != nil {
		t.Fatalf("sample host: %v", err)
	}
	return New(host, store), store
}

func loginGuest(t *testing.T, runtime *Runtime) string {
	t.Helper()
	out, disconnect := runtime.HandleLine("GUEST")
	if disconnect {
		t.Fatal("guest login disconnected")
	}
	return out
}

func TestLoginLooksLikeErikaKAndSupportsPasswordStep(t *testing.T) {
	runtime, _ := sampleRuntime(t)
	welcome := runtime.Welcome()
	if !strings.Contains(welcome, "YOUR ID:") || !strings.Contains(welcome, "HAKATA CANAL NET") {
		t.Fatalf("Erika-style ID prompt missing: %q", welcome)
	}

	out, disconnect := runtime.HandleLine("MIKI")
	if disconnect || out != "PASSWORD:" {
		t.Fatalf("member login should request a password: %q", out)
	}
	out, disconnect = runtime.HandleLine("dummy")
	if disconnect || !strings.Contains(out, "WELCOME TO HAKATA CANAL NET") || !strings.Contains(out, "〖Ｍain Ｍenu〗") {
		t.Fatalf("post-login Erika menu missing: %q", out)
	}
	if !strings.Contains(out, "[ASET]") || !strings.Contains(out, "[MA]") || !strings.Contains(out, "[T]") {
		t.Fatalf("late Erika K menu density is missing: %q", out)
	}
}

func TestBoardHierarchyAndBJPrompt(t *testing.T) {
	runtime, _ := sampleRuntime(t)
	loginGuest(t, runtime)

	out, disconnect := runtime.HandleLine("1") // Main Menu: Board (BM)
	if disconnect || !strings.Contains(out, "ボード／フォーラムメニュー") || !strings.Contains(out, "[60]") {
		t.Fatalf("root board menu missing: %q", out)
	}
	out, disconnect = runtime.HandleLine("60")
	if disconnect || !strings.Contains(out, "コンピュータワールド") || !strings.Contains(out, "(BJ\\60) BOARD") {
		t.Fatalf("forum hierarchy prompt missing: %q", out)
	}
	out, disconnect = runtime.HandleLine("1")
	if disconnect || !strings.Contains(out, "ＰＣ－９８／ＭＯＤＥＭ") || !strings.Contains(out, "(BJ\\60\\1) BOARD") || !strings.Contains(out, "--- MSG はありません ---") {
		t.Fatalf("empty leaf board/index missing: %q", out)
	}

	out, _ = runtime.HandleLine("")
	if !strings.Contains(out, "コンピュータワールド") || !strings.Contains(out, "(BJ\\60) BOARD") {
		t.Fatalf("RETURN should move to the parent level: %q", out)
	}
	out, _ = runtime.HandleLine("/")
	if !strings.Contains(out, "〖Ｍain Ｍenu〗") {
		t.Fatalf("slash should return to main menu: %q", out)
	}
}

func TestThreadRendersAppendsTogether(t *testing.T) {
	runtime, store := sampleRuntime(t)
	root := store.AddPost(runtime.Host.ID, world.Post{BoardID: "1", Author: "SYSOP", Subject: "テスト記事", Body: "本文"})
	store.AddPost(runtime.Host.ID, world.Post{BoardID: "1", ParentID: root.ID, Author: "MARI", Subject: "Re: テスト記事", Body: "その1"})
	store.AddPost(runtime.Host.ID, world.Post{BoardID: "1", ParentID: root.ID, Author: "KAZU", Subject: "Re: テスト記事", Body: "その2"})

	loginGuest(t, runtime)
	runtime.HandleLine("1")
	runtime.HandleLine("1")

	out, disconnect := runtime.HandleLine(fmt.Sprintf("%d", root.ID))
	if disconnect {
		t.Fatal("reading a thread disconnected")
	}
	if !strings.Contains(out, "アペ 1") || !strings.Contains(out, "アペ 2") {
		t.Fatalf("thread should render its appends together: %q", out)
	}
	if !strings.Contains(out, "MARI") || !strings.Contains(out, "KAZU") {
		t.Fatalf("append authors missing: %q", out)
	}
}

func TestAppendCreatesChildPost(t *testing.T) {
	runtime, store := sampleRuntime(t)
	root := store.AddPost(runtime.Host.ID, world.Post{BoardID: "1", Author: "SYSOP", Subject: "テスト記事", Body: "本文"})

	runtime.HandleLine("TESTER")
	runtime.HandleLine("dummy")
	runtime.HandleLine("1")
	runtime.HandleLine("1")
	runtime.HandleLine(fmt.Sprintf("%d", root.ID))

	out, disconnect := runtime.HandleLine("A")
	if disconnect || !strings.Contains(out, "APE -->") {
		t.Fatalf("append prompt missing: %q", out)
	}
	out, disconnect = runtime.HandleLine("追加テストです(^^;")
	if disconnect || !strings.Contains(out, "アペ 1") || !strings.Contains(out, "追加テストです") {
		t.Fatalf("new append was not rendered: %q", out)
	}

	found := false
	for _, post := range store.ListPosts(runtime.Host.ID) {
		if post.ParentID == root.ID && post.Author == "TESTER" && post.Body == "追加テストです(^^;" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("append was not stored as a child post")
	}
}

func TestCommandModeAliasesAndHiddenBoard(t *testing.T) {
	runtime, _ := sampleRuntime(t)
	loginGuest(t, runtime)

	out, disconnect := runtime.HandleLine("H")
	if disconnect || !strings.Contains(out, "[BM]") || !strings.Contains(out, "[BX | BXS]") || !strings.Contains(out, "[BJ]") || !strings.Contains(out, "[WHO]") {
		t.Fatalf("documented command-mode aliases missing: %q", out)
	}

	out, disconnect = runtime.HandleLine("BJ 99")
	if disconnect || !strings.Contains(out, "夜更かし部屋") || !strings.Contains(out, "--- MSG はありません ---") {
		t.Fatalf("station-specific hidden board should remain directly reachable and empty: %q", out)
	}
	if strings.Contains(runtime.renderBoardMap(), "夜更かし部屋") {
		t.Fatal("hidden board leaked into MA board map")
	}
}

func TestNmodemAppearsInFileMenu(t *testing.T) {
	runtime, _ := sampleRuntime(t)
	loginGuest(t, runtime)
	out, disconnect := runtime.HandleLine("FM")
	if disconnect || !strings.Contains(out, "NMODEM") || !strings.Contains(out, "(FM) FILE") {
		t.Fatalf("Erika K file menu should expose NMODEM: %q", out)
	}
}

func TestSampleStationStartsWithNoArticlesButKeepsResidentCast(t *testing.T) {
	store := world.NewMemoryStore()
	if posts := store.ListPosts("hakata-canal-net"); len(posts) != 0 {
		t.Fatalf("HAKATA starts with %d posts, want 0", len(posts))
	}
	personas := store.ListHostPersonas("hakata-canal-net")
	if len(personas) < 10 {
		t.Fatalf("resident cast too small: %d", len(personas))
	}
	seen := map[string]bool{}
	for _, persona := range personas {
		seen[persona.Handle] = true
	}
	for _, handle := range []string{"MARI", "KAZU", "NORI", "AKI"} {
		if !seen[handle] {
			t.Fatalf("resident handle %s missing from cast", handle)
		}
	}
}

func TestConnectDoesNotFanOutObservationAcrossEmptyBoards(t *testing.T) {
	runtime, _ := sampleRuntime(t)
	if boards := runtime.ObservationBoards(); len(boards) != 0 {
		t.Fatalf("CONNECT observation boards=%d, want 0; leaf visits should drive generation", len(boards))
	}
}

func TestBoardCatalogNavigationDoesNotWaitForObservation(t *testing.T) {
	base := world.NewMemoryStore()
	host, err := base.HostByPhone("0920000196")
	if err != nil {
		t.Fatal(err)
	}
	store := &noWaitObservationStore{MemoryStore: base}
	runtime := New(host, store)
	loginGuest(t, runtime)

	out, disconnect := runtime.HandleLine("BM")
	if disconnect || !strings.Contains(out, "ボード／フォーラムメニュー") {
		t.Fatalf("root board menu missing: %q", out)
	}
	out, disconnect = runtime.HandleLine("60")
	if disconnect || !strings.Contains(out, "コンピュータワールド") {
		t.Fatalf("forum menu missing: %q", out)
	}
	out, disconnect = runtime.HandleLine("1")
	if disconnect || !strings.Contains(out, "ＰＣ－９８／ＭＯＤＥＭ") {
		t.Fatalf("board index missing: %q", out)
	}
}

func TestLeafBoardVisitStartsBackgroundCatchupWithoutWaiting(t *testing.T) {
	base := world.NewMemoryStore()
	host, err := base.HostByPhone("0920000196")
	if err != nil {
		t.Fatal(err)
	}
	store := &noWaitObservationStore{MemoryStore: base}
	runtime := New(host, store)
	loginGuest(t, runtime)

	runtime.HandleLine("BM")
	runtime.HandleLine("60")
	out, disconnect := runtime.HandleLine("1")
	if disconnect || !strings.Contains(out, "ＰＣ－９８／ＭＯＤＥＭ") {
		t.Fatalf("leaf board index missing: %q", out)
	}
	if len(store.begun) == 0 {
		t.Fatal("leaf board visit did not start background observation")
	}
	got := store.begun[len(store.begun)-1]
	if got.ID != "60/1" || got.Name != "ＰＣ－９８／ＭＯＤＥＭ" {
		t.Fatalf("unexpected observed board: %+v", got)
	}
}

func TestBare99IsNotAResetCommand(t *testing.T) {
	runtime, _ := sampleRuntime(t)
	loginGuest(t, runtime)
	out, disconnect := runtime.HandleLine("99")
	if disconnect {
		t.Fatal("bare 99 must not disconnect")
	}
	if !strings.Contains(out, "? COMMAND ERROR") {
		t.Fatalf("bare 99 should be an ordinary unknown command: %q", out)
	}

	out, disconnect = runtime.HandleLine("BJ 99")
	if disconnect || !strings.Contains(out, "夜更かし部屋") {
		t.Fatalf("BJ 99 hidden board navigation should remain available: %q", out)
	}
}
