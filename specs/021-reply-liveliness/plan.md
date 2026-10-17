# Plan: スレッドごとの盛り上がり（レス件数）

**Spec**: `spec.md`  
書いた時点で、実際のコードで確認できたものと、**実装の最初に確かめるべきもの**を分けて書く。確かめていないものは「要確認」と明記する。

## 1. 現状（コードで確認した）

| 場所 | 何をしているか |
|---|---|
| `world/thread_replies.go` `ThreadReplyCount` | 局 ID・板 ID・通し番号・`rate` のハッシュで件数を決める。0 件の確率は `exp(-0.55·rate)`、そうでなければ平均 `rate/(1-silent)` の幾何分布、上限 `MaxThreadReplies`（30） |
| `bbsengine/engine.go` `quotasForTotalSlotTarget` | 総スロット数の目標から、ルートごとの件数の枠（quota）を並べる |
| 同 `CatchUpInitialRootHistoryCount` | ルート n 本分の quota を並べる |
| 同 `CatchUpInitialBoardActivity` | 直近 10 ルートの quota。返信ヘッダの総数は `InteractiveInitialReplyLimit`（30）まで |
| `worldrepo/board_activity.go` | 板の活動量の集計のため、全ルートの件数を再計算して合計する |
| `world/reaction.go` `PlanReactions` | 反応の計画を作る。件数は `ThreadReplyCount`、または `ReplyCountOverride` |
| `bbsengine/engine.go` `catchUp` | `planSlots` でルートのスロットを作り、`addThreadReplies` で quota 分の返信スロットを足し、そのあと `Planner.PlanBBSBatch` で**全スロットの話題をまとめて**作る |
| 同 `planReactionsForRoot` | 通常運用の経路。ルートが保存されたあとに `PlanReactions` を呼ぶ（件数は `ThreadReplyCount`） |

重要な点: **初期生成では、件数の枠が、話題より前に決まる**。通常運用では、ルートの話題が決まったあとに計画を作る。

## 2. 設計

### 2.1 段階 `reply_appeal`

- 整数 0〜4。0 は「ほぼ返信を呼ばない」、2 は中位（基準）、4 は「長いやりとりになりやすい」。
- 判断器 `world.ReplyAppealJudge`（新規のインターフェース、`world` パッケージ）:

  ```go
  type ReplyAppealRequest struct {
      Host, Board   string              // 名前（ID ではなく表示名）
      BaseRate      float64             // 板の reply_rate（実効値）
      Roots         []AppealRoot        // 判断するルート。1 バッチで複数
      PostedAt      map[int64]time.Time // ルートごとの世界時刻
      Members       int                 // 局の会員数
      ActiveByRoot  map[int64]int       // その時刻に活動できる住民の数
      RecentCounts  []int               // その板の直近のスレッドの件数
  }
  type AppealRoot struct {
      ID      int64
      Subject string
      Summary string // 確定済み Situation の要約
  }
  type ReplyAppealResult struct {
      Appeal map[int64]int    // 0..4
      Reason map[int64]string // 1 文。ログ用。保存はするが、本文には使わない
  }
  type ReplyAppealJudge interface {
      JudgeReplyAppeal(context.Context, ReplyAppealRequest) (ReplyAppealResult, error)
  }
  ```

- LLM 実装は `llm` パッケージに置く（JSON スキーマ付きの構造化出力。既存の `StructuredOpenAIProvider` の流儀に合わせる）。
  プロンプトには、「件数を決めるのではなく、段階を判断する」「中位が基準で、全部を中位にしない」「質問・相談・トラブルは高め、独白・告知・日記は低め、
  時間帯と人数の影響も考える」を書く。1 バッチの段階の分布に偏りが出すぎるときの扱いは、実装後の観察で決める（要確認: 初期は制約を入れない）。
- 不正な出力（範囲外、欠落）は、そのルートだけ段階 2 として扱い、ログに記録する（FR-005）。

