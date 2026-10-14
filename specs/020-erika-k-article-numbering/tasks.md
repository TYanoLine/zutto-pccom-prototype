# Tasks: Erika-K の記事番号と返信番号の採番

**Input**: `specs/020-erika-k-article-numbering/` の `spec.md`、`plan.md`
**Prerequisites**: `AGENTS.md`、`plan.md`（特に「設計」と「金型の方針」）、spec の「判断事項」と「未決事項」

## この作業のルール（必読）

- 作業ブランチから `main` への PR を 1 本作る。`main` に直接コミットしない。
- **成果物は Go のコードの変更（実装）と、その金型。** spec のファイルを作る・変更することは、この作業の仕事ではない。
- **HAKATA の板の一覧と記事の表示は、番号の列と、リセットによる内容の違い以外、1 バイトも変えない。** これが合格条件。金型テストで確認する。
- **金型は、変更前のコードで生成し、実装より前にコミットする。** 実装の後で生成してはならない（同義反復になり、根拠にならない）。
  コミットの順序が、そのまま証拠になる。
- **金型をコミットしたあと、金型の更新用のフラグを二度と使わない。** 金型ファイルを書き換えない。
  金型テストが失敗したら、実装の誤り。金型ではなく実装を直す。2 回直して通らなければ、そこで止めて、どの行が違うかを報告する。
- **spec の「未決事項」2（リセットの方法と契機）が確定するまで、リセットの処理（T010）を書いてはならない。**
  未確定なら T010 は行わず、PR の説明に止めた理由を書く。
- **採番器の値は、残っている記事から計算しない。** `max + 1` のような実装をしてはならない（spec FR-002、FR-004）。
- スコープ外（`spec.md` の「スコープ外」）に手を出さない。特に、記事と返信の削除の操作と、1 行入力・複数行入力の変更。
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
    2. 記事 1 に返信を 2 件書く。
    3. 板 B（別の板）に記事を 1 件書く。
    4. 板 A の一覧（`BX`）を開く。
    5. 記事 1 を `BR` で読む（返信を含む）。
    6. HAKATA の板の一覧（`BM` と各板の `BX`）を開く。
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

## Phase 1: 投稿のフィールドと採番器

- [ ] **T004** `world.Post` に `BoardNo` と `ReplyNo` を追加する（`apps/server/internal/world/models.go`、L134〜149）

  `ID` の次の行に、次を挿入する（コメントは spec 020 を参照）。

  ```go
  // BoardNo is the article number a Erika-K board shows (spec 020). Zero means
  // the host does not number posts this way. Replies leave it zero.
  BoardNo int64 `json:"board_no,omitempty"`
  // ReplyNo is the number of a reply within its parent article (spec 020). Zero
  // for articles and for hosts that do not number replies this way.
  ReplyNo int64 `json:"reply_no,omitempty"`
  ```

- [ ] **T005** 採番器のインターフェースと、メモリ上の実装を書く（`apps/server/internal/world/store.go`）

  - `BoardNumberStore` を、`HostDetailStore`（L18〜20）と同じ形の任意インターフェースとして定義する（`plan.md` の設計 2）。
  - `MemoryStore` に `boardNos map[string]int64`、`replyNos map[string]int64` を足し、`NextBoardNo` と `NextReplyNo` を実装する。
    値は「最後に発行した番号」で、+1 して返す。ロックは既存の `s.mu` を使う。
  - テスト（`store_test.go` に追加）: 発行は 1 から続く。投稿を消しても発行の値は戻らない（残存記事から計算しないことの確認）。

- [ ] **T006** 採番器を snapshot に保存する（`apps/server/internal/world/development_snapshot.go`、L20〜124 付近）

  - `board_numbers` と `reply_numbers`（`map[string]int64`）を追加する。`omitempty` にする。
  - 読み込み時、この項目が無い保存データは空として読む（0 から始まる）。
  - テスト: 保存して読み直した後、次の発行が続きの番号になる。

- [ ] **T007** `worldpersist.Store` に採番器を実装する（`apps/server/internal/worldpersist/store.go`）

  - T005 と同じ 2 つのメソッドを、保存と読み込みに対応させる。
  - テスト: 再起動に見立てて読み直したあとも、番号が続く。

**Checkpoint**: 採番器が、メモリと保存の両方で、削除によって戻らないことを確認するテストが通る。

---

## Phase 2: 利用者の記事と返信の採番

