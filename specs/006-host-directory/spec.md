# Spec: センターディレクトリ（電話帳）の一覧を、サーバが返す

**Spec ID**: 006-host-directory
**Status**: Draft
**Depends on**: PR #294（局の定義の不変化）、spec 002（Web の capabilities）、spec 004・005（局の定義の外部化）。いずれもマージ済み

## 背景

**センターディレクトリ**は、メインメニューの「1 センターの呼び出し」で表示される、電話をかけられる局の一覧（電話帳）である。
一覧から局を選んで CALL を押すと、`ATDT<電話番号>` が発信される。

この一覧の中身は、現在、2 つの出所に分かれていて、整合していない。

- Web のコード（`apps/web/src/modem/CenterDirectory.ts`）に、HAKATA が `DEFAULT_CENTERS` として**固定で書かれている**。
  局の定義（preset）とは別に、名前、電話番号、ダイヤル方式、最大速度を、Web 側にも持っている。
- サーバの `/api/world/bootstrap` は、ブラウザごとの `worldKey` から、**100 局の架空の局を生成して**返す
  （初回は、Azure OpenAI で局名を作り、Postgres に保存する。待ち時間は最大 120 秒）。Web は、この応答が成功すると、
  一覧を**丸ごとそれに置き換える**ので、HAKATA は一覧から消える。
- 生成された 100 局は、サーバの電話網（`telephone.Network`）が `HostByPhone` で探すストア（preset の局だけを持つ）に存在しないため、
  **ダイヤルしても `NO ANSWER` になる**（コードからの推測。実機では未確認）。
- `localStorage` のカスタム局（`zutto.centers.v1`）は、保存する関数（`saveCenters`）が、どこからも呼ばれておらず、追加する画面もない。
  追加・削除の画面（`CenterDirectoryPanel.tsx`）は、どこからも使われていない。

電話帳には、**ダイヤルできる局だけ**を載せる。その一覧は、局の定義（preset）を持つサーバが返し、
Web は、返された一覧を表示するだけにする。局を追加するときは、preset を書くだけで、Web のコードを触らなくてよい。

## ユーザーストーリー

### US1: 電話帳に、preset で `listed: true` の局が出る（Priority: P1）

運営者として、局の preset に `listed: true` を書くだけで、その局が電話帳に出てほしい。
`listed: false` の局（テスト局など）は、電話帳に出さないが、番号を直接ダイヤルすれば繋がってほしい。

**Independent Test**: サーバの `GET /api/directory` が、`listed: true` の preset の局だけを返し、
`listed: false` の局（`busy-test`）は返さない。

### US2: Web のコードから、局固有の値が消える（Priority: P1）

運営者として、Web のコードに、局の名前、電話番号、ダイヤル方式、最大速度が書かれていない状態にしたい。

**Independent Test**: `apps/web/src` に、`DEFAULT_CENTERS`、`loadCenters`、`saveCenters`、`fetchWorldCenters` が残っていない。
HAKATA は、サーバの応答から、従来と同じ表示（`HAKATA CANAL NET [絵理香K版]`、`0920000196`、トーン、14400bps）で出る。

## 要件

- **FR-001（API）**: サーバは `GET /api/directory` を提供する。応答は JSON で、`{"centers": [ ... ]}`。
  認証は不要。`Cache-Control: no-cache`。`GET` と `HEAD` 以外は 405。
- **FR-002（内容）**: `centers` は、preset で `listed: true` の局を、preset の key の昇順で並べたもの。各要素は次の項目を持つ。

  | 項目 | 内容 |
  |---|---|
  | `id` | 局の key（例 `hakata-canal-net`） |
  | `name` | 局の名前（`host.name`） |
  | `software` | ソフトの表示名（`host.software_label`。無ければ省略） |
  | `phone` | 電話番号（数字のみ） |
  | `dialMode` | `tone` または `pulse` |
  | `maxBaud` | 最大速度 |

