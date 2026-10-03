package historicalkb

import "time"

// PeriodReferent is a narrowly sourced existence claim, not a persona fact or
// event template. AvailableFrom is conservative when only a year/month is known.
type PeriodReferent struct {
	Name string
	AvailableFrom string
	Claim string
	SourceURL string
}

// PeriodReferents returns a fresh, date-filtered copy. Invalid dates fail closed.
// See docs/research/PERIOD_REFERENTS.md for evidence scope and limitations.
func PeriodReferents(worldDate string) []PeriodReferent {
	date, err := time.Parse("2006-01-02", worldDate)
	if err != nil { return nil }
	var out []PeriodReferent
	for _, item := range periodReferents {
		start, err := time.Parse("2006-01-02", item.AvailableFrom)
		if err == nil && !date.Before(start) { out = append(out, item) }
	}
	return out
}

var periodReferents = []PeriodReferent{
	{"PC-9801", "1983-01-01", "NECのパソコンPC-9801が存在する。後継機の型番や仕様はここから補完しない。", "https://jpn.nec.com/profile/corp/history.html"},
	{"NIFTY-Serve", "1987-04-15", "NIFTY-Serveはパソコン通信サービス。この架空局とは別の外部サービスであり、会員資格・料金・フォーラム名は未設定。", "https://www.nifty.co.jp/company/history/"},
	{"SC-55", "1992-01-01", "ローランドのSC-55 SOUND CANVASは音源機器。人物の所有・使用歴、接続条件や音色数は未設定。", "https://www.roland.com/jp/company/history/"},
	{"スーパーメトロイド", "1994-03-19", "スーパーメトロイドはスーパーファミコン用ソフトとして発売済み。攻略、登場要素、人物のプレイ歴は未設定。", "https://www.nintendo.co.jp/corporate/release/2017/170627.html"},
	{"ファイナルファンタジーVI", "1994-04-02", "ファイナルファンタジーVI（FFVI）はスーパーファミコン用ソフトとして発売済み。人物のプレイ歴、キャラクター嗜好、攻略内容は未設定。", "https://support.jp.square-enix.com/faqarticle.php?id=195&kid=45701&la=0&ret=main"},
	{"スーパーストリートファイターII", "1994-06-25", "スーパーストリートファイターIIはスーパーファミコン用ソフトとして発売済み。技、キャラクター、対戦経験は未設定。", "https://www.nintendo.co.jp/corporate/release/2017/170627.html"},
	{"セガサターン", "1994-11-22", "セガサターンは家庭用ゲーム機。人物の所有・購入・流行度は未設定。", "https://www.sega.jp/history/hard/column/column_05.html"},
	{"スーパードンキーコング", "1994-11-26", "スーパードンキーコングはスーパーファミコン用ソフトとして発売済み。攻略、登場要素、人物のプレイ歴は未設定。", "https://www.nintendo.co.jp/corporate/release/2017/170627.html"},
	{"PlayStation", "1994-12-03", "PlayStation（プレイステーション）は日本で発売された家庭用ゲーム機。人物の所有・購入・流行度は未設定。", "https://www.playstation.com/ja-jp/playstation-history/1994-ps-one/"},
	{"ときめきメモリアル", "1995-01-01", "ときめきメモリアルは1994年にPCエンジン向け第1作が登場した恋愛シミュレーションゲーム。人物のプレイ歴、攻略対象、進行状況は未設定。", "https://www.konami.com/games/corporate/ja/news/topics/20250203/"},
	{"一太郎Ver.6", "1995-02-01", "一太郎Ver.6 for Windowsは日本語ワープロソフト。別バージョンの機能や対応機種を補完しない。", "https://www.ichitaro.com/history/tw06.html"},
	{"パンツァードラグーン", "1995-03-10", "パンツァードラグーンはセガサターン用シューティングゲーム。登場人物・面・攻略方法は未供給。", "https://www.sega.jp/history/hard/segasaturn/software.html"},
	{"クロノ・トリガー", "1995-03-11", "クロノ・トリガーはスーパーファミコン用RPGとして発売済み。人物のプレイ歴、進行状況、攻略内容は未設定。", "https://www.nintendo.co.jp/wii/vc/vc_chr/vc_chr_01.html"},
	{"スーパーマリオ ヨッシーアイランド", "1995-08-05", "スーパーマリオ ヨッシーアイランドはスーパーファミコン用ソフトとして発売済み。攻略、登場要素、人物のプレイ歴は未設定。", "https://www.nintendo.co.jp/corporate/release/2017/170627.html"},
	{"パネルでポン", "1995-10-27", "パネルでポンはスーパーファミコン用ソフトとして発売済み。ルール詳細、攻略、人物のプレイ歴は未設定。", "https://www.nintendo.co.jp/corporate/release/2017/170627.html"},
	{"Windows 95", "1995-11-23", "Windows 95日本語版が発売済みのOSとして存在する。人物の導入・更新や個別ソフトとの互換性は未設定。", "https://news.microsoft.com/source/1998/06/17/windows-98-available-in-japanese/"},
	{"バーチャファイター２", "1995-12-01", "バーチャファイター２はセガサターン用アクションゲーム。技・キャラクター・攻略や売上は未供給。", "https://www.sega.jp/history/hard/segasaturn/software.html"},
	{"ドラゴンクエストVI 幻の大地", "1996-01-01", "ドラゴンクエストVI 幻の大地（ドラクエVI）は1995年12月までにスーパーファミコン用RPGとして発売済み。人物のプレイ歴、進行状況、攻略内容は未設定。", "https://www.jp.square-enix.com/game/detail/dq6/"},
	{"Jリーグ実況ウイニングイレブン", "1996-01-01", "Jリーグ実況ウイニングイレブンは1995年にPlayStation向けタイトルとして存在する。選手、チーム、攻略、人物のプレイ歴は未設定。", "https://www.konami.com/corporate/ja/history/product.html"},
	{"幻想水滸伝", "1996-01-01", "幻想水滸伝は1995年に日本でリリースされたRPG。人物のプレイ歴、登場人物、攻略内容は未設定。", "https://www.konami.com/games/suikoden/ja/cp/suki"},
	{"ポケットモンスター 赤・緑", "1996-02-27", "ポケットモンスター 赤・緑はゲームボーイ用ソフト。後続作品・アニメ・キャラクター・攻略方法は未供給。", "https://www.nintendo.co.jp/ds/interview/ipkj/vol1/index.html"},
	{"スーパーマリオRPG", "1996-03-09", "スーパーマリオRPGはスーパーファミコン用RPGとして発売済み。人物のプレイ歴、登場人物、攻略内容は未設定。", "https://www.nintendo.co.jp/clvs/soft/mario_rpg.html"},
	{"星のカービィ スーパーデラックス", "1996-03-21", "星のカービィ スーパーデラックスはスーパーファミコン用ソフトとして発売済み。攻略、登場要素、人物のプレイ歴は未設定。", "https://www.nintendo.co.jp/corporate/release/2017/170627.html"},
	{"バイオハザード", "1996-03-22", "バイオハザードはPlayStation用サバイバルホラーとして発売済み。人物のプレイ歴、登場人物、攻略内容は未設定。", "https://www.capcom.co.jp/ir/news/html/200612b.html"},
}
