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

## 既存のテスト失敗の確認
- `go -C apps/server test ./internal/worldrepo -run TestErikaKChatThroughRepository -count=1`
  - 結果: `FAIL` (14 WHO rows for 16 lines: PR #383 による WHO 行数変更に伴う main 既存の不一致)