- [ ] **T008** 利用者の記事に記事番号を付ける（`apps/server/internal/hostprogram/erikak/runtime.go`、`new_body`、L120〜137）

  - `AddPost` の前に `BoardNo: r.nextBoardNo()` を入れる。`nextBoardNo` は `numbering.go` に書く（`BoardNumberStore` を型アサーションで取り、無ければ 0）。
  - 投稿完了の文言 `MSG No.%d を登録しました` の `%d` は `p.BoardNo` にする。

- [ ] **T009** 利用者の返信に返信番号を付ける（`runtime.go`、`append_body`、L155〜178）

  - `AddPost` の前に `ReplyNo: r.nextReplyNo(root.ID)` を入れる。`ParentID: root.ID` は変えない。
  - 見出し `アペ N` の番号は、現状の連番（`appendNo`）ではなく、`ReplyNo` にする。
  - 表示の `MSG No.%d へアペンドします`（L153 付近）は、親の `BoardNo` にする。

- [ ] **T010** 確認

  ```bash
  go -C apps/server build ./... && go -C apps/server test ./internal/hostprogram/erikak/... ./internal/world/... -count=1
  ```

  金型テストは、この時点で番号の列の差分が出てよい。差分が番号以外に出ていたら、実装を直す。

**Checkpoint**: 利用者の記事と返信が、板ごと・親ごとの番号を持つ（テストで確認）。

---

## Phase 3: 生成された記事と返信（共有エンジン）

- [ ] **T011** 共有エンジンに採番の関数を任意で渡せるようにする（`apps/server/internal/bbsengine/engine.go`、`catchUpInitial` 周辺）

  - 記事と返信を作るときに使う、任意の採番の関数を受け取れるようにする。既定は `nil`（番号を付けない）。
  - Erika-K だけが、`NextBoardNo` と `NextReplyNo` を使う関数を渡す。
  - 他のホスト（TurboBBS、VS、bbs など）は関数を渡さない。挙動は変えない。

- [ ] **T012** テストと確認

  - 生成された記事が、採番器から番号を取ること（遅延生成の順に続くこと）。
  - 他のホストの番号が 0 のままであること。

  ```bash
  go -C apps/server build ./... && go -C apps/server test ./internal/bbsengine/... ./internal/hostprogram/... -count=1
  ```

  共有エンジンの既存のテストが、すべて通ること。通らなければ、番号の関数を渡していないホストの挙動が変わっている。実装を直す。

**Checkpoint**: 生成された記事と返信も、採番器の続きの番号を持つ。

---

## Phase 4: 表示と番号による指定

- [ ] **T013** 一覧の番号列を記事番号にする（`runtime.go` の `renderBoardIndex`、L720〜750 付近）

  - `fmt.Fprintf(&b, "%02d %4d %s %s %s %s\r\n", keyInt, p.ID, ...)` の `p.ID` を `p.BoardNo` にする。
  - 幅（`%4d`）と並び順（`roots` を後ろから 10 件）は変えない。
  - `BoardNo` が 0 の記事（リセット前の記事）は、リセットの後には残らない（T015）。

- [ ] **T014** 番号による指定を、板と記事番号で解決する（`runtime.go`、`rootPost` の呼び出し 11 箇所）

  - `rootPost(id int64)` を `rootPostByNo(boardPath string, no int64) (world.Post, bool)` に置き換える（`ParentID == 0`、`BoardID == boardPath`、`BoardNo == no`）。
  - `grep -n "rootPost(" apps/server/internal/hostprogram/erikak/runtime.go` の結果を、すべて読んで置き換える。
  - 返信の指定（`BR n/m`。構文は spec 未決事項 1）は、親を `rootPostByNo` で引き、その子で `ReplyNo == m` を引く。構文は未決事項 1 の確定後に実装する。
    未確定なら、返信の指定はこの PR に含めず、PR の説明に書く。

- [ ] **T015** 既存の記事のリセット（spec 未決事項 2 が確定してから）

  - **未決事項 2 が確定していなければ、この T015 は行わない。** PR の説明に「未確定のためリセットは未実施」と書く。
  - 確定していれば: Erika-K の局の記事だけを消去し、採番器（`board_numbers`、`reply_numbers`）を 0 から始める。
  - テスト: 他の局の記事、局の定義、利用者の情報が残ること。Erika-K の記事が消え、採番が 1 から始まること。

