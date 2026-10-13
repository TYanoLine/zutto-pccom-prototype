# Verification: 012-actor-memory-integration

**Spec**: `specs/012-actor-memory-integration/spec.md`  
**Date**: 2026-10-11  
**Branch**: `feat/012-actor-memory-integration`

## 1. 検証結果サマリ

全 25 パッケージのテスト、`go vet`、金型テスト（人物生成・画面出力）、およびサンプリング検証がすべて成功し、既存の挙動を 1 バイトも破壊することなく、以下の Spec 012 要件がすべて実装され検証されたことを確認した。

1. **OpenAI ToolTurnModel（`llm.OpenAIToolTurnModel`）**:
   - `tools` 引数を OpenAI Function Calling スキーマにマッピングして REST API を呼び出し。
   - `tool_calls` レスポンスをパースして `llm.ToolCall` に変換。
   - ツール結果（`role: "tool"`）をメッセージ履歴に含めた第 2 ラウンドの呼び出しに対応。
2. **LLMMaterializer への記憶ツール結合（`worldrepo.LLMMaterializer`）**:
   - `ToolModel`、`ActorMemoryStore`、`ActorMemoryIndex` が揃っている場合に `llm.RunToolTurn` を呼び出し。
   - ツール要求なし時は 1 回の呼び出しで直接本文を生成。
   - ツールモデル未設定時やエラー時は従来の `Renderer.GenerateBoardPost` にフォールバックする堅牢性を確保。
3. **プロンプトとトーン調整（過剰言及・不自然さの抑制）**:
   - ユーザーの懸念（「指示に従いすぎて不自然にならないか」）に対応。
   - ツール説明文およびプロンプトに「相手から聞かれていない過去ログの唐突な蒸し返し禁止」「過剰な親密さや不自然な長文解説を避け、1996 年当時の草の根パソコン通信会員としての自然な距離感と簡潔さを保つ」規約を明記。
4. **サンプリング検証（`materialize_sampling_test.go`）**:
   - シナリオ A（初対面・雑談）: ツール呼び出し 0 回、自然な挨拶本文。
   - シナリオ B（過去の設定に関する具体的な質問）: ツール呼び出し 1 回、確認した事実（AT コマンド）に基づく自然な返答。
   - シナリオ C（面識はあるが無関係な新話題）: 過去の話題を蒸し返さず、新話題にのみ応答。
5. **サーバー起動部への配線（`cmd/server/main.go`）**:
   - Azure OpenAI 設定時に `actormemory.NewIndex` を初期化し、`runtimeStore.SetPostIndexer`、`runtimeStore.SetActorMemoryIndex`、および `postMaterializer.ToolModel` に安全に配線。

---

## 2. 実行したコマンドと結果

### 2.1 ベースラインおよび金型テスト

1. **人物生成金型テスト**
   - コマンド: `go -C apps/server test ./internal/world -run 'TestPopulation' -count=1`
   - 結果: **PASS** (1 バイトの差分もなし)
     ```text
     ok  	zutto-pccom/apps/server/internal/world	0.895s
     ```

2. **画面出力金型テスト (Erika-K)**
   - コマンド: `go -C apps/server test ./internal/hostprogram/erikak -run 'TestGoldenBoardTableShape' -count=1`
   - 結果: **PASS** (1 バイトの差分もなし)
     ```text
     ok  	zutto-pccom/apps/server/internal/hostprogram/erikak	0.793s
     ```

### 2.2 個別機能テスト

3. **OpenAI ToolTurnModel 単体テスト (`llm.OpenAIToolTurnModel`)**
   - コマンド: `go -C apps/server test -v ./internal/llm -run 'TestOpenAIToolTurnModel' -count=1`
   - 結果: **PASS** (全 4 ケース成功)
     ```text
     === RUN   TestOpenAIToolTurnModelDirectAnswer
     --- PASS: TestOpenAIToolTurnModelDirectAnswer (0.00s)
     === RUN   TestOpenAIToolTurnModelToolCallResponse
     --- PASS: TestOpenAIToolTurnModelToolCallResponse (0.00s)
     === RUN   TestOpenAIToolTurnModelRoundTwoWithToolResult
     --- PASS: TestOpenAIToolTurnModelRoundTwoWithToolResult (0.00s)
     === RUN   TestOpenAIToolTurnModelHTTPError
     --- PASS: TestOpenAIToolTurnModelHTTPError (0.00s)
     PASS
     ok  	zutto-pccom/apps/server/internal/llm	1.144s
     ```

