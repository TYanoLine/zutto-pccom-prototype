# Plan: Erika-K の板の計画と記事番号

対応する spec: `specs/020-erika-k-article-numbering/spec.md`

## 現状の分類

| 項目 | 現状（読んだコード） | 場所 |
|---|---|---|
| 投稿の内部 ID | 店舗全体で 1 つのカウンタ（`s.next++`） | `apps/server/internal/world/store.go` の `MemoryStore.AddPost`（L365 付近）、`worldpersist/store.go` の `AddPost`（L159 付近）、`worldrepo/repository.go` の `AddPost`（L243 付近） |
| 保存 | スナップショットに `next_post_id` と投稿の配列 | `apps/server/internal/world/development_snapshot.go`（L20、L71、L102〜124） |
| 投稿の型 | `world.Post`（`ID`、`BoardID`、`ParentID` など） | `apps/server/internal/world/models.go` L134〜149 |
| 任意の機能の受け渡し | `HostDetailStore` のように、任意のインターフェースを型アサーションで使う | `apps/server/internal/world/store.go` L18〜20 |
| 一覧の番号 | `p.ID` を `%4d` で表示 | `apps/server/internal/hostprogram/erikak/runtime.go` の `renderBoardIndex`（L720 付近〜、L750 付近） |
| 投稿完了の番号 | `MSG No.%d を登録しました`、`p.ID` | `runtime.go` の `new_body`（L120〜137） |
| アペンド | `ParentID: root.ID` で `AddPost`（内部 ID を消費） | `runtime.go` の `append_body`（L155〜178） |
| 番号による指定 | `rootPost(id)` が `p.ID == id` で引く | `runtime.go` L1081〜1088。呼び出し 4 箇所（L147、L160、L444、L457） |
| 活動計画 | `planBoardActivity(host, board, now)` が、呼び出し時刻から `TotalRoots` などを計算する。時間とともに変わる | `apps/server/internal/worldrepo/board_activity.go`（L47〜157） |
| 板の開始日 | `board.OpenedOn`、無ければ `host.FoundedOn` | `board_activity.go` L48〜52 |
| 初回の記事の作成 | 記事の無い板を開くと `CatchUpInitialBoardActivity` が、最新の記事（最大 10 件）と返信を、1 回の `PlanBBSBatch` で計画する | `worldrepo/observation.go`（L159 付近）、`bbsengine/engine.go` の `catchUp`（L265〜310） |
| タイトル・状況の生成 | batch の中で、状況は 6 件ずつ、タイトルは 20 件ずつ LLM に頼む | `worldrepo/bbs_situation_first_planner.go` L22〜24 |
| その後の成長 | `batchSize` で 3〜7 件ずつ（`MaxBatchSize = 7`） | `bbsengine/engine.go` L27、L540 付近 |
| batch 内の並び | `SlotIndex` の順 | `llm/bbs_article_batch.go` の `sortBBSBatchPosts`（L273） |

**分類**: 採番と計画は Erika-K の表示と指定にだけ使う。共有の `Post` と共有エンジンは、後方互換な任意フィールドと、
Erika-K から渡される値だけを扱う。他のホストは番号と計画を持たない（spec FR-008）。

## 設計

### 1. 投稿のフィールド

`world.Post` に、任意のフィールドを 2 つ足す。

```go
// BoardNo is the article number a Erika-K board shows (spec 020). Zero means
// the host does not number posts this way. Replies leave it zero.
BoardNo int64 `json:"board_no,omitempty"`
// ReplyNo is the number of a reply within its parent article (spec 020). Zero
// for articles and for hosts that do not number replies this way.
ReplyNo int64 `json:"reply_no,omitempty"`
```

### 2. 板の計画（確定して保存する。個々の日付は持たない）

```go
// BoardPlan is fixed once per board when the board index first shows its
// counts (spec 020, FR-002). It is never recomputed after it is stored.
type BoardPlan struct {
    HostID    string
    BoardID   string
    OpenedOn  time.Time // date of article 1
    Total     int64     // N: the largest article number in the plan
    PlannedAt time.Time // no planned article is dated after this
    // The oldest article made so far. The next (older) batch is made below it.
    FloorNo   int64     // N+1 before any article is made
    FloorDate time.Time // PlannedAt before any article is made
}
```