### 2.2 換算式（架空の調整値）

```
mult[appeal]   = {0: 0.2, 1: 0.6, 2: 1.0, 3: 1.7, 4: 2.6}
time_factor    = 活動できる住民の数 / 板の標準的な活動人数  （0.3〜1.5 に丸める）
mean_replies   = reply_rate × mult[appeal] × time_factor
silent         = exp(-0.55 × mean_replies)         // 既存と同じ形
answered_mean  = mean_replies / (1 - silent)
n              = silent で 0、そうでなければ幾何分布（平均 answered_mean）
n              = min(n, MaxThreadReplies, max(0, active - 1))
```

- 「板の標準的な活動人数」は、局の会員数から決まる（係数は `plan` の実装時に決める）。値は定数にせず、`world` の関数にまとめて、テストで固定する。
- 乱数源は `interface{ Float64() float64 }`（`math/rand.Rand` で満たせる）。テストでは固定の乱数源を渡す。
- `active - 1`: 同じ人が連続で返信しない規則（spec 008 FR-003）のもとで、活動できる住民が `active` 人のとき、スレ主を除いた人数が実質の上限。

### 2.3 データの流れ

**通常運用（ルートが保存されたあと）**

```
ルート保存 → planReactionsForRoot
           → judge.JudgeReplyAppeal（1 ルート、または同じ観測の中の複数ルート）
           → 換算式 → ReactionPlan.Count, ReactionPlan.Appeal を保存 → PlanReactions（担い手・時刻）
```

**初期生成（要確認: 実装の最初に確かめる）**

現状は、件数の枠が先、話題が後。ルートの話題が分からない時点では、段階を判断できない。次の方式を第一候補とする。

1. 第 1 段: ルートのスロットだけを `PlanBBSBatch` に渡し、話題を作って保存する。
2. 判断器を**1 回**呼び、保存済みのルート全体の段階を得る（FR-007）。
3. 換算式で quota を決める。
4. 第 2 段: 返信スロットを、`ReplyToPostID` で保存済みのルートを指す形で、もう一度 `PlanBBSBatch` に渡す。

要確認の点:
- 返信スロットが、保存済みの投稿を `ReplyToPostID` で指せるか。`engine.go` の `catchUp` には、`slot.ReplyToPostID` で既存の投稿を引く分岐があるので、できる見込み。
- 第 2 段の返信スロットの話題の質（先行する返信を見る）が、現状と同等か。
- 第 1 段と第 2 段に分けると、LLM の呼び出しが増える（バッチ 1 回 → 2 回 + 判断 1 回）。コストと所要時間を、実装後に測る。

第一候補が成り立たない場合（`PlanBBSBatch` が、ルートと返信を同じ呼び出しで作ることを前提にしている、など）は、**止まって、PR の説明に書く**
（`AGENTS.md`: spec とコードが食い違うときは、推測で別の設計を選ばない）。代替案は次のとおり:

- 代替 A: 件数の枠を先に決める現状を保ち、**話題に依存しない要素**（時間帯、人数、板の基準率）だけを反映する。話題の効果は、通常運用の経路だけに適用する。
- 代替 B: `PlanBBSBatch` の 1 回の呼び出しの中で、ルートごとの `reply_appeal` を LLM に出させ、返信スロットは第 2 呼び出しで足す。

### 2.4 保存

`world.ReactionPlan` に次の項目を足す（JSON は `omitempty`。既存の保存データを読めること）:

```go
Count  int    `json:"count,omitempty"`   // 決めた件数。0 件が「判断済み」を表せるよう、下の Judged と組で使う
Appeal int    `json:"appeal,omitempty"`  // 0..4
Judged bool   `json:"judged,omitempty"`  // 段階の判断（または既定への退避）を終えた
Reason string `json:"reason,omitempty"`  // 判断器の理由（1 文）
```

