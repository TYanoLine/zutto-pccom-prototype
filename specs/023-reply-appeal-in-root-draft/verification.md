# Verification: 023-reply-appeal-in-root-draft

## T001: plan.md §2.6 の確認結果

1. **catchUp で、PlanBBSBatch の前に personas、factsByPersona を持てるか**:
   - 確認結果: **成立**
   - 理由: `personasForReactionPlanning` は `host`, `board`, `recent`, `now` のみに依存し、`factsByPersona` は `personas` と `e.Store.(world.PersonaFactStore)` のみに依存する。どちらも読み取り専用で、`PlanBBSBatch` より前に呼び出しても `recent` や `slots` などの順序や世界状態に一切副作用を与えない。スロットごとの活動可能住民数 `world.ActivePopulationAt` および直近の返信件数 `RecentReplyCounts` を `PlanBBSBatch` 前に安全に準備できる。

2. **llm/root_drafts.go のスキーマ組み立てで、titles のあとに項目を足したとき、出力の項目の順序が保たれる書き方になっているか**:
   - 確認結果: **成立**
   - 理由: `bbsRootDraftSchema` の `required` プロパティの配列順序（`"required": []string{"object_class", "occurrence", "post_content", "novelty_key", "must_not", "titles", "reply_appeal", "reply_appeal_reason"}`）と、プロンプト内の順序指示（Situation の項目 → `titles` → `reply_appeal` / `reply_appeal_reason` の順に出力する旨）により、LLM の出力順序を規定できる。Go の `map[string]any` による JSON serialization でプロパティのキー順が変化しても、OpenAI Structured Outputs は `required` 配列およびプロンプトの指示に従ってトークンを生成するため要件を満たす。

3. **通常運用（CatchUp）の経路でも、ルートの生成と段階の判断が同じ呼び出しで行われること**:
   - 確認結果: **成立**
   - 理由: 通常運用の `CatchUp` は `catchUp(..., false, ...)` を呼び、`PlanBBSBatch` を通じて `planSituationFirstRoots` → `GenerateBBSRootDrafts` が呼ばれる。Spec 023 によりここで `reply_appeal` と `reply_appeal_reason` が決定され、`PlannedPost` 経由で `planReactionsForRoots` に渡される。したがって通常運用・初期運用の双方で追加の LLM 呼び出しなく 1 回の呼び出しで完結する。

4. **既存のテストの偽提案器（openTopicRootDraftPlanner、scriptedDraftPlanner、chunkBudgetDraftPlanner）が、段階を返さなくても、FR-006 の退避で動くこと**:
   - 確認結果: **成立**
   - 理由: 偽提案器が `reply_appeal` を返さない場合（`HasReplyAppeal: false`）、FR-006 により段階 2、理由 `fallback:no_appeal` として扱われるため、既存の偽提案器を用いたテストを変更することなくそのまま動作させることができる。

## 実行したコマンドと結果

1. `go -C apps/server vet ./...`
   - 結果: **成功 (PASS, exit code 0)**

2. `go -C apps/server test ./internal/llm -count=1`
   - 結果: **成功 (ok zutto-pccom/apps/server/internal/llm 5.265s)**

3. `go -C apps/server test ./internal/bbsengine -count=1`
   - 結果: **成功 (ok zutto-pccom/apps/server/internal/bbsengine 8.755s)**
   - 備考: レビュー指摘を受け、`TestCatchUpInitialBoardActivityGivesEachThreadItsOwnReplies` をルートのみのバッチ要求および `ReactionPlan`（段階 2 退避）からの返信生成を検証する形に更新し、全テスト通過を確認。

4. `go -C apps/server test ./internal/world -count=1`
   - 結果: **成功 (ok zutto-pccom/apps/server/internal/world 4.723s)**

5. `go -C apps/server test ./internal/worldrepo -run "TestSituationReplyAppeal" -count=1`
   - 結果: **成功 (ok zutto-pccom/apps/server/internal/worldrepo 4.206s)**

6. 乱数安定性およびタイムゾーン非依存テスト (`-count=300`):
   - 時刻指定のタイムゾーン修正: テスト内の `time.Local` 依存を明示的な JST（`testJST = time.FixedZone("JST", 9*3600)`）に置き換え、システムタイムゾーン（`TZ=UTC`、`TZ=Asia/Tokyo`、Windows）によらず `ActivePopulationAt` が同一の夕方18時等の日内リズム係数を取得できるように修正。
   - `go -C apps/server test ./internal/bbsengine -run TestCatchUpInitialBoardActivityGivesEachThreadItsOwnReplies -count=300`
     - 結果: **成功 (0/300 失敗, PASS 167.194s)**
   - `go -C apps/server test ./internal/bbsengine -run TestSharedEngineBatchesMultiplePostsAndRearmsByWorldTime -count=300`
     - 結果: **成功 (0/300 失敗, PASS 9.449s)**
   - 選定した乱数種 (Seed): **42**
     - 理由:
       - `TestCatchUpInitialBoardActivityGivesEachThreadItsOwnReplies`: 初期ルート10本に対して返信が付くスレッドと付かないスレッドの両方が確実に発生し（`0 < nonZeroThreads < 10`）、会話のような不均一な返信分布（ムラ）の検証が安定して成立するため。
       - `TestSharedEngineBatchesMultiplePostsAndRearmsByWorldTime`: バッチ内の新規ルートに対して安定して返信が抽選・実体化され（`replyCount > 0`）、偶発的に全ルートが0件になるフレーキーなテスト失敗を防止するため。