- **N と `OpenedOn`**: 既存の活動計画の式（`planBoardActivity` の `totalRoots` と `OpenedOn`）から、板一覧の表示時刻で算出する。
  確定の後は再計算しない。
- **`FloorNo` と `FloorDate`**: batch を作るたびに、その batch の最も古い番号と日付で更新する。残っている記事から計算しない。
- **保存**: snapshot（`development_snapshot.go`）に `board_plans` として保存する。キーは `hostID|boardID`。

### 3. 採番器（保存する、残存記事から計算しない）

- `world.Store` の任意機能として `BoardNumberStore` を定義する（`HostDetailStore` と同じ形）。

  ```go
  // BoardNumberStore keeps Erika-K board plans and number counters (spec 020).
  // Issued values are never decreased, even when posts are removed.
  type BoardNumberStore interface {
      BoardPlan(hostID, boardID string) (BoardPlan, bool)
      SetBoardPlan(plan BoardPlan) error         // only when absent: plans are fixed once
      AdvanceBoardPlanFloor(hostID, boardID string, no int64, date time.Time) error
      NextBoardNo(hostID, boardID string) int64  // after the plan: N+1, N+2, ...
      NextReplyNo(hostID string, parentID int64) int64
  }
  ```

- 実装: `MemoryStore`（`world/store.go`）と `worldpersist.Store`（`worldpersist/store.go`）。
  - 板の採番器は「最後に発行した番号」。計画を確定したとき、その値を N にする。
  - 返信の採番器は、親の内部 ID をキーにした「最後に発行した返信番号」。
  - snapshot に `board_plans`、`board_numbers`、`reply_numbers` として書き出す。`omitempty`。
- `SetBoardPlan` は、既に計画がある板には書かない（固定）。
- `AdvanceBoardPlanFloor` は、`FloorNo` を小さくする方向にだけ動かす。

### 4. 計画の記事の作成（必要な件数を、新しい順に、範囲を指定して）

記事が必要になったとき（板を開いたときの初回の作成、さらに古い記事が必要になったとき）に、次の順で作る。

1. 件数 K を決める（初回は既存の `InteractiveInitialRootLimit`、最大 10 件。`FloorNo - 1` を超えない）。
2. 番号を決める: `FloorNo - 1`、`FloorNo - 2`、…、`FloorNo - K`（新しい順）。
3. 範囲を決める:
   - 上限（to）: `FloorDate`（初回は `PlannedAt`）。
   - 下限の目安（from）: 板の累積の活動量の曲線で、番号 `FloorNo - K` に当たる時刻（累積の割合 `(FloorNo - K - 1) / N`）。
     ただし、番号 1 を含む batch の下限は `OpenedOn`。
   - 目安が上限より後になった場合は、下限を `OpenedOn` にする。
4. LLM に、件数、番号、範囲、ペルソナ（既存の slot の作者）を渡し、**新しい記事から順に** 作らせる。
   各記事の日付は LLM が範囲の中で決める。記事の間の関係（前の記事への言及など）を意識させる。
5. 検証（`numbering.go`）:
   - 番号の大きい順に、日付が同じか古くなっている。
   - すべての日付が `OpenedOn` 以上、上限以下。
   - 番号 1 の記事は `OpenedOn` の日付。
   失敗したら、既存の試行回数（`productionSituationChunkAttempts`）の範囲で作り直す。
   それでも失敗したら、その batch を保存しない。日付を作り替えない。
6. 保存: 記事を `BoardNo` 付きで `AddPost` し、`AdvanceBoardPlanFloor` で `FloorNo` と `FloorDate` を更新する。
   記事の保存と floor の更新は、同じ時点で行う（片方だけ残らない）。

