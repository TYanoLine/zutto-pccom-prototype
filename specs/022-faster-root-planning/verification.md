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

- **SC-001**: 偽の提案器の遅延テストで、N チャンクの所要時間が、直列の合計より短く、最大 1 チャンクの 2 倍以内。
  - 確認: `TestParallelSituationChunksTotalDuration` でパス（2チャンク並列で直列合計より短縮）。
- **SC-002**: 同じ `novelty_key` を返す偽の提案器で、バッチが成功し、提案器の呼び出し回数が増えない。
  - 確認: `TestDuplicateNoveltyKeyInChunkDoesNotReject` でパス。一致してもリジェクトせずログ警告を記録し採用。
- **SC-003**: 並列のチャンクの結果が、スロットの順序どおりに並ぶ。同じ入力で、同じ順序（偽の提案器は決定的）。
  - 確認: `TestParallelSituationChunksOrderAndDeterminism` でスロット順序と決定性がパス。
- **SC-004**: 出力の切れ（`ErrBBSWorldSituationOutputTruncated`）で分割したチャンクが、並列のもとでも、全ルートを受理する。
  - 確認: `TestOutputBudgetExhaustionSplitsChunk` で分割・結合・全ルート受理がパス。
- **SC-005**: `go test ./...` が通る。
  - 確認: `apps/server` 内の全テストが通過。
- **SC-006（デプロイ後に確認）**: Render のログの `planner_total` が、改善前（31〜55 秒）より下がる。
  - **未実施: デプロイ後に確認**
- **SC-008**: 偽の統合器で、ルート n 本の LLM 呼び出しの回数が `ceil(n/6)`。
  - 確認: `TestRootDraftCallCountIsCeilNOverSix`（1, 6, 7, 12, 13 スロット）で全サブテストがパス。
- **SC-009**: 統合した出力の件名が不正なチャンクで、そのチャンクが再試行され、再試行で正しい出力が得られたら、すべてのルートが受理される。他のチャンクの結果は変わらない。
  - 確認: `TestInvalidTitleTriggersRetryAndSucceeds`、`TestChunkFailureRetriesUpToThreeTimes` でパス。
- **統合した件名の質**:
  - **未評価: 人の目で、デバッグトレースと `BBS title shape` のログから確認する**（実装者は評価しない）


## 変更・修正点

- 旧 `openai_world_situation_proposer.go` および `situation_titles.go` を削除し、`root_drafts.go` に統合。
- `bbs_situation_chunk_retry_test.go` において、旧仕様のチャンクサイズ縮退を引き継ぐ動作がなくなったため、現行の固定チャンクサイズ＋局所二分割＋リトライ仕様に合わせて調整。