- `Judged` が true の計画は、件数を再計算しない。
- 保存先は `worldpersist/store.go` の既存の `ReactionPlanStore`。スキーマの変更は JSON の項目追加のみ（要確認: 保存形式が JSON か。`worldpersist` を読んで確かめる）。
- `board_activity.go` の件数の再計算は、保存済みの計画があればそれを使い、なければ基準の段階（2）で換算した期待値を使う。
  （板の活動量の集計は、全ルートの件数を必要とするが、すべてのルートに LLM 判断を行うわけではない。履歴の古いルートは、基準の段階で足りる。）

### 2.5 ハッシュの廃止の範囲

- `ThreadReplyCount(hostID, boardID, ordinal, rate)` は残す（既存のテストと、段階を使わない呼び出し元が使う）が、
  新しい `ReplyCountFor(rng, rate, appeal, active)` を足し、**新しい経路はこちらを使う**。
- 既存の関数を削除しない（スコープを小さく保つ）。呼び出し元を順に新関数へ移す。

## 3. 検証の方針

- 換算式は純関数にして、統計テスト（固定の乱数源、十分な回数）で、単調性（段階が高いほど期待値が大きい）、上限（`active - 1`、30）、
  0 件の割合を確かめる。
- 判断器は偽実装を使う。LLM のプロバイダを呼ばないテストで、`ReactionPlan` の保存と再計算の抑止を確かめる。
- 金型（変更前の出力の記録）は、**この spec では作らない**。件数の列が変わることが目的であり、変えてはいけない出力はない。
  ただし、人物生成の金型（`population_golden_test.go`）と、Erika-K の画面の金型（`screens_golden.txt`）は変わってはならない。既存のテストで確かめる。
- 実際の LLM の判断の質（偏り、段階の分布）は、Render のログと `docs/DEBUG_INSPECTION.md` の手順で、実装後に観察する。
  生成ログに、段階と理由を出す（FR-005 の退避もここに出す）。

## 4. 変更するファイル

| ファイル | 内容 |
|---|---|
| `apps/server/internal/world/reply_appeal.go`（新規） | 判断器のインターフェース、換算式、乱数源のインターフェース |
| `apps/server/internal/world/reply_appeal_test.go`（新規） | 換算式の統計テスト |
| `apps/server/internal/world/reaction.go` | `ReactionPlan` に項目を追加。`PlanReactions` が保存済みの `Count` を使う |
| `apps/server/internal/llm/reply_appeal.go`（新規） | LLM による判断器 |
| `apps/server/internal/llm/reply_appeal_test.go`（新規） | スキーマと不正出力の扱い |
| `apps/server/internal/bbsengine/engine.go` | 初期生成の 2 段化、通常運用の経路での判断 |
| `apps/server/internal/worldrepo/board_activity.go` | 保存済みの計画の件数を使う |
| `apps/server/internal/worldpersist/store.go` | 追加項目の保存（要確認） |
| `docs/WORLD_SIMULATION.md`、`docs/BOARD_ACTIVITY_PLANNING.md` | 件数の決め方を更新 |

## 5. リスクと対策

| リスク | 対策 |
|---|---|
| LLM が、段階を中位ばかりにする | プロンプトで、中位が基準であって、全部を中位にしないよう指示する。観察して調整する |
| 初期生成の LLM 呼び出しが増え、遅くなる | 判断は 1 バッチ 1 回（FR-007）。測って、問題なら代替 A に倒す |
| 初期生成の 2 段化で、返信の話題の質が落ちる | 第 2 段のプロンプトに、ルートの話題と先行する返信を渡す（現状の返信スロットと同じ材料） |
| 保存済みの計画が、新しい項目を持たない | `omitempty` で読めるようにし、`Judged` が false の計画は、通常どおり扱う |
| 板の活動量の集計と、実際の件数がずれる | 集計は保存済みの計画があればそれを使う。なければ基準の段階の期待値 |
