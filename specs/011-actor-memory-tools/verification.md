# Verification: 011-actor-memory-tools

**Spec**: `specs/011-actor-memory-tools/spec.md`  
**Date**: 2026-10-11  
**Branch**: `feat/011-actor-memory-tools`

## 1. 検証結果サマリ

全テストおよび金型テスト（人物生成・画面出力）がすべて成功し、既存の挙動を 1 バイトも破壊することなく、以下の Spec 011 要件がすべて実装され検証されたことを確認した。

1. **オーケストレーター（`llm.RunToolTurn`）**: 最大 1 往復に制約され、ツール要求時は 2 回のモデル呼び出しで停止し（2 回目はツール定義なし）、要求なし時は 1 回で完結。2 回目もツール要求を返した場合はループせず `ErrNoFinalText` で終了。
2. **非同期マイクロバッチ埋め込みインデックス（`actormemory.Index`）**: 10 件到達または 200ms 経過で `EmbeddingClient` にバッチ処理。キュー投入は呼び出し元をブロックせず、同一イベント ID の多重投入は冪等に除外。埋め込み失敗時もワーカーがクラッシュせず再試行可能。
3. **アクター記憶ツール実行器（`actormemory.Executor`）**: `llm.StandardActorTools` の 6 ツールに対応。自局・自アクター関連の観測範囲外のデータは一切返さず、件数は 1〜5 に丸め、本文は 160 文字で抜粋。埋め込みクライアント未指定時や失敗時も通常結果として「利用不可」メッセージを返しシステム全体は縮退稼働。
4. **リポジトリ配線（`worldrepo.Repository`）**: `PostIndexer` を設定することで `AddPost` 後に非同期でインデックスへ投入。遅延する埋め込み API でも `AddPost` の保存レイテンシに一切影響を与えない。
5. **LLM 呼び出し 0 回の保証**: ツール実行・埋め込みインデックス処理において、生成 LLM の呼び出し回数は 0 回。

---

## 2. 実行したコマンドと結果

### 2.1 ベースラインおよび金型テスト

1. **人物生成金型テスト**
   - コマンド: `go -C apps/server test -v ./internal/world -run 'TestPopulation' -count=1`
   - 結果: **PASS** (1バイトの差分もなし)
     ```text
     === RUN   TestPopulationMatchesGolden
     --- PASS: TestPopulationMatchesGolden (0.07s)
     === RUN   TestPopulationShape
     --- PASS: TestPopulationShape (0.05s)
     PASS
     ok  	zutto-pccom/apps/server/internal/world	0.915s
     ```

2. **画面出力金型テスト (Erika-K)**
   - コマンド: `go -C apps/server test -v ./internal/hostprogram/erikak -run 'TestGolden' -count=1`
   - 結果: **PASS** (1バイトの差分もなし)
     ```text
     === RUN   TestGoldenBoardTableShape
     --- PASS: TestGoldenBoardTableShape (0.01s)
     PASS
     ok  	zutto-pccom/apps/server/internal/hostprogram/erikak	0.799s
     ```

### 2.2 個別機能テスト

3. **オーケストレーター単体テスト (`llm.RunToolTurn`)**
   - コマンド: `go -C apps/server test -v ./internal/llm -run 'TestRunToolTurn' -count=1`
   - 結果: **PASS**
     ```text
     === RUN   TestRunToolTurnNoToolRequestUsesOneModelCall
     --- PASS: TestRunToolTurnNoToolRequestUsesOneModelCall (0.00s)
     === RUN   TestRunToolTurnOneRoundTripWithoutToolsInRoundTwo
     --- PASS: TestRunToolTurnOneRoundTripWithoutToolsInRoundTwo (0.00s)
     === RUN   TestRunToolTurnExecutesOnlyFirstOfSeveralToolCalls
     --- PASS: TestRunToolTurnExecutesOnlyFirstOfSeveralToolCalls (0.00s)
     === RUN   TestRunToolTurnStopsAfterTwoModelCallsWhenToolRequestedAgain
     --- PASS: TestRunToolTurnStopsAfterTwoModelCallsWhenToolRequestedAgain (0.00s)
     === RUN   TestRunToolTurnZeroPolicyOffersNoTools
     --- PASS: TestRunToolTurnZeroPolicyOffersNoTools (0.00s)
     === RUN   TestRunToolTurnReturnsModelErrors
     --- PASS: TestRunToolTurnReturnsModelErrors (0.00s)
     === RUN   TestRunToolTurnNilModel
     --- PASS: TestRunToolTurnNilModel (0.00s)
     PASS
     ok  	zutto-pccom/apps/server/internal/llm	1.620s
     ```

