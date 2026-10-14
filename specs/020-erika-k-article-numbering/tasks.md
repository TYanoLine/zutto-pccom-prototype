# Tasks: Erika-K の板の計画と記事番号

**Input**: `specs/020-erika-k-article-numbering/` の `spec.md`、`plan.md`
**Prerequisites**: `AGENTS.md`、`plan.md`（特に「設計」と「金型の方針」）、spec の「判断事項」と「未決事項」

## この作業のルール（必読）

- 作業ブランチから `main` への PR を 1 本作る。`main` に直接コミットしない。
- **成果物は Go のコードの変更（実装）と、その金型。** spec のファイルを作る・変更することは、この作業の仕事ではない。
- **HAKATA の画面の形式（罫線、欄の幅、文言、コマンドの応答の形）は 1 バイトも変えない。** これが合格条件。
  番号と、板の計画による日付・件数は、マスクして比べる（`plan.md` の「金型の方針」）。
- **金型は、変更前のコードで生成し、実装より前にコミットする。** 実装の後で生成してはならない（同義反復になり、根拠にならない）。
  コミットの順序が、そのまま証拠になる。
- **金型をコミットしたあと、金型の更新用のフラグを二度と使わない。** 金型ファイルを書き換えない。
  金型テストが失敗したら、実装の誤り。金型ではなく実装を直す。2 回直して通らなければ、そこで止めて、どの行が違うかを報告する。
- 板の計画は、板一覧の表示時に、板ごとに初めて確定する（spec「計画の確定の時点」）。確定の後は再計算しない。
- **記事番号を、残っている記事から計算しない。** `max + 1` のような実装をしてはならない（spec FR-004）。
- **計画を確定したあと、N と最初の投稿日を再計算しない。** 保存された計画を読むだけにする（spec FR-002）。
- 返信番号による指定（`BR 5/2` などの構文）は実装しない（spec スコープ外）。
- スコープ外（`spec.md` の「スコープ外」）に手を出さない。特に、記事と返信の削除の操作と、既存の記事のリセット。
- 既存ファイルに整形だけの差分を作らない。`gofmt -w` を既存ファイル全体にかけない。
- 1 タスク = 1 コミット（メッセージは `T0XX: 内容`）。途中でビルドが通らないタスクは、1 つのコミットにまとめる。
- 実行していないコマンドを「成功した」と書かない。実行できなかったものは「未実施」と理由を書く。
- この作業で `.github/` 配下のファイルは変更しない。
- spec と実際のコードが食い違っていたら、推測で進めず、PR の説明に書いて止まる。

---

## Phase 0: 金型（変更前のコードで生成する）

- [ ] **T001** 金型のテストを書く（`apps/server/internal/hostprogram/erikak/golden_numbering_test.go`、新規）

  - HAKATA の局の定義（`hakata-canal-net`）で Erika-K の `Runtime` を作る。既存の `golden_test.go` の作り方に合わせる。
  - 次の操作をスクリプトにして、各画面の出力を金型ファイル（`testdata/numbering_golden.json`）に保存する。
    1. 板 A（板の一覧の最初の板）に記事を 2 件書く。
    2. 記事 1 に返信を 2 件書く。
    3. 板 B（別の板）に記事を 1 件書く。
    4. 板 A の一覧（`BX`）を開く。
    5. 記事 1 を `BR` で読む（返信を含む）。
    6. HAKATA の板の一覧（`BM` と各板の `BX`）を開く。
  - 金型は、番号と日付（数字の列）をマスクした形で保存する。マスクの関数は、テストの中に 1 つだけ書く（`maskNumbers`）。
  - 金型を読むのは、1 つの関数（`goldenNumberingTranscript`）だけにする。実装後は、その関数の本体だけを書き換えられるようにする。
  - 記事を書くときの時刻は、既存の金型と同じ固定の世界時刻を使う（出力が決定的であること）。

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

## Phase 1: 投稿のフィールドと、保存する板の計画・採番器

- [ ] **T004** `world.Post` に `BoardNo` と `ReplyNo` を追加する（`apps/server/internal/world/models.go`、L134〜149）

  `ID` の次の行に、`plan.md` の設計 1 のとおりに挿入する。既存の JSON 形式は変わらない（`omitempty`）。

- [ ] **T005** `BoardPlan`、`PlanWindow`、`BoardNumberStore` を定義し、メモリ上の実装を書く（`apps/server/internal/world/store.go`）

  - `plan.md` の設計 2・3 のとおり。`HostDetailStore`（L18〜20）と同じ形の任意インターフェースにする。
  - `SetBoardPlan` は、既に計画がある板には書かない（固定）。
  - テスト（`store_test.go` に追加）: 発行は 1 から続く。投稿を消しても発行の値は戻らない。計画は 2 回目の確定で変わらない。

- [ ] **T006** 計画と採番器を snapshot に保存する（`apps/server/internal/world/development_snapshot.go`）

  - `board_plans`、`board_numbers`、`reply_numbers` を追加する。`omitempty`。
  - 読み込み時、項目が無い保存データは空として読む。
  - テスト: 保存して読み直した後、計画と次の発行の値が変わらない。