- 作る順と番号の順が一致するので、ソートは要らない。batch 内の並びは、既存の `SlotIndex` の順のまま。
- LLM を使わない動作では、範囲を K 等分した決定的な日付を、新しい順に使う。

### 5. 計画の後の記事と返信

| 作られ方 | 割り当て |
|---|---|
| 計画の後の利用者の記事・ペルソナの記事 | `BoardNo = NextBoardNo(host, board)` |
| 利用者の返信（`append_body`） | `ReplyNo = NextReplyNo(host, root.ID)`。`ParentID: root.ID` |
| 生成された返信（共有エンジン） | 同じく `NextReplyNo`。日付は既存の遅れの計算のまま。返信を作る経路は 3 つ（`bbsengine/engine.go` L398 付近と L831 付近、`worldrepo/reaction.go` L354 付近）。すべて下の「返信の番号づけの判断」に従う |

**返信の番号づけの判断**: 共有の層が「この局は返信を番号づけする」と判断しない。既に各経路が呼んでいるホストプログラムの
返信の表現（`hostprogram.ProjectReply` → `erikak.ProjectReply`）に、後方互換な任意フィールド `NumbersReplies bool` を足し、
Erika-K だけが `true` を返す。3 つの経路は、`ParentID != 0` かつ `NumbersReplies` のときだけ `NextReplyNo` を呼ぶ。
局の名前や ID で分岐しない（AGENTS.md）。

### 6. 番号による指定と表示

- `rootPost(id)` を `rootPostByNo(boardPath, no)` に変える（`ParentID == 0`、`BoardID == boardPath`、`BoardNo == no`）。
- `BR n` と `A n` の `n` は、板の記事番号として解決する。返信番号による指定はしない（spec FR-007）。
- 利用者の入力を解決する箇所（`append_target`、`BR n`、`A n`）だけが `rootPostByNo` を使う。`append_body` は `r.threadID`（内部 ID）から親を引くので、
  内部 ID で引く `rootPost` を残す。番号と内部 ID を取り違えない。
- 一覧の番号列は `BoardNo`。`%4d` の幅は変えない。
- `MSG No.%d を登録しました` は `BoardNo`。
- アペンドの見出しは `アペ N`（`ReplyNo`）。現状の連番（`appendNo`）を `ReplyNo` に置き換える。
- `ap/ref` 列の数え方（`appendCountFrom`）は変えない。

### 7. 既存の記事（本仕様の範囲外）

既存の記事のリセットは、別の手段で行う。本仕様は、計画と採番器が空の状態から始まる板を前提にする。

## 変更するファイル

| ファイル | 変更 |
|---|---|
| `apps/server/internal/world/models.go` | `Post.BoardNo`、`Post.ReplyNo`、`BoardPlan` を追加 |
| `apps/server/internal/world/store.go` | `BoardNumberStore` インターフェース、`MemoryStore` の実装 |
| `apps/server/internal/worldpersist/store.go` | `BoardNumberStore` の実装（保存と読み込み） |
| `apps/server/internal/world/development_snapshot.go` | `board_plans`、`board_numbers`、`reply_numbers` の保存と読み込み |
| `apps/server/internal/worldrepo/board_activity.go` | 計画の算出（N と `OpenedOn`）。板一覧の表示時刻を引数で受け取る形にする。累積の活動量の曲線で、番号に当たる時刻の目安を返す関数 |
| `apps/server/internal/worldrepo/bbs_situation_first_planner.go` ほか、計画の記事を作る経路 | 件数、番号、範囲を LLM の要求に渡す。新しい順に作る指示 |
| `apps/server/internal/llm/`（記事の batch の要求と検証） | 要求に範囲と番号を足す。返った日付の検証 |
| `apps/server/internal/hostprogram/erikak/runtime.go` | 利用者の記事と返信の採番、`rootPostByNo`、一覧と `MSG No.` の表示、`アペ N`、`BR n` と `A n` の解決 |
| `apps/server/internal/hostprogram/erikak/numbering.go`（新規） | 採番の呼び出し、batch の範囲の決定、日付の検証 |
| `apps/server/internal/bbsengine/engine.go` | 計画の記事の番号と範囲、返信の番号を、任意で渡す（既定は何もしない）。返信の 2 経路 |
| `apps/server/internal/worldrepo/reaction.go` | 人間の記事へのペルソナの反応の返信に、`NumbersReplies` のときだけ `ReplyNo` を付ける |
| `apps/server/internal/hostprogram/replymodel/` と `bbsengine.ReplyRepresentation` | 任意フィールド `NumbersReplies bool` |
| `apps/server/internal/hostprogram/erikak/reply_semantics.go` | `ProjectReply` が `NumbersReplies: true` を返す |
| `apps/server/internal/hostprogram/erikak/golden_numbering_test.go`（新規） | 金型テスト（画面の形式） |
| `apps/server/internal/hostprogram/erikak/numbering_test.go`（新規） | US1〜US6 のテスト |
| `apps/server/internal/hostprogram/erikak/testdata/numbering_golden.json`（新規） | 変更前に生成する |
| `specs/020-erika-k-article-numbering/verification.md`（新規） | 実装した人が書く記録 |
| `docs/host-programs/erika-k.md` | 仕様の節を、判断事項の結果に合わせて更新 |