4. **埋め込みインデックス・ツール実行器・LLM呼び出し0回テスト (`actormemory`)**
   - コマンド: `go -C apps/server test -v ./internal/actormemory -count=1`
   - 結果: **PASS**
     ```text
     === RUN   TestGetMyRecentPostsNewestFirstAndBoardFilter
     --- PASS: TestGetMyRecentPostsNewestFirstAndBoardFilter (0.01s)
     === RUN   TestPostBodiesAreExcerpted
     --- PASS: TestPostBodiesAreExcerpted (0.01s)
     === RUN   TestGetThreadContextStartsAtRootAndKeepsNewest
     --- PASS: TestGetThreadContextStartsAtRootAndKeepsNewest (0.01s)
     === RUN   TestGetInteractionHistoryWithCounterpart
     --- PASS: TestGetInteractionHistoryWithCounterpart (0.01s)
     === RUN   TestChatAndMailFiltersAreEmptyNotErrors
     --- PASS: TestChatAndMailFiltersAreEmptyNotErrors (0.01s)
     === RUN   TestGetMyProfileFactsAndOpenLoops
     --- PASS: TestGetMyProfileFactsAndOpenLoops (0.01s)
     === RUN   TestLimitIsClamped
     --- PASS: TestLimitIsClamped (0.00s)
     === RUN   TestToolsNeverReturnAnotherHostsData
     --- PASS: TestToolsNeverReturnAnotherHostsData (0.02s)
     === RUN   TestSearchFindsVisibleItemsOnly
     --- PASS: TestSearchFindsVisibleItemsOnly (0.02s)
     === RUN   TestSearchDegradesToANormalResult
     --- PASS: TestSearchDegradesToANormalResult (0.01s)
     === RUN   TestExecuteRejectsBadInput
     --- PASS: TestExecuteRejectsBadInput (0.01s)
     === RUN   TestToolTurnThroughRegistryMakesNoExtraModelCalls
     --- PASS: TestToolTurnThroughRegistryMakesNoExtraModelCalls (0.01s)
     === RUN   TestIndexBatchesAtBatchSize
     --- PASS: TestIndexBatchesAtBatchSize (0.01s)
     === RUN   TestIndexTwentyFiveItemsUseAtMostThreeCalls
     --- PASS: TestIndexTwentyFiveItemsUseAtMostThreeCalls (0.00s)
     === RUN   TestIndexFlushesPartialBatchAfterInterval
     --- PASS: TestIndexFlushesPartialBatchAfterInterval (0.03s)
     === RUN   TestIndexEnqueueDoesNotWaitForEmbedding
     --- PASS: TestIndexEnqueueDoesNotWaitForEmbedding (0.00s)
     === RUN   TestIndexSurvivesEmbedFailureAndAllowsRetry
     --- PASS: TestIndexSurvivesEmbedFailureAndAllowsRetry (0.08s)
     === RUN   TestIndexEnqueueIsIdempotentPerEvent
     --- PASS: TestIndexEnqueueIsIdempotentPerEvent (0.04s)
     === RUN   TestIndexSearchRanksFiltersAndDropsNonPositive
     --- PASS: TestIndexSearchRanksFiltersAndDropsNonPositive (0.01s)
     === RUN   TestIndexWithoutClientIsUnavailable
     --- PASS: TestIndexWithoutClientIsUnavailable (0.00s)
     === RUN   TestIndexCloseFlushesAndRejectsLaterItems
     --- PASS: TestIndexCloseFlushesAndRejectsLaterItems (0.00s)
     PASS
     ok  	zutto-pccom/apps/server/internal/actormemory	1.256s
     ```

