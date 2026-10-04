package erikak

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"zutto-pccom/apps/server/internal/hostcatalog"
	"zutto-pccom/apps/server/internal/world"
)

func sampleDetail() *hostcatalog.ErikaKDetail {
	store := world.NewMemoryStore()
	host, _ := store.HostByPhone("0920000196")
	detail, _ := store.HostDetail(host.ID)
	return detail.ErikaK
}

type noWaitObservationStore struct {
	*world.MemoryStore
	begun  []world.Board
	waited []world.Board
}

type flakyBoardObservationStore struct {
	*world.MemoryStore
	waitCalls int
	failures  int
}

type articleDetailFailureObservationStore struct{ *world.MemoryStore }

func (s *articleDetailFailureObservationStore) WaitForArticleBody(_ context.Context, host world.Host, _ world.Board, postID int64) (world.Post, bool, error) {
	for _, post := range s.ListPosts(host.ID) {
		if post.ID == postID {
			return post, true, fmt.Errorf("error stage=article-detail: planner unavailable")
		}
	}
	return world.Post{}, false, nil
}

func (s *flakyBoardObservationStore) BeginHostObservation(world.Host, []world.Board) {}

func (s *flakyBoardObservationStore) WaitForBoardHeaders(_ context.Context, host world.Host, board world.Board) ([]world.Post, error) {
	s.waitCalls++
	if s.waitCalls <= s.failures {
		return nil, fmt.Errorf("transient board header failure %d", s.waitCalls)
	}
	return s.ListBoardPosts(host, board.ID, board.Name), nil
}

func (s *flakyBoardObservationStore) WaitForArticleBody(context.Context, world.Host, world.Board, int64) (world.Post, bool, error) {
	panic("article body wait is not expected in a board-index retry test")
}

func (s *noWaitObservationStore) BeginHostObservation(_ world.Host, boards []world.Board) {
	s.begun = append(s.begun, boards...)
}

