# Tasks: 記事本文生成への記憶ツール結合とトーン調整（012-actor-memory-integration）

**Input**: `specs/012-actor-memory-integration/` の `spec.md`、`plan.md`  
**Prerequisites**: `AGENTS.md`、`docs/sdd/SDD_005_CROSS_CHANNEL_INTERACTION_FEEDBACK.md`、Spec 010、Spec 011

## この作業のルール（必読）

- 作業ブランチから `main` への PR を 1 本作る。`main` に直接コミットしない。
- **成果物は Go のコード（実装）とテスト。** spec のファイルは変更しない。追加してよい spec フォルダのファイルは `verification.md` だけ。
- **既存の人物生成・画面金型出力を 1 バイトも変えない。**
- スコープ外（チャット UI、メール送受信、局間伝播）に手を出さない。
- 既存ファイルに整形だけの差分を作らない。`gofmt -w` を既存ファイル全体にかけない。
- 1 タスク = 1 コミット（`T0XX: 内容`）。
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

## Phase 1: OpenAI ToolTurnModel（`llm`）

- [ ] **T002** `apps/server/internal/llm/tool_turn_openai.go` に `OpenAIToolTurnModel` を作成（OpenAI 形式の tools 定義・tool_calls デコード・履歴管理）
- [ ] **T003** `apps/server/internal/llm/tool_turn_openai_test.go` に以下をテストする

  - ツール要求なし時: 正常にテキストと使用トークンが返る
  - ツール要求あり時: `ToolCalls` が正しくパースされ、引数 JSON が保持される
  - 2 回目（ツール結果返却時）: ツール結果が正しくペイロードに渡される
  - エラー時のハンドリング

  ```bash
  go -C apps/server test ./internal/llm -run 'TestOpenAIToolTurnModel' -count=1
  ```

---

## Phase 2: リポジトリ・マテリアライザーへの記憶ツール統合（`worldrepo`）

- [ ] **T004** `apps/server/internal/worldrepo/repository.go` に `SetActorMemoryIndex`、`ActorMemoryIndex` を追加する
- [ ] **T005** `apps/server/internal/worldrepo/llm_materializer.go` にツールターン生成パス（`ToolModel`、`ActorMemoryIndex`、`ActorMemoryStore` がある場合に `RunToolTurn` を呼び出し）を追加する
- [ ] **T006** `apps/server/internal/worldrepo/materialize_tool_test.go` に以下をテストする

  - ツール要求あり時: `Executor` が呼ばれ、ツール結果を踏まえた本文が返る
  - ツール要求なし時: 1 回のモデル呼び出しで本文が返る
  - ツールモデル未設定時: 従来のレンダラーで正常に本文が返る（フォールバック）

  ```bash
  go -C apps/server test ./internal/worldrepo -run 'TestMaterializeTool' -count=1
  ```

---

## Phase 3: プロンプトとトーン調整（`llm`）

- [ ] **T007** `apps/server/internal/llm/board_post_prompt.go` および `tools.go` で、不自然な記憶連呼の抑制規約（聞かれていない過去の出来事を無理に蒸し返さない、1996 年の自然な対等な距離感）を追加する
- [ ] **T008** `apps/server/internal/worldrepo/materialize_sampling_test.go` にサンプリングハーネスを作成し、代表シナリオでの出力文章とツール呼び出しの有無を検証する

  ```bash
  go -C apps/server test -v ./internal/worldrepo -run 'TestMaterializeSampling' -count=1
  ```

---

## Phase 4: サーバー起動部への配線（`cmd/server`）

- [ ] **T009** `apps/server/cmd/server/main.go` で、埋め込みクライアント設定時に `actormemory.NewIndex` を作成し、`runtimeStore.SetPostIndexer` および `runtimeStore.SetActorMemoryIndex` に配線する

---

## Phase 5: 回帰と検証記録

- [ ] **T010** 金型と全テストを実行する

  ```bash
  go -C apps/server test ./internal/world -run 'TestPopulation' -count=1
  go -C apps/server test ./internal/hostprogram/erikak -run 'TestGolden' -count=1
  go -C apps/server test ./... -count=1
  go -C apps/server vet ./internal/actormemory ./internal/llm ./internal/worldrepo ./cmd/server
  ```

- [ ] **T011** `specs/012-actor-memory-integration/verification.md` に、実行したコマンドとサンプリング結果・検証証跡を書く
- [ ] **T012** Draft PR を作成する（完了定義）

**Done**: 上記チェックボックスがすべて埋まり、PR が作成された状態。
