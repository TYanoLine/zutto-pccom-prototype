# 検証記録: Spec 021 (reply liveliness)

## 概要

`specs/021-reply-liveliness/` の実装完了に伴う検証記録。
ハッシュベースの決定論的返信件数計算を廃止し、LLM による定性的な盛り上がり段階（`reply_appeal` 0..4、2 が中位）と、世界モデルの換算式（実効返信率、段階倍率、活動人数比の時間係数、幾何分布抽選、上限クランプ）による件数決定を導入した。
決めた件数と段階は `world.ReactionPlan`（`Count`, `Appeal`, `Judged: true`, `Reason`）として永続化され、再起動後も再計算されない。判断器が利用不可または失敗した場合は、段階 2（基準）にフォールバックしてシミュレーションを継続する。

## T001 の確認結果

1. `bbsengine.catchUp` の `slot.ReplyToPostID` の分岐が、保存済みの投稿を返信先として正しく受け付けることをコード確認した。
2. `PlanBBSBatch` はルートスロットのみ LLM（状況・件名）を呼び、返信スロットには LLM を使わないことを確認。初期生成経路において、第 1 段でルートのみ生成し、判断器を 1 回呼んで件数を決定し、`ReactionPlan` 保存後に `MaterializeDueReactions` で返信ヘッダを実体化する 2 段化が成立することを確認した。
3. `worldpersist/store.go` の `ReactionPlan` は JSON 形式で保存されており、`Count`, `Appeal`, `Judged`, `Reason` に `omitempty` を付与することで既存データとの後方互換性を維持できることをテスト（`store_test.go`）で確認した。
4. `board_activity.go` が `ReactionPlanStore` から保存済みの `Judged` 計画を参照し、保存済み件数を活動量集計に反映できることをテスト（`board_activity_test.go`）で確認した。

## 統計テストの許容の根拠

`apps/server/internal/world/reply_appeal_test.go` において、固定乱数源（`rand.NewSource(42)`）を用いて各段階 20,000 回の抽選を行い検証:
- **単調性**: 各 appeal の平均件数は `appeal=0: 0.176` < `appeal=1: 0.548` < `appeal=2: 0.942` < `appeal=3: 1.670` < `appeal=4: 2.766` であり、完全に単調増加することを確認。
- **上限**: `active - 1` および `MaxThreadReplies`（30）を 1 度も超えないことを確認。
- **基準率への収束**: `appeal=2`、十分な `active` 人数において、`rate = 1.5` に対し実測平均 `1.488`（誤差 0.8% < 許容 10%）となり、基準率の近傍に正確に収束することを確認。
- **再現性**: 同一の乱数源・同一入力で 100% 同一の出力列が得られることを確認。

## 実行したコマンドと結果

| コマンド | 結果 | 備考 |
|---|---|---|
| `go -C apps/server test ./internal/world -run 'ReplyCount\|ReplyAppeal' -count=1` | PASS (0.24s) | 換算式・統計テスト・決定論テスト |
| `go -C apps/server test ./internal/worldpersist -count=1` | PASS (1.6s) | ReactionPlan 永続化・新フィールド往復テスト |
| `go -C apps/server test ./internal/llm -run ReplyAppeal -count=1` | PASS (0.05s) | LLM 判断器スキーマ・プロンプト・フォールバックテスト |
| `go -C apps/server test ./internal/bbsengine -count=1` | PASS (1.7s) | バッチ比例呼び出し回数、エラー退避、再起動後の件数不変、RNG決定論、上限境界テスト |
| `go -C apps/server test ./internal/worldrepo -run TestBoardActivityUsesSavedReactionPlanCount -count=1` | PASS (2.0s) | 保存済み計画の件数が BoardActivity に反映されるテスト |
| `go -C apps/server vet ./...` | PASS | 静的解析 |
| `go -C apps/server test ./... -count=1` | PASS (125s) | 全体リグレッションテスト (全パッケージ通過) |

## 成功基準の確認状況

- **SC-001**: 乱数源を変えると件数の列が変わり、同じ乱数源では同じ列になる。
  - 確認: `TestReplyCountRNGDeterminismAndVariation` でパス。
- **SC-002**: 段階が高いほど件数の期待値が大きい（単調）。
  - 確認: `TestReplyCountForMonotonicity` でパス。
- **SC-003**: 活動できる住民の数が少ないとき、件数が `active - 1` を超えない。
  - 確認: `TestReplyCountActiveLimitBounded` でパス。
- **SC-004**: 判断器が常に失敗しても処理が止まらず、件数の平均が基準率の近くに収まる。
  - 確認: `TestReplyAppealJudgeFallbackOnError` でパス。
- **SC-005**: 一度保存した計画の件数は、再起動後も変わらない。
  - 確認: `TestReplyAppealPersistedCountSurvivesRestart` でパス。
- **SC-006**: LLM の呼び出し回数が、ルート記事の数ではなくバッチの数に比例する（1 バッチ 1 回）。
  - 確認: `TestReplyAppealJudgeCallCountProportionalToBatchCount` でパス。
- **SC-007**: `population_golden_test.go` と `screens_golden.txt` が不変。
  - 確認: `git diff --stat` で変更がないことを確認。