5. **リポジトリ配線テスト (`worldrepo`)**
   - コマンド: `go -C apps/server test -v ./internal/worldrepo -run 'TestAddPost' -count=1`
   - 結果: **PASS**
     ```text
     === RUN   TestAddPostHandsTheSavedPostToTheIndexer
     --- PASS: TestAddPostHandsTheSavedPostToTheIndexer (0.01s)
     === RUN   TestAddPostWithoutIndexerBehavesAsBefore
     --- PASS: TestAddPostWithoutIndexerBehavesAsBefore (0.01s)
     === RUN   TestAddPostIsNotDelayedBySlowEmbeddingAndBecomesSearchable
     --- PASS: TestAddPostIsNotDelayedBySlowEmbeddingAndBecomesSearchable (0.01s)
     === RUN   TestAddPost_RecordsInteractionEvent
     --- PASS: TestAddPost_RecordsInteractionEvent (0.08s)
     === RUN   TestAddPost_UpdatesRelationshipEdge
     --- PASS: TestAddPost_UpdatesRelationshipEdge (0.02s)
     PASS
     ok  	zutto-pccom/apps/server/internal/worldrepo	1.193s
     ```

### 2.3 静的解析 (go vet)

6. **関連パッケージの `go vet`**
   - コマンド: `go -C apps/server vet ./internal/actormemory ./internal/llm ./internal/worldrepo`
   - 結果: **PASS (exit code 0, 警告なし)**

### 2.4 全パッケージ回帰テスト

7. **全パッケージテスト**
   - コマンド: `go -C apps/server test ./... -count=1`
   - 結果: **PASS (全25パッケージ成功)**
     ```text
     ok  	zutto-pccom/apps/server/cmd/server	2.206s
     ok  	zutto-pccom/apps/server/cmd/titlereplay	1.597s
     ?   	zutto-pccom/apps/server/cmd/titlestats	[no test files]
     ok  	zutto-pccom/apps/server/internal/actormemory	2.137s
     ok  	zutto-pccom/apps/server/internal/azureopenai	1.467s
     ok  	zutto-pccom/apps/server/internal/bbs	1.179s
     ok  	zutto-pccom/apps/server/internal/bbsengine	1.533s
     ok  	zutto-pccom/apps/server/internal/buildinfo	1.065s
     ok  	zutto-pccom/apps/server/internal/config	1.140s
     ok  	zutto-pccom/apps/server/internal/historicalkb	1.455s
     ok  	zutto-pccom/apps/server/internal/hostcatalog	1.399s
     ok  	zutto-pccom/apps/server/internal/hostprogram	1.267s
     ok  	zutto-pccom/apps/server/internal/hostprogram/erikak	2.185s
     ?   	zutto-pccom/apps/server/internal/hostprogram/replymodel	[no test files]
     ok  	zutto-pccom/apps/server/internal/turbobbs	1.391s
     ok  	zutto-pccom/apps/server/internal/llm	4.328s
     ok  	zutto-pccom/apps/server/internal/telephone	1.434s
     ok  	zutto-pccom/apps/server/internal/titleshape	1.272s
     ok  	zutto-pccom/apps/server/internal/world	1.989s
     ok  	zutto-pccom/apps/server/internal/worldcatalog	1.406s
     ok  	zutto-pccom/apps/server/internal/worldclock	1.091s
     ok  	zutto-pccom/apps/server/internal/worldengine	2.383s
     ok  	zutto-pccom/apps/server/internal/worldpersist	1.809s
     ok  	zutto-pccom/apps/server/internal/worldrepo	2.704s
     ok  	zutto-pccom/apps/server/internal/ws	1.718s
     ```

### 2.5 競合検出 (`-race`) について

