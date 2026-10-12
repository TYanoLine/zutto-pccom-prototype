# Tasks: 住民の接続（在席）と回線占有、暦に連動した世界時計（015-persona-presence）

**Input**: `specs/015-persona-presence/` の `spec.md`、`plan.md`
**Prerequisites**: `AGENTS.md`、`docs/WORLD_SIMULATION.md`、`docs/HISTORICAL_ACCURACY.md`、`docs/research/NTT_DIAL_TARIFF_1996.md`、spec 008 FR-012

## この作業のルール（必読）

- 作業ブランチから `main` への PR を 1 本作る。`main` に直接コミットしない。
- **成果物は Go / TypeScript / YAML のコードとテスト、`docs/research/CALENDAR_1996.md`、文書の更新、`verification.md`。**
  spec のファイル（`spec.md`、`plan.md`、`tasks.md`）は変更しない。
- **既存の金型を 1 バイトも変えない。** `apps/server/internal/world/testdata/` の人物生成の金型と、
  `apps/server/internal/hostprogram/erikak/testdata/` の画面・板の金型。金型の更新用フラグ（`-update-golden`、`-update-erikak-golden`）を使わない。
  金型テストが失敗したら実装の誤り。2 回直して通らなければ止めて、違う行を報告する。
- 在席・BUSY の決定に LLM を使わない。
- 係数（確率、倍率、時間）は架空の調整値であることを、コードのコメントに書く。テストの閾値は `plan.md` §9 の値で書き、
  実測値に合わせて緩めない。
- 暦データの値は `plan.md` の候補を写すのではなく、出典で確認してから入れる。確認できない項目は入れず、`CALENDAR_1996.md` に
  「未確認」として書く。
- スコープ外（`spec.md` §8）に手を出さない。特に、チャット、ライフサイクル、`bbsengine`、料金計算、アクセス記録の固定の名前。
- 既存ファイルに整形だけの差分を作らない。`gofmt -w` を既存ファイル全体にかけない（新規ファイルにはかける）。
- 1 タスク = 1 コミット（`T0XX: 内容`）。途中でビルドが通らないタスクは、1 つのコミットにまとめてよい。
- 実行していないコマンドを「成功した」と書かない。`.github/` を変更しない。LF 改行。
- spec と実際のコードが食い違ったら、推測で進めず、PR の説明に書いて止まる。

---

## Phase 0: ベースライン

- [ ] **T001** 変更前に、全テストが通ることを確認し、結果を `verification.md` に書く

  ```bash
  go -C apps/server test ./... -count=1
  go -C apps/server vet ./...
  npm --prefix apps/web test
  ```

  失敗するテストがあれば、直さずに名前を記録する（以後、そのテストは比較の対象外）。

---

## Phase 1: 世界時計（FR-001）

- [ ] **T002** `internal/worldclock` に `CalendarClock` と `NewCalendar(location, yearOffset)` / `NewCalendarFrom(source, location, yearOffset)` を追加し、
  `plan.md` §3.3 の表をテストにする（`clock_test.go`）。既存の `MappedClock` とそのテストは変えない。
- [ ] **T003** `internal/config/config.go` に `WorldYearOffset`（`WORLD_YEAR_OFFSET`、既定 30）を加え、`WorldDate` の既定を空文字にする。
  `cmd/server/main.go` で、`WorldDate` が空なら `CalendarClock`、空でなければ `MappedClock` を使う。`WorldDate` を他へ渡している箇所は、
  空のとき `clock.Now().Format(time.DateOnly)` を渡す。`/api/health` に `world_clock` を加える。
- [ ] **T004** `apps/web/src/time/WorldClock.ts` に `CalendarWorldClock` を追加し、§3.3 の同じ表でテストする。`App.tsx` で
  `VITE_WORLD_DATE` が空なら `CalendarWorldClock` を使う（`VITE_WORLD_YEAR_OFFSET`、既定 30）。

  ```bash
  go -C apps/server test ./internal/worldclock ./internal/config -count=1
  npm --prefix apps/web test
  ```

---

## Phase 2: 暦データ（FR-002、FR-003）

- [ ] **T005** `docs/research/CALENDAR_1996.md` を書く。祝日（出典: 国立天文台の暦要項など）、期間と日の各項目の分類と出典、
  `events` に入れた出来事と入れなかった候補。`docs/research/NTT_DIAL_TARIFF_1996.md` の書き方に合わせる。
- [ ] **T006** `internal/worldcalendar` を作る: `data/1996.yaml`、`data/effects.yaml`、`calendar.go`（`Day`）、`effects.go`（キーと生活類型と時刻から倍率）。
  テスト: 祝日が 19 日で、`apps/web/src/billing/pseudoTariffs.ts` の `JAPAN_1996_HOLIDAYS` と同じ集合（テストに同じ日付の一覧を書く）、
  1996-11-04 が祝日、1996-12-24 が `christmas_eve`、1997-01-02 が `year_end_new_year`、1998 年の日付が `Known == false`。

  ```bash
  go -C apps/server test ./internal/worldcalendar -count=1
  ```

---

## Phase 3: 接続の計算（FR-004、FR-005、FR-006）

- [ ] **T007** `internal/world/presence.go` に `PresenceSession`、`HumanCall` の型と、`store.go` に `PresenceStore`、`LineOccupancy`、
  `PresenceObserver` のインターフェースを追加する（`plan.md` §7）。
