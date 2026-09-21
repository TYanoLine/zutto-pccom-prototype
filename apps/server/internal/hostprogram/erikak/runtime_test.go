package erikak

import (
	"strings"
	"testing"

	"zutto-pccom/apps/server/internal/world"
)

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
