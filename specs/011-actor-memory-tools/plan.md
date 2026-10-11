# Plan: エージェント記憶想起ツールと非同期マイクロバッチ埋め込み

`spec.md` の設計。名前は、読んだコードで確認したものだけを書いている。

## 1. 現状（実コードにもとづく）

| 項目 | 場所 | 現状 | 本 Spec での変更 |
|---|---|---|---|
| ツール定義・レジストリ | `llm/tools.go` | `StandardActorTools()`（6 件）、`ToolRegistry`、`ActorScopedToolExecutor`、`MockToolExecutor`、`DefaultToolExecutionPolicy()`（上限 1） | 変更しない。オーケストレーターを別ファイルで追加 |
| 埋め込み | `llm/embedding.go` | `EmbeddingClient`、`AzureOpenAIEmbeddingClient`、`CosineSimilarity`、`MockEmbeddingClient` | 変更しない |
| 台帳・関係 | `world`（Spec 010） | `InteractionEvent`、`RelationshipEdge`、`InteractionLedgerStore` | 読むだけ |
| 人物事実 | `world.PersonaFactStore` | `ListPersonaFacts(personaID)` | 読むだけ |
| 投稿保存 | `worldrepo/repository.go` | `Repository.AddPost` が台帳追記（Spec 010） | 保存後に任意の `PostIndexer` へ非同期投入 |
| 実行器の実体 | なし | — | 新パッケージ `actormemory` |

`llm` と `world` は互いに import していないため、`actormemory` が両方を import しても循環しない。

## 2. 全体の流れ

```text
保存側（非同期）
  Repository.AddPost ── 保存・台帳追記（既存）
        │
        └─ indexer != nil ─▶ Index.Enqueue(item)        … 即座に返る（チャネルへ投入のみ）
                                   │
                          ワーカー（常駐 goroutine）
                          10 件 or 200ms でバッチ化 ─▶ EmbeddingClient.Embed(texts)  … 1 回
                                   │
                          ベクトルを map に保存（EventID 単位で冪等）

読み出し側（生成の 1 リクエスト内）
  llm.RunToolTurn
    Round 1: Messages + Tools ─▶ model.Complete
        ├─ ToolCalls なし ─▶ 完了（モデル 1 回）
        └─ ToolCalls あり ─▶ 先頭 1 件だけ registry.Execute（actormemory.Executor）
    Round 2: Messages + ToolResult、Tools なし ─▶ model.Complete ─▶ 完了（モデル 2 回）
         └─ なお ToolCalls が返り Text が空 ─▶ ErrNoFinalText
```

## 3. パッケージ `actormemory`

### 3.1 行為者の指定

`actorID` は `world.ActorRef.String()` の形式（`persona:<id>` / `member:<id>`）。
`counterpart_id` は同じ形式、または種別なしの素の ID を受け付け、素の ID は persona・member の両方に一致させる。

### 3.2 `Source`（読み取りに必要な最小の面）

```go
type Source interface {
    ListPosts(hostID string) []world.Post
    ListInteractionEvents(hostID string, limit int) []world.InteractionEvent
    ListPersonaFacts(personaID string) []world.PersonaFact
}
```

`world.MemoryStore` と `worldpersist.Store` がこれを満たす（確認はテストでコンパイル時に行う）。

### 3.3 `Executor`

```go
func NewExecutor(hostID string, src Source, idx *Index) *Executor
func (e *Executor) Execute(ctx context.Context, actorID string, call llm.ToolCall) (llm.ToolResult, error)
```

| ツール | 実装 |
|---|---|
| `get_my_recent_posts` | `ListPosts(host)` から `AuthorPersonaID`/`AuthorMemberID` が行為者に一致するものを、ID 降順で `limit` 件。`board_id` で絞り込み |
| `get_thread_context` | 根記事と、その子孫（`ParentID` を辿る）を ID 昇順に並べ、根を先頭に、最新側から `limit - 1` 件を加えて返す |
| `get_interaction_history_with` | `ListInteractionEvents(host, 500)` から、(行為者→相手) または (相手→行為者宛て) のイベントを新しい順に `limit` 件。本文は `ContentRef` から投稿を引く |
| `search_my_observed_events` | `Index.Search`。観測範囲の絞り込みは `visibleTo(actor)` |
| `get_my_open_loops` | 固定文「未決事項はありません」 |
| `get_my_profile_facts` | `ListPersonaFacts(personaID)`。`topic` があれば `Topic` で絞り込み。行為者が member のときは空 |

出力は人が読める短い箇条書きの文字列（モデルにそのまま渡す）。1 件は `#<記事番号> [<板>] <件名> (<日時>) <投稿者>: <本文抜粋>` の形。

### 3.4 観測範囲 `visibleTo`

台帳イベントは `AudienceScope`（`public_board:<host>:<board>`）を持つ。現状のチャネルは公開板だけなので、
同じ局の `public_board:` スコープは行為者が観測できるものとして扱い、その他のスコープ
（将来の `mail`・`chat`）は、`Actor` か `AddressedTo` に行為者がいるときだけ観測できる。
別の局のデータは、どちらの場合も返さない。

