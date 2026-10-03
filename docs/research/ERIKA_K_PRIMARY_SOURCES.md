# 絵理香K版（ERIKA-K）一次史料・画面キャプチャ・ログ調査レポート

調査日: 2026-09-27  
保存先ディレクトリ: [`docs/research/erika-k-sources/`](file:///C:/Trunk/GitHub/zutto-pccom-prototype/docs/research/erika-k-sources/)

---

## 1. 調査背景と目的
本プロジェクト（ずっとパソコン通信）における「絵理香K版」ホストプログラム再現（`apps/server/internal/hostprogram/erikak/`）を、単なる汎用BBSメニューのスキンではなく、当時の実機動作・操作体系・画面遷移に忠実な実装へと増強するため、Webアーカイブおよび当時のネット史料から「絵理香K版」を利用していたホスト局の実画面画像・ログ・運営史料を網羅的に発掘・保全しました。

---

## 2. 発掘された一次史料（画面キャプチャ・ログ）

### (1) K&Kネット（宮崎県都城市・個人運営）実画面キャプチャ（1996年12月19日採録）
- **ホスト概要**:
  - ホストOS / 機種: NEC PC-9801VX21 + MS-DOS 5.0
  - ホストプログラム: **絵理香K版 Ver. 1.93C**
  - 回線数: 3回線（24時間運用、最大28,800bps、モデム: がらくた一號 / USR SPORTSTER 14.4k）
  - 開局日: 1988年11月25日（10年間運用後、1998年11月27日廃局）
- **保全ファイル**:
  - [`kklog02.gif`](file:///C:/Trunk/GitHub/zutto-pccom-prototype/docs/research/erika-k-sources/kklog02.gif)（掲示板メニュー及びインデックス表示画面）
  - [`kklog01.gif`](file:///C:/Trunk/GitHub/zutto-pccom-prototype/docs/research/erika-k-sources/kklog01.gif)（ファイルライブラリメニュー及びインデックス表示画面）

#### ◆ 画面構成とプロンプト構文の分析結果:
1. **TOPメニュープロンプト**:
   ```text
   %top> [?]HELP [M]MAP >>
   ```
2. **ボードメニュー（BM）画面ヘッダと凡例**:
   ```text
   《K&KネットBDメニュー》(BV)        @=GUEST可 S=Read Only < >=階層Menu

    1: @ 気まぐれ伝言板               7:   2&4W
    2:   AV+映画・TV                  8:   酒とバラの日々
    3:   アウトドアライフ              9: @ ネットワーク/テレコム情報
    4:   メンバー情報局               28: $ システム連絡
    5:   お堅いのがお好き             32: @ うぉっちBC
    6:   52番街スイングストリート

   <10><SIG>   @ SIG/CUG/NEWB       <12><JITEN> 乱調電脳辞典
   <11><COMP>  @ パーソナルコンピューティング  <46><OLD>   T アーカイブ
   -----------------------------------------------------------------
   [0][00][T][D]未読関連  [=][L]検索  [/][.][`cr`]メニュー移動  [H][?]説明
   %BD Menu(BM)> [M]MENU [?]HELP >> 1
   ```
3. **ボード最新インデックス表示（BX）とプロンプト**:
   ```text
   ★BD# 01 きまぐれ伝言板
   # 最新10インデックス表示
   ___No. __date__ time_ _author_  ap/ref___________i n d e x_______________
   01  231 96/12/18 00:24 GACGAO       原因の一部判明
   01  230 96/12/17 07:00 GACGAO       ＭＩＤＩボードの設定を教えてください。
   01  229 96/12/15 12:05 GACGAO       おかげで完却できました。
   01  228 96/12/12 05:06 GACGAO       エプソン４８６ＡＵ再ド・・・・・
   01  227 96/12/10 20:59 PEERGYNT   1 研修会を催したいのですが。
   01  226 96/12/10 14:06 NOJMI        皆さん久しぶりです。
   01  225 96/12/06 00:28 NICK       2 宮崎インターネットの近況？
   01  224 96/12/05 19:51 GACGAO       とゆるせんぞＣＵＢＩＳシステム
   01  223 96/12/03 12:09 KANTA        ツアー初体験
   01  222 96/12/01 01:00 TATI         ひさびさ
   %BX>`cr`番号[0|00|T][N|B][W][A][J][=][+-][F][S|L][R][K|KA][X|WX][.|/][H|?]
   >>
   ```
   - **「ap/ref」列の存在**: 親記事に対して何件の「アペンド（レス）」が付いているかが件数（例: `1`, `2`）として明示され、アペンドがない場合は空白。
   - **インデックス下のコマンド一覧**:
     - `番号`: 指定記事の閲覧
     - `0` / `00` / `T`: 未読
     - `N` / `B`: 次 / 前（Next / Back）
     - `W`: 新規書き込み（Write）
     - `A`: アペンド書き込み（Append）
     - `J`: ジャンプ（Jump）
     - `F`: 検索（Find）
     - `S` / `L`: 短縮表示 / 詳細表示
     - `K` / `KA`: 既読化（Kill / Kill All）
     - `.` / `/`: 親階層へ戻る / メインメニューへ直帰

4. **ファイルライブラリ（FM/FJ）画面**:
   ```text
   《K&Kネットファイルライブラリ》(FM)FJ\          [G]=GUEST可
   -----------------------------
   階層 <1><DOS1>   実用MS-DOS         ●MS-DOS、Windowsは米国Microsoft社の商標です
   階層 <2><DOS2>   お遊びMS-DOS       ●UNIX(DX/Open)ナンバーノミテッドがライセンス...
        3.          BASIC
        4.          画像
   階層 <5><MINOR>  弱マイナー機種      ●FL #6に登録される各種文書、データ類は
   [G]  6           ドキュメント/PDD      GNU General Public Licenceに準じる扱いとします
        32.         Windows
   階層 <86><lib>   UNIX方面
   -----------------------------------------------------------------
   [0][00][T][D]新着情報  [F][L]検索  [/][.][`cr`]メニュー移動  [H][?]説明
   %FL Menu(FM)> [M]MENU [?]HELP >> 6
   ☆FL# 06 ドキュメント/PDD
   # 最新10インデックス表示
   ___No. __date__ time_ _author_  ap/ref___________i n d e x_______________
   06  184 96/12/18 11:26 SYSOP      1( 55K) sho3rc.lzh 中村正三郎著作物
   06  183 96/12/16 22:48 SYSOP      2( 12K) kkn_data.gif 弊局のアクセス記録
   %FX>番号[`cr`|-][+][W][0|00|T][F][D][N|B][S|L][K][.|/][c][*][H|?] >>
   ```

---

## 3. 発掘された運用ホスト局と証言記録

### (1) 東京謎ねっと２３（東京都文京区・てんてん氏運営）
- **運用期間**: 1991年10月23日〜2000年代初頭
- **アクセス電話番号**: `03-5261-7683`（24時間運用、1200〜28800bps）
- **ホストプログラム**: **絵理香K版**（telnet経由アクセス `happy.tokyo-nazo.net` も併設）
- **史料**:
  - [`tokyo_nazo_20000421.html` 〜 `20000424.html`](file:///C:/Trunk/GitHub/zutto-pccom-prototype/docs/research/erika-k-sources/)「絵理香の話 １〜４」（2000年4月執筆）
  - [`nazo23_access.html`](file:///C:/Trunk/GitHub/zutto-pccom-prototype/docs/research/erika-k-sources/nazo23_access.html), [`nazo23_whatnazo.html`](file:///C:/Trunk/GitHub/zutto-pccom-prototype/docs/research/erika-k-sources/nazo23_whatnazo.html)
- **歴史的背景の証言**:
  - もともと電電公社九州総局が電話代需要喚起のためBCCとともに開発・展開（九州各県に1局ずつ構築）したものが「第一世代絵理香局」。
  - その後、東京の「BBS工事現場」がオープンソース版絵理香を独自改造して劇的に使いやすくしたものが「**工事現場版＝K版**」。
  - 関東に一気に7局以上のK版局が誕生し、サポートBBSも設立された。
  - 後にBCCがK版の商業的成功に目をつけ、権利問題でトラブルが発生しBCC直販に組み込まれた経緯が記録されている。
  - Y2K（2000年問題）非対応であったため、平成パッチ等のワークアラウンドがとられた。

### (2) 草の根BBS どんぐり倶楽部（茨城県常陸太田市・MIHOSHI氏運営）
- **運用期間**: 1994年5月25日開局〜2000年代
- **アクセス電話番号**: `0294-73-2555`（3回線、300〜33600bps）
- **ホストプログラム**: **絵理香K版 Plus!（Main sys: 12 Line対応）** + PDHS!（Sub sys）
- **ホストハードウェア**: NEC PC-9821As2 (DX4-100MHz), RAM 48MB, HDD 1.3GB, USR Courier ×3, MC-RS98シリアルボード
- **史料**:
  - [`donguri_dong.html`](file:///C:/Trunk/GitHub/zutto-pccom-prototype/docs/research/erika-k-sources/donguri_dong.html)（局概要・ホスト室写真説明）
  - [`donguri_new.html`](file:///C:/Trunk/GitHub/zutto-pccom-prototype/docs/research/erika-k-sources/donguri_new.html)（入会案内・HyperTerminal接続設定）

### (3) 『通信用語の基礎知識 ２０００』所収「絵理香」解説（kose3採録）
- **保全ファイル**: [`kose3_erika.html`](file:///C:/Trunk/GitHub/zutto-pccom-prototype/docs/research/erika-k-sources/kose3_erika.html)
- **販売価格**: 標準版 98,000円 / K版 130,000円（BCC直販）
- **最大手導入局**: 当時1万人超の会員を抱えた伝説の大規模草の根BBS「**東京がらくた工房**」（吉野洋充氏 / EXPERT）が絵理香K版を採用。

---

## 4. 本トランク実装へのフィードバック項目

現在のプロトタイプ（`apps/server/internal/hostprogram/erikak/runtime.go`）と一次史料画面の対比により、以下の改善・増強点が具体化されました。

1. **インデックス表示レイアウトの正確化**:
   - `___No. __date__ time_ _author_  ap/ref___________i n d e x_______________`
   - `ap/ref` 列にアペンド数を表示する（親記事＋アペンド構造の視覚化）。
2. **プロンプト文字列の正確化**:
   - トップメニュー: `%top> [?]HELP [M]MAP >> `
   - ボードメニュー: `%BD Menu(BM)> [M]MENU [?]HELP >> `
   - インデックス画面: `%BX>`cr`番号[0|00|T][N|B][W][A][J][=][+-][F][S|L][R][K|KA][X|WX][.|/][H|?] >> `
   - ファイルメニュー: `%FL Menu(FM)> [M]MENU [?]HELP >> `
   - ファイルインデックス: `%FX>番号[`cr`|-][+][W][0|00|T][F][D][N|B][S|L][K][.|/][c][*][H|?] >> `
3. **階層表記・ナビゲーション**:
   - 階層メニューは `<番号><ALIAS>` 形式（例: `<1><COMP>`, `<12><JITEN>`）
   - 親階層へは `.` または `RETURN`、メインへは `/`
