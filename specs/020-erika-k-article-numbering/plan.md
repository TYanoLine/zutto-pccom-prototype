# Plan: Erika-K の記事番号の板ごとの採番

対応する spec: `specs/020-erika-k-article-numbering/spec.md`

## 現状の分類

| 項目 | 現状（読んだコード） | 場所 |
|---|---|---|
| 投稿の内部 ID | 店舗全体で 1 つのカウンタ（`s.next++`） | `apps/server/internal/world/store.go` の `MemoryStore.AddPost`（L365 付近）、`worldpersist/store.go` の `AddPost`（L159 付近）、`worldrepo/repository.go` の `AddPost`（L243 付近） |
| 投稿の保存 | スナップショットに `next_post_id` と投稿の配列 | `apps/server/internal/world/development_snapshot.go`（L20、L71、L102〜124） |
| 投稿の型 | `world.Post`（`ID`、`BoardID`、`ParentID` など） | `apps/server/internal/world/models.go` L134〜149 |
| 一覧の番号 | `p.ID` を `%4d` で表示 | `apps/server/internal/hostprogram/erikak/runtime.go` の `renderBoardIndex`（L720 付近〜、L750 付近） |
| 投稿完了の番号 | `MSG No.%d を登録しました`、`p.ID` | `runtime.go` の `new_body`（L120〜137） |
| アペンド | `ParentID: root.ID` で `AddPost`（内部 ID を消費） | `runtime.go` の `append_body`（L155〜178） |
| 番号による指定 | `rootPost(id)` が `p.ID == id` で引く | `runtime.go` L1081〜1088。呼び出し 11 箇所（`grep -n "rootPost(" runtime.go`） |
| 生成された記事 | `BoardActivityState` から、番号の順に `catchUpInitial` で `AddPost` | `apps/server/internal/bbsengine/engine.go`（L205〜260 付近）。共有エンジン |
| 活動計画 | `TotalRoots` を持つ。返信の順番は `totalRoots-roots+1+i` | `apps/server/internal/worldrepo/board_activity.go`、`bbsengine/engine.go` |

**分類**: 採番の変更は、Erika-K の表示と指定にだけ影響させる。共有の `Post` と共有エンジンは、後方互換な任意フィールドと、
Erika-K から渡される番号だけを扱う。他のホストは番号を持たない（0）ままにする（spec FR-009）。

## 設計

### 1. 記事番号のフィールド

`world.Post` に、任意のフィールドを 1 つ足す。

```go
// BoardNo is the article number a host program shows within its board (spec 020).
// Zero means the host does not number posts this way. Replies never carry one.
BoardNo int64 `json:"board_no,omitempty"`
```

- 既存の JSON には無いので、既存データは `0` として読める。
- `omitempty` なので、他のホストの保存形式は変わらない。

### 2. 番号の割り当て（記事の作成時）

記事番号は、記事を作るときに決め、以後は変えない。割り当ては Erika-K の中で行う（共有の採番器は変えない）。

| 作られ方 | 割り当て |
|---|---|
| 生成された記事（`catchUpInitial`） | `BoardActivityState.TotalRoots - roots + 1 + i`。返信の順番（`ThreadReplyCount` の ordinal）と同じ値 |
| 人間の記事（`new_body`） | `max(TotalRoots, その板の既存の BoardNo の最大) + 1`（spec 未決事項 2 の提案） |
| 人間のアペンド（`append_body`） | 割り当てない（0）。`ParentID` で親に結びつける |
| 生成された返信 | 割り当てない（0） |

- 「その板の既存の BoardNo の最大」は、`ListPosts(host)` を板と `ParentID == 0` で絞って求める。
  削除は無いので、最大 + 1 は欠番を作らない。削除の操作を入れるときは、番号を消さずに残すこと（spec FR-004）。
- 生成された記事への割り当ては、共有エンジンに「番号を付ける関数」を渡す形にする。
  Erika-K 以外のホストは、関数を渡さず、番号は 0 のままにする。

### 3. 番号による指定

- `rootPost(id)` を `rootPostByNo(boardPath, no)` に変える（板と記事番号で引く）。
- `BR n` と `A n`（`handleBoard`、`handleBoard` の `A ` 形式）の `n` は、板の記事番号として解決する。
- 解決できない場合は、既存の文言（`そのMSGはありません。`）のまま。

