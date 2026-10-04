# Tasks: generic debug names

**Input**: `specs/003-generic-debug-names/` の `spec.md`、`plan.md`
**Prerequisites**: `AGENTS.md`

## この作業のルール（必読）

- 作業ブランチから `main` への PR を 1 本作る。`main` に直接コミットしない。
- **挙動を変えない**。既定値、判定ロジック、HTTP の応答、エンドポイントのパスは変更しない。
- `spec.md` の「スコープ外」と、`plan.md` の「残してよい HAKATA の記述」は変更しない。
- 既存ファイルに整形だけの差分を作らない。`gofmt -w` を既存ファイル全体にかけない。
  変更した行だけを、周囲の書式に合わせて直す。
- 1 タスク = 1 コミット。メッセージは `T0XX: 内容`。
- 実行していないコマンドを「成功した」と書かない。実行できなかったものは「未実施」と理由を書く。
- 同じテスト失敗を 2 回直して通らなければ、そこで止めて状況を報告する。
- この作業で `.github/` 配下のファイルは変更しない。

## Format

- 各タスクの「確認」は、そのタスクの終わりに実行するコマンド。

---

## Phase 1: 設定と環境変数（互換を最優先）

- [ ] **T001** `apps/server/internal/config/config.go` を変更する

  1. `config.go` を最後まで読み、既存の `env` / `envBool` の挙動（特に空文字列の扱い）を確認する。
  2. `plan.md` の「環境変数の別名の読み方」に従い、`envBoolWithLegacy` を追加する。
  3. フィールドを改名し、環境変数の読み方を次のとおりにする。

     | フィールド（新） | 読み方 |
     |---|---|
     | `DebugLogGeneratedContent` | `envBoolWithLegacy("DEBUG_LOG_GENERATED_CONTENT", "DEBUG_LOG_HAKATA_GENERATED", true)` |
     | `DebugGenerationTrace` | `envBoolWithLegacy("DEBUG_GENERATION_TRACE", "DEBUG_HAKATA_LLM_TRACE", true)` |
     | `GenerationFreeformBody` | `envBoolWithLegacy("GENERATION_FREEFORM_BODY", "HAKATA_FREEFORM_BODY", true)` |

  4. この時点では `go build ./...` が他のファイルでエラーになる。T004 以降で直すので、コミットだけ先に行う。

- [ ] **T002** `apps/server/internal/config/config_test.go` を更新する

  1. 既存のテスト 2 つ（HAKATA を含む名前のもの）を、`plan.md` の改名表の新名に改名し、
     環境変数とフィールド名を新名に合わせる。意味（既定で true、`0` で無効化できる）は変えない。
  2. 3 つの設定それぞれについて、次のテストを追加する（テーブル駆動でよい）。

     | ケース | 設定 | 期待 |
     |---|---|---|
     | 新名のみ | 新名 = `0` | `false` |
     | 旧名のみ | 旧名 = `0` | `false` |
     | 両方 | 新名 = `1`、旧名 = `0` | `true`（新名が優先） |
     | 両方 | 新名 = `0`、旧名 = `1` | `false`（新名が優先） |
     | どちらも無し | 両方とも空 | `true`（既定値） |

  3. 旧名を使ったときに非推奨のログが出ることは、テストしなくてよい（出力先が標準ログのため）。

- [ ] **T003** `.env.example` を更新する

  - 3 つの変数を新名に書き換える。コメントの「HAKATA-only」の記述は、「局のフラグで対象になった局だけに効く」
    という説明に直す。値の例は現状のまま。
  - 旧名は、ファイルの末尾に次の形で 1 か所だけ記載する。

    ```
    # Deprecated aliases (still read when the new name is unset; remove in a later release):
    #   DEBUG_LOG_HAKATA_GENERATED -> DEBUG_LOG_GENERATED_CONTENT
    #   DEBUG_HAKATA_LLM_TRACE     -> DEBUG_GENERATION_TRACE
    #   HAKATA_FREEFORM_BODY       -> GENERATION_FREEFORM_BODY
    ```

