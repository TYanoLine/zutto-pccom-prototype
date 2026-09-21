package erikak

import (
	"context"
	"strings"
	"testing"

	"zutto-pccom/apps/server/internal/world"
)

type noWaitObservationStore struct {
	*world.MemoryStore
	begun []world.Board
}

type hiddenResetStore struct {
	*world.MemoryStore
	calls   int
	removed int
	kept    int
	ok      bool
}

func (s *hiddenResetStore) ResetBBSGeneratedArticles(world.Host) (int, int, bool) {
	s.calls++
	return s.removed, s.kept, s.ok
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
	if disconnect || !strings.Contains(out, "ＰＣ－９８／ＭＯＤＥＭ") || !strings.Contains(out, "(BJ\\60\\1) BOARD") || !strings.Contains(out, "0201") {
		t.Fatalf("leaf board/index missing: %q", out)
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
	runtime, _ := sampleRuntime(t)
	loginGuest(t, runtime)
	runtime.HandleLine("1")
	runtime.HandleLine("1")

	out, disconnect := runtime.HandleLine("101")
	if disconnect {
		t.Fatal("reading a thread disconnected")
	}
	if !strings.Contains(out, "アペ 1") || !strings.Contains(out, "アペ 2") {
		t.Fatalf("thread should render its appends together: %q", out)
	}
	if !strings.Contains(out, "MARI") || !strings.Contains(out, "KAZU") || !strings.Contains(out, "(BR\\1\\101) BOARD") {
		t.Fatalf("append authors/current-path prompt missing: %q", out)
	}
}

func TestAppendCreatesChildPost(t *testing.T) {
	runtime, store := sampleRuntime(t)
	runtime.HandleLine("TESTER")
	runtime.HandleLine("dummy")
	runtime.HandleLine("1")
	runtime.HandleLine("1")
	runtime.HandleLine("101")

	out, disconnect := runtime.HandleLine("A")
	if disconnect || !strings.Contains(out, "APE -->") {
		t.Fatalf("append prompt missing: %q", out)
	}
	out, disconnect = runtime.HandleLine("追加テストです(^^;")
	if disconnect || !strings.Contains(out, "アペ 3") || !strings.Contains(out, "追加テストです") {
		t.Fatalf("new append was not rendered: %q", out)
	}

	found := false
	for _, post := range store.ListPosts("hakata-canal-net") {
		if post.ParentID == 101 && post.Author == "TESTER" && post.Body == "追加テストです(^^;" {
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
	if disconnect || !strings.Contains(out, "夜更かし部屋") || !strings.Contains(out, "0901") {
		t.Fatalf("station-specific hidden board should remain directly reachable: %q", out)
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

func TestSampleStationSeedsAtLeastFortyRootArticlesPerLeafBoard(t *testing.T) {
	store := world.NewMemoryStore()
	posts := store.ListPosts("hakata-canal-net")
	for _, node := range boardTree {
		hasChildren := false
		for _, child := range boardTree {
			if child.Parent == node.Path {
				hasChildren = true
				break
			}
		}
		if hasChildren {
			continue
		}
		count := 0
		for _, post := range posts {
			if post.BoardID == node.Path && post.ParentID == 0 {
				count++
			}
		}
		if count < 40 {
			t.Fatalf("board %s (%s) has %d root articles, want at least 40", node.Path, node.Name, count)
		}
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

func TestHidden99ResetsGeneratedBBSHistoryAndDisconnects(t *testing.T) {
	base := world.NewMemoryStore()
	host, err := base.HostByPhone("0920000196")
	if err != nil {
		t.Fatal(err)
	}
	store := &hiddenResetStore{MemoryStore: base, removed: 17, kept: 923, ok: true}
	runtime := New(host, store)
	login := loginGuest(t, runtime)
	if strings.Contains(login, "[99]") || strings.Contains(login, "RESET") {
		t.Fatalf("hidden debug command leaked into main menu: %q", login)
	}

	out, disconnect := runtime.HandleLine("99")
	if !disconnect {
		t.Fatal("hidden 99 reset must disconnect immediately")
	}
	if store.calls != 1 {
		t.Fatalf("reset calls=%d, want 1", store.calls)
	}
	if !strings.Contains(out, "BBS GENERATED HISTORY RESET") ||
		!strings.Contains(out, "removed=17") ||
		!strings.Contains(out, "kept=923") ||
		!strings.Contains(out, "NO CARRIER") {
		t.Fatalf("unexpected reset output: %q", out)
	}
}

func TestHidden99StillLeavesBJ99AsHiddenBoardNavigation(t *testing.T) {
	base := world.NewMemoryStore()
	host, err := base.HostByPhone("0920000196")
	if err != nil {
		t.Fatal(err)
	}
	store := &hiddenResetStore{MemoryStore: base, ok: true}
	runtime := New(host, store)
	loginGuest(t, runtime)

	out, disconnect := runtime.HandleLine("BJ 99")
	if disconnect {
		t.Fatal("BJ 99 must navigate, not trigger reset")
	}
	if store.calls != 0 {
		t.Fatalf("BJ 99 unexpectedly triggered reset: calls=%d", store.calls)
	}
	if !strings.Contains(out, "夜更かし部屋") {
		t.Fatalf("BJ 99 hidden board missing: %q", out)
	}
}