- [ ] **T008** `internal/presence` に接続属性（`profile.go`、`plan.md` §5）を実装する。人物の `rand.Rand` を使わない。
  テスト: 職業からの類型の決定、同じ入力で同じ値、保存済みの人物事実があればそれを使う。
- [ ] **T009** `internal/presence` に層ごとの係数（`profiles.go`、`factors.go`、`plan.md` §6.1）を実装し、層ごとに単体テストする（`plan.md` §9「層ごとの係数」）。
- [ ] **T010** `internal/presence/compute.go` に接続の計算（`plan.md` §6.2、§6.3）を実装する。テスト: 決定性、回線数、テレホタイム、平日昼
  （`plan.md` §9 の閾値）。開発用に `TestPresenceHistogram`（`-v` のときだけ時刻ごとの接続数を出力し、検証はしない）を加える。
- [ ] **T011** 人物生成の金型が変わらないことを確認する

  ```bash
  go -C apps/server test ./internal/presence ./internal/world -count=1
  go -C apps/server test ./internal/world -run 'TestPopulation' -count=3
  ```

---

## Phase 4: 保存・ダイヤル・人間の通話（FR-007、FR-008）

- [ ] **T012** `world.MemoryStore` に `PresenceStore` と `LineOccupancy` を実装する。`worldrepo.Repository` に `PresenceObserver` を実装し、
  局単位で直列化する。観測した住民の接続属性を人物事実として保存する。テスト: 観測の保存と不変（住民を追加しても残る）、
  同時の観測で保存が 1 回、同じ接続を 2 回保存しない。
- [ ] **T013** `world.DevelopmentHostSnapshot` に観測した接続を加え、`worldpersist` で保存・復元する。古いスナップショットが読めることをテストする。
- [ ] **T014** `telephone.Network.dialAt` を `plan.md` §7.1 のとおりにする。テスト: 満杯で BUSY、空きで CONNECT と最小の空き回線番号、
  `0459999999` の挙動の維持、能力を持たない Store での従来挙動。
- [ ] **T015** `ws.SessionManager` に任意の `world.LineOccupancy` を渡し、CONNECT で `BeginHumanCall`、`End` と期限切れで `EndHumanCall` を呼ぶ。
  `BeginHumanCall` が失敗したら BUSY を返す。テスト: 占有と解放、期限切れでの解放。

  ```bash
  go -C apps/server test ./internal/world ./internal/worldrepo ./internal/worldpersist ./internal/telephone ./internal/ws -count=1
  ```

---

## Phase 5: WHO（FR-009）

- [ ] **T016** `hostprogram` に `CallInfoReceiver` を追加し、`ws` が CONNECT 後に呼ぶ。絵理香K版がログインで
  `SetHumanCallHandle` を呼ぶ。
- [ ] **T017** 絵理香K版の 2 か所の WHO を、`PresenceObserver` の接続を書式化する関数に置き換える。書式は現在の文字列から写す。
  `golden_test.go` の `screenTranscript` に、`plan.md` §8 の試験用の `PresenceObserver` と `SetCallInfo` の注入**だけ**を加える。
  画面金型ファイルは変えない。

  ```bash
  go -C apps/server test ./internal/hostprogram/... -count=1
  git diff --stat origin/main -- apps/server/internal/hostprogram/erikak/testdata apps/server/internal/world/testdata
  ```

  2 つ目のコマンドの出力が空であること。

---

## Phase 6: 文書・全体の確認・PR

- [ ] **T018** `docs/WORLD_SIMULATION.md`（「Lines and presence」「Time」、閏年の既知の制約）、`docs/PRODUCT_SPEC.md`（「World-time strategy」）、
  `docs/TERMINAL_AND_MODEM.md`（暦の記述）を更新する。
- [ ] **T019** 全体の確認

  ```bash
  go -C apps/server test ./... -count=1
  go -C apps/server vet ./...
  npm --prefix apps/web test
  grep -n '"MARI\|MARI ' apps/server/internal/hostprogram/erikak/runtime.go
  ```

  最後の grep は、アクセス記録（範囲外）の 1 行だけが残ること。
- [ ] **T020** `specs/015-persona-presence/verification.md` に、実行したコマンドと結果、`TestPresenceHistogram -v` の出力（平日・土曜・12/24・12/31 の各 1 日分）、
  実施しなかった確認と理由を書く。
- [ ] **T021** draft の PR を開く。説明に、デプロイ設定から `WORLD_DATE` / `VITE_WORLD_DATE` を外す必要があること、
  `CALENDAR_1996.md` で確認できなかった項目、spec と食い違った点を書く。

## 完了の定義

PR の差分に次が含まれ、T019 のコマンドがすべて成功している:
`internal/worldclock`、`internal/config`、`cmd/server/main.go`、`internal/worldcalendar/`、`internal/presence/`、`internal/world/`、
`internal/worldrepo/`、`internal/worldpersist/`、`internal/telephone/`、`internal/ws/`、`internal/hostprogram/`、`apps/web/src/time/`、`apps/web/src/App.tsx`、
`docs/research/CALENDAR_1996.md`、更新した `docs/`、`specs/015-persona-presence/verification.md`。
金型ファイルの差分がない。