- [ ] **T016** テスト（US1〜US5）

  `numbering_test.go` に、次を書く。

  | テスト | 内容 |
  |---|---|
  | US1 | 2 つの板に記事を書き、それぞれの一覧の記事番号が板ごとに続いている。返信を書いても記事番号が変わらない |
  | US2 | 板 A の番号 5 を `BR 5` で開くと板 A の記事 5 が出る。板 B に同じ番号があっても板 A の一覧から開いたときは板 A の記事を指す |
  | US3 | 記事 1 に返信を 3 件書く（番号 1、2、3）。返信 2 を削除すると、残りは 1 と 3。次の返信は 4 |
  | US4 | 記事 1〜3 を書き、記事 3 を削除（テストで直接用意する）。次の記事は 4 |
  | 遅延生成 | 生成された記事が、採番器の続きの番号を持つ |

- [ ] **T017** 確認

  ```bash
  go -C apps/server build ./... && go -C apps/server vet ./... && go -C apps/server test ./internal/hostprogram/erikak/... -count=1
  ```

  - 金型テストは、**差分が番号の列と、リセットによる内容だけであること**を確認する（`TestGoldenNumbering` の差分確認）。金型ファイルは変えない。
  - 差分が番号以外の欄に出たら、実装を直す。金型は変えない。

**Checkpoint**: US1〜US5 が通り、金型の差分は番号の列とリセットの内容だけ。

---

## Phase 5: 全体の確認とドキュメント

- [ ] **T018** 他のホストを含む全体の確認

  ```bash
  go -C apps/server build ./...
  go -C apps/server vet ./...
  go -C apps/server test ./... -count=1
  git diff --stat <金型のコミット>..HEAD -- apps/server/internal/hostprogram/erikak/testdata
  git log --oneline -- apps/server/internal/hostprogram/erikak/testdata
  ```

  - 金型ファイルの `git diff --stat` が空であること。`git log` で、金型のコミットが 1 つだけであること。

- [ ] **T019** ドキュメントを更新する

  - `docs/host-programs/erika-k.md` の「Article numbering and append rules」を、判断事項の結果に合わせて更新する。
    - アペンドは、記事番号は持たないが、**親の中の返信番号を持つ**（以前の「内部 ID のみ」の記述を置き換える）。
    - 既存の記事はリセットする（実施した場合は、その旨）。
  - `specs/README.md` の 020 の行は変えない。

- [ ] **T020** 結果を `specs/020-erika-k-article-numbering/verification.md`（新規）に記録する

  ```markdown
  # Verification: Erika-K の記事番号と返信番号の採番

  | コマンド | 結果 | 備考 |
  |---|---|---|
  | `go -C apps/server build ./...` | 成功 / 失敗 / 未実施 | |
  | `go -C apps/server vet ./...` | 〃 | |
  | `go -C apps/server test ./...` | 〃 | |
  | 金型テスト（3 回連続） | 〃 | |

  ## 金型が、変更前のコードで生成されたこと
  （金型のコミットの SHA と、実装のコミットの SHA。金型が先であること）

  ## 金型の差分
  （差分が番号の列とリセットの内容だけであることの確認）

  ## 未決事項の結果
  - 未決事項 1（返信の指定の構文）: 確定した構文 / 未確定（実装の範囲）
  - 未決事項 2（リセットの方法と契機）: 確定した方法 / 未確定（T015 未実施）

  ## 手動確認
  - 局の画面で板の一覧を開き、番号が板ごとに続くこと、返信の番号が親の中で続くこと: 確認済み / 未実施
  ```

- [ ] **T021** PR を作る

  - `main` への PR。**draft** で作成する。
  - タイトル: `Erika-K の記事番号を板ごと、返信番号を親ごとの採番にする（金型を変えず番号の列のみ）`
  - 説明に含める: 目的、変更の要約、**金型のコミットが実装より前であること**、`verification.md` の結果（未実施を含めて正直に）、未決事項の扱い、スコープ外の項目。

---

## 実行順序

```text
Phase 0 (T001 → T002 → T003)   ← 金型は実装より前にコミット
  └─ Phase 1 (T004 → T005 → T006 → T007)
       └─ Phase 2 (T008 → T009 → T010)
            └─ Phase 3 (T011 → T012)
                 └─ Phase 4 (T013 → T014 → T015 → T016 → T017)
                      └─ Phase 5 (T018 → T019 → T020 → T021)
```

## 完了の定義

- `spec.md` の FR と SC を満たす（未決事項で未確定のものは、PR に明記する）。
- 金型テストが、変更前に生成した金型ファイルを変更せずに通る。差分は番号の列とリセットの内容だけ。金型は実装より前のコミットにある。
- 他のホストのテストが、すべて通る（`go -C apps/server test ./...`）。
- 変更が `plan.md` の「変更するファイル」の表の範囲に収まっている。
