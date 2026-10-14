# Plan: Erika-K の記事番号と返信番号の採番

対応する spec: `specs/020-erika-k-article-numbering/spec.md`

## 現状の分類

| 項目 | 現状（読んだコード） | 場所 |
|---|---|---|
| 投稿の内部 ID | 店舗全体で 1 つのカウンタ（`s.next++`） | `apps/server/internal/world/store.go` の `MemoryStore.AddPost`（L365 付近）、`worldpersist/store.go` の `AddPost`（L159 付近）、`worldrepo/repository.go` の `AddPost`（L243 付近） |
| 保存 | スナップショットに `next_post_id` と投稿の配列 | `apps/server/internal/world/development_snapshot.go`（L20、L71、L102〜124） |
| 投稿の型 | `world.Post`（`ID`、`BoardID`、`ParentID` など） | `apps/server/internal/world/models.go` L134〜149 |
| 任意の機能の受け渡し | `HostDetailStore` のように、任意のインターフェースを型アサーションで使う | `apps/server/internal/world/store.go` L18〜20、L22〜 |
| 一覧の番号 | `p.ID` を `%4d` で表示 | `apps/server/internal/hostprogram/erikak/runtime.go` の `renderBoardIndex`（L720 付近〜、L750 付近） |
| 投稿完了の番号 | `MSG No.%d を登録しました`、`p.ID` | `runtime.go` の `new_body`（L120〜137） |
| アペンド | `ParentID: root.ID` で `AddPost`（内部 ID を消費） | `runtime.go` の `append_body`（L155〜178） |
| 番号による指定 | `rootPost(id)` が `p.ID == id` で引く | `runtime.go` L1081〜1088。呼び出し 11 箇所 |
| 生成された記事 | 共有エンジン `bbsengine` が `AddPost` する | `apps/server/internal/bbsengine/engine.go`（`catchUpInitial`、L205〜260 付近）。共有 |
| 既存のリセット | debug 用。シードと利用者の投稿を残す | `apps/server/internal/worldrepo/bbs_article_reset.go`（`ResetBBSGeneratedArticles`） |

**分類**: 採番は Erika-K の表示と指定にだけ使う。共有の `Post` と共有エンジンは、後方互換な任意フィールドと、
Erika-K から渡される採番の関数だけを扱う。他のホストは番号を持たない（0）ままにする（spec FR-009）。

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

既存の JSON には無いので、既存データは 0 として読める。`omitempty` なので、他のホストの保存形式は変わらない。

### 2. 採番器（保存する、残存記事から計算しない）

採番器の値は、残っている記事から計算せず、**発行の履歴として保存する**。

- `world.Store` の任意機能として `BoardNumberStore` を定義する（`HostDetailStore` と同じ形）。

  ```go
  // BoardNumberStore issues per-board article numbers and per-parent reply numbers
  // (spec 020). Issued values are never decreased, even when posts are removed.
  type BoardNumberStore interface {
      NextBoardNo(hostID, boardID string) int64
      NextReplyNo(hostID string, parentID int64) int64
  }
  ```

- 実装: `MemoryStore`（`world/store.go`）と `worldpersist.Store`（`worldpersist/store.go`）。
  - 保存キーは `hostID|boardID`（記事番号）と `hostID|parentID`（返信番号）。値は「最後に発行した番号」。
  - スナップショットに `board_numbers`（`map[string]int64`）として書き出す（`development_snapshot.go`）。
  - 読み込み時、この項目が無い保存データは空として読む（0 から始まる）。
- 採番は `NextBoardNo` が「+1 して返す」形にする。ロックは既存の `s.mu` を使う。
- 記事の作成（`AddPost`）とは別の操作にする。記事を作る側が、採番してから `BoardNo` を入れて `AddPost` する。

### 3. 記事番号と返信番号の割り当て

| 作られ方 | 割り当て |
|---|---|
| 利用者の記事（`new_body`） | `BoardNo = NextBoardNo(host, board)` |
| ペルソナの記事・生成された記事（共有エンジン） | 共有エンジンに採番の関数を任意で渡す。Erika-K だけが `NextBoardNo` を渡す。既定は何もしない |
| 利用者の返信（`append_body`） | `ReplyNo = NextReplyNo(host, parentID)`。`ParentID: root.ID` |
| ペルソナの返信・生成された返信 | 同じく `NextReplyNo`。共有エンジンに関数を任意で渡す |

- 採番器の値は、記事（返信）が作られた時点で 1 度だけ進む。作成に失敗したら、採番は進めない
  （採番と `AddPost` の順序は、実装時に失敗の扱いを揃える。PR に書く）。

### 4. 番号による指定

- `rootPost(id)` を `rootPostByNo(boardPath, no)` に変える（`ParentID == 0`、`BoardID == boardPath`、`BoardNo == no`）。
- 返信の指定は、親の記事番号と返信番号（構文は spec 未決事項 1、提案 `/`）。`rootPostByNo` で親を引き、その子で `ReplyNo` を引く。
- `BR n` と `A n` の `n` は、板の記事番号として解決する。
- 解決できない場合は、既存の文言（`そのMSGはありません。`）のまま。

