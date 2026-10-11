# Plan: インタラクション台帳と主体識別の基盤

`spec.md` の設計。数や名前は、読んだコードで確認したものだけを書いている。

## 1. 現状の分類（実コードにもとづく）

| 項目 | 場所 | 現状 | 本 Spec での変更 |
|---|---|---|---|
| 人間の投稿保存 | `hostprogram/erikak/runtime.go:106`, `144`<br>`hostprogram/turbobbs/runtime.go:539` | `world.Post{Author: r.handle, ...}` のみ。会員IDなし | `AuthorMemberID: r.callerID`（または `r.handle` 由来の会員ID）を付与 |
| 記事モデル | `world/models.go` の `Post` | `Author`, `AuthorPersonaID` を持つ | `AuthorMemberID string` を追加 |
| 台帳の存在 | なし（各板の `Post` のみ） | 共通台帳なし | `world.InteractionEvent` と `InteractionLedgerStore` を追加 |
| 関係エッジ | なし（spec 008 では関係項は 1 固定） | 関係の保存・計算なし | `world.RelationshipEdge` と `RelationshipStore` を追加 |
| 保存処理 | `world/store.go` の `Store.AddPost` | メモリ保存のみ | 記事保存時に決定論的に `InteractionEvent` を生成・追記し関係エッジを更新 |

## 2. 全体の流れ

```text
記事の保存（人間 / NPC）
   │
   ▼
1. world.Post の確定
   ├─ NPC: AuthorPersonaID が設定されている
   └─ 人間: AuthorMemberID が設定されている（未設定時は Author 文字列からフォールバック）
   │
   ▼
2. InteractionEvent の生成・追記（純関数・LLMなし）
   ├─ EventID: 冪等な一意キー（"post:<HostID>:<PostID>"）
   ├─ Actor: NPCなら ActorRef{Kind: "persona", ID: AuthorPersonaID}
   │         人間なら ActorRef{Kind: "member", ID: AuthorMemberID}
   ├─ Channel: 親記事なしなら "board_root"、返信なら "board_reply"
   ├─ Scope: "public_board:<HostID>:<BoardID>"
   ├─ AddressedTo: 返信なら [親記事の著者ActorRef]、新規なら []
   └─ ContentRef: PostID
   │
   ▼
3. RelationshipEdge の更新（返信時のみ・LLMなし）
   ├─ 返信者 Actor ──> 親記事著者 Actor の有向エッジ
   ├─ BoardReplyCount をインクリメント
   ├─ LastInteractionAt を投稿日時に更新
   └─ 同一 EventID の二重適用を防止（冪等性）
```

## 3. データ構造

### 3.1 `world.ActorRef`（新規）

```go
type ActorRef struct {
    Kind string `json:"kind"` // "persona" | "member"
    ID   string `json:"id"`   // persona_id または member_id
}

func (a ActorRef) IsZero() bool {
    return a.Kind == "" && a.ID == ""
}

func (a ActorRef) String() string {
    return a.Kind + ":" + a.ID
}
```

### 3.2 `world.Post` へのフィールド追加

```go
type Post struct {
    ID              int64      `json:"id"`
    BoardID         string     `json:"board_id,omitempty"`
    ParentID        int64      `json:"parent_id,omitempty"`
    Author          string     `json:"author"`
    AuthorPersonaID string     `json:"author_persona_id,omitempty"`
    AuthorMemberID  string     `json:"author_member_id,omitempty"` // 新規追加
    Subject         string     `json:"subject"`
    Intent          PostIntent `json:"intent,omitempty"`
    Body            string     `json:"body"`
    CreatedAt       time.Time  `json:"created_at"`
}
```

### 3.3 `world.InteractionEvent`（新規）

```go
type InteractionEvent struct {
    EventID          string     `json:"event_id"`
    HostID           string     `json:"host_id"`
    Channel          string     `json:"channel"` // "board_root", "board_reply"
    Actor            ActorRef   `json:"actor"`
    AudienceScope    string     `json:"audience_scope"` // "public_board:<host_id>:<board_id>"
    AddressedTo      []ActorRef `json:"addressed_to,omitempty"`
    RespondsToEventID string    `json:"responds_to_event_id,omitempty"`
    ContentRef       string     `json:"content_ref"` // post_id (例: "1001")
    OccurredAt       time.Time  `json:"occurred_at"`
    SourceKind       string     `json:"source_kind"` // "npc_world_selected", "human_authored"
}
```

### 3.4 `world.RelationshipEdge`（新規）

```go
type RelationshipEdge struct {
    HostID             string    `json:"host_id"`
    FromActor          ActorRef  `json:"from_actor"`
    ToActor            ActorRef  `json:"to_actor"`
    BoardReplyCount    int       `json:"board_reply_count"`
    LastInteractionAt  time.Time `json:"last_interaction_at"`
    LastEventID        string    `json:"last_event_id"`
}
```

## 4. 保存層（Store）インターフェースの拡張

`world/store.go` の `Store` インターフェースに以下を追加する:

```go
type InteractionLedgerStore interface {
    AppendInteractionEvent(event InteractionEvent) error
    ListInteractionEvents(hostID string, limit int) []InteractionEvent
    GetInteractionEvent(eventID string) (InteractionEvent, bool)
}

type RelationshipStore interface {
    GetRelationship(hostID string, from, to ActorRef) (RelationshipEdge, bool)
    UpdateRelationship(edge RelationshipEdge) error
    ListRelationshipsFrom(hostID string, from ActorRef) []RelationshipEdge
}
```

- `MemoryStore` にスライスおよびマップによる実装を追加。
- `worldpersist/store.go` に永続化アダプタを追加（既存の `BoardActivityStateStore` や `PersonaFactStore` と同様のパターン）。

## 5. 変更するファイル

1. `apps/server/internal/world/models.go`: `ActorRef`, `InteractionEvent`, `RelationshipEdge` の定義、`Post.AuthorMemberID` の追加。
2. `apps/server/internal/world/store.go`: `InteractionLedgerStore`, `RelationshipStore` インターフェースおよび `MemoryStore` での実装。
3. `apps/server/internal/worldpersist/store.go`: 永続ストアへの委譲・保存メソッドの追加。
4. `apps/server/internal/hostprogram/erikak/runtime.go`: 人間が投稿する際、会員ID（`r.handle` やセッションのID）を `Post.AuthorMemberID` にセット。
5. `apps/server/internal/hostprogram/turbobbs/runtime.go`: 同様に `AuthorMemberID` をセット。
6. `apps/server/internal/worldrepo/repository.go`: `AddPost` 保存時に自動的に `InteractionEvent` を記録し関係エッジを更新する配線。

## 6. 検証と金型テスト方針

- **金型の保全**: 人物生成テスト（`population_golden_test.go`）や画面金型（`screens_golden.txt`）は、既存の乱数消費順序や表示出力を一切変更しないため、1バイトも変更なくパスすることを確認する。
- **新規単体テスト**:
  - 人間投稿時およびNPC投稿時の `InteractionEvent` 正常追記テスト。
  - 返信による `RelationshipEdge` のインクリメントと冪等性テスト（重複追記でカウントが増えないこと）。
  - LLMプロバイダ呼び出し回数が 0 回であることの検証テスト。
