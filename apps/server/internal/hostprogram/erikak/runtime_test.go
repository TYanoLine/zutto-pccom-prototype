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

func TestWelcomeAndNumericBoardNavigation(t *testing.T) {
	runtime, _ := sampleRuntime(t)
	if got := runtime.Welcome(); !strings.Contains(got, "絵理香K版") {
		t.Fatalf("welcome does not identify Erika K runtime: %q", got)
	}

	out, disconnect := runtime.HandleLine("")
	if disconnect || !strings.Contains(out, "MAIN MENU") {
		t.Fatalf("login should enter main menu: %q", out)
	}

	out, disconnect = runtime.HandleLine("1")
	if disconnect || !strings.Contains(out, "雑談・ローカル") || !strings.Contains(out, "101") {
		t.Fatalf("numeric board shortcut failed: %q", out)
	}
}

func TestThreadRendersAppendsTogether(t *testing.T) {
	runtime, _ := sampleRuntime(t)
	runtime.HandleLine("GUEST")
	runtime.HandleLine("1")

	out, disconnect := runtime.HandleLine("101")
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
	runtime.HandleLine("TESTER")
	runtime.HandleLine("1")
	runtime.HandleLine("101")

	out, disconnect := runtime.HandleLine("A")
	if disconnect || !strings.Contains(out, "アペを入力") {
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

func TestHiddenNumericBoardIsReachableButNotListed(t *testing.T) {
	runtime, _ := sampleRuntime(t)
	out, _ := runtime.HandleLine("")
	if strings.Contains(out, "夜更かし部屋") {
		t.Fatalf("hidden board leaked into visible main menu: %q", out)
	}

	out, disconnect := runtime.HandleLine("9")
	if disconnect || !strings.Contains(out, "夜更かし部屋") || !strings.Contains(out, "901") {
		t.Fatalf("hidden numeric board should remain directly reachable: %q", out)
	}
}