### 5. 表示

- 一覧の番号列は `BoardNo`。`%4d` の幅は変えない。
- `MSG No.%d を登録しました` は `BoardNo`。
- アペンドの見出しは、`アペ n`（`ReplyNo`）。現状の `appendNo` の連番を、`ReplyNo` に置き換える。
- `ap/ref` 列の数え方（`appendCountFrom`）は変えない。

### 6. 既存の記事のリセット（spec 判断事項 1、未決事項 2）

- 対象: Erika-K の局の記事（保存データのうち、その局の `Post`）。他のホスト、局の定義、利用者の情報には触れない。
- 採番器（`board_numbers`）は、リセット後 0 から始める。
- 既存の debug 用のリセット（`ResetBBSGeneratedArticles`）は使わない。シードと利用者の投稿を残すため、目的が違う。
- 方法と適用の契機は、未決事項 2 の確認後に決める。提案: 保存データのバージョンを上げ、読み込み時に Erika-K の記事を消す一度きりの処理。

### 7. 遅延生成の帰結

板を初めて開くと過去の記事が作られる（遅延生成）。判断事項 2 により、その記事の番号は、作られた時点の採番器の続きになる。
日付の順と番号の順が一致しない板が生じうる。spec の「注意」に記載のとおり、これは仕様の帰結であり、実装では直さない。

## 変更するファイル

| ファイル | 変更 |
|---|---|
| `apps/server/internal/world/models.go` | `Post.BoardNo`、`Post.ReplyNo` を追加（任意、`omitempty`） |
| `apps/server/internal/world/store.go` | `BoardNumberStore` インターフェース、`MemoryStore` の実装 |
| `apps/server/internal/worldpersist/store.go` | `BoardNumberStore` の実装（保存と読み込み） |
| `apps/server/internal/world/development_snapshot.go` | `board_numbers` の保存と読み込み |
| `apps/server/internal/hostprogram/erikak/runtime.go` | 利用者の記事と返信の採番、`rootPostByNo`、一覧と `MSG No.` の表示、`アペ n`、`BR n` と `A n` の解決、返信の指定 |
| `apps/server/internal/hostprogram/erikak/numbering.go`（新規） | 採番の呼び出しをまとめる関数、返信の指定の解析 |
| `apps/server/internal/bbsengine/engine.go` | 生成された記事と返信に採番の関数を任意で渡す（既定は何もしない） |
| `apps/server/internal/hostprogram/erikak/golden_numbering_test.go`（新規） | 金型テスト |
| `apps/server/internal/hostprogram/erikak/numbering_test.go`（新規） | US1〜US5 のテスト |
| `apps/server/internal/hostprogram/erikak/testdata/numbering_golden.json`（新規） | 変更前に生成する |
| 既存のリセットの処理（新規） | 未決事項 2 の確認後に決める |
| `specs/020-erika-k-article-numbering/verification.md`（新規） | 実装した人が書く記録 |
| `docs/host-programs/erika-k.md` | 仕様の節を、判断事項の結果に合わせて更新 |

## 金型の方針

- 変更前のコードで、HAKATA の板の一覧、数件の記事と返信の表示、`BR n` と `A n` の応答を金型として記録する（実装より前にコミット）。
- 変更後、金型との差分は、番号の列（記事番号、アペンドの見出し）と、リセットによる内容だけであることを、テストで確認する。
- 金型ファイルは、変更後に書き換えない（AGENTS.md の方針）。

## 検証

- 金型テスト（上記、差分は番号の列とリセットの内容だけ）。
- US1〜US5 の Independent Test（`numbering_test.go`）。
- `go -C apps/server build ./...`、`go -C apps/server vet ./...`、`go -C apps/server test ./...`。
- 手動確認は、局の画面で板の一覧を開き、番号が板ごとに続くこと、返信の番号が親の中で続くことを確かめる（実施できれば）。

## リスクと対策

| リスク | 対策 |
|---|---|
| 採番器の値が、保存されずに再起動で戻る | 採番器を snapshot に保存し、読み込みのテストで確認（US4 の前提） |
| 採番と `AddPost` の間で失敗し、番号が欠ける | 採番は記事が作られた後の 1 度だけ。失敗時の扱いを PR に書く。欠番は仕様上許容される |
| 共有エンジンの変更で、他のホストの番号や順序が変わる | 採番の関数は任意。Erika-K 以外には渡さない。他のホストのテストを全件通す |
| `rootPost` の呼び出し 11 箇所で、番号と内部 ID を取り違える | 呼び出しをすべて読み、`rootPostByNo` に置き換える。`grep` で残りを確認する |
| リセットで、Erika-K 以外の記事を消す | 対象の判定を Erika-K の局に限り、テストで他の局の記事が残ることを確認する |
| 金型を実装後に作り直してしまう | 金型のコミットを実装より前に置き、`git log` で確認する（tasks.md） |
