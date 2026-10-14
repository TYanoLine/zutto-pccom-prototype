# Tasks: Erika-K の記事番号の板ごとの採番

**Input**: `specs/020-erika-k-article-numbering/` の `spec.md`、`plan.md`
**Prerequisites**: `AGENTS.md`、`plan.md`（特に「設計」の 2 番目の表と「金型の方針」）、spec の「未決事項」

## この作業のルール（必読）

- 作業ブランチから `main` への PR を 1 本作る。`main` に直接コミットしない。
- **成果物は Go のコードの変更（実装）と、その金型。** spec のファイルを作る・変更することは、この作業の仕事ではない。
- **HAKATA の板の一覧と記事の表示は、番号の列以外 1 バイトも変えない。** これが合格条件。金型テストで確認する。
- **金型は、変更前のコードで生成し、実装より前にコミットする。** 実装の後で生成してはならない（同義反復になり、根拠にならない）。
  コミットの順序が、そのまま証拠になる。
- **金型をコミットしたあと、金型の更新用のフラグを二度と使わない。** 金型ファイルを書き換えない。
  金型テストが失敗したら、実装の誤り。金型ではなく実装を直す。2 回直して通らなければ、そこで止めて、どの行が違うかを報告する。
- **spec の「未決事項」の 1 が未確定のまま、既存の記事の番号を補完する実装をしてはならない。**
  確定していなければ、T010 の補完は行わず、PR の説明に止めた理由を書く。
- スコープ外（`spec.md` の「スコープ外」）に手を出さない。特に、記事の削除の操作と、1 行入力・複数行入力の変更。
- 既存ファイルに整形だけの差分を作らない。`gofmt -w` を既存ファイル全体にかけない。
- 1 タスク = 1 コミット（メッセージは `T0XX: 内容`）。途中でビルドが通らないタスクは、1 つのコミットにまとめる。
- 実行していないコマンドを「成功した」と書かない。実行できなかったものは「未実施」と理由を書く。
- この作業で `.github/` 配下のファイルは変更しない。
- spec と実際のコードが食い違っていたら、推測で進めず、PR の説明に書いて止まる。

---

## Phase 0: 金型（変更前のコードで生成する）

- [ ] **T001** 金型のテストを書く（`apps/server/internal/hostprogram/erikak/golden_numbering_test.go`、新規）

  - HAKATA の局の定義（`hakata-canal-net`）で Erika-K の `Runtime` を作る。既存の `golden_test.go` の作り方に合わせる。
  - 次の操作をスクリプトにして、各画面の出力を、金型ファイル（`testdata/numbering_golden.json`）に保存する。
    1. 板 A（板の一覧の最初の板）に記事を 2 件書く。
    2. 板 A の記事に、アペンドを 2 件書く。
    3. 板 B（別の板）に記事を 1 件書く。
    4. 板 A の一覧（`BX`）を開く。
    5. 板 A の記事を `BR` で読む。
    6. HAKATA の板の一覧（全板。`BM` と各板の `BX`）を開く。
  - 金型を読むのは、1 つの関数（`goldenNumberingTranscript`）だけにする。実装後は、その関数の本体だけを書き換えられるようにする。
  - 注意: 記事を書くときの時刻は、既存の金型と同じ固定の世界時刻を使う（出力が決定的であること）。

- [ ] **T002** 生成の前に、確認用のテストが現在のコードで通ることを確認する

  ```bash
  go -C apps/server test ./internal/hostprogram/erikak -run 'TestGolden' -count=1
  ```

  既存の金型テストが通らない場合は、既存のコードは直さず、PR の説明に書いて止まる。

- [ ] **T003** 金型を生成してコミットする（**変更前のコードで。実装より前に**）

  ```bash
  go -C apps/server test ./internal/hostprogram/erikak -run 'TestGoldenNumbering' -update-golden -count=1
  go -C apps/server test ./internal/hostprogram/erikak -count=1
  go -C apps/server test ./internal/hostprogram/erikak -run 'TestGoldenNumbering' -count=3
  ```

  - 生成した金型ファイルと T001 のテストを、1 つのコミットにする。このコミットが、T004 以降のどのコミットよりも前になること。
  - 3 回連続で通ること（出力が決定的であること）を確認する。

**Checkpoint**: 変更前のコードに対する金型がコミットされ、通っている。ここから先、金型ファイルは変更しない。

---

## Phase 1: 記事番号のフィールドと、番号の割り当て

- [ ] **T004** `world.Post` に `BoardNo` を追加する（`apps/server/internal/world/models.go`、L134〜149）

  `ID` の次の行に、次を挿入する（`AuthorHumanID` の前、コメントは spec 020 を参照）。

  ```go
  // BoardNo is the article number a host program shows within its board (spec 020).
  // Zero means the host does not number posts this way. Replies never carry one.
  BoardNo int64 `json:"board_no,omitempty"`
  ```

  既存の JSON 形式は変わらない（`omitempty`）。

