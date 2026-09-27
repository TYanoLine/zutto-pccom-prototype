# Implementation Plan: BBS本文の人物別具体化

**Branch**: `001-body-concretization` | **Date**: 2026-09-27 | **Spec**: [spec.md](spec.md)

**Input**: `specs/001-body-concretization/spec.md`

## Summary

選択済みで本文未確定の記事を初めて読む時、投稿者の時点有効な人物事実、本人の先行発言、返信スレッドを参照し、0〜2件の記事ローカル詳細を本文より先に確定する。この処理を共有記事具体化エンジンへ移し、title-first フラグや通常閲覧・Lab といった入口に依存せず適用する。正常な0件を再提案しないよう `PostIntent` に明示的な完了状態を保存し、Detail の生成・検証・保存が失敗した場合は本文生成へ進まない。品質評価は利用可能な複数返信スレッドで行い、20〜30本は十分な標本がある場合の目安とする。

## Technical Context

**Language/Version**: Go 1.23。既存の評価ビューは TypeScript/React。

**Primary Dependencies**: 既存の `worldrepo`、`world`、`bbsengine`、`hostprogram`、`llm` と `BBSTitleArticleDetailPlanner`。Article Detail の能力を本文 renderer の具象型から取り出し、共有記事具体化処理が明示的な planner 能力として利用する。

**Storage**: 記事の正本は `Post.Intent` と `Post.Body`。0〜2件の詳細は従来どおり `SituationFacts` の `article_detail=` 項目、処理完了は新しい `PostIntent.ArticleDetailsMaterialized` 真偽値に保存する。`PostUpdater` の成功後に保存済み記事を本文処理へ渡す。現行の開発永続化は `PostIntent` を含む JSONB スナップショットで、新フィールドは欠落時 `false` として旧データを読み込める。Lab アーカイブは評価証拠であり世界正本ではない。

**Testing**: Go の単体・ホスト実行テスト、materialization Lab と読み取り専用評価ビューによる人手評価。

**Target Platform**: Go サーバー上の共有 BBS 記事具体化エンジン。通常ホスト、開発用の直接確認、Lab は同じ記事単位の処理を呼び出す。

**Project Type**: Web アプリケーションを含むモジュラーモノリス。

**Performance Goals**: 人物履歴は最大6件。Article Detail は現行30秒、本文処理は35秒の期限内で各最大2試行。品質評価は、確認可能な複数返信スレッドと記事を対象にする。20〜30本は十分な標本がある場合の目安で、最低件数や完了条件ではない。

**Constraints**: 件名、世界行動、返信先を変更しない。未来事実や無関係な過去記事本文を参照しない。0件の詳細は正常。Detail の planner 不在・生成エラー・検証エラー・保存失敗は正常0件に変換しない。1996年の知識境界、ホスト別表示、AIを使用しないホスト実行を維持する。

**Scale/Scope**: 呼び出し元に依存しない遅延本文具体化とその検証。タイトル選定、投稿判断、SDD-002〜004 の人物事実抽出と社会的記憶は対象外。

## Constitution Check

*GATE: Phase 0 前と Phase 1 後に確認。*

`.specify/memory/constitution.md` は未記入のテンプレートであり追加ゲートを定めていない。`AGENTS.md`、`docs/PRODUCT_SPEC.md`、`docs/ARCHITECTURE.md`、`docs/HISTORICAL_ACCURACY.md`、`docs/WORLD_SIMULATION.md`、`docs/LLM_POLICY.md` を適用する。

| ゲート | Phase 0 前 | Phase 1 後 |
| --- | --- | --- |
| 世界事実は保存した記事・人物情報が正本 | 適合。選択済み記事だけを具体化 | 適合。詳細保存後に本文を確定する状態遷移を設計 |
| 人物・時系列・1996年の知識境界 | 適合。投稿時点で入力を限定 | 適合。契約と検証ガイドに確認方法を記載 |
| ホストごとの操作・返信表示 | 適合。共通世界層は意味だけ扱う | 適合。Erika-K のアペンド表示は runtime に残す |
| 状態遷移のテストと実測報告 | 適合。既存テストを調査 | 適合。欠落ケースと品質標本を検証ガイドに記載 |
| AI 能力は世界正本と分離し、障害を事実へ変換しない | 適合。Detail 失敗は本文を中断 | 適合。正常0件と失敗を明示状態で区別し、失敗時は完了を保存しない |

未解決のゲート違反はない。入口によって挙動を分けず、共有エンジンに統合することを設計条件とする。

## Project Structure

### Documentation (this feature)

```text
specs/001-body-concretization/
├── spec.md
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/article-body-read.md
└── checklists/requirements.md
```

### Source Code (repository root)

```text
apps/server/internal/world/models.go
apps/server/internal/worldrepo/materialization_interactive_detail.go
apps/server/internal/worldrepo/materialization_worker_context.go
apps/server/internal/worldrepo/materialization_article_debug.go
apps/server/internal/worldrepo/observation.go
apps/server/internal/worldrepo/materialization_worker_context_test.go
apps/server/internal/worldrepo/materialization_interactive_latency_test.go
apps/server/internal/llm/bbs_title_article_details.go
apps/server/internal/llm/bbs_title_article_details_test.go
apps/server/internal/hostprogram/erikak/runtime.go
apps/server/internal/hostprogram/erikak/runtime_test.go
apps/server/cmd/server/materialization_lab_fresh.go
apps/web/src/poc/MaterializationLabViewerPage.tsx
```

**Structure Decision**: `materialization_interactive_detail.go` の title-first 固有処理を、同じ `worldrepo` 内の記事共通処理へ一般化する。共有層が Detail の状態・入力・保存を所有し、各ホストは記事表示と失敗文言だけを所有する。公開 API は追加しない。

## Design Sequence

1. `PostIntent.ArticleDetailsMaterialized` を追加し、スナップショット複製・保存・復元・デバッグ出力で保持されることを確認する。旧データの欠落値は未処理として扱う。
2. title-first 固有の `materializeInteractiveTitleArticleDetails` を、本文未確定の選択済み記事へ適用する共有 Article Detail 処理へ一般化する。件名の有無や応答先から意味上の件名を組み立てる既存規則は維持する。
3. planner が正常に0件を返した場合も完了状態を立て、詳細と完了状態を一度の `UpdatePost` で保存する。保存できなければエラーとし、本文 renderer を呼ばない。planner 不在・生成失敗・検証失敗も本文を中断し、完了状態を保存しない。
4. 保存済み完了状態があれば Detail を再実行しない。未処理で本文のない旧記事は初回閲覧時に処理する。既に本文がある記事は互換性のため再具体化しない。
5. 通常閲覧・開発用直接確認・Lab のすべてが `MaterializationArticleWithDebug` から同じ共有処理を通るようにし、入口固有の Article Detail 分岐を除く。Lab の隔離、ホストの操作系・表示系は維持する。
6. 人物履歴、時点人物事実、Detail 入力、0件完了、生成・保存失敗、再閲覧・再起動、並行閲覧、件名保持、メタデータ拒否、Erika-K 失敗表示の状態遷移を検証する。
7. 同じ共有エンジンを通る通常閲覧と Lab の結果を集め、複数返信スレッドと抽象件名の記事をレビューする。標本が少ない場合は実数と制約を記録し、意味の反復、一人称事実の根拠、背景の話題化、未来漏洩を記事ごとに判定する。

## Agent Context

`.specify/scripts/` に agent context 更新スクリプトが存在しないため、自動更新は実行できない。既存の `AGENTS.md` を作業規則として参照する。