**Checkpoint**: 設定の読み込みが新名と旧名の両方に対応している。

---

## Phase 2: `worldrepo` の改名

- [ ] **T004** `git mv` でファイルを移す

  ```bash
  git mv apps/server/internal/worldrepo/hakata_freeform_body.go apps/server/internal/worldrepo/freeform_body.go
  git mv apps/server/internal/worldrepo/hakata_freeform_body_test.go apps/server/internal/worldrepo/freeform_body_test.go
  ```

- [ ] **T005** `worldrepo` の識別子を改名する

  `plan.md` の「`worldrepo`」の表のとおりに改名する（メソッド 4 つ、フィールド 2 つ）。
  `repository.go`、`bbs_content_log.go`、`freeform_body.go`、`bbs_article_body.go` と、
  それらを使うテストが対象。使用箇所は、`go build ./...` と `go vet ./...` のエラーで洗い出す。
  `bbs_content_log.go` と `freeform_body.go` の「The HAKATA-specific name is historical」の注記は削除する。

- [ ] **T006** `worldrepo` のテスト名とメッセージを直す

  `plan.md` の「テスト関数名」の表のとおりに改名する。テスト内のエラーメッセージで
  HAKATA 専用であるかのように書いているもの（例: `HAKATA freeform mode must not call research`）は、
  `freeform mode must not call research` のように局名を除く。

- [ ] **T007** 確認

  ```bash
  go -C apps/server build ./internal/worldrepo/... ./internal/config/... && go -C apps/server test ./internal/worldrepo/... ./internal/config/...
  ```

---

## Phase 3: `cmd/server`

- [ ] **T008** `cmd/server/main.go` と `generation_trace_handler.go` を直す

  - `main.go`: 設定のフィールドとセッターの呼び出しを新名に合わせる。

    ```go
    runtimeStore.SetDebugLogGeneratedContent(cfg.DebugLogGeneratedContent)
    runtimeStore.SetGenerationTraceEnabled(cfg.DebugGenerationTrace)
    runtimeStore.SetFreeformBody(cfg.GenerationFreeformBody)
    ```

    `newHakataTraceHandler(cfg.DebugHakataLLMTrace, runtimeStore)` は
    `newGenerationTraceHandler(cfg.DebugGenerationTrace, runtimeStore)` にする。
  - `generation_trace_handler.go`: 関数名を `newGenerationTraceHandler` にし、
    エラー文言を `generation trace is disabled` にする。コメントの
    「fictional HAKATA evaluation deployment」は、
    「an evaluation deployment where the host opts in with debug.generation_trace」のような中立な表現にする。
  - HTTP の応答コード（403 など）と JSON の形は変えない。
  - `generation_trace_handler_test.go` の関数名の呼び出しと、文言を検査している箇所を新名に合わせる。

- [ ] **T009** 確認

  ```bash
  go -C apps/server build ./... && go -C apps/server test ./cmd/server/...
  ```

---

## Phase 4: コメントと文言

- [ ] **T010** `plan.md` の「コメントと文言を中立にする箇所」の表のファイルを直す

  コメントのみの変更。コードは変えない。表に無いファイルでも、`grep` で見つかった
  テスト以外のコメントのうち、HAKATA 専用であるかのように書かれたものは、同じ方針で直す。
  ただし `plan.md` の「残してよい HAKATA の記述」は直さない。

---

## Phase 5: ドキュメント

- [ ] **T011** [P] `docs/DEBUG_INSPECTION.md` を更新する

  旧名 `DEBUG_HAKATA_LLM_TRACE` を新名 `DEBUG_GENERATION_TRACE` に書き換える。
  本文の「HAKATA」を、「`debug.generation_trace` を持つ局」のように、フラグでの説明に直す。
  旧名が非推奨の別名として今も読めることを、1 文で書く。挙動の説明（既定値、HTTP 403 など）は変えない。

