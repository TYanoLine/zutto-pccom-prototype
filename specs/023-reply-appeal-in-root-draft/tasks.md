# Tasks: 返信の盛り上がりの判断を、ルート記事の生成に含める

**Input**: `specs/023-reply-appeal-in-root-draft/` の `spec.md`、`plan.md`
**Prerequisites**: `AGENTS.md`、`docs/LLM_POLICY.md`（「Scope of LLM judgement (relaxed)」節）、`plan.md`（特に §2.6）

## この作業のルール（必読）

- 作業ブランチから `main` への PR を 1 本作る（最後のタスクで**ドラフト**として作る）。`main` に直接コミットしない。
- **成果物は Go のコードの変更とテスト。** spec のファイル（`spec.md`、`plan.md`、`tasks.md`）を作る・変更することは、この作業の仕事ではない。
  この作業で足す spec フォルダのファイルは、`verification.md` だけ。
- スコープ外（`spec.md` 末尾）に手を出さない。特に、件数への換算式、段階の倍率、`ActivePopulationAt`、チャンクサイズ、並列度は、変えない。
- **切り替えの環境変数や、旧い経路の分岐を残さない。** 旧い経路（`ReplyAppealJudge` など）は削除する。
- 人物生成の金型（`population_golden_test.go`）と、Erika-K の画面の金型（`screens_golden.txt`）は、1 バイトも変えない。
- 既存ファイルに整形だけの差分を作らない。`gofmt -w` を既存ファイル全体にかけない（新規ファイルは、かけてよい）。
  **既存ファイルの改行コード（CRLF のファイルがある）を変えない。** 変更後に `git diff --stat` で、変更行数が意図した量であることを確かめる。
- 1 タスク = 1 コミット（メッセージは `T0XX: 内容`）。
- 実行していないコマンドを「成功した」と書かない。実行できなかったものは「未実施」と理由を書く。
- この作業で `.github/` 配下のファイルは変更しない。
- spec と実際のコードが食い違っていたら、推測で進めず、PR の説明に書いて止まる。
- 同じテストの失敗が、2 回直しても残るときは、止まって、どの行が違うかを報告する。
- `go test ./internal/worldrepo` の `TestErikaKChatThroughRepository` は、`main` でも失敗する既存の失敗（WHO の行数）なら、この作業の失敗ではない。
  `main` で失敗することを確かめてから、PR の説明に書く。

---

## Phase 0: 確認（コードは変えない）

- [ ] **T001** `plan.md` §2.6 の 4 点を、コードを読んで確かめ、結果を `verification.md` に書く。
  1 〜 3 のどれかが成り立たなければ、**ここで止まる**。

## Phase 1: ルート生成の出力に足す（`llm`）

- [ ] **T002** `llm/root_drafts.go` に、型・スキーマ・プロンプト・整形を足す（`plan.md` §2.1〜§2.3）。
  判断器のプロンプトの文言（`llm/reply_appeal.go` の `buildReplyAppealPrompt`）を引き継ぐ。**新しい文言を作らない。**
- [ ] **T003** `llm/root_drafts_test.go` に、次のテストを足す: スキーマの項目の順序（`reply_appeal` が `titles` のあと）、範囲外・欠落の退避（エラーにならない）、
  プロンプトに段階の定義・「中位にしない」旨・活動住民数が入ること。

```bash
go -C apps/server test -race ./internal/llm -count=1
```

## Phase 2: 受け渡し（`bbsengine`、`worldrepo`）

- [ ] **T004** `bbsengine.PlannedPost`、`BatchRequest` に項目を足し、`catchUp` が、`PlanBBSBatch` の前に、ルートのスロットごとの活動人数と、直近の返信件数を用意する（`plan.md` §2.2、§2.4）。
- [ ] **T005** `worldrepo/bbs_situation_first_planner.go` が、材料を `BBSRootDraftRequest` へ、段階と理由を `PlannedPost` へ写す。
- [ ] **T006** `planReactionsForRoots` が、運ばれた段階と理由で件数を決める。運ばれなかったルートは段階 2、`fallback:no_appeal`（FR-006）。判断器を呼ぶ分岐を削除する。
- [ ] **T007** 偽の提案器で、SC-001〜SC-003 のテストを足す。

```bash
go -C apps/server test -race ./internal/bbsengine ./internal/worldrepo -count=1
```

## Phase 3: 旧い経路の削除

- [ ] **T008** `plan.md` §2.5 のとおり、旧い経路（`llm/reply_appeal.go`、`world` の旧い型、`Engine.ReplyAppealJudge`、`SetReplyAppealJudge` ほか）を削除する。
  旧い経路を使っていたテストと偽実装は、新しい経路へ移す（アサーションの意図は保つ）。意図を保てないものは、削除せずに止まって報告する。
  `grep -rn "JudgeReplyAppeal\|ReplyAppealJudge" apps/server` がテスト以外で 0 件になること（SC-006）。

## Phase 4: 文書と最終確認

- [ ] **T009** `docs/WORLD_SIMULATION.md` と `docs/DEBUG_INSPECTION.md` を更新する（判断の場所が変わったこと、`header_materialize_total` − `planner_total` を見ること）。
- [ ] **T010** 全体のテストを実行する。

```bash
go -C apps/server vet ./...
go -C apps/server test ./... -count=1
go -C apps/server test -race ./internal/worldrepo ./internal/bbsengine ./internal/llm ./internal/world -count=1
```

  失敗したものは、そのまま `verification.md` に書く。人物生成と Erika-K の金型が変わっていないことを、`git diff --stat` で確かめる。
- [ ] **T011** `specs/023-reply-appeal-in-root-draft/verification.md` を書く: 実行したコマンドと結果（未実施は理由つき）、T001 の確認結果。
  SC-008（デプロイ後の差）は、**「未実施: デプロイ後に確認」**と書く。
  成功基準の番号は、`spec.md` の番号に合わせる。
- [ ] **T012** 作業ブランチを push し、**ドラフトの PR** を作る。説明に、実行したコマンドと結果、T001 の結果を書く。

## 完了の定義

PR の差分に、次が含まれる:

- `llm/root_drafts.go` の変更とテスト、`llm/reply_appeal.go` の削除
- `bbsengine/engine.go`、`worldrepo/bbs_situation_first_planner.go`、`worldrepo/repository.go`、`world/reply_appeal.go` の変更とテスト
- `docs/` の更新
- `specs/023-reply-appeal-in-root-draft/verification.md`

「spec のファイルは既にあります」「作業ツリーがきれいです」は、完了ではない。
