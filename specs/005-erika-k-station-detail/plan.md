# Plan: Erika-K station detail

**Spec**: `specs/005-erika-k-station-detail/spec.md`（改訂 2）

## 現状の分類（`erikak/runtime.go`）

| 種類 | 内容 | この spec での扱い |
|---|---|---|
| 局のデータ | `boardTree`（29 件）、`unreadBoard`（5 件） | `detail.erika_k.boards` に外部化 |
| 局に固有の文字列 | ログイン後の見出し 2 行（`WELCOME TO …`、`ERIKA-K`）、`■` の罫線と局のメッセージ、会員へのあいさつ、メインメニューの見出し、終了時の「また … でお会いしましょう。」 | `detail.erika_k.texts` に外部化。**ホストは、そのまま出力するだけ** |
| ソフトの動作 | 状態遷移、コマンド、画面の組み立て、ヘルプ、`Config`（機能の有効・無効）、「前回アクセス」の行、「ご利用ありがとうございました。」の行 | `erikak` に残す |
| プロトタイプ用の固定画面 | `V`、`WHO`、`MEMB`、メール一覧、ファイル一覧、`JUNK`、前回アクセスの日時 | **スコープ外。コードに残す** |

## 設計の原則

- 局の文字列は、**局の定義に書き、ホストは加工せずに出力する**。罫線を描く、見出しを組み立てる、局名を差し込む、
  全角に変換する、といった処理を、ホストに持たない。
- **省略したキーは、何も出力しない。** プログラムは、既定の文面を持たない（局が書かなければ、その部品は画面に出ない）。
- 局の文字列は、名前の付いた部品の表（`texts`）にする。将来、メインメニューなどの部品を、局の定義で上書きできるように
  するときは、キーを足すだけにする（スキーマの作り直しをしない）。

## データの流れ

```text
presets/*.yaml ──(ParsePreset が構造を検証)──▶ hostcatalog.Preset.Detail.ErikaK
      │
      ▼
world.presetData.details[key]  ──▶ MemoryStore.HostDetail(hostID)   ※ world.HostDetailStore
      │                                      ▲
      │                          worldrepo.Repository.HostDetail   （Base に委譲）
      ▼
erikak.New(host, store) が store.(world.HostDetailStore) から取得 ──▶ Runtime が texts と板のカタログを持つ
```

- `world.Host` には入れない（`Host` は `==` で比較される。スライスを入れると壊れる）。
- 型の定義は `hostcatalog` に置く（`Population` と同じ方針）。`hostcatalog` は `erikak` も `world` も import しない。
  `erikak` が `hostcatalog` を import する（循環しない）。
- 板の一覧は、`Runtime` が構築時に自分のカタログへコピーする。プロセス全体で共有するグローバル変数を持たない。
- `*world.MemoryStore` を埋め込んだ既存のテスト用ストアは、メソッドの昇格で `HostDetailStore` を満たす。

## スキーマ

```yaml
detail:
  erika_k:
    texts:                     # 画面の部品。書いたものを、そのまま出力する。省略したキーは何も出さない
      login_banner:            # ログイン直後（「前回アクセス」の行の次）に出す行。1 要素 = 1 行
        - "######################### WELCOME TO HAKATA CANAL NET ##########################"
        - "■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■"
        - "■  博多から、夜更かしネットワーカーのみなさんへ。                            ■"
        - "■  23:00以降は混み合います。長時間の席取りはほどほどに(^^;                   ■"
        - "■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■"
        - "################################### ERIKA-K ####################################"
      login_greeting: "深夜のアクセスご苦労様！ {handle}さん、いらっしゃいませ。"   # 空行を前後に付けて出す
      main_menu_title: "-ＨＡＫＡＴＡ ＣＡＮＡＬ ＮＥＴ-  〖Ｍain Ｍenu〗  絵理香Ｋ版"   # メインメニューの 1 行目
      goodbye: "また HAKATA CANAL NET でお会いしましょう。"                       # 「ご利用ありがとうございました。」の次の行
    boards:                    # 表示順。親は子より前でなくてもよいが、存在する必要がある
      - path: "1"
        name: "事務局からのお知らせ"
        scope: "HAKATA局のSYSOPによる…"
        root_author_policy: sysop_only
        activity_weight: 0.10
        reply_rate: 0.20
        retained_root_cap: 24
        unread: true
      - path: "10"
        alias: "HAKATA"
        name: "博多・天神広場"
      - path: "99"
        name: "夜更かし部屋"
        hidden: true
        ...
```

- 上の `texts` の値は説明用の抜粋。**実際の値は、変更前のコードで生成した金型（`screens_golden.txt`）から、
  末尾の空白を含めて、そのまま写す**（下の「`texts` の値の写し方」）。罫線や、右端の `■` までの空白を、手で整えない。
- 置換できるのは `{handle}` だけ。どの部品の中でも使える（現状は `login_greeting` だけが使う）。

### `texts` の部品と、ホストの出力

