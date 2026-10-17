# Tasks: ルート記事の計画を速くし、話題の重複を拒否しない

**Input**: `specs/022-faster-root-planning/` の `spec.md`、`plan.md`
**Prerequisites**: `AGENTS.md`、`docs/LLM_POLICY.md`（「Scope of LLM judgement (relaxed)」節）、`plan.md`

## この作業のルール（必読）

- 作業ブランチから `main` への PR を 1 本作る（最後のタスクで**ドラフト**として作る）。`main` に直接コミットしない。
- **成果物は Go のコードの変更とテスト。** spec のファイル（`spec.md`、`plan.md`、`tasks.md`）を作る・変更することは、この作業の仕事ではない。
  この作業で足す spec フォルダのファイルは、`verification.md` だけ。
- スコープ外（`spec.md` 末尾）に手を出さない。特に、モデルや推論の重さの変更、本文の生成は、しない。
- 採用された Situation の項目と、保存される事実は、変えない。人物生成の金型（`population_golden_test.go`）と、Erika-K の画面の金型
  （`screens_golden.txt`）は、1 バイトも変えない。
- 既存ファイルに整形だけの差分を作らない。`gofmt -w` を既存ファイル全体にかけない（新規ファイルは、かけてよい）。
- 1 タスク = 1 コミット（メッセージは `T0XX: 内容`）。
- 実行していないコマンドを「成功した」と書かない。実行できなかったものは「未実施」と理由を書く。
- この作業で `.github/` 配下のファイルは変更しない。
- spec と実際のコードが食い違っていたら、推測で進めず、PR の説明に書いて止まる。
- 同じテストの失敗が、2 回直しても残るときは、止まって、どの行が違うかを報告する。

---

## Phase 0: 所要時間のログ

- [ ] **T001** `plan.md` §2.1 のとおり、`BBS timing: … phase=situation_chunk` のログを足す。
  `go -C apps/server test ./internal/worldrepo -count=1` が、変更前と同じ結果で通ることを確かめる。

## Phase 1: 重複を拒否しない

- [ ] **T002** `bbs_situation_first_planner.go` の `novelty_key` の重複による不採用を、ログ（`BBS Situation novelty overlap`）に変える
  （`plan.md` §2.2）。空の `novelty_key` の不採用は残す。重複を返す偽の提案器で、バッチが成功し、呼び出しが 1 回であるテストを足す（SC-002）。
```bash
go -C apps/server test ./internal/llm ./internal/worldrepo -count=1
```

## Phase 2: 並列化

- [ ] **T003** `planSituationFirstRoots` のチャンク処理を、`plan.md` §2.3 のとおり並列にする。
  - `storeDevelopmentPlanningUsage` の並行性を確かめる（`p.repo` のロック）。安全でなければ、集めてから順に記録する。
  - `go.mod` に `golang.org/x/sync` があるか確かめる。なければ `sync.WaitGroup` を使う。**新しい依存を足さない**。
- [ ] **T004** `bbs_situation_chunk_retry_test.go` を、並列に合わせて直す（呼び出しサイズの集合を確かめる、偽の提案器に排他制御を足す）。
  遅延テスト（SC-001）、順序の決定性（SC-003）、切れの分割（SC-004）のテストを足す。

```bash
go -C apps/server test -race ./internal/worldrepo -count=1
go -C apps/server test ./internal/llm ./internal/worldrepo -count=1
```

## Phase 3: Situation と件名の統合

- [ ] **T005** `apps/server/internal/llm/root_drafts.go`（新規）を書く（`plan.md` §2.4）。
  既存の Situation のプロンプト（`openai_world_situation_proposer.go`）と件名のプロンプト（`situation_titles.go` の `buildSituationTitlePrompt`）から、
  文言を引き継ぐ。**新しい文言を作らない。** 再利用する部品は、関数として切り出して使う。
  スキーマは、ルートごとに Situation の項目、続いて `titles`。
- [ ] **T006** `root_drafts_test.go`（新規）: スキーマの項目の順序（`titles` が最後）、`MultiVariant` の案数の上限（3）、
  不正な出力（件名が空など）の扱い、プロンプトに既存の方針の文言（「対象の名前を題名のどこかに入れてください」など）が入ることを、偽の HTTP プロバイダで確かめる。
- [ ] **T007** `planSituationFirstRoots` を、統合した経路に**置き換える**: 統合したチャンクの呼び出し（並列、Phase 2 と同じ仕組み）→
  スロット順に直列で件名を選択（`pickTitleVariant`）。件名が不正なチャンクは、既存の Situation の再試行に乗せる（`plan.md` §2.4）。
  あわせて、旧い経路（`GenerateBBSWorldSituationProposals`、`GenerateBBSSituationTitles`、`wordRootTitles` の `requestTitles`、
  それぞれのインターフェースと、`sharedBBSArticleEngineEnabled`・`PlanBBSBatch` の型検査）を**削除する**。切り替えの環境変数は作らない。
- [ ] **T008** 旧い経路を使っていた既存のテストと偽実装（`openTopicSituationProposer`、`openTopicTitlePlanner`、`chunkBudgetProposer`、
  `llm` のプロンプト・スキーマのテストなど）を、統合した呼び出しの偽実装・関数へ移す。**アサーションの意図は保つ**（プロンプトの方針の文言、
  スキーマの形、分割の動作など）。意図を保てないものは、削除せずに PR の説明に書いて止まる。
  偽の統合器で SC-008（呼び出し回数 `ceil(n/6)`）と SC-009（不正な件名のチャンクを再試行して受理）を新しくテストする。

```bash
go -C apps/server test -race ./internal/llm ./internal/worldrepo -count=1
```

## Phase 4: 文書と最終確認

- [ ] **T009** `docs/DEBUG_INSPECTION.md` に、新しい `BBS timing` の行と、`BBS Situation novelty overlap` の行の見方を書く。
- [ ] **T010** 全体のテストを実行する。

```bash
go -C apps/server vet ./...
go -C apps/server test ./... -count=1
```

  失敗したものは、そのまま `verification.md` に書く。人物生成と Erika-K の金型が変わっていないことを、`git diff --stat` で確かめる。
- [ ] **T011** `specs/022-faster-root-planning/verification.md` を書く: 実行したコマンドと結果（未実施は理由つき）。
  SC-006（デプロイ後の `planner_total`）は、**「未実施: デプロイ後に確認」**と書く。
  統合した件名の質は、**「未評価: 人の目で、デバッグトレースと `BBS title shape` のログから確認する」**と書く（実装者は評価しない）。
- [ ] **T012** 作業ブランチを push し、**ドラフトの PR** を作る。説明に、実行したコマンドと結果、`bbs_situation_chunk_retry_test.go` を直した理由
  （小さいサイズを引き継ぐ動作がなくなったため）を書く。

## 完了の定義

PR の差分に、次が含まれる:

- `worldrepo/bbs_situation_first_planner.go`、`bbs_title_words.go` の変更
- `llm/` のプロンプトの文言の変更、`llm/root_drafts.go` とそのテスト、`config` の変更
- 上記に対応するテストの追加と修正
- `docs/DEBUG_INSPECTION.md` の更新
- `specs/022-faster-root-planning/verification.md`

「spec のファイルは既にあります」「作業ツリーがきれいです」は、完了ではない。