- [ ] **T007** `worldpersist.Store` に同じ機能を実装する（`apps/server/internal/worldpersist/store.go`）

  - T005 と同じメソッドを、保存と読み込みに対応させる。
  - テスト: 読み直したあとも、計画と採番が続く。

**Checkpoint**: 計画と採番器が、メモリと保存の両方で、固定され、戻らないことを確認するテストが通る。

---

## Phase 2: 板の計画の算出と、日付の決定

- [ ] **T008** 板の計画を算出する（`apps/server/internal/worldrepo/board_activity.go`、`planBoardActivity` の周辺）

  - 板一覧の表示時刻を引数で受け取り、`N`（既存の `totalRoots` の式）と `OpenedOn` を返す。
  - 窓（`PlanWindow`）を、累積の活動量の曲線から、N 等分の区間として作る（`plan.md` の設計 2）。
  - テスト: 同じ入力で、同じ N、同じ `OpenedOn`、同じ窓が返ること。確定の後に時刻を変えても、保存された計画は変わらないこと。

- [ ] **T009** 日付の決定と検証（`apps/server/internal/hostprogram/erikak/numbering.go`、新規）

  - LLM に、窓の範囲と、前の記事の日付を渡して、各記事の日付を決めさせる。
  - 検証: `OpenedOn` 以上、表示時刻以下、窓の中、単調増加。満たさなければ窓の中央値を使う（代替したことを記録する）。
  - テスト: 検証の各条件で失敗するケース。代替が中央値になること。
  - LLM を使わない経路（テスト用の決定的な値）を用意し、テストは LLM に依存しない（AGENTS.md の「AI は任意」）。

**Checkpoint**: 板の計画が、固定された N と `OpenedOn`、窓の中の単調な日付を持つ。

---

## Phase 3: 計画の記事と、計画の後の記事

- [ ] **T010** 板を初めて必要としたときに、計画の記事を作る（`runtime.go` と `numbering.go`）

  - 計画の記事 k（1〜N）を、`BoardNo = k`、日付 `Dates[k-1]` で `AddPost` する。
  - 計画の保存と記事の作成は、同じ時点で行う（片方だけ残らない）。
  - テスト: 作られた記事の番号が 1〜N の順、日付が番号の順に単調増加。

- [ ] **T011** 利用者の記事に記事番号を付ける（`runtime.go`、`new_body`、L120〜137）

  - `AddPost` の前に `BoardNo: NextBoardNo(host, board)` を入れる。
  - 投稿完了の文言 `MSG No.%d を登録しました` の `%d` は `p.BoardNo` にする。

- [ ] **T012** 利用者の返信に返信番号を付ける（`runtime.go`、`append_body`、L155〜178）

  - `AddPost` の前に `ReplyNo: NextReplyNo(host, root.ID)` を入れる。`ParentID: root.ID` は変えない。
  - 見出し `アペ N` の番号を、現状の連番（`appendNo`）から `ReplyNo` にする。
  - 表示の `MSG No.%d へアペンドします`（L153 付近）は、親の `BoardNo` にする。

- [ ] **T013** 確認

  ```bash
  go -C apps/server build ./... && go -C apps/server test ./internal/hostprogram/erikak/... ./internal/world/... -count=1
  ```

  金型テストは、この時点で番号と日付の差分が出てよい。画面の形式（罫線、欄、文言）の差分が出ていたら、実装を直す。

**Checkpoint**: 計画の記事、利用者の記事、返信が、板ごと・親ごとの番号を持つ（テストで確認）。

---

## Phase 4: 共有エンジンと、番号による指定・表示

- [ ] **T014** 共有エンジンに、番号と日付を任意で渡せるようにする（`apps/server/internal/bbsengine/engine.go`、`catchUpInitial` 周辺）

  - 記事と返信を作るときに使う、任意の関数を受け取れるようにする。既定は `nil`（番号を付けない）。
  - Erika-K だけが、計画の番号と日付、および `NextReplyNo` を使う関数を渡す。
  - 他のホスト（TurboBBS、VS、bbs など）は関数を渡さない。挙動は変えない。

- [ ] **T015** 共有エンジンと他のホストのテスト

  ```bash
  go -C apps/server build ./... && go -C apps/server test ./internal/bbsengine/... ./internal/hostprogram/... -count=1
  ```

  共有エンジンの既存のテストが、すべて通ること。通らなければ、関数を渡していないホストの挙動が変わっている。実装を直す。

- [ ] **T016** 一覧の番号列を記事番号にする（`runtime.go` の `renderBoardIndex`、L720〜750 付近）

  - `fmt.Fprintf(&b, "%02d %4d %s %s %s %s\r\n", keyInt, p.ID, ...)` の `p.ID` を `p.BoardNo` にする。
  - 幅（`%4d`）と並び順（`roots` を後ろから 10 件）は変えない。