| キー | 型 | ホストの出力 | 省略したとき |
|---|---|---|---|
| `login_banner` | 文字列のリスト | 各要素に `{handle}` の置換をして、`"\r\n"` を付けて、順に出す。位置は、`前回アクセス …` の行と、その後の空行の次 | 何も出さない |
| `login_greeting` | 文字列 | `"\r\n" + 本文 + "\r\n"`（`{handle}` を置換）。`login_banner` の次、メインメニューの前 | 何も出さない |
| `main_menu_title` | 文字列 | メインメニューの 1 行目として、`"\r\n" + 本文 + "\r\n"` の後に区切り線以降を続ける | 見出しの行を出さない（`"\r\n"` の次に、区切り線から始める） |
| `goodbye` | 文字列 | `"\r\nご利用ありがとうございました。\r\n"` の次に、`本文 + "\r\n"` | 「ご利用ありがとうございました。」の行だけを出す |

ログイン直後の出力の組み立て（現在のコードと同じ並びになる）:

```text
"\r\n前回アクセス <日時>\r\n\r\n"      ← ホストの標準の動作（コードに残す）
login_banner の各行 + "\r\n"            ← 局の文字列
"\r\n" + login_greeting + "\r\n"        ← 局の文字列（省略なら出さない）
メインメニュー                          ← main_menu_title が 1 行目
```

### `texts` の値の写し方（HAKATA）

変更前のコードで生成した画面の金型（`screens_golden.txt`）の、**ゲストでログインした出力**から、次のとおりに写す。

| キー | 写し元 |
|---|---|
| `login_banner` | `前回アクセス …` と、その次の空行より後の、**`ERIKA-K` の見出し行まで**（6 行）。`<CRLF>` の記号は除く。行末の空白は、すべて残す |
| `login_greeting` | `深夜のアクセスご苦労様！ GUESTさん、いらっしゃいませ。` の行。`GUEST` を `{handle}` に置き換える |
| `main_menu_title` | ログイン直後のメインメニューの 1 行目（`-ＨＡＫＡＴＡ … 絵理香Ｋ版`） |
| `goodbye` | `9` の出力の、`ご利用ありがとうございました。` の次の行 |

写したあとの出力が、金型と 1 バイトも違わないことを、金型テストで確認する。

### 板の項目と、現在の Go の項目の対応

| YAML | Go（`boardNode`） | 備考 |
|---|---|---|
| `path` | `Path` | 引用符で囲む（`"10/1"`）。必須 |
| （書かない） | `Key` | `path` の最後の区切りから導出する（`"10/1"` → `"1"`、`"10"` → `"10"`） |
| （書かない） | `Parent` | `path` から最後の区切りを除いたもの（`"10/1"` → `"10"`、トップレベルは空） |
| `alias` | `Alias` | 省略可 |
| `name` | `Name` | 必須 |
| `hidden` | `Hidden` | 省略時 `false` |
| `scope` | `SemanticScope` | 省略可 |
| `root_author_policy` | `RootAuthorPolicy` | `sysop_only` か省略 |
| `activity_weight` | `ActivityWeight` | 省略時 `0` |
| `reply_rate` | `ReplyRate` | 省略時 `0` |
| `retained_root_cap` | `RetainedRootCap` | 省略時 `0` |
| `verified_referent_rate` | `VerifiedReferentRate` | 省略時 `0` |
| `unread: true` | `unreadBoard[path]` | 現在 `1`、`4`、`10/2`、`60/1`、`60/3` が対象 |

- `boardTree` の**全 29 件（トップレベル 15 件、子 14 件）を、現在の順序のまま**写す。ゼロ値の項目は書かない。
- 値は 1 文字も変えない。数値は `.10` を `0.10` と書くなど、表記だけ YAML に合わせる（同じ値になる）。
- 文字列はすべて二重引用符で囲む（コロンや記号を含む文字列があるため）。
- `boardTree` の上にある 2 つのコメント（「Activity values below are HAKATA station fiction…」と、
  「"夢工房はかた" の史実上の用途は未確認…」）は、YAML のコメントとして、対応する板の近くに移す。

## 検証規則

`ParsePreset` が構造を、`erikak.ValidateDetail` が表示の都合を検査する。すべての違反を一度に報告する。

| 規則 | 場所 |
|---|---|
| `detail.erika_k` は、`host.program` が `erika-k` のときだけ書ける | `ParsePreset` |
| `boards[].path` は必須で、`^[0-9]+(/[0-9]+)*$` に一致する | `ParsePreset` |
| `boards[].path` は重複しない | `ParsePreset` |
| `boards[].path` の親（最後の区切りを除いたもの）は、存在する | `ParsePreset` |
| `boards[].name` は、前後の空白を除いて空でない | `ParsePreset` |
| `root_author_policy` は、空か `sysop_only` | `ParsePreset` |
| `activity_weight`、`reply_rate`、`retained_root_cap`、`verified_referent_rate` は 0 以上（`verified_referent_rate` は 0〜1） | `ParsePreset` |
| `texts` のすべての文字列に、制御文字（`\r`、`\n`、`\t`、ESC など、0x20 未満と 0x7F）を含まない | `ParsePreset` |
| `texts` のすべての文字列に、`{handle}` 以外の `{…}` を含まない | `ParsePreset` |
| `texts` の各文字列（`login_banner` は各要素）の表示幅は、80 セル以下（`{handle}` は 8 セルとして数える） | `erikak.ValidateDetail` |

