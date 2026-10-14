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
| 番号による指定 | `rootPost(id)` が `p.ID == id` で引く | `runtime.go` L1081〜1088。呼び出し 11 箇所 |
| 活動計画 | `planBoardActivity(host, board, now)` が、呼び出し時刻から `TotalRoots` などを計算する。時間とともに変わる | `apps/server/internal/worldrepo/board_activity.go`（L47〜157） |
| 生成された記事 | 共有エンジン `bbsengine` が、活動計画の番号（ordinal）で `AddPost` する | `apps/server/internal/bbsengine/engine.go`（`catchUpInitial` 付近、L205〜260） |
| 板の開始日 | `board.OpenedOn`、無ければ `host.FoundedOn` | `board_activity.go` L48〜52 |

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

### 2. 板の計画（確定して保存する）

板ごとに、一度だけ確定する計画を持つ。

```go
// BoardPlan is fixed once per board (spec 020, FR-002). It is never recomputed
// from the current world time after it is stored.
type BoardPlan struct {
    HostID   string
    BoardID  string
    OpenedOn time.Time  // first post date
    Total    int64      // N: the largest article number in the plan
    Windows  []PlanWindow // one per article number 1..N, in date order
    Dates    []time.Time  // decided per article by the LLM inside its window
}
type PlanWindow struct {
    Earliest time.Time // not before the previous article's date
    Latest   time.Time
}
```

- **N と最初の投稿日**: 既存の活動計画の式（`planBoardActivity` の `totalRoots` と `OpenedOn`）から算出する。
  算出の時刻は、確定の時刻（spec 未決事項 1）とする。確定の後は再計算しない。
- **窓**: 板の累積の活動量の曲線（会員数の積分 × 人気度 × 板の活動量）を、N 等分した区間にする。
  記事 k の窓は、累積の活動量が `(k-1)/N` から `k/N` までに対応する時刻の範囲とする。
  これにより、記事の間隔は板の活動の大きさと矛盾しない。
- **日付の決定**: LLM は、各記事の窓の中で、前の記事の日付より後の具体的な日付を決める。
  LLM が返した日付は、窓と単調性を検証してから保存する。検証に失敗したら、窓の中央値を使う（決定的な代替）。
- **保存**: `BoardPlan` は、snapshot（`development_snapshot.go`）に板ごとの項目として保存する。
  キーは `hostID|boardID`。読み込み後は、保存された値をそのまま使う。

### 3. 採番器（保存する、残存記事から計算しない）

- `world.Store` の任意機能として `BoardNumberStore` を定義する（`HostDetailStore` と同じ形）。

  ```go
  // BoardNumberStore issues Erika-K numbers (spec 020). Issued values are never
  // decreased, even when posts are removed.
  type BoardNumberStore interface {
      BoardPlan(hostID, boardID string) (BoardPlan, bool)
      SetBoardPlan(plan BoardPlan) error  // only when absent: plans are fixed once
      NextBoardNo(hostID, boardID string) int64
      NextReplyNo(hostID string, parentID int64) int64
  }
  ```

- 実装: `MemoryStore`（`world/store.go`）と `worldpersist.Store`（`worldpersist/store.go`）。
  - 板の採番器は「最後に発行した番号」。計画を確定したとき、その値を N にする（計画の記事 1〜N を発行済みにする）。
  - 返信の採番器は、親の内部 ID をキーにした「最後に発行した返信番号」。
  - 両方を snapshot に `board_plans`、`board_numbers`、`reply_numbers` として書き出す。`omitempty`。
- `SetBoardPlan` は、既に計画がある板には書かない（固定）。

### 4. 記事番号と返信番号の割り当て

| 作られ方 | 割り当て |
|---|---|
| 計画の記事 k（1〜N、計画の確定時に作られる） | `BoardNo = k`、日付は `Dates[k-1]` |
| 計画の後の利用者の記事・ペルソナの記事 | `BoardNo = NextBoardNo(host, board)` |
| 利用者の返信（`append_body`） | `ReplyNo = NextReplyNo(host, root.ID)`。`ParentID: root.ID` |
| 生成された返信（共有エンジン） | 同じく `NextReplyNo` |

- 計画の記事は、板を開くときに、窓の中の日付を付けて `AddPost` で作る。番号は日付順なので、遅延して見えても食い違わない。
- 計画の記事の作成と計画の保存は、同じ時点で行う（片方だけ保存されない）。

### 5. 番号による指定と表示

