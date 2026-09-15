package materializationdemo

import (
	"strings"
	"testing"

	"zutto-pccom/apps/server/internal/world"
)

func TestWelcomeShowsBuildIdentity(t *testing.T) {
	t.Setenv("ZUTTO_BUILD_COMMIT", "abcdef1234567890")
	t.Setenv("ZUTTO_BUILD_BRANCH", "test-build")
	runtime := New(world.Host{Name: "TEST", Region: "TEST", Software: "TEST", Lines: 1, MaxBaud: 14400}, world.NewMemoryStore())

	welcome := runtime.Welcome()
	if !strings.Contains(welcome, "[DEV] BUILD") {
		t.Fatalf("welcome missing build line: %q", welcome)
	}
	if !strings.Contains(welcome, "abcdef123456") {
		t.Fatalf("welcome missing short commit: %q", welcome)
	}
	if !strings.Contains(welcome, "branch=test-build") {
		t.Fatalf("welcome missing branch: %q", welcome)
	}
}
