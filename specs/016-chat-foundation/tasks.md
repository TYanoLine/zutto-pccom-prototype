# Tasks: 非同期出力、通話の中の行動、絵理香K版の電報・チャット画面（016-chat-foundation）

**Input**: `specs/016-chat-foundation/` の `spec.md`、`plan.md`
**Prerequisites**: `AGENTS.md`、`docs/WORLD_SIMULATION.md`「Lines and presence」、`docs/LLM_POLICY.md`、spec 015、
`docs/research/erika-k-sources/garakuta_D-00078.html`（cp932）

## この作業のルール（必読）

- 作業ブランチから `main` への PR を 1 本作る。`main` に直接コミットしない。
- **成果物は Go / YAML のコードとテスト、文書の更新、`verification.md`。** spec のファイル（`spec.md`、`plan.md`、`tasks.md`）は変更しない。
- **既存の金型を 1 バイトも変えない。** `apps/server/internal/hostprogram/erikak/testdata/` と `apps/server/internal/world/testdata/`。
  金型の更新用フラグを使わない。金型テストが失敗したら実装の誤り。2 回直して通らなければ止めて、違う行を報告する。
- 住民の行動の決定に LLM を使わない。人間がいることを計算に入れない。
- 係数は架空の調整値であることを、コードのコメントに書く。
- 書式は `plan.md` §2 から写す。記録と違う書式を作らない。記録にない場合の扱いは `plan.md` に書いたとおりにする。
- 局固有の文面はホストプログラムに書かない。`hakata-canal-net.yaml` の文面は架空の局のものとして新しく書く（GARAKUTA の文面を写さない）。
- スコープ外（`spec.md` §8）に手を出さない。特に、住民の発言、LLM の呼び出し、電報の返信、メインメニューの WHO。
- 既存ファイルに整形だけの差分を作らない。`gofmt -w` を既存ファイル全体にかけない（新規ファイルにはかける）。
  既存ファイルの改行コード（CRLF のファイルがある。例 `turbobbs/runtime.go`）を変えない。
- 1 タスク = 1 コミット（`T0XX: 内容`）。途中でビルドが通らないタスクは、1 つのコミットにまとめてよい。
- 実行していないコマンドを「成功した」と書かない。`.github/` を変更しない。
- spec と実際のコードが食い違ったら、推測で進めず、PR の説明に書いて止まる。

---

## Phase 0: ベースライン

- [ ] **T001** 変更前に、全テストが通ることを確認し、結果を `verification.md` に書く

  ```bash
  go -C apps/server test ./... -count=1
  go -C apps/server vet ./...
  ```

---

## Phase 1: 非同期出力（FR-001）

- [ ] **T002** `hostprogram/runtime.go` に `AsyncOutput` を追加する（`plan.md` §4）。
- [ ] **T003** `ws/sessions.go` の `CallSession` に通話ごとのロックを加え、`ws/session.go` の `line` の処理をそのロックの中で行う。
  接続ごとの書き込みロックを加え、すべての `writeJSON` を通す。
- [ ] **T004** `ws/session.go` に、CONNECT / Resume の成功後に起動するポーリングを追加する（`PollInterval = time.Second`）。
  テスト（`ws`）: `Poll` を実装した試験用の Runtime で、出力が `terminal` メッセージとして届く。入力と並行して壊れない。

  ```bash
  go -C apps/server test ./internal/ws -count=1
  go -C apps/server test -race ./internal/ws -count=1
  ```

---

## Phase 2: 通話の中の行動（FR-002〜FR-004）

- [ ] **T005** `world/presence.go` に `CallState`、`CallSegment`、`CallActivity`、`world/store.go` の `PresenceStore` に
  `ListCallActivities`、`SaveCallActivities` を追加し、`MemoryStore` に実装する（`plan.md` §5.1）。
- [ ] **T006** `hostcatalog` に `ErikaKChat`、`ErikaKRoom` と読み込み時の検証を追加する（`plan.md` §3）。テスト: 検証の各規則。
- [ ] **T007** `internal/callplan` を作る（`plan.md` §5.2）。テスト: 決定性、`Chat=false` で `CHAT` なし、定員を超えない、
  人間の通話の有無で住民の計画が同じ、23:30 の `CHAT` の区間数が 14:00 より多い（多数の日で数える）。