## 金型の方針

- 変更前のコードで、HAKATA の画面の形式（板の一覧の罫線と欄、記事と返信の表示の形、`BR n` と `A n` の応答の形）を金型として記録する（実装より前にコミット）。
- 金型は、番号と、計画による日付・件数を含めない形（マスクした形）で比べる。
  比べる対象は、罫線、欄の幅、文言、コマンドの応答の形。
- 番号と日付・件数は、新しい計画で変わることを許容する。その代わり、US3（順序）と US4（固定）をテストで確認する。
- 金型ファイルは、変更後に書き換えない（AGENTS.md の方針）。

## 検証

- 金型テスト（画面の形式のみ。差分が出ないこと）。
- US1〜US6 の Independent Test（`numbering_test.go`）。LLM を使わない決定的な経路で行う。
- `go -C apps/server build ./...`、`go -C apps/server vet ./...`、`go -C apps/server test ./...`。
- 手動確認は、局の画面で板を初めて開き、番号の順と日付の順が一致すること、さらに古い記事を開いても一致することを確かめる（実施できれば）。

## リスクと対策

| リスク | 対策 |
|---|---|
| 計画が時間とともに変わる（現状の `planBoardActivity` は呼び出し時刻に依存） | 計画を一度確定して保存し、以後は読むだけにする。US4 のテスト |
| LLM の日付が範囲や順序を破る | 保存前に検証し、既存の試行回数で作り直す。失敗したら batch を保存しない |
| 前の batch が古い日付に寄り、古い側の範囲が狭くなる | 下限を目安として扱い、目安が上限を超えたら `OpenedOn` まで広げる |
| 記事の保存と floor の更新が片方だけ残る | 同じ時点で行い、失敗時は両方を行わない |
| 共有エンジンの変更で、他のホストの番号や日付が変わる | 任意の値として渡す。Erika-K 以外には渡さない。他のホストのテストを全件通す |
| `rootPost` の呼び出し 4 箇所で、番号と内部 ID を取り違える（特に `append_body` の `threadID`） | 利用者入力の 3 箇所だけ `rootPostByNo`。`append_body` は内部 ID のまま。`grep` で確認し、テストで「返信が正しい親に付く」を確かめる |
| 返信を作る経路の取りこぼし（`reaction.go` を含め 3 経路）で、`ReplyNo == 0` の返信ができる | T014 で 3 経路を列挙。テストで、各経路の返信が `ReplyNo > 0` で、欠番なく続くことを確かめる |
| マージ時に、保存済みの Erika-K の記事（`BoardNo == 0`）が開けなくなる | 本仕様は既存の記事を扱わない。PR の説明に、マージ前後のリセットが必要なことを必ず書く（T023） |
| 金型を実装後に作り直してしまう | 金型のコミットを実装より前に置き、`git log` で確認する（tasks.md） |
