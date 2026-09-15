package worldrepo

import (
	"strings"
	"testing"

	"zutto-pccom/apps/server/internal/llm"
	"zutto-pccom/apps/server/internal/world"
)

func TestDevelopmentArticleReportsAndRetainsTokenUsage(t *testing.T) {
	base := world.NewMemoryStore()
	engine := &flowEngine{}
	renderer := &fakeBoardRenderer{draft: llm.BoardPostDraft{
		Author:  "IGNORED",
		Subject: "IGNORED",
		Body:    "Personaに沿って補完された本文です。",
		Usage: llm.TokenUsage{
			InputTokens:       700,
			CachedInputTokens: 128,
			OutputTokens:      80,
			ReasoningTokens:   12,
			TotalTokens:       780,
			Model:             "gpt-test",
		},
	}}
	repo := New(base, engine, LLMMaterializer{Renderer: renderer, Fallback: FallbackMaterializer{}}, "1996-08-29")
	h, err := repo.HostByPhone("0450000196")
	if err != nil {
		t.Fatal(err)
	}
	boards, _ := repo.MaterializationBoards(h)
	headers, _ := repo.MaterializationPersonaArticleHeaders(h, boards[0])
	if len(headers) == 0 {
		t.Fatal("no headers")
	}

	p, found, created, usage := repo.MaterializationArticleWithDebug(h, boards[0], headers[0].ID)
	if !found || !created || p.Body == "" {
		t.Fatalf("article found=%v created=%v body=%q", found, created, p.Body)
	}
	for _, want := range []string{"model=gpt-test", "input=700", "cached=128", "output=80", "reasoning=12", "total=780"} {
		if !strings.Contains(usage, want) {
			t.Fatalf("usage %q missing %q", usage, want)
		}
	}
	if total := repo.MaterializationUsageTotalText(); !strings.Contains(total, "model=gpt-test") || !strings.Contains(total, "total=780") {
		t.Fatalf("unexpected cumulative usage: %q", total)
	}

	_, found, created, reusedUsage := repo.MaterializationArticleWithDebug(h, boards[0], headers[0].ID)
	if !found || created {
		t.Fatalf("re-read found=%v created=%v", found, created)
	}
	if reusedUsage != usage {
		t.Fatalf("usage not retained for debug re-read: first=%q reused=%q", usage, reusedUsage)
	}
}