- [ ] **T005** 板ごとの次の番号を求める関数を書く（`apps/server/internal/hostprogram/erikak/numbering.go`、新規）

  - `nextBoardNo(posts []world.Post, boardPath string, totalRoots int) int64`
    - 板の `ParentID == 0` かつ `BoardNo > 0` の記事の最大値と `totalRoots` の大きいほう、に 1 を足す。
  - テーブル駆動のテストを `numbering_test.go` に書く（T008 でまとめて実行する）。

- [ ] **T006** 人間の記事に番号を付ける（`runtime.go` の `new_body`、L120〜137）

  `AddPost` の前に `BoardNo: nextBoardNo(r.Store.ListPosts(r.Host.ID), r.boardPath, r.boardTotalRoots())` を入れる。
  `boardTotalRoots()` は、板の活動計画（`BoardActivity`）の `TotalRoots` を返す（板が無い、または計画が無いときは 0）。
  - 投稿完了の文言 `MSG No.%d を登録しました` の `%d` は、`p.ID` ではなく `p.BoardNo` にする。

- [ ] **T007** アペンドには番号を付けない（`runtime.go` の `append_body`、L155〜178）

  - `BoardNo` は 0 のまま（変更なし）。`ParentID: root.ID` は変えない。
  - 表示の番号（`MSG No.%d へアペンドします`、L153 付近）は、親の `BoardNo` にする。

**Checkpoint**: 人間の記事とアペンドの番号の割り当てがコードにあり、まだ表示と指定は変えていない。

---

## Phase 2: 生成された記事の番号（共有エンジン）

- [ ] **T008** 生成された記事に番号を付ける（`apps/server/internal/bbsengine/engine.go`、`catchUpInitial` 周辺）

  - 共有エンジンに、任意の関数（`BoardNoFunc func(root int) int64` のような形）を渡せるようにする。既定は `nil`（番号を付けない）。
  - Erika-K だけが、ordinal `totalRoots-roots+1+i` の値を返す関数を渡す（返信の順番と一致させる）。
  - 他のホスト（TurboBBS、VS、bbs など）は関数を渡さない。挙動は変えない。

  テスト: 生成された記事の番号が、`TotalRoots` からの連番になること。返信の数の計算（`ThreadReplyCount`）の順番と一致すること。

- [ ] **T009** 確認

  ```bash
  go -C apps/server build ./... && go -C apps/server test ./internal/bbsengine/... ./internal/hostprogram/... -count=1
  ```

  共有エンジンの既存のテストが、すべて通ること。通らなければ、番号の関数を渡していないホストの挙動が変わっている。実装を直す。

**Checkpoint**: 生成された記事と、人間の記事の番号が、同じ板の中で連続する（テストで確認）。

---

## Phase 3: 表示と番号による指定

- [ ] **T010** 保存データの既存の記事（spec 未決事項 1）

  - **先に、spec の未決事項 1 について確認する。** 確認されていなければ、この T010 は行わない。PR の説明に「未確認のため補完は未実施」と書く。
  - 確認された場合: 読み込み後、`BoardNo == 0` の根（`ParentID == 0`）に、板ごとに内部 ID の順で番号を補う（生成された記事の計画上の番号の後ろから続ける）。
    `numbering.go` の `backfillBoardNo(posts, boards)` として書き、テストを付ける。

- [ ] **T011** 一覧の番号列を記事番号にする（`runtime.go` の `renderBoardIndex`、L720〜750 付近）

  - `fmt.Fprintf(&b, "%02d %4d %s %s %s %s\r\n", keyInt, p.ID, ...)` の `p.ID` を `p.BoardNo` にする。
  - 幅（`%4d`）と並び順（`roots` を後ろから 10 件）は変えない。
  - 一覧に出す記事は、`BoardNo` が 0 のものを含めない（補完されていない記事）。ただし T010 を行わなかった場合は、理由を PR に書いて、一覧の挙動を決める。

- [ ] **T012** 番号による指定を、板と記事番号で解決する（`runtime.go`、`rootPost` の呼び出し 11 箇所）

  - `rootPost(id int64)` を `rootPostByNo(boardPath string, no int64) (world.Post, bool)` に置き換える（`ParentID == 0`、`BoardID == boardPath`、`BoardNo == no`）。
  - `grep -n "rootPost(" apps/server/internal/hostprogram/erikak/runtime.go` の結果を、すべて読んで置き換える。
  - `handleBoard` の `BR n`、`A n` と、`handleThread` の `BR`（`r.threadID` を使う箇所）は、`threadID` を内部 ID のまま使ってよい。ただし、利用者の入力の番号は記事番号で解決する。

