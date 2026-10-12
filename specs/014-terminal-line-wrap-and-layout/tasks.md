# Tasks: Spec 014 80桁端末環境に即した記事レイアウトと改行指針 (terminal-line-wrap-and-layout)

## ルール (Rules)

- **1 タスク = 1 コミット**
- **コミットメッセージ形式**: `T001: Description`
- **金型テストは 1 バイトも変えない**
- **LF 改行コードの徹底**
- **報告は正直に**

---

## Phase 0: ベースライン確認

- [ ] **T001** ベースラインテストの実行と確認
  ```bash
  go -C apps/server test ./... -count=1
  ```

---

## Phase 1: プロンプトへの改行・レイアウト指針の導入

- [ ] **T002** `apps/server/internal/llm/board_post_prompt.go` および `board_post_freeform.go` に全角30〜35文字目安・文節改行の指針を追加
- [ ] **T003** `apps/server/internal/llm/board_post_prompt_test.go` または `board_post_freeform_test.go` のプロンプト内容検証テストを更新

---

## Phase 2: サンプリング検証と行長レイアウトの確認

- [ ] **T004** `apps/server/internal/worldrepo/materialize_sampling_test.go` に行長と改行レイアウトのサンプリング検証を追加し、動作確認

  ```bash
  go -C apps/server test -v ./internal/worldrepo -run 'TestMaterializeSampling' -count=1
  ```

---

## Phase 3: 金型保全・全テスト・静的解析

- [ ] **T005** 金型保全と全テスト・静的解析を実行

  ```bash
  go -C apps/server test ./internal/world -run 'TestPopulation' -count=1
  go -C apps/server test ./internal/hostprogram/erikak -run 'TestGoldenBoardTableShape' -count=1
  go -C apps/server test ./... -count=1
  go -C apps/server vet ./...
  ```

---

## Phase 4: 検証記録と PR 作成

- [ ] **T006** `specs/014-terminal-line-wrap-and-layout/verification.md` を作成
- [ ] **T007** Draft PR を作成（完了定義）