func (s *noWaitObservationStore) WaitForBoardHeaders(_ context.Context, host world.Host, board world.Board) ([]world.Post, error) {
	s.waited = append(s.waited, board)
	return s.ListBoardPosts(host, board.ID, board.Name), nil
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
	if disconnect || !strings.Contains(out, "ボード／フォーラムメニュー") || !strings.Contains(out, "<60><COMP>") {
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
	store.AddPost(runtime.Host.ID, world.Post{BoardID: "1", ParentID: root.ID, Author: "MARI", Subject: "", Body: "その1"})
	store.AddPost(runtime.Host.ID, world.Post{BoardID: "1", ParentID: root.ID, Author: "KAZU", Subject: "", Body: "その2"})

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
			if post.Subject != "" {
				t.Fatalf("Erika-K append stored synthetic subject %q", post.Subject)
			}
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

func TestTransferProtocolMenuDefaultsToAllKnownProtocols(t *testing.T) {
	runtime, _ := sampleRuntime(t)
	loginGuest(t, runtime)
	if out, disconnect := runtime.HandleLine("FM"); disconnect || !strings.Contains(out, "(FM) FILE") {
		t.Fatalf("file menu missing: %q", out)
	}
	out, disconnect := runtime.HandleLine("FR")
	if disconnect {
		t.Fatal("opening transfer protocol menu disconnected")
	}
	for _, label := range []string{"無手順", "XMODEM", "XMODEM CRC", "XMODEM 1K", "YMODEM", "YMODEM-g", "ZMODEM", "NMODEM"} {
		if !strings.Contains(out, label) {
			t.Fatalf("default transfer menu missing %q: %q", label, out)
		}
	}
}

func TestHostMasterFeatureGatePrecedesRoleAccess(t *testing.T) {
	_, store := sampleRuntime(t)
	host, err := store.HostByPhone("0920000196")
	if err != nil {
		t.Fatal(err)
	}
	cfg := DefaultConfig()
	cfg.Features[FeatureFile] = false
	runtime := NewWithConfig(host, store, cfg)
	menu := loginGuest(t, runtime)
	if strings.Contains(menu, "ファイル(FM)") || strings.Contains(menu, "[BAT]") {
		t.Fatalf("disabled file feature leaked into main menu: %q", menu)
	}
	out, disconnect := runtime.HandleLine("FM")
	if disconnect || !strings.Contains(out, "利用できません") {
		t.Fatalf("direct command bypassed station master switch: %q", out)
	}
}

func TestDisabledTransferProtocolsAreHiddenAndRejected(t *testing.T) {
	_, store := sampleRuntime(t)
	host, err := store.HostByPhone("0920000196")
	if err != nil {
		t.Fatal(err)
	}
	cfg := DefaultConfig()
	cfg.TransferProtocols["nmodem"] = false
	cfg.TransferProtocols["ymodem_g"] = false
	runtime := NewWithConfig(host, store, cfg)
	loginGuest(t, runtime)
	runtime.HandleLine("FM")
	out, _ := runtime.HandleLine("FR")
	if strings.Contains(out, "NMODEM") || strings.Contains(out, "YMODEM-g") {
		t.Fatalf("disabled protocols leaked into selection menu: %q", out)
	}
	if !strings.Contains(out, "ZMODEM") || !strings.Contains(out, "XMODEM CRC") {
		t.Fatalf("enabled protocols disappeared from selection menu: %q", out)
	}
	runtime.HandleLine("")
	out, disconnect := runtime.HandleLine("NMODEM")
	if disconnect || !strings.Contains(out, "利用できません") {
		t.Fatalf("disabled protocol should be rejected even by direct name: %q", out)
	}
}

func TestSampleStationStartsWithNoArticlesButKeepsResidentCast(t *testing.T) {
	store := world.NewMemoryStore()
	if posts := store.ListPosts("hakata-canal-net"); len(posts) != 0 {
		t.Fatalf("HAKATA starts with %d posts, want 0", len(posts))
	}
	personas := store.ListHostPersonas("hakata-canal-net")
	if len(personas) != 326 {
		t.Fatalf("HAKATA membership population=%d, want 326", len(personas))
	}
	seen := map[string]bool{}
	for _, persona := range personas {
		if seen[strings.ToLower(persona.Handle)] {
			t.Fatalf("duplicate HAKATA handle: %q", persona.Handle)
		}
		seen[strings.ToLower(persona.Handle)] = true
		if strings.TrimSpace(persona.ActivityPattern) == "" {
			t.Fatalf("member %q lacks activity skeleton", persona.Handle)
		}
	}
	for _, handle := range []string{"MARI", "KAZU", "NORI", "AKI"} {
		if !seen[strings.ToLower(handle)] {
			t.Fatalf("resident handle %s missing from population", handle)
		}
	}
}

func TestConnectDoesNotFanOutObservationAcrossEmptyBoards(t *testing.T) {
	runtime, _ := sampleRuntime(t)
	if boards := runtime.ObservationBoards(); len(boards) != 0 {
		t.Fatalf("CONNECT observation boards=%d, want 0; leaf visits should drive generation", len(boards))
	}
}

func TestBoardCatalogNavigationDoesNotPrefetchAndWaitsOnlyAtLeaf(t *testing.T) {
	base := world.NewMemoryStore()
	host, err := base.HostByPhone("0920000196")
	if err != nil {
		t.Fatal(err)
	}
	store := &noWaitObservationStore{MemoryStore: base}
	runtime := New(host, store)
	loginGuest(t, runtime)
	if len(store.begun) != 0 {
		t.Fatalf("login must not start background generation: %+v", store.begun)
	}

	out, disconnect := runtime.HandleLine("BM")
	if disconnect || !strings.Contains(out, "ボード／フォーラムメニュー") {
		t.Fatalf("root board menu missing: %q", out)
	}
	out, disconnect = runtime.HandleLine("60")
	if disconnect || !strings.Contains(out, "コンピュータワールド") {
		t.Fatalf("forum menu missing: %q", out)
	}
	if len(store.begun) != 0 {
		t.Fatalf("forum navigation must not prefetch child boards: %+v", store.begun)
	}
	if len(store.waited) != 0 {
		t.Fatalf("forum navigation should not block on article headers: %+v", store.waited)
	}
	out, disconnect = runtime.HandleLine("1")
	if disconnect || !strings.Contains(out, "ＰＣ－９８／ＭＯＤＥＭ") {
		t.Fatalf("board index missing: %q", out)
	}
	if len(store.begun) != 1 || store.begun[0].ID != "60/1" {
		t.Fatalf("leaf observation=%+v, want only 60/1", store.begun)
	}
	if len(store.waited) != 1 || store.waited[0].ID != "60/1" {
		t.Fatalf("leaf wait=%+v, want exactly 60/1", store.waited)
	}
}

func TestLeafBoardVisitStartsDemandedObservationAndWaitsForHeaders(t *testing.T) {
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
	if len(store.begun) != 0 {
		t.Fatalf("navigation unexpectedly started generation: %+v", store.begun)
	}
	out, disconnect := runtime.HandleLine("1")
	if disconnect || !strings.Contains(out, "ＰＣ－９８／ＭＯＤＥＭ") {
		t.Fatalf("leaf board index missing: %q", out)
	}
	if len(store.begun) == 0 {
		t.Fatal("leaf board visit did not start/join observation")
	}
	got := store.begun[len(store.begun)-1]
	if got.ID != "60/1" || got.Name != "ＰＣ－９８／ＭＯＤＥＭ" {
		t.Fatalf("unexpected observed board: %+v", got)
	}
	if len(store.waited) == 0 || store.waited[len(store.waited)-1].ID != "60/1" {
		t.Fatalf("leaf board did not wait for its own headers: %+v", store.waited)
	}
}

func TestLeafBoardReadRetriesOneTransientHeaderFailure(t *testing.T) {
	base := world.NewMemoryStore()
	host, err := base.HostByPhone("0920000196")
	if err != nil {
		t.Fatal(err)
	}
	store := &flakyBoardObservationStore{MemoryStore: base, failures: 1}
	runtime := New(host, store)
	loginGuest(t, runtime)

	runtime.HandleLine("BM")
	out, disconnect := runtime.HandleLine("1")
	if disconnect {
		t.Fatal("board read disconnected after a transient header failure")
	}
	if strings.Contains(out, "? BOARD READ ERROR") || !strings.Contains(out, "MSG はありません") {
		t.Fatalf("transient header failure leaked into Erika-K UI: %q", out)
	}
	if store.waitCalls != 2 {
		t.Fatalf("WaitForBoardHeaders calls=%d, want 2", store.waitCalls)
	}
}

func TestLeafBoardReadRetryRemainsBounded(t *testing.T) {
	base := world.NewMemoryStore()
	host, err := base.HostByPhone("0920000196")
	if err != nil {
		t.Fatal(err)
	}
	store := &flakyBoardObservationStore{MemoryStore: base, failures: 3}
	runtime := New(host, store)
	loginGuest(t, runtime)

	runtime.HandleLine("BM")
	out, disconnect := runtime.HandleLine("1")
	if disconnect {
		t.Fatal("board read error should not disconnect")
	}
	if !strings.Contains(out, "? BOARD READ ERROR") {
		t.Fatalf("persistent failure should remain visible after one retry: %q", out)
	}
	if store.waitCalls != 2 {
		t.Fatalf("WaitForBoardHeaders calls=%d, want bounded retry count 2", store.waitCalls)
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

func TestBoardByPathResolvesCanonicalLeaf(t *testing.T) {
	board, ok := BoardByPath(sampleDetail(), "70/1")
	if !ok {
		t.Fatal("70/1 was not resolved")
	}
	if board.ID != "70/1" || board.Name != "ＰＣ－９８" {
		t.Fatalf("board=%+v", board)
	}
	if _, ok := BoardByPath(sampleDetail(), "99"); ok {
		t.Fatal("hidden board must not be exposed through debug lookup")
	}
	if _, ok := BoardByPath(sampleDetail(), "missing"); ok {
		t.Fatal("unknown board unexpectedly resolved")
	}
}

func TestBoardIndexFormatAndCommands(t *testing.T) {
	runtime, store := sampleRuntime(t)
	root := store.AddPost(runtime.Host.ID, world.Post{BoardID: "1", Author: "SYSOP", Subject: "今週末のメンテナンス", Body: "本文"})
	store.AddPost(runtime.Host.ID, world.Post{BoardID: "1", ParentID: root.ID, Author: "MARI", Subject: "", Body: "その1"})
	store.AddPost(runtime.Host.ID, world.Post{BoardID: "1", ParentID: root.ID, Author: "KAZU", Subject: "", Body: "その2"})

	loginGuest(t, runtime)

	// Navigate to board 1 (事務局からのお知らせ)
	runtime.HandleLine("1")
	out, _ := runtime.HandleLine("1")

	// 1. Verify index format and ap/ref column
	if !strings.Contains(out, "BD# 01") || !strings.Contains(out, "ap/ref___________i n d e x_______________") {
		t.Fatalf("board index header missing or incorrect: %q", out)
	}
	// Post has 2 appends, so ap/ref column should have " 2 "
	if !strings.Contains(out, fmt.Sprintf("%4d", root.ID)) || !strings.Contains(out, " 2 今週末のメンテナンス") {
		t.Fatalf("post index row with append count missing: %q", out)
	}
	if !strings.Contains(out, "[BW/W]書く [A]アペ") {
		t.Fatalf("guidance line should mention [BW/W] and [A]: %q", out)
	}

	// 2. Test 'A' command from board index to append
	cmd := fmt.Sprintf("A %d", root.ID)
	out, _ = runtime.HandleLine(cmd)
	if !strings.Contains(out, fmt.Sprintf("MSG No.%d へアペンドします。", root.ID)) || !strings.Contains(out, "APE -->") {
		t.Fatalf("%s should prompt for append: %q", cmd, out)
	}
	out, _ = runtime.HandleLine("") // cancel
	if !strings.Contains(out, "アペを中止しました。") {
		t.Fatalf("empty line should cancel append: %q", out)
	}

	// 3. Test '.' navigation to return to parent menu
	out, _ = runtime.HandleLine(".")
	if !strings.Contains(out, "ボード／フォーラムメニュー") {
		t.Fatalf("'.' should return to parent menu: %q", out)
	}
}

func TestThreadShowsAppendLoadFailurePlaceholderInsteadOfBlankAppend(t *testing.T) {
	runtime, store := sampleRuntime(t)
	root := store.AddPost(runtime.Host.ID, world.Post{
		BoardID: "1",
		Author:  "MARU",
		Subject: "セーブの場所を決めてます",
		Body:    "本文",
	})
	store.AddPost(runtime.Host.ID, world.Post{
		BoardID:  "1",
		ParentID: root.ID,
		Author:   "MINT-Y",
		Body:     "",
	})
	runtime.boardPath = "1"

	out := runtime.renderThread(root.ID)
	if !strings.Contains(out, "(アペンドの読み込みに失敗しました)") {
		t.Fatalf("blank append did not expose load failure placeholder: %q", out)
	}
}

func TestThreadShowsAppendFailureWhenSharedDetailPipelineFails(t *testing.T) {
	base := world.NewMemoryStore()
	host, err := base.HostByPhone("0920000196")
	if err != nil {
		t.Fatal(err)
	}
	store := &articleDetailFailureObservationStore{MemoryStore: base}
	runtime := New(host, store)
	root := base.AddPost(host.ID, world.Post{BoardID: "1", Author: "MARU", Subject: "セーブの場所", Body: "本文"})
	base.AddPost(host.ID, world.Post{BoardID: "1", ParentID: root.ID, Author: "MINT-Y", Subject: "", Body: ""})
	runtime.boardPath = "1"

	out := runtime.renderThread(root.ID)
	if !strings.Contains(out, "セーブの場所") || !strings.Contains(out, "(アペンドの読み込みに失敗しました)") {
		t.Fatalf("shared detail failure did not preserve the header and show host failure text: %q", out)
	}
}

func TestExistingPC98HeadersDoNotLaunchMoreGenerationOnIndexReturn(t *testing.T) {
	base := world.NewMemoryStore()
	host, err := base.HostByPhone("0920000196")
	if err != nil {
		t.Fatal(err)
	}
	board, ok := BoardByPath(sampleDetail(), "70/1")
	if !ok {
		t.Fatal("PC-98 board missing")
	}
	saved := base.AddPost(host.ID, world.Post{BoardID: board.ID, Author: "MIKI", Subject: "PC-98の起動ディスク", Body: "本文", CreatedAt: time.Date(1996, 7, 17, 21, 0, 0, 0, time.Local)})
	store := &noWaitObservationStore{MemoryStore: base}
	runtime := New(host, store)
	runtime.boardPath = board.ID
	runtime.state = "board"
	for i := 0; i < 2; i++ {
		out := runtime.renderBoardIndex()
		if !strings.Contains(out, saved.Subject) || !strings.Contains(out, "最新10インデックス") {
			t.Fatalf("PC-98 existing headers missing after return: %q", out)
		}
	}
	if len(store.begun) != 0 || len(store.waited) != 0 {
		t.Fatalf("existing board index started redundant LLM catch-up: begun=%#v waited=%#v", store.begun, store.waited)
	}
}

func TestHakataNoticeBoardIsReadOnlyUntilAuthenticatedAdminPostingExists(t *testing.T) {
	runtime, store := sampleRuntime(t)
	if board, ok := BoardByPath(sampleDetail(), "1"); !ok || board.RootAuthorPolicy != "sysop_only" {
		t.Fatalf("station staff notice policy not exported: board=%+v ok=%v", board, ok)
	}
	loginGuest(t, runtime)
	runtime.HandleLine("1") // board menu
	runtime.HandleLine("1") // notice board
	for _, cmd := range []string{"BW", "BWX", "NEW"} {
		out, disconnect := runtime.HandleLine(cmd)
		if disconnect || !strings.Contains(out, "事務局のみ") || runtime.state != "board" {
			t.Fatalf("guest root write %q not blocked: output=%q state=%s", cmd, out, runtime.state)
		}
	}
	if got := store.ListPosts(runtime.Host.ID); len(got) != 0 {
		t.Fatalf("notice writes occurred despite read-only gate: %+v", got)
	}
	root := store.AddPost(runtime.Host.ID, world.Post{
		BoardID: "1", Author: "SYSOP", Subject: "保守のお知らせ", Body: "本文"})
	runtime.HandleLine(fmt.Sprintf("%d", root.ID))
	out, _ := runtime.HandleLine("BW")
	if !strings.Contains(out, "事務局のみ") || runtime.state != "thread" {
		t.Fatalf("thread entry bypasses station posting policy: output=%q state=%s", out, runtime.state)
	}
}

func TestHakataBoardPurposesDoNotLeakHistoricalUncertainty(t *testing.T) {
	dream, ok := BoardByPath(sampleDetail(), "8")
	if !ok || strings.Contains(dream.SemanticScope, "史料") ||
		strings.Contains(dream.SemanticScope, "未確認") ||
		dream.SemanticScope == "" {
		t.Fatalf("fictional Dream board scope leaks research guidance: %+v ok=%v", dream, ok)
	}
	office, _ := BoardByPath(sampleDetail(), "5")
	contact, _ := BoardByPath(sampleDetail(), "10/2")
	if office.SemanticScope == "" || contact.SemanticScope == "" ||
		office.SemanticScope == contact.SemanticScope {
		t.Fatalf("different offline station boards lost their purposes: %q / %q", office.SemanticScope, contact.SemanticScope)
	}
}