- [ ] **T013** テスト（US1〜US3）

  `numbering_test.go` に、次を書く。

  | テスト | 内容 |
  |---|---|
  | US1 | 2 つの板に記事を書き、それぞれの一覧の番号が板ごとに連続している |
  | US2 | 板 A の番号 5 を `BR 5` で開くと板 A の記事 5 が出る。板 B に同じ番号があっても、板 A の一覧から開いたときは板 A の記事を指す |
  | US3 | 板に記事 1 を書き、アペンドを 3 件書き、次の記事を書く。次の記事の番号は 2 |
  | 生成された記事 | 生成された記事と人間の記事の番号が、重ならず連続する |
  | 削除なし | 削除の操作は無いので、番号の再利用の検査は、採番関数の単体テスト（最大値 + 1）で確認する |

- [ ] **T014** 確認

  ```bash
  go -C apps/server build ./... && go -C apps/server vet ./... && go -C apps/server test ./internal/hostprogram/erikak/... -count=1
  ```

  - 金型テストは、**差分が番号の列だけであること**を確認する（`TestGoldenNumbering` の差分確認）。金型ファイルは変えない。
  - 差分が番号以外の欄に出たら、実装を直す。金型は変えない。

**Checkpoint**: US1〜US3 が通り、金型の差分は番号の列だけ。

---

## Phase 4: 全体の確認とドキュメント

- [ ] **T015** 他のホストを含む全体の確認

  ```bash
  go -C apps/server build ./...
  go -C apps/server vet ./...
  go -C apps/server test ./... -count=1
  git diff --stat <金型のコミット>..HEAD -- apps/server/internal/hostprogram/erikak/testdata
  git log --oneline -- apps/server/internal/hostprogram/erikak/testdata
  ```

  - 金型ファイルの `git diff --stat` が空であること。`git log` で、金型のコミットが 1 つだけであること。

- [ ] **T016** ドキュメントを更新する

  - `docs/host-programs/erika-k.md` の「Article numbering and append rules」に、実装した範囲と、補完（未決事項 1）の結果を追記する。
  - `apps/server/internal/hostcatalog/README.md` は変えない（局の定義の書式は変わらない）。
  - `specs/README.md` の表に、`020 | erika-k-article-numbering（...）` の行を足す。

- [ ] **T017** 結果を `specs/020-erika-k-article-numbering/verification.md`（新規）に記録する

  ```markdown
  # Verification: Erika-K の記事番号の板ごとの採番

  | コマンド | 結果 | 備考 |
  |---|---|---|
  | `go -C apps/server build ./...` | 成功 / 失敗 / 未実施 | |
  | `go -C apps/server vet ./...` | 〃 | |
  | `go -C apps/server test ./...` | 〃 | |
  | 金型テスト（3 回連続） | 〃 | |

  ## 金型が、変更前のコードで生成されたこと
  （金型のコミットの SHA と、実装のコミットの SHA。金型が先であること。`git log --oneline -- testdata` の結果）

  ## 金型の差分
  （差分が番号の列だけであることの確認。`git diff <金型のコミット>..HEAD -- testdata` が空であること）

  ## 未決事項の結果
  - 未決事項 1（既存の記事の補完）: 実施 / 未実施（理由）
  - 未決事項 2（境界の扱い）: 提案どおり / 変更（理由）
  - 未決事項 3（返信の番号）: 提案どおり / 変更（理由）

  ## 手動確認
  - 局の画面で板の一覧を開き、番号が板ごとに続くこと: 確認済み / 未実施
  ```

- [ ] **T018** PR を作る

  - `main` への PR。**draft** で作成する。
  - タイトル: `Erika-K の記事番号を板ごとの連番にし、アペンドには記事番号を振らない（金型を変えず番号の列のみ）`
  - 説明に含める: 目的、変更の要約、**金型のコミットが実装より前であること**、`verification.md` の結果（未実施を含めて正直に）、未決事項の扱い、スコープ外の項目。

---

## 実行順序

```text
Phase 0 (T001 → T002 → T003)   ← 金型は実装より前にコミット
  └─ Phase 1 (T004 → T005 → T006 → T007)
       └─ Phase 2 (T008 → T009)
            └─ Phase 3 (T010 → T011 → T012 → T013 → T014)
                 └─ Phase 4 (T015 → T016 → T017 → T018)
```

## 完了の定義

- `spec.md` の FR と SC を満たす（未決事項の扱いは PR に明記する）。
- 金型テストが、変更前に生成した金型ファイルを変更せずに通る。差分は番号の列のみ。金型は実装より前のコミットにある。
- 他のホストのテストが、すべて通る（`go -C apps/server test ./...`）。
- 変更が `plan.md` の「変更するファイル」の表の範囲に収まっている。
