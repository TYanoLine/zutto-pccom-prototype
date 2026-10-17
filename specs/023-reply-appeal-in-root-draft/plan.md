# Plan: 返信の盛り上がりの判断を、ルート記事の生成に含める

**Spec**: `spec.md`  
実際のコード（`main` の `a53cb40`）を読んで確かめた事実と、実装で確かめるべき点を分けて書く。

## 1. 現状（コードで確認した）

| 場所 | 内容 |
|---|---|
| `llm/root_drafts.go` | `BBSRootDraftRequest`、`BBSRootDraft`（`Titles` まで）、`bbsRootDraftWire`、`GenerateBBSRootDrafts`（`bbsWorldSituationMaxTokens`: ルート 1 本 2000 トークン、4000〜12000） |
| `llm/reply_appeal.go` | `buildReplyAppealPrompt`（段階の定義 0〜4、メリハリの指示、時刻・人数・直近の傾向）、`replyAppealSchema`、`JudgeReplyAppeal`（`responseTextWithJSONSchema(..., "low", 2500, ...)`） |
| `world/reply_appeal.go` | `ReplyAppealRequest`、`AppealRoot`、`ReplyAppealResult`、`ReplyAppealJudge`、`ActivePopulationAt`、`ReplyCountForWithStandard` |
| `bbsengine/engine.go` `planReactionsForRoots` | 保存済みのルートを受け取り、`ActivePopulationAt` で人数を数え、`e.ReplyAppealJudge.JudgeReplyAppeal` を呼び、`appealMap`、`reasonMap` を作り、`ReplyCountForWithStandard` で件数を決め、`PlanReactions` と `SaveReactionPlan` |
| `bbsengine/engine.go` `catchUp` | `Planner.PlanBBSBatch` のあとにルートを保存（`newRoots`）。そのあと `planReactionsForRoots`（初期生成、通常運用の両方） |
| `worldrepo/bbs_situation_first_planner.go` `planSituationFirstRoots` | チャンクごとに `GenerateBBSRootDrafts` を並列に呼び、`PlannedPost` を作る（`productionSituationChunkSize = 5`） |
| `worldrepo/repository.go` | `replyAppealJudgeFromMaterializer`、`SetReplyAppealJudge`（`r.bbsArticles.ReplyAppealJudge = ...`） |

## 2. 設計

### 2.1 ルート生成の出力に足す（FR-001）

- `llm.BBSRootDraft` に `ReplyAppeal int`、`ReplyAppealReason string`、`HasReplyAppeal bool` を足す。
  `HasReplyAppeal` は、出力に項目があり、範囲内（0〜4）だったとき true（FR-003 の退避の判別に使う）。
- `bbsRootDraftWire` に `ReplyAppeal *int`（`json:"reply_appeal"`）、`ReplyAppealReason string`（`json:"reply_appeal_reason"`）を足す。
- スキーマ（`GenerateBBSRootDrafts` の構造化出力）は、ルートごとのオブジェクトで、`titles` の**あとに** `reply_appeal`（整数、0〜4）と `reply_appeal_reason`（文字列）を `required` に足す。
  （`additionalProperties: false` の既存の流儀に合わせる。要確認: 既存のスキーマの組み立て方を読み、項目の順序が保たれる書き方にする。）
- 出力トークンの上限: 現状ルート 1 本あたり 2000。足す出力は 1 本あたり約 60〜100 トークンなので、`bbsWorldSituationMaxTokens` は変えない。
  切れ（`ErrBBSWorldSituationOutputTruncated`）が増えないことを、実装後のログ（`splitting local chunk`）で確かめる。

### 2.2 判断の材料を入れる（FR-002）

- `BBSRootDraftRequest` に、次を足す。

  ```go
  BaseReplyRate    float64        // 板の実効返信率
  Members          int            // 局の会員数
  RecentReplyCounts []int         // 板の直近のスレッドの返信件数（最大 10）
  ```

  ルートごとの活動人数は、`BBSWorldWindowEvent` に `ActiveResidents int`（`json:"active_residents,omitempty"`）を足し、`compactSituationEvent` にも同じ項目を足す
  （プロンプトに渡るイベントの JSON に入る）。
