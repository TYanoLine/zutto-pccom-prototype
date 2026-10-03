package turbobbs

import (
	"strings"
	"testing"
	"time"

	"zutto-pccom/apps/server/internal/world"
)

// sampleHost is a TurboBBS station defined only for these tests. Its settings
// (2400 bps, one line, no guests) are what the runtime tests were written against.
func sampleHost() world.Host {
	return world.Host{ID: "turbobbs-test", Phone: "0470000001", Name: "TURBOBBS TEST", Region: "千葉県", Software: "TurboBBS 1.08 compatible", SoftwareID: "turbobbs", Lines: 1, Popularity: .18, MaxBaud: 2400, Members: 52, FoundedOn: "1989-08-20", ANSI: false, GuestAllowed: false, TelehoFriendly: true}
}

// sampleRuntime returns a runtime over a store that already holds four messages
// (#601-#604, one per section 1/4/2/8) posted before the saved high-water mark.
func sampleRuntime(t *testing.T) (*Runtime, *world.MemoryStore) {
	t.Helper()
	store := world.NewMemoryStore()
	host := sampleHost()
	store.SaveHost(host)
	store.ReplaceHostPosts(host.ID, []world.Post{
		{ID: 601, BoardID: "1", Author: "SYSOP", Subject: "まだ動いてます", Body: "1980年代から手を入れながら使っているTurboBBSです。\r\n古い作りですが、のんびり使ってください(^^)", CreatedAt: time.Date(1996, 8, 24, 22, 18, 0, 0, time.Local)},
		{ID: 602, BoardID: "4", Author: "TARO YAMADA", Subject: "2400bpsモデム", Body: "高速局が増えましたが、このくらいの速度も落ち着きますね。\r\n巡回にはちょっと時間がかかります(^^;", CreatedAt: time.Date(1996, 8, 25, 0, 14, 0, 0, time.Local)},
		{ID: 603, BoardID: "2", Author: "MIKA", Subject: "98のDOS環境", Body: "CONFIG.SYSを整理したら空きメモリが少し増えました。\r\nまだDOSも手放せません。", CreatedAt: time.Date(1996, 8, 25, 21, 47, 0, 0, time.Local)},
		{ID: 604, BoardID: "8", Author: "KEN", Subject: "夏も終わりかな", Body: "夜は少し涼しくなってきましたね。\r\n電話代を気にしつつ、また深夜に来ます(笑)", CreatedAt: time.Date(1996, 8, 26, 1, 8, 0, 0, time.Local)},
	})
	return New(host, store), store
}

func loginRegular(t *testing.T, runtime *Runtime) {
	t.Helper()
	out, disconnect := runtime.HandleLine("TARO YAMADA;MODEM")
	if disconnect {
		t.Fatal("regular login disconnected")
	}
	if !strings.Contains(out, "Command:") {
		t.Fatalf("login did not reach command prompt: %q", out)
	}
}

func TestWelcomeAndNewUserFlow(t *testing.T) {
	runtime, _ := sampleRuntime(t)

	welcome := runtime.Welcome()
	for _, want := range []string{"TURBOBBS TEST", "TurboBBS version 1.08", "What is your full name?"} {
		if !strings.Contains(welcome, want) {
			t.Fatalf("welcome missing %q: %q", want, welcome)
		}
	}

	out, disconnect := runtime.HandleLine("NEW CALLER")
	if disconnect || !strings.Contains(out, "is this correct") {
		t.Fatalf("new user name confirmation: disconnect=%v out=%q", disconnect, out)
	}
	out, _ = runtime.HandleLine("Y")
	if !strings.Contains(out, "password") {
		t.Fatalf("new user did not enter password setup: %q", out)
	}
	_, _ = runtime.HandleLine("SECRET")
	out, _ = runtime.HandleLine("SECRET")
	if !strings.Contains(out, "Terminal parameters") {
		t.Fatalf("new user did not enter terminal setup: %q", out)
	}
	out, disconnect = runtime.HandleLine("0")
	if disconnect || !strings.Contains(out, "Welcome, NEW CALLER") {
		t.Fatalf("new user did not finish login: disconnect=%v out=%q", disconnect, out)
	}

	out, _ = runtime.HandleLine("E")
	if !strings.Contains(out, "requires regular access") {
		t.Fatalf("level-2 new user unexpectedly allowed to post: %q", out)
	}
}

