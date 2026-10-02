package worldrepo

import (
    "testing"

    "zutto-pccom/apps/server/internal/world"
)

// Board domain is internal routing context, not a preset topic for the model.
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