- **FR-003（出所）**: 一覧は、preset から導出する。実行時に変更されない（局の定義と同じく不変）。
  `world` に `HostDirectoryStore`（任意機能）を置き、`MemoryStore` が実装し、`worldrepo.Repository` が素通しする。
- **FR-004（`listed` とダイヤルの分離）**: `listed: false` の局は、電話帳に出ないが、ダイヤルは従来どおりできる
  （`hostcatalog` の「`Listed` はダイヤルに影響しない」を保つ）。
- **FR-005（Web: 取得と表示）**: Web は `/api/directory` から一覧を取得し、表示する。局の表示名は、`software` があれば
  `<name> [<software>]`、無ければ `<name>`。HAKATA は、従来と同じ `HAKATA CANAL NET [絵理香K版]` になる。
- **FR-006（Web: 削除）**: Web から、次を削除する: `DEFAULT_CENTERS`、`loadCenters`、`saveCenters`、`fetchWorldCenters`、
  `getOrCreateWorldKey`、`RegisteredCenter.builtIn`、`CenterDirectoryPanel.tsx`。Web は、`/api/world/bootstrap` を呼ばない。
- **FR-007（旧データの掃除）**: Web は、起動時に 1 回、`localStorage` の旧キー（`zutto.centers.v1`、`zutto.worldKey.v1`）を削除する。
  `localStorage` が使えなくても、エラーにしない。
- **FR-008（失敗時の動作）**: 取得に失敗したとき、一覧が空のとき、応答の形が不正なときの動作は、従来のまま
  （`CENTER API ERROR: …` の表示、ESC でメインメニューに戻る）。
- **FR-009（既存の API）**: `/api/world/bootstrap`、`/api/centers`、`worldcatalog` は、変更しない。
- **FR-010（ドキュメント）**: `packages/protocol/README.md` に `/api/directory` を記載する。`hostcatalog/README.md` に、
  電話帳が `listed: true` の局から作られることを記載する。

## 成功基準

- **SC-001**: `go -C apps/server test ./...`、`npm --prefix apps/web test`、`npm --prefix apps/web run build` が通る。
- **SC-002**: サーバの契約テストが、HAKATA の要素を、**従来の `DEFAULT_CENTERS` の値と完全に一致する値**で検査している
  （`id` `hakata-canal-net`、`name` `HAKATA CANAL NET`、`software` `絵理香K版`、`phone` `0920000196`、`dialMode` `tone`、`maxBaud` `14400`）。
- **SC-003**: `apps/web/src`（テストを除く）で、`DEFAULT_CENTERS|loadCenters|saveCenters|fetchWorldCenters|getOrCreateWorldKey|builtIn|bootstrap` が、
  1 件も見つからない。
- **SC-004**: `apps/web/src`（テストを除く）に残る `0920000196` は、「ターミナル・モードの案内文の例」と
  「`VITE_TELEHODAI_NUMBERS` の既定値」の 2 か所だけ（スコープ外）。

## スコープ外（変更しない）

- 繋がらない生成局を、電話帳に出すこと。今回は出さない。
- `/api/world/bootstrap`、`worldcatalog`、生成局の仕組み（サーバ側に残す。削除するかは別途判断する）。
- **テレホーダイ登録番号**（`VITE_TELEHODAI_NUMBERS` の既定値 `0920000196`）。利用者が選ぶ設定であり、局の性質ではないため、別 spec で扱う。
- ターミナル・モードの案内文の例示番号（`ATDT0920000196`）。
- 電話帳の並べ替え、検索、ページ分け、キャッシュの調整。
- `telephone` の、`0459999999` に固定されたダイヤルの挙動（`dial.behavior` への移行）。
- 局の追加画面（利用者が電話帳に局を追加する機能）。
- サーバにつながらないとき（standalone）の、組み込みの局の表示。一覧は空で、従来のエラー表示になる。
