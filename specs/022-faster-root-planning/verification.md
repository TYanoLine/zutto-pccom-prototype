# 検証記録: Spec 022 (faster root planning)

## 概要

`specs/022-faster-root-planning/` の実装完了に伴う検証記録。
Situation（状況）と件名（タイトル）の 2 段階 LLM 呼び出しを、1 回の構造化出力（`BBSRootDraftPlanner`）に統合し、トップレベルの並列実行（チャンクサイズ 6、並列度 4）、時間計測ログ（`phase=situation_chunk`）、重複ログ記録（リジェクトなし）、および出力トークン枯渇時（`ErrBBSWorldSituationOutputTruncated`）の局所的二分割リトライを実装した。

## 実行したコマンドと結果

| コマンド | 結果 | 備考 |
|---|---|---|
| `go -C apps/server test ./internal/llm -count=1` | PASS | `BBSRootDraftPlanner`、スキーマ、プロンプト、切り捨て検出、タイトルバリアント生成のテスト |
| `go -C apps/server test ./internal/worldrepo -count=1` | PASS (50.9s) | ルートドラフト統合、チャンク分割、並列実行、リトライ、タイトル選択、フォールバックの全テスト |
| `go -C apps/server test ./internal/bbsengine -count=1` | PASS (1.7s) | BBS エンジンとの結合確認 |
| `go -C apps/server vet ./...` | PASS | 静的解析 |
| `go -C apps/server test ./... -count=1` | PASS (125s) | 全体リグレッションテスト (全パッケージ通過) |

## 成功基準の確認状況

- **SC-001**: 正常系でのプロバイダ呼び出し回数が $\lceil N / 6 \rceil$ 回。
  - 確認: `TestRootDraftCallCountIsCeilNOverSix`（1, 6, 7, 12, 13 スロット）で全サブテストがパス。
- **SC-002**: 出力トークン予算枯渇（`ErrBBSWorldSituationOutputTruncated`）時に局所的な二分割で救済。
  - 確認: `TestOutputBudgetExhaustionSplitsChunk` で分割・結合がパス。
- **SC-003**: チャンク失敗時の再試行（最大 3 回）が機能。
  - 確認: `TestChunkFailureRetriesUpToThreeTimes`、`TestInvalidTitleTriggersRetryAndSucceeds` でパス。
- **SC-004**: 新規キー重複時にリジェクトせずログ警告を記録し、後続の同一キーのみ別ドラフトを採択。
  - 確認: `TestDuplicateNoveltyKeyInChunkDoesNotReject` でパス。
- **SC-005**: チャンク単位の所要時間がログに記録される。
  - 確認: `log.Printf("BBS timing: host=%s board=%s phase=situation_chunk duration=%s events=%d", ...)` により `phase=situation_chunk` が出力されることを確認。
- **SC-006**: デプロイ後の `planner_total` の短縮。
  - **未実施: デプロイ後に確認**
- **統合した件名の質**:
  - **未評価: 人の目で、デバッグトレースと `BBS title shape` のログから確認する**（実装者は評価しない）
- **SC-007**: `population_golden_test.go` と `screens_golden.txt` が不変。
  - 確認: `git diff --stat` で変更がないことを確認。

## 変更・修正点

- 旧 `openai_world_situation_proposer.go` および `situation_titles.go` を削除し、`root_drafts.go` に統合。
- `bbs_situation_chunk_retry_test.go` において、旧仕様のチャンクサイズ縮退を引き継ぐ動作がなくなったため、現行の固定チャンクサイズ＋局所二分割＋リトライ仕様に合わせて調整。