- 値の出どころ（`bbsengine`）: `bbsengine.BatchRequest` に `BaseReplyRate float64`、`RecentReplyCounts []int`、`ActiveBySlot map[int]int` を足す。
  `catchUp` が、`Planner.PlanBBSBatch` を呼ぶ**前**に、ルートのスロットごとに `world.ActivePopulationAt(host, board, personas, factsByPersona, slot.CreatedAt)` の `active` を数えて、`ActiveBySlot` に入れる。
  `personas`、`factsByPersona` は、`catchUp` が持っている（`personasForReactionPlanning` ほか）。呼び出しの位置が合わなければ、位置を直す。
  `RecentReplyCounts` は、現在の `planReactionsForRoots` が作っている処理（板のルートの `ReactionPlan.Count`、最大 10）を、関数に切り出して再利用する。
- `worldrepo.planSituationFirstRoots` が、`BatchRequest` の値を、`BBSWorldWindowEvent.ActiveResidents`（`ActiveBySlot[slot.Index]`）と `BBSRootDraftRequest` の項目へ写す。
- プロンプト: `llm/reply_appeal.go` の `buildReplyAppealPrompt` から、段階の定義と重要な指示の文言を、ほぼそのまま移す（関数として切り出せるなら切り出す）。
  「各ルートごとに 1 文の簡潔な理由」は、ルート生成の出力項目の説明に合わせて短く直す。「件名と状況要約は、あなたが先に書いた Situation と件名を指す」旨を 1 文足す。

### 2.3 検証と退避（FR-003）

`worldrepo/bbs_situation_first_planner.go` の `validateProductionDraft` は変えない。段階の範囲外・欠落は、`llm` 層（`GenerateBBSRootDrafts` の出力の整形）で、
`HasReplyAppeal=false`、`ReplyAppeal=2`、`ReplyAppealReason="fallback:..."` にして返す。**エラーにしない**（チャンクの再試行を起こさない）。ログに 1 行残す。

### 2.4 保存までの受け渡し（FR-004）

- `bbsengine.PlannedPost` に `ReplyAppeal int`、`ReplyAppealReason string`、`HasReplyAppeal bool` を足す。
- `worldrepo.planSituationFirstRoots` が、`BBSRootDraft` から `PlannedPost` へ写す。
- `bbsengine.catchUp` は、保存した（`saved`）ルートの `ID` に対して、`planned[slot]` の段階と理由を、`map[int64]appealDecision` に覚える（`newRoots` の追加と同じ場所）。
- `planReactionsForRoots` の引数に、この `map[int64]appealDecision` を足す。段階の決め方:
  1. マップにあるルート → その段階と理由。
  2. ないルート（FR-006）→ 段階 2、理由 `fallback:no_appeal`。判断器は呼ばない。
- `planReactionsForRoots` の `ReplyAppealJudge` を呼ぶ分岐、`appealRoots`、`postedAt`、`recentCounts` の組み立て（判断器のためだけの部分）は削除する。
  人数（`activeByRoot`、`standardByRoot`）の数え方、件数の抽選、`PlanReactions`、`SaveReactionPlan`、`BBS reply appeal:` のログは、そのまま。

### 2.5 旧い経路の削除（FR-005）

削除するもの: `llm/reply_appeal.go`（プロンプトの文言は 2.2 で移したあと）、`world.ReplyAppealJudge` ほかの型、`bbsengine.Engine.ReplyAppealJudge`、
`worldrepo.replyAppealJudgeFromMaterializer`、`Repository.SetReplyAppealJudge`、およびそれらを使うテスト・偽実装（新しい経路のテストへ移す。アサーションの意図は保つ）。

### 2.6 要確認の点（T001）

1. `catchUp` で、`PlanBBSBatch` の前に `personas`、`factsByPersona` を持てるか（今は後で読んでいる）。位置を前に出すことで、他の処理（`recent` など）の順序が変わらないこと。
2. `llm/root_drafts.go` のスキーマの組み立てで、`titles` のあとに項目を足したとき、出力の項目の順序が保たれる書き方になっているか（`map[string]any` は順序を持たない。
   既存の `properties` の順序の扱いを読み、文字列の順序が問題になるなら、`required` の並びと説明で補う）。
