# Tasks: インタラクション台帳と主体識別の基盤（010-interaction-ledger）

**Input**: `specs/010-interaction-ledger/` の `spec.md`、`plan.md`  
**Prerequisites**: `AGENTS.md`、`docs/sdd/SDD_005_CROSS_CHANNEL_INTERACTION_FEEDBACK.md`

## この作業のルール（必読）

- 作業ブランチから `main` への PR を 1 本作る。`main` に直接コミットしない。
- **成果物は Go のコードの変更（実装）とテスト。** spec のファイルを作る・変更することは、この作業の仕事ではない。
- **既存の人物生成・画面金型出力を 1 バイトも変えない。** これが合格条件（`population_golden_test.go`、`screens_golden.txt`）。
- スコープ外（メール・チャット本体の実装、心理値の減衰、エージェントツールのバックエンド）に手を出さない。
- 既存ファイルに整形だけの差分を作らない。`gofmt -w` を既存ファイル全体にかけない。
- 1 タスク = 1 コミット（メッセージは `T0XX: 内容`）。ただし、途中でビルドが通らないタスクは、1 つのコミットにまとめる。
- 実行していないコマンドを「成功した」と書かない。実行できなかったものは「未実施」と理由を書く。
- この作業で `.github/` 配下のファイルは変更しない。
- spec と実際のコードが食い違っていたら、推測で進めず、PR の説明に書いて止まる。

---

## Phase 0: ベースライン確認

- [ ] **T001** 変更前のコードで既存の金型テストおよび全テストが通ることを確認する

  ```bash
  go -C apps/server test ./internal/world -run 'TestPopulationGolden' -count=1
  go -C apps/server test ./internal/hostprogram/erikak -run 'TestGolden' -count=1
  go -C apps/server test ./... -count=1
  ```

**Checkpoint**: 既存テストがすべてパスしていることを確認する。

---

## Phase 1: モデルとデータ型の定義

- [ ] **T002** `apps/server/internal/world/models.go` に `ActorRef`, `InteractionEvent`, `RelationshipEdge` を定義し、`Post` に `AuthorMemberID string` を追加する

  - `ActorRef`: `Kind` ("persona" | "member"), `ID` (文字列)。`IsZero()`, `String()` メソッド。
  - `InteractionEvent`: イベントID、局ID、チャネル種別、アクター、公開スコープ、宛先アクタースライス、応答元イベントID、コンテンツ参照、発生日時、ソース種別。
  - `RelationshipEdge`: 局ID、FromActor、ToActor、BoardReplyCount、LastInteractionAt、LastEventID。
  - `Post`: `AuthorMemberID string` フィールドの追加。

- [ ] **T003** コンパイルが通ることを確認する

  ```bash
  go -C apps/server test ./internal/world -count=1
  ```

---

## Phase 2: メモリストアの実装

- [ ] **T004** `apps/server/internal/world/store.go` に `InteractionLedgerStore` および `RelationshipStore` インターフェースを定義する
- [ ] **T005** `world.MemoryStore` に台帳イベントおよび関係エッジの保存・検索メソッドを実装する

  - `AppendInteractionEvent(event InteractionEvent) error`: 重複イベントIDは無視またはエラー（冪等性）。
  - `ListInteractionEvents(hostID string, limit int) []InteractionEvent`: 発生日時降順で取得。
  - `GetInteractionEvent(eventID string) (InteractionEvent, bool)`: ID引き。
  - `GetRelationship(hostID string, from, to ActorRef) (RelationshipEdge, bool)`: エッジ取得。
  - `UpdateRelationship(edge RelationshipEdge) error`: エッジ更新。
  - `ListRelationshipsFrom(hostID string, from ActorRef) []RelationshipEdge`: 特定アクターからのエッジ一覧。

- [ ] **T006** `apps/server/internal/world/store_test.go` に単体テストを追加して検証する

  ```bash
  go -C apps/server test -v ./internal/world -run 'TestInteractionLedgerStore|TestRelationshipStore'
  ```

---

## Phase 3: 永続ストアへの委譲

- [ ] **T007** `apps/server/internal/worldpersist/store.go` の `Store` 構造体に `InteractionLedgerStore` と `RelationshipStore` の委譲メソッドを追加する
- [ ] **T008** `worldpersist` のテストが通ることを確認する

  ```bash
  go -C apps/server test ./internal/worldpersist -count=1
  ```

---

## Phase 4: 人間投稿時の会員ID付与

- [ ] **T009** `apps/server/internal/hostprogram/erikak/runtime.go` の `Store.AddPost` 呼び出し箇所（新規投稿および返信）において、`AuthorMemberID` を設定する
  - 例: `AuthorMemberID: r.caller`（または `r.handle` を元にした会員ID）
- [ ] **T010** `apps/server/internal/hostprogram/turbobbs/runtime.go` の `Store.AddPost` 呼び出し箇所においても同様に `AuthorMemberID` を設定する
- [ ] **T011** `hostprogram` のテストが通ることを確認する

  ```bash
  go -C apps/server test ./internal/hostprogram/... -count=1
  ```

---

## Phase 5: リポジトリでの台帳追記と関係更新

- [ ] **T012** `apps/server/internal/worldrepo/repository.go` の `AddPost` メソッドにおいて、記事保存時に決定論的台帳イベント追記と関係エッジ更新を統合する
  - 記事の親記事（`ParentID` または `ResponseTargetID`）の存在有無から `channel`（`board_root` または `board_reply`）を判定。
  - 著者アクター（NPCなら `kind=persona`, 人間なら `kind=member`）を構築。
  - 返信の場合、親記事の著者アクターを解決し `AddressedTo` にセット、および返信者→被返信者の `RelationshipEdge`（`BoardReplyCount`+1, `LastInteractionAt`更新）を更新。
  - 生成した `InteractionEvent` を `Store.AppendInteractionEvent` で追記。
- [ ] **T013** LLM プロバイダの呼び出しが 0 回であることを確認するテストを含む `repository_interaction_test.go` を作成する

  ```bash
  go -C apps/server test -v ./internal/worldrepo -run 'TestAddPost_RecordsInteractionEvent|TestAddPost_UpdatesRelationshipEdge'
  ```

---

## Phase 6: 回帰テストと検証記録

- [ ] **T014** 金型テストおよび全テストを実行し、1バイトも壊れていないことを検証する

  ```bash
  go -C apps/server test ./internal/world -run 'TestPopulationGolden' -count=1
  go -C apps/server test ./internal/hostprogram/erikak -run 'TestGolden' -count=1
  go -C apps/server test ./... -count=1
  ```

- [ ] **T015** `specs/010-interaction-ledger/verification.md` を作成し、実行したコマンドとテスト結果を記録する
- [ ] **T016** Draft PR を作成する（完了定義）

**Done**: 上記チェックボックスがすべて埋まり、PR が作成された状態。
