# Tasks: エージェント記憶想起ツールと非同期マイクロバッチ埋め込み（011-actor-memory-tools）

**Input**: `specs/011-actor-memory-tools/` の `spec.md`、`plan.md`  
**Prerequisites**: `AGENTS.md`、`docs/sdd/SDD_005_CROSS_CHANNEL_INTERACTION_FEEDBACK.md`、Spec 010

## この作業のルール（必読）

- 作業ブランチから `main` への PR を 1 本作る。`main` に直接コミットしない。
- **成果物は Go のコード（実装）とテスト。** spec のファイルは変更しない。追加してよい spec フォルダのファイルは `verification.md` だけ。
- **既存の人物生成・画面金型出力を 1 バイトも変えない。**
- スコープ外（BBS/チャット本文生成への組み込み、function calling アダプター、未決事項の保存、`main.go` の変更）に手を出さない。
- 既存ファイルに整形だけの差分を作らない。`gofmt -w` を既存ファイル全体にかけない。
- 1 タスク = 1 コミット（`T0XX: 内容`）。途中でビルドが通らないタスクは 1 コミットにまとめる。
- 実行していないコマンドを「成功した」と書かない。
- `.github/` 配下を変更しない。
- Go のファイルと Markdown は LF 改行で保存する。
- spec と実際のコードが食い違ったら、推測で進めず、PR の説明に書いて止まる。同じ失敗が 2 回続いたら止まる。

---

## Phase 0: ベースライン

- [ ] **T001** 変更前に全テストが通ることを確認する

  ```bash
  go -C apps/server test ./... -count=1
  ```

---

## Phase 1: オーケストレーター（`llm`）

- [ ] **T002** `apps/server/internal/llm/tool_turn.go` に `ToolTurnMessage`、`ToolTurnRequest`、`ToolTurnResponse`、`ToolTurnModel`、`ToolTurnOutcome`、`ErrNoFinalText`、`RunToolTurn` を作る（`plan.md` 4 章）
- [ ] **T003** `tool_turn_test.go` に次をテストする

  - ツール要求なし → `ModelCalls == 1`、`Tools` が渡されている
  - ツール要求あり → `ModelCalls == 2`、2 回目の `Tools` は空、`ToolCallsExecuted == 1`
  - 1 回目に 2 件の要求 → 実行されるのは先頭 1 件
  - 2 回目も要求のみ（`Text` 空）→ `ErrNoFinalText`、モデル呼び出しは 2 回で止まる
  - `MaxToolCallsPerTurn == 0` → `Tools` なしで 1 回
  - モデルのエラーがそのまま返る

  ```bash
  go -C apps/server test ./internal/llm -run 'TestRunToolTurn' -count=1
  ```

---

## Phase 2: 埋め込みインデックス（`actormemory`）

- [ ] **T004** `apps/server/internal/actormemory/index.go` に `Item`、`IndexOptions`、`Index`、`Hit`、`SearchQuery`、`ErrIndexUnavailable` を作る（`plan.md` 3.5）
- [ ] **T005** `index_test.go` に次をテストする

  - 10 件の投入で 1 回の `Embed`（`BatchSize` 到達）、25 件で `Embed` は 3 回以下
  - `FlushInterval` 経過で、10 件未満でもフラッシュされる
  - `Embed` がブロックしていても `Enqueue` が即座に返る
  - `Embed` が失敗してもワーカーが続き、次の投入が処理される。失敗したバッチは再投入できる
  - 同じ `EventID` の再投入でベクトルが増えない
  - `Search`: 類似度の降順、`visible` で外れたものは返らない、類似度 ≤ 0 は返らない
  - クライアント nil → `Enqueue` が `false`、`Search` が `ErrIndexUnavailable`
  - `Close` が残りをフラッシュして戻る

  ```bash
  go -C apps/server test ./internal/actormemory -run 'TestIndex' -count=1 -race
  ```

---

## Phase 3: ツール実行器（`actormemory`）

- [ ] **T006** `apps/server/internal/actormemory/executor.go` に `Source`、`Executor`、`visibleTo`、各ツールの実装を作る（`plan.md` 3.1〜3.4）
- [ ] **T007** `executor_test.go` に、`world.NewMemoryStore()` に固定データを入れて次をテストする

  - 6 つのツールそれぞれの結果（順序・件数・抜粋の長さ）
  - `limit` の丸め（0 → 3、99 → 5）
  - 別の局の投稿・イベントが、どのツールからも返らない
  - `chat`・`mail` の `channel_filter` が空の結果になる（エラーにならない）
  - 検索ツールが、インデックス nil や埋め込み失敗のとき通常の結果で「利用できません」を返し、他のツールは動く
  - `*world.MemoryStore` が `Source` を満たす（コンパイル時の代入）
  - `llm.NewToolRegistry(llm.StandardActorTools(), executor)` 経由で `Execute` できる

  ```bash
  go -C apps/server test ./internal/actormemory -run 'TestExecutor' -count=1 -race
  ```

---

## Phase 4: リポジトリへの配線

- [ ] **T008** `apps/server/internal/actormemory/indexer.go` に `PostIndexer`（`Index` を包み、`world.Post` から `Item` を作る。`EventID` は `post:<host>:<id>`、`Scope` は `public_board:<host>:<board>`、本文は `Subject + "\n" + Body`）を作る
- [ ] **T009** `apps/server/internal/worldrepo/repository.go` に `PostIndexer` インターフェース、`SetPostIndexer`、`AddPost` 末尾の呼び出しを加える。`New` の引数・既定の挙動は変えない
- [ ] **T010** `apps/server/internal/worldrepo/repository_index_test.go` に次をテストする

  - インデクサ設定時、`AddPost` の後に `IndexPost` が呼ばれる（`Post.ID` が採番済み）
  - 未設定（nil）のとき、既存の `AddPost` の結果と台帳が変わらない
  - 遅いインデクサでも `AddPost` が待たされない（`actormemory.PostIndexer` + ブロックする埋め込みで確認）

  ```bash
  go -C apps/server test ./internal/worldrepo ./internal/actormemory -count=1 -race
  ```

---

## Phase 5: LLM 呼び出し 0 回の確認

- [ ] **T011** `actormemory` のテストで、生成 LLM のフェイク（呼ばれたらカウントするだけ）を用意し、ツール実行と索引の更新を通して 0 回であることを確認する（`executor_test.go` に含めてよい）

---

## Phase 6: 回帰と検証記録

- [ ] **T012** 金型と全テストを実行する

  ```bash
  go -C apps/server test ./internal/world -run 'TestPopulation' -count=1
  go -C apps/server test ./internal/hostprogram/erikak -run 'TestGolden' -count=1
  go -C apps/server test ./... -count=1
  go -C apps/server vet ./internal/actormemory ./internal/llm ./internal/worldrepo
  ```

- [ ] **T013** `specs/011-actor-memory-tools/verification.md` に、実行したコマンドと結果を書く（`-race` が実行できなければ「未実施」と理由）
- [ ] **T014** Draft PR を作成する（完了定義）

**Done**: 上記チェックボックスがすべて埋まり、PR が作成された状態。