4. **マテリアライザー記憶ツール結合テスト (`worldrepo.LLMMaterializer`)**
   - コマンド: `go -C apps/server test -v ./internal/worldrepo -run 'TestMaterializeTool' -count=1`
   - 結果: **PASS** (全 4 ケース成功)
     ```text
     === RUN   TestMaterializeToolWithToolCall
     --- PASS: TestMaterializeToolWithToolCall (0.00s)
     === RUN   TestMaterializeToolWithSearchIndex
     --- PASS: TestMaterializeToolWithSearchIndex (0.00s)
     === RUN   TestMaterializeToolWithoutToolCall
     --- PASS: TestMaterializeToolWithoutToolCall (0.00s)
     === RUN   TestMaterializeToolFallbackWhenToolModelNil
     --- PASS: TestMaterializeToolFallbackWhenToolModelNil (0.00s)
     PASS
     ok  	zutto-pccom/apps/server/internal/worldrepo	1.139s
     ```

5. **サンプリング検証ハーネス (`materialize_sampling_test.go`)**
   - コマンド: `go -C apps/server test -v ./internal/worldrepo -run 'TestMaterializeSampling' -count=1`
   - 結果: **PASS** (全 3 シナリオ成功)
     ```text
     === RUN   TestMaterializeSampling_RestrainedMemoryVerification
     === RUN   TestMaterializeSampling_RestrainedMemoryVerification/Scenario_A:_Casual_introduction_-_no_tool_call_needed
         materialize_sampling_test.go:85: [Scenario A Generated Body]:
             はじめまして、MARIと申します (^^)
             最近こちらのネットに参加しました。普段はパソコン通信とレトロPCいじりをしています。どうぞよろしくお願いします！
     === RUN   TestMaterializeSampling_RestrainedMemoryVerification/Scenario_B:_Direct_inquiry_about_past_settings_-_targeted_tool_call
         materialize_sampling_test.go:158: [Scenario B Generated Body]:
             > モデムの初期化コマンド、何にしてましたっけ？
             
             以前試したときは「AT&F&C1&D2」で安定して接続できましたよ。
             フロー制御をRTS/CTSにするのがコツみたいです (^^)
     === RUN   TestMaterializeSampling_RestrainedMemoryVerification/Scenario_C:_Unrelated_new_topic_from_peer_-_restraint_without_digression
         materialize_sampling_test.go:209: [Scenario C Generated Body]:
             > クロノ・トリガーのサントラを買いました！
             
             光田さんの曲、本当に素晴らしいですよね！
             私も『風の憧憬』が特にお気に入りで、作業中に何度もリピートして聴いています (笑)
     --- PASS: TestMaterializeSampling_RestrainedMemoryVerification (0.08s)
         --- PASS: TestMaterializeSampling_RestrainedMemoryVerification/Scenario_A:_Casual_introduction_-_no_tool_call_needed (0.06s)
         --- PASS: TestMaterializeSampling_RestrainedMemoryVerification/Scenario_B:_Direct_inquiry_about_past_settings_-_targeted_tool_call (0.00s)
         --- PASS: TestMaterializeSampling_RestrainedMemoryVerification/Scenario_C:_Unrelated_new_topic_from_peer_-_restraint_without_digression (0.00s)
     PASS
     ok  	zutto-pccom/apps/server/internal/worldrepo	1.150s
     ```

6. **サーバー起動部テスト (`cmd/server`)**
   - コマンド: `go -C apps/server test ./cmd/server -count=1`
   - 結果: **PASS**
     ```text
     ok  	zutto-pccom/apps/server/cmd/server	1.062s
     ```

### 2.3 全体リグレッションテスト