### 4. 表示

- 一覧の番号列は `BoardNo`。`%4d` の幅は変えない。
- `MSG No.%d を登録しました` と `MSG No.%d へアペンドします` の番号は、`BoardNo`（アペンドは親の `BoardNo`）。
- `ap/ref` 列の数え方（`appendCountFrom`）は変えない。

### 5. 保存データの既存の記事（spec 未決事項 1）

提案どおりに進める場合:

- 読み込み後、`BoardNo == 0` の記事（`ParentID == 0` のもの）に、板ごとに内部 ID の順で番号を補う。
  生成された記事（計画上の番号を持つ）が先にある板では、計画上の `TotalRoots` の後ろから続ける。
- 補った番号は、次の保存で書き出される。
- この変更の影響は、既存の HAKATA の番号が一度だけ変わること。Erika-K 以外には影響しない。

この方針は仕様の判断事項である。実装前に確認する。確認されなければ、補完は行わず、番号が 0 の記事は
一覧で内部 ID を表示するが、それは仕様の正しい表示ではないことを PR に書く。

## 変更するファイル

| ファイル | 変更 |
|---|---|
| `apps/server/internal/world/models.go` | `Post.BoardNo` を追加（任意、`omitempty`） |
| `apps/server/internal/hostprogram/erikak/runtime.go` | 番号の割り当て（人間の記事）、`rootPostByNo`、一覧と `MSG No.` の表示、`BR n` と `A n` の解決 |
| `apps/server/internal/hostprogram/erikak/numbering.go`（新規） | 板ごとの次の番号、番号の補完（読み込み後）、既存の記事の判定 |
| `apps/server/internal/bbsengine/engine.go` | 生成された記事に番号を付ける関数を、任意で受け取る（既定は何もしない） |
| `apps/server/internal/hostprogram/erikak/golden_test.go` | 金型テストの追加（既存の金型は変えない） |
| `apps/server/internal/hostprogram/erikak/numbering_test.go`（新規） | US1〜US3 のテスト |
| `apps/server/internal/hostprogram/erikak/testdata/*.json`（新規の金型） | 変更前に生成する |
| `specs/020-erika-k-article-numbering/verification.md`（新規） | 実装した人が書く記録 |
| `docs/host-programs/erika-k.md` | 仕様の節に、実装済みの状態と「保存データの扱い」の結果を追記 |

## 金型の方針

- 変更前のコードで、次を金型として記録する（実装より前にコミット）。
  - HAKATA の板の一覧（全板。`renderBoardIndex` の出力）
  - 数件の記事を書き込んだあとの、板の一覧と記事の表示
  - `BR n` と `A n` の応答（既存の番号で）
- 変更後、金型との差分は **番号の列のみ** であることを、テストで確認する。
  具体的には、差分の各行について、一覧の番号の欄だけが変わっていることを確かめる。
  番号以外の欄（日付、投稿者、件名、`ap/ref`、本文、罫線）は 1 バイトも変わらない。
- 金型ファイルは、変更後に書き換えない（AGENTS.md の方針）。

## 検証

- 金型テスト（上記、差分は番号列のみ）。
- US1〜US3 の Independent Test（`numbering_test.go`）。
- `go -C apps/server build ./...`、`go -C apps/server vet ./...`、`go -C apps/server test ./...`。
- 手動確認は、局の画面で板の一覧を開き、番号が板ごとに続くことを確かめる（実施できれば）。

## リスクと対策

| リスク | 対策 |
|---|---|
| 生成された記事の番号と、人間の記事の番号が重なる | 人間の記事は `TotalRoots` より後ろから始める（上の表）。テストで確認 |
| 共有エンジンの変更で、他のホストの番号や順序が変わる | 番号を付ける関数は任意。Erika-K 以外には渡さない。他のホストのテストを全件通す |
| 既存の HAKATA の番号が変わる | 未決事項 1 で確認してから実装する |
| `rootPost` の呼び出し 11 箇所で、番号と内部 ID を取り違える | 呼び出しをすべて読み、`rootPostByNo` に置き換える。`grep` で残りを確認する |
| 金型を実装後に作り直してしまう | 金型のコミットを実装より前に置き、`git log` で確認する（tasks.md） |
