# Tasks: スレッドごとの盛り上がり（レス件数）

**Input**: `specs/021-reply-liveliness/` の `spec.md`、`plan.md`
**Prerequisites**: `AGENTS.md`、`docs/LLM_POLICY.md`（「Scope of LLM judgement (relaxed)」節）、`plan.md` の §2.3

## この作業のルール（必読）

- 作業ブランチから `main` への PR を 1 本作る（最後のタスクで**ドラフト**として作る）。`main` に直接コミットしない。
- **成果物は Go のコードの変更（実装）とテスト。** spec のファイル（`spec.md`、`plan.md`、`tasks.md`）を作る・変更することは、この作業の仕事ではない。
  この作業で足す spec フォルダのファイルは、`verification.md` だけ。
- スコープ外（`spec.md` 末尾）に手を出さない。特に、返信の担い手・予定時刻・関連度の決め方、返信本文の生成は、変えない。
- 人物生成の金型（`population_golden_test.go`）と、Erika-K の画面の金型（`screens_golden.txt`）は、1 バイトも変えない。
- 既存ファイルに整形だけの差分を作らない。`gofmt -w` を既存ファイル全体にかけない（新規ファイルは、かけてよい）。
- 1 タスク = 1 コミット（メッセージは `T0XX: 内容`）。
- 実行していないコマンドを「成功した」と書かない。実行できなかったものは「未実施」と理由を書く。
- この作業で `.github/` 配下のファイルは変更しない。
- spec と実際のコードが食い違っていたら、推測で進めず、PR の説明に書いて止まる。
- 同じテストの失敗が、2 回直しても残るときは、止まって、どの行が違うかを報告する。

---

## Phase 0: 確認（コードは変えない）

- [ ] **T001** `plan.md` §2.3 の「要確認」を確かめ、結果を `verification.md` に書く

  読んで確かめること:
  1. `bbsengine.catchUp` の `slot.ReplyToPostID` の分岐が、保存済みの投稿を返信先にできること。
  2. `PlanBBSBatch` を、ルートのスロットだけ、返信のスロットだけで、別々に呼べること（`llm/bbs_article_batch.go`、`BatchRequest`）。
  3. `worldpersist/store.go` の `ReactionPlan` の保存形式（JSON か）と、項目追加で既存データが読めること。
  4. `board_activity.go` が、保存済みの計画を参照できること。

  1 または 2 が成り立たない場合は、**ここで止まる**。`plan.md` の代替 A、B のどちらにするかは、実装者が決めず、PR の説明に書いて報告する。

---

## Phase 1: 換算式（`world`、LLM なし）

- [ ] **T002** `apps/server/internal/world/reply_appeal.go`（新規）に、`ReplyAppealJudge`、関連する型、乱数源のインターフェース、
  `ReplyCountFor(rng, rate, appeal, active, maxReplies)` を書く（`plan.md` §2.1、§2.2）。
- [ ] **T003** `reply_appeal_test.go`（新規）に、統計テストを書く:
  - 段階が高いほど期待値が大きい（固定の乱数源、各段階 2 万回）。
  - `active - 1` と `MaxThreadReplies` を超えない。
  - `appeal=2`、`active` が十分大きいとき、平均が `rate` の近く（許容は実測して決め、`verification.md` に書く）。
  - 同じ乱数源・同じ入力で同じ結果。

```bash
go -C apps/server test ./internal/world -run 'ReplyCount|ReplyAppeal' -count=1
```

**Checkpoint**: `world` のテストが通る。LLM を呼ぶコードはまだない。

---

## Phase 2: 保存

- [ ] **T004** `world.ReactionPlan` に `Count`、`Appeal`、`Judged`、`Reason` を足す（`plan.md` §2.4）。
  `PlanReactions` が `Judged` の計画の `Count` を使うようにする。既存の保存データが読めるテストを足す。
- [ ] **T005** `worldpersist/store.go` で、追加項目が保存・読込される（往復する）テストを足す。

```bash
go -C apps/server test ./internal/world ./internal/worldpersist -count=1
```

---

## Phase 3: LLM の判断器

- [ ] **T006** `apps/server/internal/llm/reply_appeal.go`（新規）に、構造化出力の判断器を書く。
  スキーマ: ルートごとに `id`、`appeal`（0〜4）、`reason`（1 文）。範囲外・欠落のルートは、段階 2 として扱い、ログに残す。
- [ ] **T007** 偽の HTTP プロバイダで、スキーマ、不正出力、タイムアウトのテストを書く（実際のプロバイダは呼ばない）。

```bash
go -C apps/server test ./internal/llm -run 'ReplyAppeal' -count=1
```

---

## Phase 4: エンジンへの接続

- [ ] **T008** 通常運用の経路（`planReactionsForRoot`）に、判断を接続する。判断器が `nil` または失敗のときは、段階 2 で換算する。
- [ ] **T009** 初期生成の経路を、T001 で確認した方式で 2 段化する（`plan.md` §2.3）。判断は 1 バッチ 1 回。
- [ ] **T010** `board_activity.go` が、保存済みの計画があればその件数を使うようにする。
- [ ] **T011** 偽の判断器を使い、(a) 判断器の呼び出し回数がバッチ数に比例する、(b) 判断器が常に失敗しても処理が止まらず件数が基準率の近くに収まる、
  (c) 保存済みの件数が再起動後も変わらない、をテストで確かめる。

```bash
go -C apps/server test ./internal/bbsengine ./internal/worldrepo -count=1
```

---

## Phase 5: 文書と最終確認

- [ ] **T012** `docs/WORLD_SIMULATION.md` と `docs/BOARD_ACTIVITY_PLANNING.md` に、件数の決め方（段階、換算式、保存、退避）を書く。
  `docs/DEBUG_INSPECTION.md` に、段階と理由のログの見方を書く。
- [ ] **T013** 全体のテストを実行する。

```bash
go -C apps/server vet ./...
go -C apps/server test ./... -count=1
```

  失敗したものは、そのまま `verification.md` に書く。人物生成と Erika-K の金型が変わっていないことを、`git diff --stat` で確かめる。
- [ ] **T014** `specs/021-reply-liveliness/verification.md` を書く: 実行したコマンドと結果（未実施は理由つき）、T001 の確認結果、統計テストの許容の根拠。
- [ ] **T015** 作業ブランチを push し、**ドラフトの PR** を作る。説明に、実行したコマンドと結果、T001 の結果、代替案を選んだ場合はその理由を書く。

## 完了の定義

PR の差分に、次が含まれる:

- `world/reply_appeal.go`、`reply_appeal_test.go`、`world/reaction.go` の変更
- `llm/reply_appeal.go`、`reply_appeal_test.go`
- `bbsengine/engine.go`、`worldrepo/board_activity.go`、`worldpersist/store.go` の変更とテスト
- `docs/` の更新
- `specs/021-reply-liveliness/verification.md`

「spec のファイルは既にあります」「作業ツリーがきれいです」は、完了ではない。