- `login_banner` の要素は、空文字列でもよい（空行を表す）。
- 表示幅は、`erikak` の既存の `displayCellWidth` で数える。独自の幅計算を作らない。
- `ValidateDetail` は、埋め込みの全 preset に対して呼ぶテストを置く（読み込み時の検査には含めない。
  `hostcatalog` が `erikak` を import できないため）。

## 金型（変更前のコードで記録する）

| 金型 | 内容 | ファイル |
|---|---|---|
| 板の表 | `boardTree` の全項目（導出される `Key`、`Parent` を含む）と、未読マークの有無。JSON | `erikak/testdata/boards_golden.json` |
| 画面の出力 | スクリプト化した操作の出力の全文。テキスト | `erikak/testdata/screens_golden.txt` |

画面のスクリプト（HAKATA の局、`*world.MemoryStore`、投稿なし）:

1. `Welcome()` の出力。
2. ゲストでログイン（`GUEST`）の出力。
3. `1`（板のルートメニュー）、`/`。
4. 板の表の**全項目（隠し板を含む）**について、表の順に `BJ <path>` と `/`。
5. `MA`、`T`、`H`、`MEMB`、`V`、`WHO`、`9`（終了）。
6. 別の `Runtime` で、会員ログイン（`MIKI`、パスワード `dummy`）の出力。

各入力と出力を、区切り付きで 1 つのテキストにする。投稿が無いストアなので、日時を含む画面は出ない
（出力は、時刻や乱数に依存しない）。

**金型は、変更前の `main` のコード（`boardTree` がまだある状態）で生成し、最初のコミットに含める。以降、金型ファイルを書き換えない。**
実装後のコードで生成した金型は、同義反復になり、根拠にならない。失敗したら実装の誤りを疑う。

## 変更するファイル

| 層 | ファイル | 内容 |
|---|---|---|
| 金型 | `erikak/golden_test.go`（新規）、`erikak/testdata/*`（新規） | 変更前のコードで生成 |
| hostcatalog | `program_detail.go`（新規） | `ErikaKDetail`、`ErikaKTexts` などの型と、構造の検証 |
| hostcatalog | `preset.go` | `PresetDetail.ErikaK`、プログラムの一致検査、検証の呼び出し |
| hostcatalog | `program_detail_test.go`（新規） | 検証規則のテスト |
| hostcatalog | `presets/hakata-canal-net.yaml` | `detail.erika_k` を追加、`revision: 4` |
| world | `store.go` | `HostDetailStore`、`MemoryStore.details`、`HostDetail` |
| world | `preset_hosts.go` | `presetData` に `details` を追加 |
| world | `host_detail_test.go`（新規） | `HostDetail` のテスト |
| worldrepo | `repository.go`、テスト | `Repository.HostDetail`（`Base` に委譲） |
| erikak | `runtime.go` | グローバルな板の表を、`Runtime` のカタログに置き換え。局の文字列を、`texts` から出力 |
| erikak | `catalog.go`（新規） | 板のカタログ（`boardCatalog`） |
| erikak | `detail.go`（新規） | 詳細の取得、`ValidateDetail` |
| erikak | `runtime_test.go`、`width_test.go` ほか | 局の定義を持つ `Runtime` を使う形に更新 |
| server | `cmd/server/main.go` | `BoardByPath` に、`HostDetailStore` から得た定義を渡す |
| Doc | `hostcatalog/README.md` | `detail.erika_k` の書式 |

## リスクと対策

| リスク | 対策 |
|---|---|
| 画面の 1 文字の違い（空白、改行、全角・半角、罫線の長さ） | 画面の金型。変更前に生成し、変更しない。`texts` の値は、金型から写す |
| 金型が、実装後のコードから作られて、根拠にならない | 金型の生成を、変更前の `main` で行う（T001〜T003）。レビューで、金型ファイルの履歴が 1 コミットだけで、実装より前にあることを確認する |
| 板の順序や項目の取りこぼし | 板の表の金型。`Key`・`Parent` の導出も含めて比較する |
| `Runtime{}`（ゼロ値）を直接組み立てるテストが panic する | カタログと `texts` のゼロ値を安全にする（FR-008）。`finishLogin` などを使うテストは、局の定義付きの `Runtime` に直す |
| 既存のテストが、グローバルな `BoardByPath` を呼んでいる | 定義を引数に取る形に直し、テスト用の取得ヘルパー（`sampleDetail`）を置く |
| YAML の記法（コロン、引用符、数値の表記、行末の空白） | すべての文字列を二重引用符で囲む。金型が検出する |
| 罫線・見出しをコードで描く処理が残る | `login_banner` の出力に `doubleCellRule`・`boxedLine`・`decorativeLine` を使わない。他で使われなくなれば削除する |
| `Host` の比較が壊れる | 詳細を `Host` に入れない |
| 住民の定義（spec 004）と同時に変更する | 別 PR。`population` の部分には触れない |