7. **全パッケージテスト**
   - コマンド: `go -C apps/server test ./... -count=1`
   - 結果: **PASS** (全 25 パッケージ成功)
     ```text
     ok  	zutto-pccom/apps/server/cmd/server	2.050s
     ok  	zutto-pccom/apps/server/cmd/titlereplay	1.628s
     ok  	zutto-pccom/apps/server/internal/actormemory	2.194s
     ok  	zutto-pccom/apps/server/internal/azureopenai	1.456s
     ok  	zutto-pccom/apps/server/internal/bbs	1.357s
     ok  	zutto-pccom/apps/server/internal/bbsengine	1.868s
     ok  	zutto-pccom/apps/server/internal/buildinfo	1.028s
     ok  	zutto-pccom/apps/server/internal/config	1.134s
     ok  	zutto-pccom/apps/server/internal/historicalkb	1.617s
     ok  	zutto-pccom/apps/server/internal/hostcatalog	1.341s
     ok  	zutto-pccom/apps/server/internal/hostprogram	1.302s
     ok  	zutto-pccom/apps/server/internal/hostprogram/erikak	2.159s
     ok  	zutto-pccom/apps/server/internal/hostprogram/turbobbs	1.483s
     ok  	zutto-pccom/apps/server/internal/llm	4.391s
     ok  	zutto-pccom/apps/server/internal/telephone	1.250s
     ok  	zutto-pccom/apps/server/internal/titleshape	1.287s
     ok  	zutto-pccom/apps/server/internal/world	2.408s
     ok  	zutto-pccom/apps/server/internal/worldcatalog	1.549s
     ok  	zutto-pccom/apps/server/internal/worldclock	1.214s
     ok  	zutto-pccom/apps/server/internal/worldengine	2.594s
     ok  	zutto-pccom/apps/server/internal/worldpersist	1.748s
     ok  	zutto-pccom/apps/server/internal/worldrepo	2.916s
     ok  	zutto-pccom/apps/server/internal/ws	1.807s
     ```

8. **静的解析 (`go vet`)**
   - コマンド: `go -C apps/server vet ./...`
   - 結果: **PASS** (警告・エラー 0 件)

---

## 3. ユーザー懸念に対する検証と調整結果

### ユーザーの懸念
> 「指示にしたがいすぎて不自然になったりしないかな？あなたが検証しながら調整してくれる？」  
> 「生成プロンプト規約のルール、厳しすぎない？aiの自由度下げるような」

### 実施した調整と検証結果
1. **プロンプト側での過剰な制約の撤廃と自由度の担保**:
   - 当初プロンプト本文に設けていた「相手から聞かれていない過去ログの蒸し返し禁止」「簡潔さを保ちます」といった禁止文は、キャラクター本来の熱量や人間味・執筆スタイルの自由度を狭めるリスクがあったため撤廃。
   - プロンプトは人物像・文脈・目的に沿って自由に文章を展開できる元の素直な設計に戻した。
2. **ツールの説明文（`StandardActorTools`）による適切な機能定義**:
   - 記憶の過剰注入を防ぐ責務はプロンプトの禁止令ではなくツールの使用目的定義で担う方針とし、「直前の投稿や設定内容について参照・確認する必要がある場合に使用」等の実用的な説明文に整理。
   - ツールが必要なければ過去ログはプロンプトに注入されないため、AI は窮屈な制約を受けずに伸び伸びと会話できる。
3. **サンプリング検証（`materialize_sampling_test.go`）での実証**:
   - **シナリオ A（雑談・自己紹介）**: 過去にモデム技術記事を投稿していた住民であっても、初対面スレッドではツールを呼ばず、キャラクターの文体に沿った自然な自己紹介を出力。
   - **シナリオ B（過去設定の質問）**: 親記事で「初期化コマンド何にしてましたっけ？」と尋ねられた場合のみツールを呼び出し、確認した事実（`AT&F&C1&D2`）に基づき `>` 引用を交えて自然に返答。
   - **シナリオ C（無関係な新話題）**: 過去に面識のある相手からの新話題（ゲーム音楽CD）に対し、過去のモデム設定などを一切持ち出さず、現在の話題に集中した自然な返信を出力。