- [ ] **T008** `worldrepo/callplan.go` に `CallActivitiesNow`、`ChatEvents`、`SetResidentChat` を実装し、`world/store.go` に
  `ChatHost` を置く（`plan.md` §5.3、§5.4）。テスト: 観測した計画が住民の追加後も残る、同時の観測で保存 1 回、
  `ChatEvents` が時刻順で重複しない、`SetResidentChat(false)` で `CHAT` なし。
  WHO 1 回分（`CallActivitiesNow`）の所要時間を 300 人の試験局で測り、`verification.md` に書く。200 ms を超えたら
  `plan.md` §8 の対策を入れ、前後の値を書く。
- [ ] **T009** `world.DevelopmentHostSnapshot` に計画を加え、`worldpersist` で保存・復元する。古いスナップショットが読めることをテストする。

  ```bash
  go -C apps/server test ./internal/world ./internal/callplan ./internal/worldrepo ./internal/worldpersist ./internal/hostcatalog -count=1
  ```

---

## Phase 3: 絵理香K版（FR-005、FR-006）

- [ ] **T010** 電報・チャットの記録（`ChatUtterance`、`Telegram`）と台帳への記録を `world` と `worldrepo` に追加する
  （`plan.md` §6 の最後）。`worldpersist` で保存・復元する。
- [ ] **T011** 絵理香K版に `call`、`telegram_body`、`chat_menu`、`chat_room` の状態、WHO 行の書式、電報、入退室、`Poll` を実装する
  （`plan.md` §2、§6）。局の定義に `chat` がない局は今のスタブのまま。
  テスト: 電報メニューと WHO 行（全角のハンドル、`ring`、`CHAT #n`、自分の行は `WHO`）、電報の 3 つの場合、入室から入退室の表示、
  `..` の退室の出力順、チャットのメニューの空行でメインメニュー、本文入力中は `Poll` が空、`chat` のない局の出力が変わらない。
- [ ] **T012** `hakata-canal-net.yaml` に `chat` を加える（架空の文面、部屋 2〜3 室）。
- [ ] **T013** `cmd/server/main.go` で `SetResidentChat(cfg.AzureOpenAIKey != "")`。

  ```bash
  go -C apps/server test ./internal/hostprogram/... ./internal/hostcatalog ./cmd/server -count=1
  git diff --stat origin/main -- apps/server/internal/hostprogram/erikak/testdata apps/server/internal/world/testdata
  ```

  2 つ目のコマンドの出力が空であること。

---

## Phase 4: 文書・全体の確認・PR

- [ ] **T014** `docs/host-programs/erika-k.md`、`docs/ARCHITECTURE.md`、`docs/WORLD_SIMULATION.md` を更新する（`plan.md` §7）。
- [ ] **T015** 全体の確認

  ```bash
  go -C apps/server test ./... -count=1
  go -C apps/server vet ./...
  go -C apps/server test -race ./internal/ws ./internal/worldrepo -count=1
  ```

- [ ] **T016** `specs/016-chat-foundation/verification.md` に、実行したコマンドと結果、T008 の所要時間、実施しなかった確認と理由を書く。
  ローカルでサーバーを起動して電報・チャット画面を手で確かめた場合は、その記録（入力と出力の抜粋）も書く。
- [ ] **T017** draft の PR を開く。説明に、`plan.md` §2 の「Fictional reconstruction」とした項目、spec と食い違った点を書く。

## 完了の定義

PR の差分に次が含まれ、T015 のコマンドがすべて成功している:
`internal/hostprogram/runtime.go`、`internal/ws/`、`internal/world/`、`internal/callplan/`、`internal/worldrepo/`、`internal/worldpersist/`、
`internal/hostcatalog/`（`presets/hakata-canal-net.yaml` を含む）、`internal/hostprogram/erikak/`、`cmd/server/main.go`、更新した `docs/`、
`specs/016-chat-foundation/verification.md`。金型ファイルの差分がない。