### 3.5 `Index`（非同期マイクロバッチ）

```go
type Item struct {
    EventID    string
    HostID     string
    PostID     int64
    BoardID    string
    Scope      string
    Author     world.ActorRef
    Addressed  []world.ActorRef
    Text       string
    OccurredAt time.Time
}

type IndexOptions struct {
    BatchSize     int           // 既定 10
    FlushInterval time.Duration // 既定 200ms
    QueueSize     int           // 既定 1024
}

func NewIndex(client llm.EmbeddingClient, opts IndexOptions) *Index // ワーカーを起動
func (i *Index) Enqueue(item Item) bool                              // 非ブロッキング
func (i *Index) Search(ctx context.Context, q SearchQuery) ([]Hit, error)
func (i *Index) Close()                                              // 残りをフラッシュして停止
func (i *Index) Len() int
```

- `Enqueue` はチャネルが満杯のときは落として `false` を返す（保存を止めない）。`seen` は `Enqueue` 成功時に印を付け、埋め込みが失敗したバッチは印を外す（再投入可能）。
- ワーカーは `select` で、`BatchSize` 到達または `FlushInterval` のタイマーでフラッシュする。`Embed` の失敗はログに出し、ワーカーは続ける。
- `Search` は、問い合わせを `Embed([]string{q})` で 1 回埋め込み、ロック下でスナップショットを取って、`visible` 関数で絞り込み、`llm.CosineSimilarity` の降順に並べる。類似度 ≤ 0 は除く。
- `client == nil` のとき `Enqueue` は何もせず `false`、`Search` は `ErrIndexUnavailable` を返す。

## 4. `llm.RunToolTurn`

```go
type ToolTurnMessage struct {
    Role       string // "system" | "user" | "assistant" | "tool"
    Content    string
    ToolCalls  []ToolCall
    ToolResult *ToolResult
}
type ToolTurnRequest  struct { Messages []ToolTurnMessage; Tools []ToolDefinition }
type ToolTurnResponse struct { Text string; ToolCalls []ToolCall }
type ToolTurnModel interface { Complete(context.Context, ToolTurnRequest) (ToolTurnResponse, error) }

type ToolTurnOutcome struct { Text string; ModelCalls, ToolCallsExecuted int }

var ErrNoFinalText = errors.New("model returned no final text")

func RunToolTurn(ctx context.Context, model ToolTurnModel, reg *ToolRegistry, policy ToolExecutionPolicy,
    actorID string, messages []ToolTurnMessage) (ToolTurnOutcome, error)
```

- `reg == nil`、または `policy.MaxToolCallsPerTurn <= 0` のときは `Tools` を渡さず 1 回だけ呼ぶ。
- 1 回目の `ToolCalls` が複数でも、実行するのは先頭の 1 件。2 回目の `Messages` には、assistant（その 1 件のツール要求）と tool（結果）を足す。
- 2 回目の `Tools` は空。`Text` が空で `ToolCalls` が返ったら `ErrNoFinalText`。

## 5. `worldrepo` への配線

```go
// worldrepo
type PostIndexer interface{ IndexPost(hostID string, p world.Post) }

func (r *Repository) SetPostIndexer(ix PostIndexer)
```

- `AddPost` の末尾で、`ix != nil` のとき `ix.IndexPost(hostID, saved)` を呼ぶ。
- `actormemory.Index` を `PostIndexer` にする薄い実装 `actormemory.PostIndexer`（`Index` を包み、`Post` から `Item` を作る。`EventID` は Spec 010 と同じ `post:<host>:<id>`）。
- `cmd/server` での有効化は、埋め込みのエンドポイントとキーが設定されているときだけ。本 spec では `main.go` を変更しない（後続 spec で有効化）。

## 6. 変更するファイル

1. `apps/server/internal/llm/tool_turn.go`（新規）と `tool_turn_test.go`
2. `apps/server/internal/actormemory/`（新規）: `executor.go`、`index.go`、`indexer.go`、各 `_test.go`
3. `apps/server/internal/worldrepo/repository.go`: `PostIndexer`、`SetPostIndexer`、`AddPost` 末尾の呼び出し
4. `specs/011-actor-memory-tools/verification.md`（新規）

## 7. 検証と金型テスト方針

- **金型の保全**: 既存の出力は変えない（`AddPost` の戻り値・台帳・関係は同じ）。`population_golden_test.go`、`TestGolden*` が変更なしで通る。
- **新規テスト**: 各ツールの結果、観測範囲外が返らないこと、マイクロバッチ（10 件／200ms／失敗継続／ブロック中の `Enqueue`）、冪等、縮退、`RunToolTurn` の呼び出し回数、LLM 呼び出し 0 回。
- **並行性**: ワーカーとテストが触る箇所は `go test -race` で確認する（実行できない環境では「未実施」と理由を書く）。
