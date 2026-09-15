package buildinfo

import "testing"

func TestCurrentPrefersExplicitBuildCommit(t *testing.T) {
	t.Setenv("ZUTTO_BUILD_COMMIT", "1234567890abcdef")
	t.Setenv("ZUTTO_BUILD_BRANCH", "test-branch")
	t.Setenv("RENDER_GIT_COMMIT", "ffffffffffffffff")
	t.Setenv("RENDER_GIT_BRANCH", "main")
	t.Setenv("RENDER", "true")

	got := Current()
	if got.Commit != "1234567890abcdef" {
		t.Fatalf("commit = %q", got.Commit)
	}
	if got.ShortCommit != "1234567890ab" {
		t.Fatalf("short commit = %q", got.ShortCommit)
	}
	if got.Branch != "test-branch" {
		t.Fatalf("branch = %q", got.Branch)
	}
	if got.Source != "render" {
		t.Fatalf("source = %q", got.Source)
	}
	if got.StartedAt.IsZero() {
		t.Fatal("started_at must be populated")
	}
}

func TestShortCommit(t *testing.T) {
	if got := ShortCommit("abcdef"); got != "abcdef" {
		t.Fatalf("short commit = %q", got)
	}
	if got := ShortCommit(""); got != "unknown" {
		t.Fatalf("empty short commit = %q", got)
	}
}
