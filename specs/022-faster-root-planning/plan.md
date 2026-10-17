# Plan: ルート記事の計画を速くし、話題の重複を拒否しない

**Spec**: `spec.md`  
実際のコードを読んで確かめた事実と、実装で確かめるべき点を分けて書く。

## 1. 現状（コードで確認した）

| 場所 | 内容 |
|---|---|
| `worldrepo/bbs_situation_first_planner.go` | `productionSituationChunkSize = 6`、`productionSituationChunkAttempts = 3`、`productionTitleChunkSize = 20` |
| 同 `planSituationFirstRoots`（約 196〜311 行） | チャンクごとに `proposer.GenerateBBSWorldSituationProposals` を直列に呼ぶ。検証（空の項目、`novelty_key` の重複）に失敗すると次の試行へ。`ErrBBSWorldSituationOutputTruncated` なら `chunkSize` を半分にして、**そのバッチの残りのチャンクにも**その小さい値を使う |
| 同（約 258〜275 行） | `normalizeSituationNoveltyKey` が空なら不採用。`noveltyOwners`（先のチャンク）と `chunkNovelty`（同じチャンク）に一致があれば不採用にし、`avoidSituations` に `DO NOT REUSE novelty_key=…` を足す |
| 同（約 189〜190 行） | `recentContext`、`avoidSituations` は、直近 20 件の件名から作る |
| `llm/openai_world_situation_proposer.go` | プロンプトに「rootどうしが同じ形に偏らないよう…散らしてください」「recent/avoid material は重複を避けるための材料」。`responseTextWithJSONSchema(ctx, prompt, "low", maxTokens, …)` |
| `worldrepo/bbs_title_words.go` `wordRootTitles` | チャンク 20 本ずつ、直列。完全一致の件名は 1 回だけ再生成し、残りは受け入れる（`DuplicateAccepted`） |
| `llm/bbs_article_batch.go` ほか | 件名候補のプロンプトに「互いに言い換えにならないよう、対象、用件、文型、具体性を十分に散らしてください」 |
| `worldrepo/bbs_situation_chunk_retry_test.go` | 出力の切れでチャンクを分割する既存のテスト。`wantCalls := []int{6,3,2,2,2,2,2}`（直列で、小さいサイズを引き継ぐ前提） |

実測（Render、`BBS timing: phase=planner_total`）: 10/16〜10/17 の 10 回で 31〜55 秒。

## 2. 設計

### 2.1 所要時間のログ（FR-001）

- `planSituationFirstRoots` の Situation 呼び出しの前後と、`wordRootTitles` の `requestTitles` の前後で、
  `log.Printf("BBS timing: host=%s board=%s phase=situation_chunk duration=%s events=%d", …)` を出す。
- 件名は `phase=title_chunk`。リトライの 2 回目以降も、1 回ごとに出す。
- 既存の `planner_total` は変えない。

### 2.2 重複を拒否しない（FR-002、FR-003、FR-005）

- 約 264〜275 行の `noveltyOwners` と `chunkNovelty` による不採用を、**ログ**に変える:
  `log.Printf("BBS Situation novelty overlap: host=%s board=%s key=%s events=%s,%s", …)`。採用は続ける。
- 空の `novelty_key` による不採用（約 258〜263 行）は残す。`validateProductionSituation` は変えない。
- `noveltyOwners`、`chunkNovelty`、`avoidSituations` への `DO NOT REUSE` の追記は、不要になるので削除する。
  採用後に `avoidSituations` へ足す `already accepted novelty_key=… occurrence=…` は、チャンクを並列にすると他のチャンクから見えない（2.3）ので、
  並列の実装では、バッチの先頭で渡す直近の件名だけにする。
- プロンプトの文言（`openai_world_situation_proposer.go`）:
  - 「recent/avoid material は重複を避けるための材料です。避けるときも…」→
    「recent/avoid material は文脈の参考です。同じ話題や続きの話が出てもかまいません。」
  - 「rootどうしが同じ形…に偏らないよう、板の範囲の中で形と話題を散らしてください」→
    「形や話題は、板の範囲の中で自然に混ざればよい。無理に散らさなくてよい。」
- 件名側: `wordRootTitles` の完全一致の再生成（約 82〜135 行）を削除し、一致は `DuplicateAccepted` に数える。
  `llm/bbs_title_candidates.go`（約 125 行目の「20個は互いに言い換えにならないよう…散らして」）を、「自然に混ざればよい」に弱める。
  （文言の置換は、実際の行を確認してから行う。他の文言は変えない。）

### 2.3 並列化（FR-004）

