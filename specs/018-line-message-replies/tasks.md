# Tasks: 電報への返信と住民からの電報（018-line-message-replies）

**Input**: `specs/018-line-message-replies/` の `spec.md`、`plan.md`
**Prerequisites**: `AGENTS.md`、`docs/WORLD_SIMULATION.md`、`docs/LLM_POLICY.md`、`docs/host-programs/erika-k.md`、spec 016、spec 017、spec 019

## この作業のルール（必読）

- spec 017（PR #360）と spec 019（PR #362）の実装が `main` に入ってから始める。入っていなければ止めて、そう報告する。
- 作業ブランチから `main` への PR を 1 本作る。`main` に直接コミットしない。
- **成果物は Go のコードとテスト、文書の更新、`verification.md`。** spec のファイル（`spec.md`、`plan.md`、`tasks.md`）は変更しない。
- **既存の金型を 1 バイトも変えない。** `apps/server/internal/hostprogram/erikak/testdata/` と `apps/server/internal/world/testdata/`。
  金型の更新用フラグを使わない。金型テストが失敗したら実装の誤り。2 回直して通らなければ止めて、違う行を報告する。
- **画面の文字列は、電報が届いたときの割り込みだけを足す。** 既存の出力を変えない。人間 ID を端末・LLM の材料・ログに出さない。
- **LLM は本文だけを作る。** 話者・宛先・時刻・きっかけを変えさせない。人間の電報の本文を枠の計算に使わない。
- **失敗は沈黙。** 決まり文句の代わりの本文を作らない。
- **古い記録を書き換えない。**
- テストで実際の LLM を呼ばない。
- スコープ外（`spec.md` §8）に手を出さない。
- 既存ファイルに整形だけの差分を作らない。`gofmt -w` を既存ファイル全体にかけない（新規ファイルにはかける）。既存ファイルの改行コードを変えない。
- 1 タスク = 1 コミット（`T0XX: 内容`）。途中でビルドが通らないタスクは、1 つのコミットにまとめてよい。
- 実行していないコマンドを「成功した」と書かない。`.github/` を変更しない。
- spec と実際のコードが食い違ったら、推測で進めず、PR の説明に書いて止まる。

---

## Phase 0: ベースライン

- [ ] **T001** 変更前に、全テストが通ることを確認し、結果を `verification.md` に書く。`plan.md` §1 の関数名・行を実際の値で確かめ、違えば `verification.md` に書く。

  ```bash
  go -C apps/server test ./... -count=1
  go -C apps/server vet ./...
  ```

---

## Phase 1: 型（`plan.md` §2）

- [ ] **T002** `world.LineMessage` に `Slot`、`RespondsTo` を足し、`LineMessageHost` を定義する。
  テスト: 新しい欄のない古い JSON が `worldpersist` で読める。`MemoryStore` が新しい欄を保存して返す。

  ```bash
  go -C apps/server test ./internal/world ./internal/worldpersist -count=1
  ```

---

## Phase 2: 枠（`plan.md` §3）

- [ ] **T003** `internal/lineslot` に `answer` と `hail` の枠の計算を作る。定数は 1 つの `const` ブロック。
  テスト（ストーリー 2、3、4、5、9）: 同じ入力で同じ枠、率が 1 未満、関係が高いほど高い、部屋の区間の終わりより前に `answer` の枠がない、
  `GUEST` に `hail` の枠がない、1 接続で同じ住民から 2 通を超えない、本文だけが違う 2 通の電報で枠が同じ。
- [ ] **T004** 上限（1 時間あたりの通数、同じ住民から同じ人間への最短の間隔）を作る。
  テスト（ストーリー 7）: 住民の多い局で 1 時間の枠が上限以内。

  ```bash
  go -C apps/server test ./internal/lineslot -count=1
  ```

---

## Phase 3: 本文の生成（`plan.md` §4）

- [ ] **T005** `llm` に `LineMessageWriter`、`LineMessageRequest`、プロンプト、スキーマ、`ValidateLineMessage`、構造化出力の実装を作る。
  テスト: プロンプトで `Incoming` が相手の言葉として区別され、指示の欄に混ざらない。検証が空・改行・300 桁超・表示できない文字・未来の語・`HANDLE>` 前置きを落とす。
  リクエストの JSON に人間 ID の文字列が含まれない。

  ```bash
  go -C apps/server test ./internal/llm -count=1
  ```

---

## Phase 4: 記録と配達（`plan.md` §5）

- [ ] **T006** `worldrepo` に `LineMessageEvents`、生成の開始・再試行、`recordLineMessage`、台帳、ログ、`SetLineMessageWriter` を作る。同時呼び出し数は `chatTalk` と共有する。
  テスト（ストーリー 1、3、5、6、8）: 試験用の生成器で、電報に返信が 1 通記録され、`RespondsTo`、宛先、時刻が正しい。部屋の区間の終わりの前に記録されない。
  同じ入力と、もう一度の問い合わせで電報が変わらない。人間の接続がないとき、生成器の呼び出しが 0 回。失敗・検証に落ちる生成器で何も記録されない。
  生成器がないとき何も起きない。`-race` で通る。
- [ ] **T007** `cmd/server` で、LLM の設定があるときに `SetLineMessageWriter` を呼ぶ。テスト: 設定がないとき呼ばれない（既存の設定のテストの形に合わせる）。

  ```bash
  go -C apps/server test ./internal/worldrepo ./cmd/... -count=1
  go -C apps/server test -race ./internal/worldrepo -count=1
  ```

---

## Phase 5: 絵理香K版（`plan.md` §6）

- [ ] **T008** `Poll` に電報の配達を足す（状態 `call`、`chat_menu`）。
  テスト（ストーリー 10、11）: `call` で `\a`、`#NN HANDLE>本文\a`、空行、WHO、空行、ヒント、`CALL> ->` の形で出る。同じ電報は 2 回出ない。
  `telegram_body`、`chat_room` では空で、`call` に戻ったあとに `N 件の電報が届いています。` の行に続けてまとめて出る（1 通でも同じ。割り込みの 1 通には付かない）。電報のない画面の出力が変わらない。

  ```bash
  go -C apps/server test ./internal/hostprogram/... -count=1
  ```

---

## Phase 6: 文書と仕上げ

- [ ] **T009** 文書（`plan.md` §7、FR-012）。
- [ ] **T010** 全体の確認をし、`verification.md` に結果を書く。

  ```bash
  go -C apps/server test ./... -count=1
  go -C apps/server vet ./...
  go -C apps/server test -race ./internal/worldrepo ./internal/ws -count=1
  git diff --stat origin/main -- apps/server/internal/hostprogram/erikak/testdata apps/server/internal/world/testdata
  ```

  金型の差分は空であること。
- [ ] **T011** `main` への draft PR を開く。説明に、実行したコマンドと結果、未実施のもの（実際の LLM での確認、実機のブラウザでの確認など）と理由、
  `plan.md` と違う点を書く。

## 完了の定義

- PR の差分に、`internal/lineslot` とテスト、T002〜T009 の変更、`verification.md` が含まれる。
- T010 のコマンドがすべて通り、金型の差分がない。
- draft PR が開いている。