func TestChainedLoginAndNewReadUsesSavedHighMessage(t *testing.T) {
	runtime, _ := sampleRuntime(t)
	loginRegular(t, runtime)

	out, disconnect := runtime.HandleLine("N")
	if disconnect {
		t.Fatal("N disconnected")
	}
	if strings.Contains(out, "#601") || strings.Contains(out, "#602") {
		t.Fatalf("N included messages at/before saved high-water mark: %q", out)
	}
	for _, want := range []string{"#603", "#604"} {
		if !strings.Contains(out, want) {
			t.Fatalf("N missing %s: %q", want, out)
		}
	}
}

func TestRegularUserCanStorePublicMessage(t *testing.T) {
	runtime, store := sampleRuntime(t)
	loginRegular(t, runtime)

	steps := []struct {
		in   string
		want string
	}{
		{"E", "To (or ALL)"},
		{"ALL", "Subject"},
		{"TEST MESSAGE", "Section number"},
		{"1", "Enter message text"},
		{"HELLO FROM 1996", "2>"},
		{"", "Edit:"},
		{"S", "stored"},
	}
	for _, step := range steps {
		out, disconnect := runtime.HandleLine(step.in)
		if disconnect {
			t.Fatalf("input %q disconnected", step.in)
		}
		if !strings.Contains(out, step.want) {
			t.Fatalf("input %q missing %q: %q", step.in, step.want, out)
		}
	}

	posts := store.ListPosts(runtime.Host.ID)
	found := false
	for _, p := range posts {
		if p.Author == "TARO YAMADA" && p.Subject == "TEST MESSAGE" {
			found = true
			if p.BoardID != "1" || p.Body != "HELLO FROM 1996" {
				t.Fatalf("stored post mismatch: %+v", p)
			}
		}
	}
	if !found {
		t.Fatal("public message was not persisted")
	}
}

func TestReadSectionAndRelog(t *testing.T) {
	runtime, _ := sampleRuntime(t)
	loginRegular(t, runtime)

	out, disconnect := runtime.HandleLine("R;S;4")
	if disconnect {
		t.Fatal("read-section chain disconnected")
	}
	if !strings.Contains(out, "2400bps") || !strings.Contains(out, "#602") {
		t.Fatalf("section read did not return section 4 message: %q", out)
	}

	out, disconnect = runtime.HandleLine("Q")
	if disconnect {
		t.Fatal("Q should relog without disconnecting")
	}
	if !strings.Contains(out, "What is your full name?") {
		t.Fatalf("Q did not return to signon: %q", out)
	}
}

func TestObservationBoardsAreTurboBBSSections(t *testing.T) {
	runtime, _ := sampleRuntime(t)
	boards := runtime.ObservationBoards()
	if len(boards) != 10 {
		t.Fatalf("got %d sections, want 10", len(boards))
	}
	if boards[0].ID != "1" || boards[0].Name != "GENERAL" {
		t.Fatalf("unexpected first section: %+v", boards[0])
	}
}

func TestReadmeNamesTheHostItIsRunningOn(t *testing.T) {
	runtime, _ := sampleRuntime(t)
	loginRegular(t, runtime)

	for _, in := range []string{"F", "T"} {
		if _, disconnect := runtime.HandleLine(in); disconnect {
			t.Fatalf("%q disconnected", in)
		}
	}
	out, disconnect := runtime.HandleLine("README.TXT")
	if disconnect {
		t.Fatal("README.TXT disconnected")
	}
	if !strings.Contains(out, "TURBOBBS TEST user information.") {
		t.Fatalf("README does not use the host name: %q", out)
	}
	if strings.Contains(out, "SILVER HORIZON") {
		t.Fatalf("README still contains a hard-coded station name: %q", out)
	}
}