- Situation のチャンクを、`errgroup` ではなく標準の `sync.WaitGroup` とバッファ付きチャネルで、同時数 `productionSituationConcurrency = 4` まで並列にする
  （新しい依存を足さない。要確認: `go.mod` に `golang.org/x/sync` が既にあれば `errgroup` を使ってよい）。
- チャンクの分割は、事前にスロットの順序でチャンクの一覧を作る。各チャンクは、独立に次を行う:
  1. 提案器を呼ぶ（最大 `productionSituationChunkAttempts` 回）。
  2. 切れ（`ErrBBSWorldSituationOutputTruncated`）なら、そのチャンクを半分に分け、各半分を**順に**処理する（そのチャンクの中だけで閉じる）。
  3. 検証（空の項目、重複するイベント、足りないイベント）。
- 結果は、イベント ID をキーに集め、最後にスロットの順序で `byEvent` に組む。ワーカーは `byEvent` に直接書かず、チャンクごとの結果を返す。
- **これまでとの違い（意図した変更）**: 小さいサイズを、バッチの残りに引き継ぐ動作は、なくなる。
  既存のテスト（`bbs_situation_chunk_retry_test.go` の `wantCalls`）は、並列のもとでは呼び出し順が決まらないので、
  呼び出しサイズの**集合**（例: 6 が 2 回、分割後の 3 が…）を確かめる形に改める。テストの偽の提案器（`sizes` への追記）は、排他制御を足す。
- `storeDevelopmentPlanningUsage` は、並列で呼ばれても安全か確かめる（要確認: `p.repo` のロック）。安全でなければ、チャンクの結果を集めたあとに、呼び出し側で順に記録する。

### 2.4 件名は並列化しない

件名の呼び出しは、現状でチャンク 20 本で 1 回（ルートが約 10 本なら 1 回）。Situation の結果を必要とし、並列化の効果が小さいので、この spec では変えない。

## 3. 検証の方針

- 遅延を入れた偽の提案器（`time.Sleep`、`sync.Mutex` で保護した呼び出し記録）で、所要時間が直列の合計より短いことを確かめる（SC-001）。
  時間の閾値は、緩く（例: 直列の 60% 以下）。
- 重複の `novelty_key` を返す偽の提案器で、バッチが成功し、呼び出し回数が 1 回であることを確かめる（SC-002）。
- 並列の結果の順序の決定性（SC-003）、切れの分割（SC-004）をテストする。
- `go test -race ./internal/worldrepo` を実行する（並列化のため）。
- 金型は作らない。変えてはいけない出力（Situation の項目、保存される事実）は、既存のテストで確かめる。
  人物生成と Erika-K の画面の金型は、変わってはならない。
- デプロイ後に、Render のログで `planner_total` と `situation_chunk` の所要時間を確認し、改善前（31〜55 秒）と比べる。

## 4. 変更するファイル

| ファイル | 内容 |
|---|---|
| `apps/server/internal/worldrepo/bbs_situation_first_planner.go` | 所要時間のログ、重複の不採用の削除、チャンクの並列化 |
| `apps/server/internal/worldrepo/bbs_title_words.go` | 所要時間のログ、完全一致の再生成の削除 |
| `apps/server/internal/llm/openai_world_situation_proposer.go` | プロンプトの文言の緩和 |
| `apps/server/internal/llm/bbs_title_candidates.go`（または該当の行がある `llm/` のファイル） | 件名候補のプロンプトの文言の緩和 |
| `apps/server/internal/worldrepo/bbs_situation_chunk_retry_test.go` | 並列に合わせたテストの修正 |
| 同 `bbs_situation_open_topics_test.go`、`bbs_title_words_test.go` | 重複を許す動作に合わせた修正と、新しいテスト |
| `docs/DEBUG_INSPECTION.md` | 新しい `BBS timing` の行の説明 |

## 5. リスクと対策

| リスク | 対策 |
|---|---|
| 重複を許すと、同じ話題が並びすぎる | ログで頻度を確かめる。多すぎるなら、プロンプトの文言を調整する（拒否は復活させない） |
| 並列で、同時に呼ぶ LLM のレート制限に当たる | 同時数を定数 4 にして小さく始める。制限のエラーは、これまでどおり試行ごとに再試行される |
| 並列のチャンクで、話題が似る（互いの結果を見ない） | 重複を許す方針のため許容する。直近の件名は全チャンクに渡す |
| テストが不安定になる | 偽の提案器は決定的にし、時間の閾値は緩くする。`-race` で実行する |
| 使用量の記録の並行性 | 2.3 のとおり確かめ、安全でなければ集めてから順に記録する |
