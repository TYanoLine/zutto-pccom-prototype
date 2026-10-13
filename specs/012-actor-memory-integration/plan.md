# Plan: 記事本文生成への記憶ツール結合とトーン調整（012-actor-memory-integration）

**Spec ID**: 012-actor-memory-integration  
**Depends on**: Spec 010, Spec 011, SDD-005 P3

## 1. アーキテクチャとデータの流れ

```text
[サーバー起動 (main.go)]
       │
       ├─> llm.OpenAIEmbeddingClient 作成
       ├─> actormemory.NewIndex 作成
       ├─> runtimeStore.SetPostIndexer(actormemory.NewPostIndexer(index))
       └─> runtimeStore.SetActorMemoryIndex(index)

[投稿保存 (AddPost)]
       │
       ├─> 台帳 (InteractionEvent) 追記
       ├─> 関係エッジ (RelationshipEdge) 更新
       └─> PostIndexer.IndexPost() (非同期バッチ投入 -> 200ms/10件で埋め込み)

[記事本文生成 (MaterializeArticle)]
       │
       ├─> Repository.materializeArticleBodyOnce
       │     └─> actormemory.NewExecutor(hostID, store, actorMemoryIndex)
       │
       └─> LLMMaterializer.GenerateBoardPostsWithUsage
             │
             ├─ [記憶ツール有効時]
             │     ├─ ToolRegistry (StandardActorTools + Executor)
             │     └─ llm.RunToolTurn (ToolTurnModel, MaxToolCallsPerTurn=1)
             │           │
             │           ├─ Round 1: プロンプト + ツール定義提示
             │           │     ├─ Case A: ツール不要 (通常会話) -> 即時本文出力 (モデル1回)
             │           │     └─ Case B: ツール要求 -> Executor 実行 (<1ms)
             │           │            └─ Round 2: ツール結果を文脈に追加 -> 最終本文出力 (モデル2回)
             │           │
             │           └─ 出力テキストを BoardPostDraft にパース・検証
             │
             └─ [ツール無効 / フォールバック時]
                   └─ 従来の Renderer.GenerateBoardPost
```

---

## 2. 変更対象コンポーネント

### 2.1 `apps/server/cmd/server/main.go`
- `cfg.AzureOpenAIEndpoint != "" && cfg.AzureOpenAIKey != ""` の場合、`llm.NewOpenAIEmbeddingClient` を作成。
- `actormemory.NewIndex(embeddingClient, actormemory.IndexOptions{BatchSize: 10, FlushInterval: 200 * time.Millisecond})` を作成。
- `runtimeStore.SetPostIndexer(actormemory.NewPostIndexer(index))` を登録。
- `runtimeStore.SetActorMemoryIndex(index)` を登録。
- サーバー終了時に `index.Close()` を呼び出し。

### 2.2 `apps/server/internal/worldrepo/repository.go`
- `Repository` に `actorMemoryIndex *actormemory.Index` フィールドを追加。
- `SetActorMemoryIndex(idx *actormemory.Index)` メソッドを追加。
- `ActorMemoryIndex() *actormemory.Index` ゲッターを追加。

### 2.3 `apps/server/internal/worldrepo/llm_materializer.go`
- `LLMMaterializer` に以下のオプショナルフィールドを追加：
  - `ToolModel llm.ToolTurnModel`: ツールターン実行用のモデルクライアント。
  - `ActorMemoryIndex *actormemory.Index`: 記憶インデックス。
  - `ActorMemoryStore actormemory.Source`: メモリストア。
- `GenerateBoardPostsWithUsage` 内で、上記が設定されており、かつ返信先またはアクターが存在する場合に `generateBoardPostWithTools` を試行。
- ツールターン実行時：
  - `actormemory.NewExecutor(req.Host.ID, m.ActorMemoryStore, m.ActorMemoryIndex)` を作成。
  - `llm.NewToolRegistry(llm.StandardActorTools(), executor)` を構築。
  - `llm.RunToolTurn` を呼び出し。
  - エラー時または未設定時は従来の `m.Renderer.GenerateBoardPost` にフォールバック。

### 2.4 `apps/server/internal/llm/tool_turn_openai.go`
- OpenAI / Azure OpenAI 用の `ToolTurnModel` 実装（`OpenAIToolTurnModel`）。
- `tools` パラメータ（`llm.ToolDefinition` を OpenAI 形式の function 定義に変換）をサポート。
- モデルの `tool_calls` レスポンスを `llm.ToolCall` にデコード。
- ツール結果（`ToolTurnMessage{Role: "tool"}`）をメッセージ履歴に含めて 2 回目を呼び出し。

### 2.5 プロンプトとトーン調整 (`BuildBoardPostPrompt` / `StandardActorTools`)
- `StandardActorTools` の説明文に「不要なときは絶対に呼ばないこと」「通常はツールを呼ばずに直接本文を返答すること」を明記。
- プロンプト規約に以下を追加：
  - **無理な過去言及の禁止**: 質問されていない過去の出来事・会話を唐突に蒸し返さない。
  - **対等な距離感**: 人間を主人公・特別扱いせず、当時のネット住民としての距離感を保つ。
  - **1996 年らしい文体**: (笑)、(^^;、適度な `>` 引用。

---

## 3. サンプリングと検証計画

### 3.1 単体テスト・統合テスト
1. **配線テスト**: `main.go` 相当の初期化で、`AddPost` された記事がインデックス経由で `Executor` から検索可能になること。
2. **ツールターン統合テスト**:
   - ケース 1: 過去ログについて質問されたプロンプト → ツール要求発生 → ツール結果を反映した本文が返る。
   - ケース 2: 初対面・通常雑談プロンプト → ツール要求なし（1 回で本文生成） → 自然な本文が返る。
   - ケース 3: ツールモデル未設定時 → 従来のレンダラーで正常生成。
3. **金型テスト保全**:
   - `TestPopulationMatchesGolden` (1 バイトも変えない)
   - `TestGoldenBoardTableShape` (1 バイトも変えない)
4. **全パッケージテスト**:
   - `go -C apps/server test ./... -count=1` がすべて PASS すること。

### 3.2 サンプリングハーネス（`materialize_sampling_test.go`）
- 複数の具体例（ゲーム板での質問、初対面挨拶、雑談）について、生成されるプロンプト、ツール呼び出しの有無、最終本文のトーンを一覧出力・検証できるようにする。
