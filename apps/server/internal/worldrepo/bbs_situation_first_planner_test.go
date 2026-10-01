package worldrepo

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"zutto-pccom/apps/server/internal/world"
)

func TestProductionSituationMaterialsContainOnlyWorldData(t *testing.T) {
	focus := productionSituationFocus(developmentSituationFacet{
		kind: "anime_episode_reaction",
		focus: "the member's reaction to an anime episode",
	})
	got := productionSituationMaterials([]string{"existing_interest=anime"}, focus)
	want := []string{
		"persona_context=existing_interest=anime",
		"situation_kind=anime_episode_reaction",
		"activity_focus=the member's reaction to an anime episode",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("automatic historical catalog or extra prompting rules leaked: got %#v, want %#v", got, want)
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
			want: "pc98_modem",
		},
		{board: world.Board{ID:"70/1",Name:"ＰＣ－９８",SemanticScope:"PC-98系機種の利用、設定、周辺機器、ソフト利用など。"}, want:"pc98"},
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

func TestAnimeBoardMaterialUsesAnimeFocusNotGameCatalog(t *testing.T) {
	board := world.Board{Name: "ANIME/MANGA", SemanticScope: "アニメ、漫画の感想。ゲームは主題にしない。"}
	persona := world.Persona{ID: "m", Interests: map[string]float64{"games": 1}}
	domain := productionBoardDomain(board, persona)
	if domain != "anime_manga" {
		t.Fatalf("domain=%q, want anime_manga", domain)
	}
	choices := productionAnimeMangaSituationFacets()
	if len(choices) == 0 { t.Fatal("missing anime/manga activity choices") }
	material := productionSituationMaterials(nil, productionSituationFocus(choices[0].developmentSituationFacet))
	for _, fact := range material {
		for _, wrong := range []string{"period_reference=", "ALLOWED HISTORICAL", "games_"} {
			if strings.Contains(fact, wrong) {
				t.Fatalf("unrelated or automatic reference leaked into anime board: %q", fact)
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

func TestProductionSituationUsesActivityFocusOnly(t *testing.T) {
	choices := developmentRichGameSituationFacets()
	if len(choices) < 20 {t.Fatalf("insufficient production GAME activity variety: %d",len(choices))}
	for _,choice := range choices {
		input:=productionSituationFocus(choice.developmentSituationFacet)
		if input.kind!=choice.kind||input.summary!=choice.focus||len(input.facts)!=1||
			input.facts[0]!="activity_focus="+choice.focus {
			t.Fatalf("non-World scenario rule leaked into production: %+v",input)
		}
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

func TestPC98FacetsAreBoardSpecificAcrossPostingModes(t *testing.T) {
	host := world.Host{ID:"hakata-canal-net"}
	persona := world.Persona{ID:"test",Interests:map[string]float64{"games":1}}
	for _,tc := range []struct{board world.Board; want string}{
		{world.Board{ID:"70/1",Name:"ＰＣ－９８",SemanticScope:"PC-98系機種の利用、設定、周辺機器、ソフト利用など。"},"pc98"},
		{world.Board{ID:"60/1",Name:"ＰＣ－９８／ＭＯＤＥＭ",SemanticScope:"PC-98系やモデム、通信環境についての相談。"},"pc98_modem"},
	}{
		domain:=productionBoardDomain(tc.board,persona)
		if domain!=tc.want {t.Errorf("%s domain=%q want=%q",tc.board.ID,domain,tc.want)}
		for _,mode:=range developmentRootDiscourseModes {
			facet,ok:=chooseProductionSituationFacet(host,tc.board,persona,time.Date(1996,7,27,20,0,0,0,time.Local),1,domain,mode,nil)
			if !ok || !strings.HasPrefix(facet.kind,"pc98_") {t.Errorf("board=%s mode=%s facet=%+v ok=%v",tc.board.ID,mode,facet,ok)}
		}
	}
}