3. 通常運用（`CatchUp`）の経路でも、ルートの生成と段階の判断が同じ呼び出しで行われること。
4. 既存のテストの偽提案器（`openTopicRootDraftPlanner`、`scriptedDraftPlanner`、`chunkBudgetDraftPlanner`）が、段階を返さなくても、FR-006 の退避で動くこと。

1〜3 のどれかが成り立たない場合は、**止まって、PR の説明に書く**（`AGENTS.md`: spec とコードが食い違うときは、推測で別の設計を選ばない）。

## 3. 検証の方針

- `llm`: スキーマの項目の順序（`reply_appeal` が `titles` のあと）、範囲外・欠落の退避、プロンプトに段階の定義と「中位にしない」旨と人数が入ること。
- `bbsengine`: 偽の提案器が段階と理由を返し、`ReactionPlan` に保存されること（SC-002）。段階が運ばれないルートの退避（FR-006）。
  判断器の呼び出しがないこと（インターフェースが存在しない）。
- `worldrepo`: 偽の提案器で、呼び出し回数が `ceil(n / productionSituationChunkSize)`（SC-001）。範囲外の段階でチャンクが再試行されない（SC-003）。
- `go test -race ./internal/worldrepo ./internal/bbsengine ./internal/llm ./internal/world`。
- 金型は作らない。人物生成と Erika-K の画面の金型は、変わってはならない。
- デプロイ後に、Render のログで `header_materialize_total` − `planner_total` を確かめる（SC-008）。

## 4. 変更するファイル

| ファイル | 内容 |
|---|---|
| `apps/server/internal/llm/root_drafts.go` | 型、スキーマ、プロンプト、整形（退避）に `reply_appeal` を足す。判断器のプロンプトの文言を移す |
| `apps/server/internal/llm/reply_appeal.go`、`reply_appeal_test.go` | 削除（文言は `root_drafts.go` へ、テストは新しい経路へ） |
| `apps/server/internal/llm/root_drafts_test.go` | スキーマの順序、退避、プロンプト |
| `apps/server/internal/world/reply_appeal.go` | 旧い型とインターフェースの削除（`ActivePopulationAt`、換算式は残す） |
| `apps/server/internal/bbsengine/engine.go` | `PlannedPost`、`BatchRequest` の項目、`catchUp` の人数・直近件数の用意、`planReactionsForRoots` の分岐の削除、受け渡し |
| `apps/server/internal/bbsengine/engine_reply_appeal_test.go` | 新しい経路のテストへ移す |
| `apps/server/internal/worldrepo/bbs_situation_first_planner.go` | 材料の受け渡しと、`PlannedPost` への写し |
| `apps/server/internal/worldrepo/repository.go` | `replyAppealJudgeFromMaterializer`、`SetReplyAppealJudge` の削除 |
| `apps/server/internal/worldrepo/*_test.go` | 偽実装・期待値の更新 |
| `docs/WORLD_SIMULATION.md`、`docs/DEBUG_INSPECTION.md` | 判断の場所が変わったこと、確認するログ |

## 5. リスクと対策

| リスク | 対策 |
|---|---|
| 判断の質（メリハリ）が、別呼び出しの時より下がる | 同じ文言を引き継ぐ。デプロイ後に `BBS reply appeal:` のログで、段階の分布（0〜4 が出るか）を見る。問題があればリバートする |
| ルート生成の出力が増え、切れが増える | 1 本あたり約 60〜100 トークンの増加。`splitting local chunk` のログで確かめる |
| スキーマの項目の順序が保たれず、判断が Situation より先に出る | 2.6 の 2 で確かめる。保てない場合は止まって報告 |
| 段階が運ばれない経路（移行期の保存データ）で件数が基準に寄る | FR-006 の退避（段階 2）は、過去の判断器の失敗時の退避と同じ。ログに `fallback:no_appeal` を出す |
| 旧い経路の削除で、テストの偽実装が壊れる | 新しい経路の偽実装へ移す。意図を保てないものは、削除せずに止まって報告 |
