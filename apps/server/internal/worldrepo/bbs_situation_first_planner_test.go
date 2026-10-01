package worldrepo

import (
	"strings"
	"testing"
	"time"

	"zutto-pccom/apps/server/internal/world"
)

func TestProductionEventPeriodFactsRespectEventDate(t *testing.T) {
	host := world.Host{ID: "h"}
	board := world.Board{ID: "game", Name: "ゲーム"}
	persona := world.Persona{ID: "p", Handle: "P"}
	before := productionEventPeriodFacts(host, board, persona, "games", time.Date(1995, 8, 20, 20, 0, 0, 0, time.Local))
	after := productionEventPeriodFacts(host, board, persona, "games", time.Date(1996, 4, 20, 20, 0, 0, 0, time.Local))

	contains := func(values []string, marker string) bool {
		for _, value := range values {
			if strings.Contains(value, marker) {
				return true
			}
		}
		return false
	}
	if contains(before, "ポケットモンスター") || contains(before, "バイオハザード") {
		t.Fatalf("future referent leaked into 1995 event: %#v", before)
	}
	// Stable six-item rotation need not include every later title, so check the
	// historical catalog through multiple event times/identities.
	foundPokemon := contains(after, "ポケットモンスター")
	for i := 0; i < 12 && !foundPokemon; i++ {
		persona.ID = string(rune('a' + i))
		values := productionEventPeriodFacts(host, board, persona, "games", time.Date(1996, 4, 20, 20, i, 0, 0, time.Local))
		foundPokemon = contains(values, "ポケットモンスター")
	}
	if !foundPokemon {
		t.Fatal("date-valid 1996 game referents never became available")
	}
}

func TestProductionBoardDomainUsesBoardMeaningBeforePersonaInterest(t *testing.T) {
	persona := world.Persona{Interests: map[string]float64{"music": .99, "games": .1}}
	board := world.Board{Name: "ＧＡＭＥ", SemanticScope: "ゲームの感想や相談"}
	if got := productionBoardDomain(board, persona); got != "games" {
		t.Fatalf("domain=%q, want games", got)
	}
}

func TestProductionBoardDomainUsesAffirmativeScopeNotExcludedWords(t *testing.T) {
	// These scopes mirror the actual Erika-K board catalog. Negative exclusions
	// later in the description must not be mistaken for the board's topic.
	cases := []struct {
		board world.Board
		want  string
	}{
		{
			board: world.Board{ID: "20/2", Name: "ＡＮＩＭＥ／ＭＡＮＧＡ", SemanticScope: "アニメ、漫画、関連する雑談や感想。ゲームやPC一般は主題にしない。"},
			want: "anime_manga",
		},
		{
			board: world.Board{ID: "20/1", Name: "ＧＡＭＥ", SemanticScope: "家庭用・PC等のゲームについての感想、攻略上の詰まり、対戦、貸し借り、購入相談など。ゲーム以外のPC一般話題を持ち込まない。"},
			want: "games",
		},
		{
			board: world.Board{ID: "60/3", Name: "ＳＯＦＴＷＡＲＥ／ＤＡＴＡ", SemanticScope: "ソフトウェア、データ、ファイル、ツール利用の情報交換。ハードやゲームそのものへ逸れすぎない。"},
			want: "software",
		},
		{
			board: world.Board{ID: "68/1", Name: "深夜雑談", SemanticScope: "深夜に接続している会員のゆるい雑談。日常、眠気、仕事・学校、食事、テレビ、音楽、趣味など幅広く、PC/ゲーム専用ではない。"},
			want: "chat",
		},
		{
			board: world.Board{ID: "60/1", Name: "ＰＣ－９８／ＭＯＤＥＭ", SemanticScope: "PC-98系やモデム、通信環境についての具体的な相談・情報交換。"},
			want: "modem",
		},
	}
	persona := world.Persona{Interests: map[string]float64{"games": .99, "music": .75}}
	for _, tc := range cases {
		if got := productionBoardDomain(tc.board, persona); got != tc.want {
			t.Errorf("board=%s routed to %q, want %q", tc.board.ID, got, tc.want)
		}
	}
}

func TestProductionAnimeMangaHasRelevantActivityForEveryDiscourseMode(t *testing.T) {
	facets := productionAnimeMangaSituationFacets()
	for _, mode := range developmentRootDiscourseModes {
		host := world.Host{ID: "hakata-canal-net"}
		board := world.Board{ID: "20/2", Name: "ＡＮＩＭＥ／ＭＡＮＧＡ", SemanticScope: "アニメ、漫画、関連する雑談や感想。ゲームやPC一般は主題にしない。"}
		persona := world.Persona{ID: "test-member", Interests: map[string]float64{"games": .9}}
		domain := productionBoardDomain(board, persona)
		if domain != "anime_manga" {
			t.Fatalf("anime/manga board domain=%q", domain)
		}
		facet, ok := chooseProductionSituationFacet(host, board, persona, time.Date(1996, 7, 27, 19, 15, 0, 0, time.Local), 1, domain, mode, nil)
		if !ok || !(strings.HasPrefix(facet.kind, "anime_") || strings.HasPrefix(facet.kind, "manga_")) {
			t.Errorf("mode=%s chose unrelated facet: %+v", mode, facet)
		}
		matching := false
		for _, candidate := range facets {
			if candidate.kind == facet.kind && developmentModeFacetAllowed(candidate, mode) {
				matching = true
			}
		}
		if !matching {
			t.Errorf("mode=%s chose an incompatible facet %q", mode, facet.kind)
		}
	}
}

