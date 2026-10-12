package worldrepo

import (
	"strings"
	"testing"
	"time"

	"zutto-pccom/apps/server/internal/historicalkb"
	"zutto-pccom/apps/server/internal/hostcatalog"
	"zutto-pccom/apps/server/internal/llm"
	"zutto-pccom/apps/server/internal/world"
	"zutto-pccom/apps/server/internal/worldengine"
)

func TestRepositorySysopPersonaProperties(t *testing.T) {
	repo := &Repository{}
	host := world.Host{
		ID:        "hakata-canal-net",
		Name:      "HAKATA CANAL NET",
		Region:    "福岡県福岡市",
		Software:  "絵理香K版",
		FoundedOn: "1994-11-03",
		Lines:     3,
		MaxBaud:   14400,
	}

	p := repo.sysopPersona(host)
	if p.ID != "hakata-canal-net:sysop" {
		t.Fatalf("expected persona ID 'hakata-canal-net:sysop', got %q", p.ID)
	}
	if p.Handle != "SYSOP" {
		t.Fatalf("expected handle 'SYSOP', got %q", p.Handle)
	}
	if p.Occupation != "局長・システムオペレータ" {
		t.Fatalf("expected occupation '局長・システムオペレータ', got %q", p.Occupation)
	}
	if len(p.EverydayContext) < 3 {
		t.Fatalf("expected at least 3 everyday context entries, got %#v", p.EverydayContext)
	}
	foundHost := false
	foundSpecs := false
	for _, ctx := range p.EverydayContext {
		if strings.Contains(ctx, "HAKATA CANAL NET") && strings.Contains(ctx, "1994-11-03") {
			foundHost = true
		}
		if strings.Contains(ctx, "3回線") && strings.Contains(ctx, "14400 bps") {
			foundSpecs = true
		}
	}
	if !foundHost {
		t.Fatalf("everyday context missing host name/founded on: %#v", p.EverydayContext)
	}
	if !foundSpecs {
		t.Fatalf("everyday context missing lines/baud specs: %#v", p.EverydayContext)
	}
}

func TestRepositoryResolvePersonaForSysop(t *testing.T) {
	repo := &Repository{}
	host := world.Host{
		ID:     "hakata-canal-net",
		Name:   "HAKATA CANAL NET",
		Region: "福岡県福岡市",
	}

	// 1. Resolve by Author == "SYSOP"
	p1 := repo.resolvePersona(host, "", "SYSOP")
	if p1 == nil || p1.Handle != "SYSOP" || p1.ID != "hakata-canal-net:sysop" {
		t.Fatalf("expected SYSOP persona resolved by author, got %#v", p1)
	}

	// 2. Resolve by personaID == "hakata-canal-net:sysop"
	p2 := repo.resolvePersona(host, "hakata-canal-net:sysop", "")
	if p2 == nil || p2.Handle != "SYSOP" || p2.ID != "hakata-canal-net:sysop" {
		t.Fatalf("expected SYSOP persona resolved by personaID, got %#v", p2)
	}
}

func TestMaterializeArticleBodyOnceForSysop(t *testing.T) {
	host := world.Host{
		ID:        "hakata-canal-net",
		Name:      "HAKATA CANAL NET",
		Region:    "福岡県福岡市",
		Software:  "絵理香K版",
		FoundedOn: "1994-11-03",
		Lines:     3,
		MaxBaud:   14400,
		Generation: hostcatalog.GenerationFlags{FreeformBody: true},
	}
	board := world.Board{
		ID:               "1",
		Name:             "事務局からのお知らせ",
		RootAuthorPolicy: "sysop_only",
	}

	renderer := &fakeBoardRenderer{
		draft: llm.BoardPostDraft{
			Author:  "SYSOP",
			Subject: "不具合報告には日時・操作・画面表示を",
			Body:    "局内で不具合が起きた際は、日時と操作内容、画面表示を添えてご連絡ください。",
		},
	}
	mat := LLMMaterializer{Renderer: renderer}
	memStore := world.NewMemoryStore()
	repo := &Repository{
		Base:         memStore,
		Materializer: mat,
		freeformBody: true,
	}

	post := world.Post{
		ID:              1840,
		BoardID:         "1",
		Author:          "SYSOP",
		AuthorPersonaID: "hakata-canal-net:sysop",
		Subject:         "不具合報告には日時・操作・画面表示を",
		CreatedAt:       time.Date(1996, 4, 16, 22, 58, 0, 0, time.Local),
	}
	memStore.AddPost(host.ID, post)

	res, found, generated, diag := repo.materializeArticleBodyOnce(host, board, post)
	if !found {
		t.Fatalf("expected post to be found")
	}
	if strings.Contains(diag, "error stage=") {
		t.Fatalf("materialization failed with diagnostic: %s", diag)
	}
	if !generated {
		t.Fatalf("expected post body to be generated")
	}
	if res.Body == "" {
		t.Fatalf("expected non-empty body")
	}
	if res.Author != "SYSOP" {
		t.Fatalf("expected Author 'SYSOP', got %q", res.Author)
	}
	if renderer.req.AuthorHandle != "SYSOP" {
		t.Fatalf("expected renderer AuthorHandle 'SYSOP', got %q", renderer.req.AuthorHandle)
	}
	if renderer.req.PersonaProfile == "" || !strings.Contains(renderer.req.PersonaProfile, "システムオペレータ") {
		t.Fatalf("expected persona profile with sysop role, got %q", renderer.req.PersonaProfile)
	}
}

var _ = historicalkb.EvidenceAtmospheric
var _ = worldengine.EvidenceDecision{}