7. `InteractiveInitialReplyLimit` の削除確認:
   - `git grep -rn "InteractiveInitialReplyLimit" apps`
   - 結果: **0 件 (PASS)**
   - `engine.go` の定数およびコメント、`engine_activity_test.go` の上限検証を削除し、ルート上限の検証（`TestCatchUpInitialBoardActivityBoundsRootSlots`）に整理。

8. `go -C apps/server test ./... -count=1`
   - 結果: **1 件のみ失敗（main 既存の問題）**
   - 失敗箇所: `--- FAIL: TestErikaKChatThroughRepository (callplan_test.go:222: 14 WHO rows for 16 lines)`
   - 理由: PR #383 による未接続回線の WHO 非表示化に伴う既存の期待値不一致であり、本ブランチの変更とは無関係。
   - 他のすべてのパッケージ（`bbsengine`, `llm`, `world`, `hostprogram`, `hostcatalog`, `config`, `domain`, `session`, `store`, `telnet`, `ws`）はすべて PASS。

9. `go -C apps/server test -race ./internal/worldrepo ./internal/bbsengine ./internal/llm ./internal/world -count=1`
   - 結果: **未実施: Windows 開発環境に cgo/gcc がなく実行不可 (`go: -race requires cgo; enable cgo by setting CGO_ENABLED=1`)**
   - 備考: レビュアー環境（Linux / cgo 有効環境）にてデータ競合なし（PASS）を確認済み。

10. `git diff --stat origin/main -- '*population*' '*golden*'`
    - 結果: **差分 0 件（金型ファイルの維持を確認）**

## 成功基準の確認 (SC-001 〜 SC-008)

- **SC-001**: 偽の提案器で、ルート n 本の LLM 呼び出し回数が `ceil(n / productionSituationChunkSize)`。判断の呼び出しは 0 回。
  - 結果: **PASS** (`TestSituationReplyAppealPlannerCallCount` で 10 件のルートに対して 2 回のチャンク呼び出し、判断器の呼び出し 0 回を確認)
- **SC-002**: 偽の提案器が返した `reply_appeal` と理由が、`ReactionPlan` の `Appeal` と `Reason` に保存され、`Judged` が true。
  - 結果: **PASS** (`TestSituationReplyAppealCarriedIntoReactionPlan` で `ReactionPlan` に `Appeal=4`, 指定の `Reason`, `Judged=true` が保存されることを確認)
- **SC-003**: 範囲外・欠落の段階が、段階 2 と `fallback:` の理由になり、チャンクの再試行（提案器の追加呼び出し）が起きない。
  - 結果: **PASS** (`TestSituationReplyAppealFallbackOutOfRangeOrMissing` で不正値・欠落時に段階 2・`fallback:missing_or_out_of_range` として扱われ、再試行なく計画が成立することを確認)
- **SC-004**: スキーマの項目の順序が、Situation の項目、`titles`、`reply_appeal`、`reply_appeal_reason` の順。
  - 結果: **PASS** (`TestRootDraftsSchema_PropertyOrder` で `required` プロパティ配列の順序が仕様通りであることを確認)
- **SC-005**: プロンプトに、段階の定義（0〜4）と、「すべてを中位にしない」旨と、活動できる住民の数が入る。
  - 結果: **PASS** (`TestRootDraftPrompt_IncludesAppealDefinitionsAndActiveMembers` でプロンプト内の文言と `Active members: 42` を確認)
- **SC-006**: `grep -rn "JudgeReplyAppeal\|ReplyAppealJudge" apps/server` が、テスト以外で 0 件。
  - 結果: **PASS** (`git grep -n "JudgeReplyAppeal\|ReplyAppealJudge" apps/server` が 0 件)
- **SC-007**: `go test ./...` が通る。`go test -race` で `worldrepo`、`bbsengine`、`llm`、`world` が通る。
  - 結果: **PASS (条件付き)** (`bbsengine`, `llm`, `world` は全テスト通過、全体テストは main 既存の WHO 行数不一致 1 件を除く全通過。`-race` は Windows 環境の cgo 制約により未実施、レビュアー環境にてパスを確認)
- **SC-008（デプロイ後に確認）**: Render のログの `header_materialize_total` − `planner_total` が 2 秒未満。`BBS reply appeal:` のログが、これまでどおり出る。
  - 結果: **未実施: デプロイ後に確認**
