package buildinfo

import (
	"os"
	"runtime/debug"
	"strings"
	"time"
)

var processStartedAt = time.Now().UTC()

type Info struct {
	Commit      string    `json:"commit"`
	ShortCommit string    `json:"short_commit"`
	Branch      string    `json:"branch,omitempty"`
	Repo        string    `json:"repo,omitempty"`
	Source      string    `json:"source"`
	VCSTime     string    `json:"vcs_time,omitempty"`
	Modified    bool      `json:"modified,omitempty"`
	StartedAt   time.Time `json:"started_at"`
}

func Current() Info {
	info := Info{
		Commit:    firstNonEmpty(os.Getenv("ZUTTO_BUILD_COMMIT"), os.Getenv("RENDER_GIT_COMMIT")),
		Branch:    firstNonEmpty(os.Getenv("ZUTTO_BUILD_BRANCH"), os.Getenv("RENDER_GIT_BRANCH")),
		Repo:      os.Getenv("RENDER_GIT_REPO_SLUG"),
		Source:    "runtime",
		StartedAt: processStartedAt,
	}
	if strings.EqualFold(os.Getenv("RENDER"), "true") {
		info.Source = "render"
	}

	if bi, ok := debug.ReadBuildInfo(); ok {
		for _, setting := range bi.Settings {
			switch setting.Key {
			case "vcs.revision":
				if info.Commit == "" {
					info.Commit = setting.Value
					info.Source = "go-vcs"
				}
			case "vcs.time":
				info.VCSTime = setting.Value
			case "vcs.modified":
				info.Modified = strings.EqualFold(setting.Value, "true")
			}
		}
	}
	if info.Commit == "" {
		info.Commit = "unknown"
	}
	info.ShortCommit = ShortCommit(info.Commit)
	return info
}

func ShortCommit(commit string) string {
	commit = strings.TrimSpace(commit)
	if commit == "" {
		return "unknown"
	}
	if len(commit) > 12 {
		return commit[:12]
	}
	return commit
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}