- [ ] **T012** [P] `apps/server/internal/hostcatalog/README.md` の「Debug and generation flags」の表を更新する

  「Also needs」の列の 3 つの環境変数を新名にする。表の下の
  「The process-wide environment switches keep their historical names for now.」は、
  「The previous HAKATA-prefixed names are still read as deprecated aliases.」に置き換える。

- [ ] **T013** [P] 旧名を書いているほかのファイルを探して更新する

  ```bash
  grep -rnE "DEBUG_LOG_HAKATA_GENERATED|DEBUG_HAKATA_LLM_TRACE|HAKATA_FREEFORM_BODY" . \
    --exclude-dir=.git --exclude-dir=node_modules
  ```

  見つかった各ファイルを、新名に更新する。ただし次は残す。
  - `config.go` の旧名（別名として読むため）と、`config_test.go` の旧名（別名のテスト）。
  - `.env.example` の「Deprecated aliases」の記載。
  - 本 spec のファイル（`specs/003-generic-debug-names/`）。
  - 過去の spec（`specs/002-host-capabilities/`）。

---

## Phase 6: 検証と PR

- [ ] **T014** 全体の確認と記録

  ```bash
  go -C apps/server build ./...
  go -C apps/server vet ./...
  go -C apps/server test ./...
  grep -rliE "hakata" apps/server --include=*.go | grep -v _test.go
  git diff --stat main...HEAD
  ```

  結果を `specs/003-generic-debug-names/verification.md`（新規）に、次の形式で記録する。

  ```markdown
  # Verification: generic debug names

  | コマンド | 結果 | 備考 |
  |---|---|---|
  | `go -C apps/server build ./...` | 成功 / 失敗 / 未実施 | |
  | `go -C apps/server vet ./...` | 〃 | |
  | `go -C apps/server test ./...` | 〃 | |

  ## 残った hakata を含む Go ファイル（テストを除く）
  （grep の結果を貼り、plan.md の許可リストに入っているかを 1 行ずつ書く。許可リスト外があれば理由）

  ## 環境変数の互換（config_test.go）
  （T002 の 5 ケースが通ったか）

  ## 手動確認
  - 旧名（`DEBUG_HAKATA_LLM_TRACE=1` など）でサーバが起動する: 確認済み / 未実施
  - 新名（`DEBUG_GENERATION_TRACE=1` など）でサーバが起動する: 確認済み / 未実施
  ```

- [ ] **T015** PR を作る

  - `main` への PR。**draft** で作成する。
  - タイトル: `共通コードから HAKATA を含む名前を除く（環境変数は旧名も受け付ける）`
  - 説明に含める: 目的（PR #294 と spec 002 の続き。名前を実態に合わせる）、改名の要約、
    **環境変数の互換（旧名は引き続き有効で、使うと非推奨のログが出る）**、挙動を変えていないこと、
    `verification.md` の結果（未実施の項目も含めて正直に）、スコープ外の項目。

---

## 実行順序

```text
Phase 1 (T001 → T002, T003) → Phase 2 (T004 → T005 → T006 → T007)
  → Phase 3 (T008 → T009) → Phase 4 (T010) → Phase 5 (T011, T012, T013) → Phase 6 (T014 → T015)
```

- T001 のコミット後は、T005〜T008 が終わるまで `go build ./...` が通らない。
  途中でビルドが通らないのは想定内。Phase 3 の終わり（T009）までに全体のビルドを通す。

## 完了の定義

- `spec.md` の FR-001〜FR-009 と SC-001〜SC-004 を満たす（手動確認が未実施なら、その旨を明記する）。
- 変更が、`plan.md` に書かれたファイルと、T013 の grep で見つかったファイルの範囲に収まっている。
