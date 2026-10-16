# Spec 008: ペルソナ特性にもとづくレス・反応の選択（人間の投稿への反応を含む）検証記録

## 1. 概要

本検証記録は、Spec 008（`specs/008-persona-reactions/`）の実装および受入条件（SC-001 〜 SC-010）に対する検証結果を報告するものである。

## 2. 実行したコマンドと結果

### 2.1 静的解析 (Static Analysis)
- **コマンド**: `go -C apps/server vet ./...`
- **結果**: PASS (エラーなし、終了コード 0)

### 2.2 単体・結合テストスイート
- **コマンド**: `go -C apps/server test ./... -count=1`
- **結果**: すべてのパッケージで PASS (終了コード 0)
  ```
  ok  	zutto-pccom/apps/server/cmd/server	2.116s
  ok  	zutto-pccom/apps/server/cmd/titlereplay	2.131s
  ?   	zutto-pccom/apps/server/cmd/titlestats	[no test files]
  ok  	zutto-pccom/apps/server/internal/actormemory	2.895s
  ok  	zutto-pccom/apps/server/internal/azureopenai	1.428s
  ok  	zutto-pccom/apps/server/internal/bbs	1.355s
  ok  	zutto-pccom/apps/server/internal/bbsengine	2.916s
  ok  	zutto-pccom/apps/server/internal/buildinfo	1.123s
  ok  	zutto-pccom/apps/server/internal/callplan	17.439s
  ok  	zutto-pccom/apps/server/internal/chatslot	2.694s
  ok  	zutto-pccom/apps/server/internal/config	1.062s
  ok  	zutto-pccom/apps/server/internal/historicalkb	1.504s
  ok  	zutto-pccom/apps/server/internal/hostcatalog	2.334s
  ok  	zutto-pccom/apps/server/internal/hostprogram	1.418s
  ok  	zutto-pccom/apps/server/internal/hostprogram/erikak	2.502s
  ?   	zutto-pccom/apps/server/internal/hostprogram/replymodel	[no test files]
  ok  	zutto-pccom/apps/server/internal/hostprogram/turbobbs	1.492s
  ok  	zutto-pccom/apps/server/internal/lineslot	1.613s
  ok  	zutto-pccom/apps/server/internal/llm	4.297s
  ok  	zutto-pccom/apps/server/internal/presence	19.975s
  ok  	zutto-pccom/apps/server/internal/telephone	1.666s
  ok  	zutto-pccom/apps/server/internal/titleshape	1.506s
  ok  	zutto-pccom/apps/server/internal/world	5.226s
  ok  	zutto-pccom/apps/server/internal/worldcalendar	1.270s
  ok  	zutto-pccom/apps/server/internal/worldcatalog	1.358s
  ok  	zutto-pccom/apps/server/internal/worldclock	1.590s
  ok  	zutto-pccom/apps/server/internal/worldengine	2.516s
  ok  	zutto-pccom/apps/server/internal/worldpersist	2.086s
  ok  	zutto-pccom/apps/server/internal/worldrepo	51.855s
  ok  	zutto-pccom/apps/server/internal/ws	1.735s
  ```

### 2.3 金型（Golden Files）不変性の検証
- **コマンド**: `git diff origin/main -- apps/server/internal/world/population_golden_test.go apps/server/internal/hostprogram/erikak/testdata/`
- **結果**: 差分なし（出力空）。金型テスト（`TestPopulationMatchesGolden` 等）は完全に既存のバイナリ・テキストと一致し、1 バイトも変更されていない。

### 2.4 未実施の項目
- なし（すべてのタスク T000〜T063 を完了し、全テストおよび静的解析を実施済み）。

---

## 3. 統計・特性テストの検証結果（抜粋）

`apps/server/internal/world/reaction_test.go` における 5,000 サンプルの擬似乱数試行検証：

1. **話題への関心度による選択率（Topic Affinity Dominance: SC-002, SC-004）**:
   - `TestTopicAffinityDominanceStatistical`: 5,000 サンプル試行。
   - 対象話題（`retro_tech`）に高い関心を持つグループ（30名）と、関心を持たないグループ（30名、別トピック高）を用意。
   - 高関心グループの選択数が低関心グループの 3 倍以上（`highCount >= 3 * lowCount`）であることを確認。
   - 同時に $\epsilon$（ベースライン関心度）およびワイルドカード（5%）の存在により、低関心グループからもゼロではない現実的な少数選択（偶発的参加）が発生することを確認。

2. **ROM / 潜伏率の抑制（Activity Pattern Suppression: SC-002）**:
   - `TestReactionActivitySuppressionStatistical`: 5,000 サンプル試行。
   - 住民 100 名（active 30, regular 40, lurker 20, dormant 10）プール。
   - `lurker` の選定率は 5% 以下（実績: < 3%）。
   - `dormant` の選定率は 1% 以下（実績: < 0.2%）。

3. **投稿者自身および直前レス者の連続排除（Self and Immediate Exclusion: SC-001）**:
   - `TestReactionWeightSelfAndImmediateExclusion`: 自身に対する重みは厳密に 0.0。直前の投稿者に対する重みは 0.0。自作自演・即時連続投稿の完全排除を検証。

4. **リズム圧縮・伸長と深夜テレホタイム集中（6-Layer Rhythm Engine: SC-003, SC-006）**:
   - `TestReactionDueAtRhythmCompressionAndStretch`: 夜間・深夜テレホタイム（23:00〜）における活動確率の急上昇と返信予定日時の前倒し、昼間の遅延伸長を検証。

5. **Jev 助言機構の有界性（Bounded Influence: SC-008）**:
   - `TestReactionAdvisorEnabledDisabledAndFailedModes`: 助言（$\beta \le 0.4$）が決定論的な特性の強弱関係（2.33倍以上の差）を逆転できないこと、助言機構が無効・失敗時にも完全決定論的フォールバックが成立することを検証。

---

## 4. 既存テストの修正理由（T051）

`apps/server/internal/bbsengine/engine_test.go` において、以下のテストケースの期待値を更新した：

- **理由**:
  - 旧実装では `(i+1)%4 == 0` という固定ハードコード（4 記事に 1 記事を固定で返信扱いとする）と一様乱数による返信先割り当てが行われていた。
  - Spec 008 の要件（T050, T051）に基づき、これをボードの `reply_rate` に従う `world.ThreadReplyCount` および `world.PickReactor` による決定論的計画へ統一した。
  - これに伴い、スレッドごとの返信本数および親記事の参照関係が幾何分布・ポアソンモデルに従って決定されるようになったため、旧ハードコードを前提とした固定値アサーションを、新仕様の決定論的性質を検証するアサーションへと更新した。

---

## 5. コンパイル修正および整合性の向上

- `apps/server/cmd/titlereplay/main.go` 等で、過去の実験コードにおけるシグネチャ不整合（コンパイル未確認のまま残っていたもの）を検出・修正し、モジュール全体の `go build` および `go test` が一貫して通過することを確認した。