- [ ] **T017** 記事番号による指定（`runtime.go`、`rootPost` の呼び出し 11 箇所）

  - `rootPost(id int64)` を `rootPostByNo(boardPath string, no int64) (world.Post, bool)` に置き換える（`ParentID == 0`、`BoardID == boardPath`、`BoardNo == no`）。
  - `grep -n "rootPost(" apps/server/internal/hostprogram/erikak/runtime.go` の結果を、すべて読んで置き換える。
  - 返信番号による指定はしない。

**Checkpoint**: 一覧と指定が記事番号で動き、他のホストは変わらない。

---

## Phase 5: テストと金型の確認

- [ ] **T018** テスト（US1〜US6）

  `numbering_test.go` に、次を書く。

  | テスト | 内容 |
  |---|---|
  | US1 | 2 つの板に記事を書き、それぞれの一覧の記事番号が板ごとに続いている。返信を書いても記事番号が変わらない |
  | US2 | 板 A の番号 5 を `BR 5` で開くと板 A の記事 5 が出る。板 B に同じ番号があっても板 A の一覧から開いたときは板 A の記事を指す |
  | US3 | 板を初めて開いたとき、計画の記事の番号の順と日付の順が一致し、日付は窓の中にある |
  | US4 | 時刻を進めても、計画の N と `OpenedOn` は変わらない。保存して読み直しても変わらない |
  | US5 | 記事 1 に返信を 3 件書く（番号 1、2、3）。返信 2 を削除（テストで直接用意）すると、残りは 1 と 3。次の返信は 4 |
  | US6 | 金型テスト（T019） |

- [ ] **T019** 金型の確認

  ```bash
  go -C apps/server test ./internal/hostprogram/erikak/... -run 'TestGoldenNumbering' -count=1
  ```

  - マスクした形で比べて、差分が出ないこと。
  - 金型ファイルの `git diff --stat` が空であること（`git diff --stat <金型のコミット>..HEAD -- testdata`）。

- [ ] **T020** 全体の確認

  ```bash
  go -C apps/server build ./...
  go -C apps/server vet ./...
  go -C apps/server test ./... -count=1
  git log --oneline -- apps/server/internal/hostprogram/erikak/testdata
  ```

  - `git log` で、金型のコミットが 1 つだけであること。金型のコミットが、実装のコミットより前にあること。

- [ ] **T021** ドキュメントを更新する

  - `docs/host-programs/erika-k.md` の「Article numbering and append rules」を、判断事項の結果に合わせて更新する。
    - 板ごとの計画（最初の投稿日、N、窓）と、記事番号は日付順であること。
    - 返信番号は親ごとで、表示のみに使うこと（返信の指定はしない）。
  - `specs/README.md` の 020 の行は変えない。

- [ ] **T022** 結果を `specs/020-erika-k-article-numbering/verification.md`（新規）に記録する

  ```markdown
  # Verification: Erika-K の板の計画と記事番号

  | コマンド | 結果 | 備考 |
  |---|---|---|
  | `go -C apps/server build ./...` | 成功 / 失敗 / 未実施 | |
  | `go -C apps/server vet ./...` | 〃 | |
  | `go -C apps/server test ./...` | 〃 | |
  | 金型テスト（3 回連続） | 〃 | |

  ## 金型が、変更前のコードで生成されたこと
  （金型のコミットの SHA と、実装のコミットの SHA。金型が先であること）

  ## 金型の差分
  （マスクした形で差分が出ないこと。金型ファイルが変わっていないこと）

  ## 計画の確定
  - 板一覧の表示時に確定すること: 確認済み / 未実施

  ## 手動確認
  - 局の画面で板を初めて開き、番号の順と日付の順が一致すること: 確認済み / 未実施
  ```

- [ ] **T023** PR を作る

  - `main` への PR。**draft** で作成する。
  - タイトル: `Erika-K の板の計画と記事番号を実装する（画面の形式は変えず、番号と計画の日付のみ）`
  - 説明に含める: 目的、変更の要約、**金型のコミットが実装より前であること**、`verification.md` の結果（未実施を含めて正直に）、未決事項の扱い、スコープ外の項目。

---

## 実行順序

```text
Phase 0 (T001 → T002 → T003)   ← 金型は実装より前にコミット
  └─ Phase 1 (T004 → T005 → T006 → T007)
       └─ Phase 2 (T008 → T009)
            └─ Phase 3 (T010 → T011 → T012 → T013)
                 └─ Phase 4 (T014 → T015 → T016 → T017)
                      └─ Phase 5 (T018 → T019 → T020 → T021 → T022 → T023)
```

## 完了の定義

- `spec.md` の FR と SC を満たす（未決事項で未確定のものは、PR に明記する）。
- 金型テストが、変更前に生成した金型ファイルを変更せずに通る（マスクした形で差分なし）。金型は実装より前のコミットにある。
- 他のホストのテストが、すべて通る（`go -C apps/server test ./...`）。
- 変更が `plan.md` の「変更するファイル」の表の範囲に収まっている。
