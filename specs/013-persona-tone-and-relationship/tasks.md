# Tasks: ペルソナ個性パラメータと関係性の反映によるBBS本文文体の生動化（013-persona-tone-and-relationship）

**Input**: `specs/013-persona-tone-and-relationship/` の `spec.md`、`plan.md`  
**Prerequisites**: `AGENTS.md`、Spec 010、Spec 011、Spec 012

## この作業のルール（必読）

- 作業ブランチから `main` への PR を 1 本作る。`main` に直接コミットしない。
- **成果物は Go のコード（実装）とテスト。** spec のファイルは変更しない。追加してよい spec フォルダのファイルは `verification.md` だけ。
- **既存の人物生成・画面金型出力を 1 バイトも変えない。**
- スコープ外に手を出さない。
- 既存ファイルに整形だけの差分を作らない。`gofmt -w` を既存ファイル全体にかけない。
- 1 タスク = 1 コミット（`T0XX: 内容`）。
- 実行していないコマンドを「成功した」と書かない。
- `.github/` 配下を変更しない。
- Go のファイルと Markdown は LF 改行で保存する。
- spec と実際のコードが食い違ったら、推測で進めず、PR の説明に書いて止まる。同じ失敗が 2 回続いたら止まる。

---

## Phase 0: ベースライン

- [ ] **T001** 変更前に全テストが通ることを確認する

  ```bash
  go -C apps/server test ./... -count=1
  ```

---

## Phase 1: ペルソナ個性プロファイルとプロンプト文体指針の改修

- [ ] **T002** `apps/server/internal/llm/board_post_prompt.go` および `board_post_freeform.go` に、1996 年当時の草の根 BBS らしい生きた会話口調と部室感の文体指針を追加し、`BoardPostRequest.RelationshipContext` の受け取りに対応する
- [ ] **T003** `apps/server/internal/worldrepo/llm_materializer.go` の `personaSummary` を改修し、年齢・職業・性別・`WritingStyle`・`Argumentativeness` を LLM が演じやすい自然な人物指針として出力する

---

## Phase 2: 関係性データの取得と配線

- [ ] **T004** `apps/server/internal/worldrepo/bbs_article_body.go` で、返信記事生成時に `r.Base.GetRelationship` から相手とのやり取り履歴（初対面 vs 常連仲間）を取得し、`RelationshipContext` を組み立てる
- [ ] **T005** `apps/server/internal/worldrepo/repository.go` の `BoardMaterializationRequest` に `RelationshipContext` を追加し、`llm_materializer.go` を経由して `llm.BoardPostRequest` に透過的に流す

---

## Phase 3: サンプリングハーネスの実装と検証

- [ ] **T006** `apps/server/internal/worldrepo/materialize_sampling_test.go` に常連同士、初対面、文体差（オタク口調、落ち着いた社会人など）のサンプリング検証シナリオを追加し、出力を検証する

  ```bash
  go -C apps/server test -v ./internal/worldrepo -run 'TestMaterializeSampling' -count=1
  ```

---

## Phase 4: 金型保全・全テスト・静的解析

- [ ] **T007** 金型と全テストを実行する

  ```bash
  go -C apps/server test ./internal/world -run 'TestPopulation' -count=1
  go -C apps/server test ./internal/hostprogram/erikak -run 'TestGoldenBoardTableShape' -count=1
  go -C apps/server test ./... -count=1
  go -C apps/server vet ./...
  ```

---

## Phase 5: 検証記録と PR 作成

- [ ] **T008** `specs/013-persona-tone-and-relationship/verification.md` に、実行したコマンドとサンプリング結果・検証証跡を書く
- [ ] **T009** Draft PR を作成する（完了定義）

**Done**: 上記チェックボックスがすべて埋まり、PR が作成された状態。