func TestAnimeBoardPeriodMaterialDoesNotInheritGameCatalog(t *testing.T) {
	host := world.Host{ID: "hakata"}
	board := world.Board{Name: "ANIME/MANGA", SemanticScope: "アニメ、漫画の感想。ゲームは主題にしない。"}
	persona := world.Persona{ID: "m", Interests: map[string]float64{"games": 1}}
	domain := productionBoardDomain(board, persona)
	if domain != "anime_manga" {
		t.Fatalf("domain=%q, want anime_manga", domain)
	}
	material := productionEventPeriodFacts(host, board, persona, domain, time.Date(1996, 7, 27, 19, 15, 0, 0, time.Local))
	for _, claim := range material {
		for _, game := range []string{"バイオハザード", "ポケットモンスター", "ファイナルファンタジー", "PlayStation"} {
			if strings.Contains(claim, game) {
				t.Fatalf("game catalog claim leaked into anime/manga board: %q", claim)
			}
		}
	}
}

func TestProductionGameFacetSelectionSpreadsKindsWithinWindow(t *testing.T) {
	host := world.Host{ID: "h"}
	board := world.Board{ID: "g", Name: "ゲーム"}
	persona := world.Persona{ID: "p", Handle: "P"}
	counts := map[string]int{}
	seen := map[string]int{}
	base := time.Date(1996, 3, 1, 20, 0, 0, 0, time.Local)
	for i := 0; i < 50; i++ {
		mode := developmentRootDiscourseModes[i%len(developmentRootDiscourseModes)]
		facet, ok := chooseProductionSituationFacet(host, board, persona, base.Add(time.Duration(i)*time.Hour), i+1, "games", mode, counts)
		if !ok {
			t.Fatalf("no facet for i=%d mode=%s", i, mode)
		}
		counts[facet.kind]++
		seen[facet.kind]++
	}
	if len(seen) < 15 {
		t.Fatalf("only %d distinct game Situation kinds across 50 roots: %#v", len(seen), seen)
	}
	max := 0
	for _, count := range seen {
		if count > max { max = count }
	}
	if max > 8 {
		t.Fatalf("one Situation kind dominated 50-root window: max=%d kinds=%#v", max, seen)
	}
}

func TestProductionSituationUsesFocusWithoutDiagnosticAnonymousExample(t *testing.T) {
	rich := developmentRichGameSituationFacets()
	var password developmentSituationFacet
	for _, candidate := range rich {
		if candidate.kind == "games_password_recording" {
			password = candidate.developmentSituationFacet
			break
		}
	}
	if password.kind == "" {
		t.Fatal("expected diagnostic password facet")
	}
	// The diagnostic PoC intentionally describes an unnamed game, but that
	// experimental constraint must not leak into production generation.
	if !strings.Contains(password.boundary, "unnamed game") {
		t.Fatal("diagnostic fixture unexpectedly changed")
	}
	got := productionSituationFocus(password)
	if got.kind != password.kind || got.summary != password.focus {
		t.Fatalf("focus not preserved: %+v", got)
	}
	// The production material has one World-selected focus, not a list of
	// instructions controlling the model's topic or prose.
	if len(got.facts) != 1 || got.facts[0] != "activity_focus="+password.focus {
		t.Fatalf("production material includes diagnostic constraints: %#v", got.facts)
	}
	if strings.Contains(strings.Join(got.facts, "\n"), password.boundary) {
		t.Fatalf("diagnostic anonymous-game condition leaked into production: %#v", got.facts)
	}
}

func TestProductionGameTopicFacetsAreBroadAndModeCompatible(t *testing.T) {
	extra := productionGameTopicFacets()
	if len(extra) < 3 {
		t.Fatalf("too few production work-specific activity focuses: %d", len(extra))
	}
	for _, candidate := range extra {
		if candidate.kind == "" || candidate.focus == "" {
			t.Fatalf("topic facet lacks a World activity focus: %+v", candidate)
		}
		if len(candidate.occurrences) != 0 || candidate.boundary != "" {
			t.Fatalf("production-only activity focus is prescribing a diagnostic incident: %+v", candidate)
		}
		if len(candidate.modes) == 0 {
			t.Fatalf("topic facet missing mode compatibility: %+v", candidate)
		}
	}
}

func TestProductionFocusIsIdenticalMaterialAcrossPostingModes(t *testing.T) {
	facet := developmentSituationFacet{kind: "chat_hobby", focus: "a short hobby update"}
	got := productionSituationFocus(facet)
	if got.kind != "chat_hobby" || got.summary != "a short hobby update" {
		t.Fatalf("lost World-selected direction: %+v", got)
	}
	if len(got.facts) != 1 || got.facts[0] != "activity_focus=a short hobby update" {
		t.Fatalf("production added an unnecessary prompt rule: %#v", got.facts)
	}
}