- コマンド: `go -C apps/server test ./internal/actormemory -count=1 -race`
- 結果: **未実施 (スキップ)**
- 理由: 実行環境 (Windows) に cgo (GCC/Clang) が導入されておらず、Go ツールチェインにより `go: -race requires cgo; enable cgo by setting CGO_ENABLED=1` と判定されて実行できないため。

---

## 3. 要件の達成状況

| 要件 ID | 要件概要 | 達成状況 | 検証証跡 |
|---|---|---|---|
| **FR-001** | `ActorScopedToolExecutor` 実装を新パッケージ `actormemory` に配置、6ツール応答 | **達成** | `actormemory/executor.go`, `TestExecuteRejectsBadInput`, 6ツール単体テスト |
| **FR-002** | 観測範囲境界（自局公開板または自身が関与したイベントのみ） | **達成** | `actormemory/executor.go` `visibleTo()`, `TestToolsNeverReturnAnotherHostsData` |
| **FR-003** | `limit` 範囲（1〜5、デフォルト3） | **達成** | `TestLimitIsClamped` |
| **FR-004** | 本文抜粋（先頭 160 ルーン） | **達成** | `TestPostBodiesAreExcerpted` |
| **FR-005** | open loops（未決事項なしを成功として返す） | **達成** | `TestGetMyProfileFactsAndOpenLoops` |
| **FR-006** | チャネル対応（`chat`・`mail` は空配列を返しエラーにしない） | **達成** | `TestChatAndMailFiltersAreEmptyNotErrors` |
| **FR-007** | 非同期マイクロバッチ（10件または200ms） | **達成** | `actormemory/index.go`, `TestIndexBatchesAtBatchSize`, `TestIndexFlushesPartialBatchAfterInterval` |
| **FR-008** | 冪等性（同一 `EventID` 多重投入で重複排除） | **達成** | `TestIndexEnqueueIsIdempotentPerEvent` |
| **FR-009** | 類似度検索（1回埋め込み、観測範囲フィルタ、コサイン類似度降順、0以下除外） | **達成** | `TestIndexSearchRanksFiltersAndDropsNonPositive`, `TestSearchFindsVisibleItemsOnly` |
| **FR-010** | 縮退（埋め込み未設定・失敗時に通常結果で利用不可を返す） | **達成** | `TestSearchDegradesToANormalResult`, `TestIndexWithoutClientIsUnavailable` |
| **FR-011** | オーケストレーター（最大1往復制約、ループ遮断） | **達成** | `llm/tool_turn.go`, `tool_turn_test.go` 全ケース PASS |
| **FR-012** | 配線（`Repository` に `PostIndexer` を追加、保存レイテンシ非ブロック） | **達成** | `worldrepo/repository.go`, `TestAddPostIsNotDelayedBySlowEmbeddingAndBecomesSearchable` |
| **FR-013** | LLM 非依存（生成 LLM 呼び出し 0 回） | **達成** | `TestToolTurnThroughRegistryMakesNoExtraModelCalls` |
| **SC-001** | 6 ツールが固定データに対して期待通りの結果を返す | **達成** | `executor_test.go` PASS |
| **SC-002** | 観測範囲外データがどのツールからも返らない | **達成** | `TestToolsNeverReturnAnotherHostsData`, `TestSearchFindsVisibleItemsOnly` PASS |
| **SC-003** | 保存レイテンシが埋め込み API の遅延に影響されない | **達成** | `TestAddPostIsNotDelayedBySlowEmbeddingAndBecomesSearchable` PASS |
| **SC-004** | モデル呼び出しが最大 2 回、不使用時 1 回 | **達成** | `TestRunToolTurn*` PASS |
| **SC-005** | 金型テスト（人物生成・画面金型）が 1 バイトも変わらない | **達成** | `TestPopulationMatchesGolden`, `TestGoldenBoardTableShape` PASS |
| **SC-006** | `go test ./...` 全パッケージ PASS | **達成** | 全 25 パッケージ PASS |