- `rootPost(id)` を `rootPostByNo(boardPath, no)` に変える（`ParentID == 0`、`BoardID == boardPath`、`BoardNo == no`）。
- `BR n` と `A n` の `n` は、板の記事番号として解決する。返信番号による指定はしない（spec FR-007）。
- 一覧の番号列は `BoardNo`。`%4d` の幅は変えない。
- `MSG No.%d を登録しました` は `BoardNo`。
- アペンドの見出しは `アペ N`（`ReplyNo`）。現状の連番（`appendNo`）を `ReplyNo` に置き換える。
- `ap/ref` 列の数え方（`appendCountFrom`）は変えない。

### 6. 既存の記事（本仕様の範囲外）

既存の記事のリセットは、別の手段で行う。本仕様は、計画と採番器が空の状態から始まる板を前提にする。

### 7. 日付の検証

LLM が決めた日付は、次を満たすまで保存しない。

- `OpenedOn` 以上、確定時刻以下。
- 記事 k の窓の中。
- 記事 k-1 の日付より後（単調増加）。

満たさない場合の代替は、窓の中央値（決定的）。代替したことは、テストで検出できるように記録する。

## 変更するファイル

| ファイル | 変更 |
|---|---|
| `apps/server/internal/world/models.go` | `Post.BoardNo`、`Post.ReplyNo`、`BoardPlan`、`PlanWindow` を追加 |
| `apps/server/internal/world/store.go` | `BoardNumberStore` インターフェース、`MemoryStore` の実装 |
| `apps/server/internal/worldpersist/store.go` | `BoardNumberStore` の実装（保存と読み込み） |
| `apps/server/internal/world/development_snapshot.go` | `board_plans`、`board_numbers`、`reply_numbers` の保存と読み込み |
| `apps/server/internal/worldrepo/board_activity.go` | 計画の算出（N と `OpenedOn`、窓）。確定時刻を引数で受け取る形にする |
| `apps/server/internal/hostprogram/erikak/runtime.go` | 利用者の記事と返信の採番、`rootPostByNo`、一覧と `MSG No.` の表示、`アペ N`、`BR n` と `A n` の解決 |
| `apps/server/internal/hostprogram/erikak/numbering.go`（新規） | 採番の呼び出し、計画の日付の検証、窓の中央値の代替 |
| `apps/server/internal/bbsengine/engine.go` | 生成された記事に、計画の番号と日付を任意で渡す（既定は何もしない） |
| `apps/server/internal/hostprogram/erikak/golden_numbering_test.go`（新規） | 金型テスト（画面の形式） |
| `apps/server/internal/hostprogram/erikak/numbering_test.go`（新規） | US1〜US6 のテスト |
| `apps/server/internal/hostprogram/erikak/testdata/numbering_golden.json`（新規） | 変更前に生成する |
| `specs/020-erika-k-article-numbering/verification.md`（新規） | 実装した人が書く記録 |
| `docs/host-programs/erika-k.md` | 仕様の節を、判断事項の結果に合わせて更新 |

## 金型の方針

- 変更前のコードで、HAKATA の画面の形式（板の一覧の罫線と欄、記事と返信の表示の形、`BR n` と `A n` の応答の形）を金型として記録する（実装より前にコミット）。
- 金型は、番号と、計画による日付・件数を含めない形（マスクした形）で比べる。
  比べる対象は、罫線、欄の幅、文言、コマンドの応答の形。
- 番号と日付・件数は、新しい計画で変わることを許容する。その代わり、US3（単調性）と US4（固定）をテストで確認する。
- 金型ファイルは、変更後に書き換えない（AGENTS.md の方針）。

## 検証

- 金型テスト（画面の形式のみ。差分が出ないこと）。
- US1〜US6 の Independent Test（`numbering_test.go`）。
- `go -C apps/server build ./...`、`go -C apps/server vet ./...`、`go -C apps/server test ./...`。
- 手動確認は、局の画面で板を初めて開き、番号の順と日付の順が一致することを確かめる（実施できれば）。

## リスクと対策

| リスク | 対策 |
|---|---|
| 計画が時間とともに変わる（現状の `planBoardActivity` は呼び出し時刻に依存） | 計画を一度確定して保存し、以後は読むだけにする。US4 のテスト |
| LLM の日付が窓や単調性を破る | 保存前に検証し、満たさなければ窓の中央値を使う。代替はテストで検出する |
| 計画の保存と記事の作成が片方だけ残る | 同じ時点で行い、失敗時は両方を行わない |
| 共有エンジンの変更で、他のホストの番号や日付が変わる | 任意の関数として渡す。Erika-K 以外には渡さない。他のホストのテストを全件通す |
| `rootPost` の呼び出し 11 箇所で、番号と内部 ID を取り違える | 呼び出しをすべて読み、`rootPostByNo` に置き換える。`grep` で残りを確認する |
| 金型を実装後に作り直してしまう | 金型のコミットを実装より前に置き、`git log` で確認する（tasks.md） |
