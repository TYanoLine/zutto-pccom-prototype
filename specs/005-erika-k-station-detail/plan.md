# Plan: Erika-K station detail

**Spec**: `specs/005-erika-k-station-detail/spec.md`

## 現状の分類（`erikak/runtime.go`）

| 種類 | 内容 | この spec での扱い |
|---|---|---|
| 局のデータ | `boardTree`（26 件）、`unreadBoard`（5 件）、ログイン後の局のメッセージ 2 行、会員へのあいさつ | `detail.erika_k` に外部化 |
| 局名から導出できる | メインメニューの見出し、`WELCOME TO HAKATA CANAL NET`、終了時の「また HAKATA CANAL NET で…」 | `Host.Name` / `Host.Software` から導出 |
| ソフトの動作 | 状態遷移、コマンド、画面の組み立て、ヘルプ、`Config`（機能の有効・無効） | `erikak` に残す |
| プロトタイプ用の固定画面 | `V`、`WHO`、`MEMB`、メール一覧、ファイル一覧、`JUNK`、前回アクセスの日時 | **スコープ外。コードに残す** |

## データの流れ

```text
presets/*.yaml ──(ParsePreset が構造を検証)──▶ hostcatalog.Preset.Detail.ErikaK
      │
      ▼
world.presetData.details[key]  ──▶ MemoryStore.HostDetail(hostID)   ※ world.HostDetailStore
      │                                      ▲
      │                          worldrepo.Repository.HostDetail   （Base に委譲）
      ▼
erikak.New(host, store) が store.(world.HostDetailStore) から取得 ──▶ Runtime が板のカタログを持つ
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
    login:
      station_message:      # ログイン後の枠付きの行（0 行以上）
        - "博多から、夜更かしネットワーカーのみなさんへ。"
        - "23:00以降は混み合います。長時間の席取りはほどほどに(^^;"
      member_greeting: "深夜のアクセスご苦労様！ {handle}さん、いらっしゃいませ。"   # {handle} は利用者のハンドルに置き換わる
    boards:                 # 表示順。親は子より前でなくてもよいが、存在する必要がある
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

- `boardTree` の**全 26 件を、現在の順序のまま**写す。ゼロ値の項目は書かない。
- 値は 1 文字も変えない。数値は `.10` を `0.10` と書くなど、表記だけ YAML に合わせる（同じ値になる）。
- 文字列はすべて二重引用符で囲む（コロンや記号を含む文字列があるため）。
- `boardTree` の上にある 2 つのコメント（「Activity values below are HAKATA station fiction…」と、
  「"夢工房はかた" の史実上の用途は未確認…」）は、YAML のコメントとして、対応する板の近くに移す。
- ログインの局のメッセージ 2 行とあいさつは、現在の `finishLogin` の文字列を、そのまま写す。
  あいさつの `%s` は `{handle}` にする。

## 局名から導出する文字列

| 場所 | 現在 | 導出 |
|---|---|---|
| メインメニューの見出し | `-ＨＡＫＡＴＡ ＣＡＮＡＬ ＮＥＴ-  〖Ｍain Ｍenu〗  絵理香Ｋ版` | `"-" + fullWidthASCII(Host.Name) + "-  〖Ｍain Ｍenu〗  " + fullWidthASCII(Host.Software)` |
| ログイン後の見出し | `WELCOME TO HAKATA CANAL NET` | `"WELCOME TO " + strings.ToUpper(Host.Name)` |
| 終了時のあいさつ | `また HAKATA CANAL NET でお会いしましょう。` | `"また " + Host.Name + " でお会いしましょう。"` |

- `fullWidthASCII`: ASCII の `0x21`〜`0x7E` の各文字を、`+0xFEE0` した全角に変える。それ以外の文字
  （空白、日本語）は変えない。空白の扱いは、**金型と一致させる**ことが条件。
  現在の見出しの区切りが半角空白か全角空白（U+3000）かを、金型のバイト列で確認し、金型に合う方にする。
  金型は変更しない。
- 局が `Host.Software` を持たないとき、ソフト名は空になる（表示の見栄えは、この spec では扱わない）。

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
| `station_message` の各行は、前後の空白を除いて空でない | `ParsePreset` |
| `member_greeting` に、`{handle}` 以外の `{…}` を含まない | `ParsePreset` |
| `station_message` の各行の表示幅は、74 セル以下（枠の内側の幅） | `erikak.ValidateDetail` |

- 74 = 80 − `■  `（4 セル）− `■`（2 セル）。`boxedLine` の内側の幅。
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

**金型は、変更前のコードで生成し、最初のコミットに含める。以降、金型ファイルを書き換えない。**
失敗したら実装の誤りを疑う。

## 変更するファイル

| 層 | ファイル | 内容 |
|---|---|---|
| 金型 | `erikak/golden_test.go`（新規）、`erikak/testdata/*`（新規） | 変更前のコードで生成 |
| hostcatalog | `program_detail.go`（新規） | `ErikaKDetail` などの型と、構造の検証 |
| hostcatalog | `preset.go` | `PresetDetail.ErikaK`、プログラムの一致検査、検証の呼び出し |
| hostcatalog | `program_detail_test.go`（新規） | 検証規則のテスト |
| hostcatalog | `presets/hakata-canal-net.yaml` | `detail.erika_k` を追加、`revision: 4` |
| world | `store.go` | `HostDetailStore`、`MemoryStore.details`、`HostDetail` |
| world | `preset_hosts.go` | `presetData` に `details` を追加 |
| world | `host_detail_test.go`（新規） | `HostDetail` のテスト |
| worldrepo | `repository.go`、テスト | `Repository.HostDetail`（`Base` に委譲） |
| erikak | `runtime.go` | グローバルな板の表を、`Runtime` のカタログに置き換え。局名の導出 |
| erikak | `catalog.go`（新規） | 板のカタログ（`boardCatalog`） |
| erikak | `detail.go`（新規） | 詳細の取得、`ValidateDetail`、`fullWidthASCII` |
| erikak | `runtime_test.go`、`width_test.go` ほか | 局の定義を持つ `Runtime` を使う形に更新 |
| server | `cmd/server/main.go` | `BoardByPath` に、`HostDetailStore` から得た定義を渡す |
| Doc | `hostcatalog/README.md` | `detail.erika_k` の書式 |

## リスクと対策

| リスク | 対策 |
|---|---|
| 画面の 1 文字の違い（全角化、空白、改行） | 画面の金型。変更前に生成し、変更しない |
| 板の順序や項目の取りこぼし | 板の表の金型。`Key`・`Parent` の導出も含めて比較する |
| `Runtime{}`（ゼロ値）を直接組み立てるテストが panic する | カタログのゼロ値を安全にする（FR-008）。`finishLogin` などを使うテストは、局の定義付きの `Runtime` に直す |
| 既存のテストが、グローバルな `BoardByPath` を呼んでいる | 定義を引数に取る形に直し、テスト用の取得ヘルパー（`sampleDetail`）を置く |
| YAML の記法（コロン、引用符、数値の表記） | すべての文字列を二重引用符で囲む。金型が検出する |
| `Host` の比較が壊れる | 詳細を `Host` に入れない |
| 住民の定義（spec 004）と同時に変更する | 別 PR。`population` の部分には触れない |
