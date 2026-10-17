# Spec: 返信の盛り上がりの判断を、ルート記事の生成に含める

**Spec ID**: 023-reply-appeal-in-root-draft  
**Status**: Draft  
**Depends on**: spec 021（返信件数を、LLM が判断した盛り上がりの段階から決める）と spec 022（Situation と件名を 1 回の呼び出しで作る）が `main` にあること

## 背景

spec 021 の実装では、盛り上がりの段階 `reply_appeal`（0〜4）を、ルート記事を保存したあとに、**別の LLM 呼び出し**（`ReplyAppealJudge`）で判断している。
spec 022 の実装後の Render ログ（hakata-canal-net、`phase=` 付きの `BBS timing`、2026-10-17 08:30〜08:39、5 板）で確認した事実:

| 板 | `planner_total` | `header_materialize_total` | 差 |
|---|---|---|---|
| board 8 | 14.9 秒 | 21.9 秒 | 7.0 秒 |
| board 20/1 | 17.7 秒 | 24.8 秒 | 7.1 秒 |
| board 20/2 | 21.6 秒 | 29.6 秒 | 8.0 秒 |
| board 7 | 21.1 秒 | 28.4 秒 | 7.3 秒 |
| board 2 | 16.5 秒 | 23.9 秒 | 7.4 秒 |

差の約 7〜8 秒は、**直列で待っている返信件数の判断**（`planReactionsForRoots` が呼ぶ `ReplyAppealJudge.JudgeReplyAppeal`）と見られる。
ストップウォッチで板を開いてから一覧が出るまで 24 秒を測った回があり、サーバー側の 23.9 秒とほぼ一致していた。つまり、利用者の待ち時間の約 3 割が、この判断の呼び出し。

判断の入力は、(a) ルートの件名と状況要約、(b) 投稿時刻、(c) その時刻に活動できる住民の数、(d) 板の基準率と会員数、(e) 板の直近の件数。
このうち (a) 以外は、ルートの生成**前**に分かっている。(a) は、ルートを生成する同じ呼び出しが出力する。つまり、
ルートの生成と同じ呼び出しで、盛り上がりの段階も一緒に判断できる。

## 設計の原則

- **盛り上がりの判断は、ルートを書いた同じ呼び出しの中で行う。** 別の呼び出しは作らない。
- **世界の事実の扱いは変えない。** 段階と理由は、これまでどおり反応の計画（`world.ReactionPlan`）に保存する。件数への換算式、上限、抽選は変えない。
- **コードを簡単に保つ。** 判断のためだけにあった呼び出しとインターフェースは、削除する（切り替えの仕組みは作らない。問題があればリバートする）。
- 係数や段階の定義は、spec 021 のとおり。この spec は、判断をどこで行うかだけを変える。

## ユーザーストーリー

### US1: 板を開いたときの待ち時間が、さらに短くなる（Priority: P1）

利用者として、未生成の板を開いたときの待ち時間が、返信件数の判断の分だけ短くなってほしい。

**Independent Test**: 偽の提案器で、ルート n 本のバッチの LLM 呼び出し回数が `ceil(n / productionSituationChunkSize)` だけ（判断の呼び出しが加わらない）。
デプロイ後の Render ログで、`header_materialize_total` と `planner_total` の差が、2 秒未満（以前は 7〜8 秒）。

### US2: 判断の内容は、これまでと同じ性質を保つ（Priority: P1）

運用者として、盛り上がりの段階と理由が、今までどおり記事ごとに付き、時間帯や人数が理由に反映されてほしい。

**Independent Test**: 偽の提案器が返した段階と理由が、そのまま `ReactionPlan` に保存される。範囲外・欠落の段階は 2 に退避する。

## 要件

- **FR-001（出力の項目）**:
  ルート生成の呼び出し（`BBSRootDraftPlanner.GenerateBBSRootDrafts`）の、ルートごとの出力に、`reply_appeal`（整数 0〜4）と
  `reply_appeal_reason`（1 文）を足す。項目の順序は、Situation の項目、`titles`、そのあとに `reply_appeal`、`reply_appeal_reason`。
  （Situation と件名を書いてから、盛り上がりを判断する順序を、スキーマで保つ。）
- **FR-002（判断の材料）**:
  呼び出しの入力（`BBSRootDraftRequest`）に、次を足す。
  - 板の基準返信率（実効値）と、局の会員数。
  - ルートごとの、その投稿時刻に活動できる住民の数（`world.ActivePopulationAt` で数えた値。spec 021 の修正で入った関数を使う）。
  - 板の直近のスレッドの返信件数（直近 10 本まで。保存済みの `ReactionPlan.Count` から）。
  投稿時刻は、既存の `created_at`（イベントごと）を使う。
  プロンプトには、spec 021 の判断器のプロンプト（段階の定義 0〜4、「すべてを中位にしない」、時刻・人数・直近の傾向も考慮する、1 文の理由）の文言を、引き継ぐ。
  新しい文言を作らない。
- **FR-003（検証と退避）**:
  `reply_appeal` が欠落、または 0〜4 の範囲外のルートは、そのルートの段階を 2、理由を `fallback:...` として扱い、ログに残す。
  **チャンクの再試行はしない**（段階の不備のために、ルートの生成をやり直さない）。件名や Situation の検証は、これまでどおり。
- **FR-004（保存までの受け渡し）**:
  生成された段階と理由を、`PlannedPost` を通して、反応の計画の作成まで運ぶ。ルートの保存後、`planReactionsForRoots` は、判断器を呼ばず、
  運ばれた段階と理由で件数を決めて、`ReactionPlan`（`Appeal`、`Reason`、`Count`、`Judged`）を保存する。
- **FR-005（旧い経路の削除）**:
  次を削除する。切り替えの環境変数や分岐は残さない。
  - `world.ReplyAppealJudge`、`world.ReplyAppealRequest`、`world.AppealRoot`、`world.ReplyAppealResult`
  - `llm.StructuredOpenAIProvider.JudgeReplyAppeal` と、`llm/reply_appeal.go`
  - `bbsengine.Engine.ReplyAppealJudge`、`worldrepo.Repository.SetReplyAppealJudge`、`replyAppealJudgeFromMaterializer`
  - 上記を使う、`planReactionsForRoots` の判断器の分岐
- **FR-006（判断が運ばれなかったルート）**:
  段階が運ばれなかったルート（保存前の計画が `Judged` でなく、かつ今回の生成に含まれないルート。移行期の保存データや、判断を出さない提案器）は、
  段階 2、理由 `fallback:no_appeal` で、同じ式で件数を決める。LLM は呼ばない。
- **FR-007（不変）**:
  件数への換算式（`ReplyCountForWithStandard`）、`AppealMultiplier`、`ActivePopulationAt`、保存済みの計画を再計算しない規則、
  ルートの件数（`InteractiveInitialRootLimit`）、チャンクの分け方と並列度は、変えない。

## 成功基準

- **SC-001**: 偽の提案器で、ルート n 本の LLM 呼び出し回数が `ceil(n / productionSituationChunkSize)`。判断の呼び出しは 0 回。
- **SC-002**: 偽の提案器が返した `reply_appeal` と理由が、`ReactionPlan` の `Appeal` と `Reason` に保存され、`Judged` が true。
- **SC-003**: 範囲外・欠落の段階が、段階 2 と `fallback:` の理由になり、チャンクの再試行（提案器の追加呼び出し）が起きない。
- **SC-004**: スキーマの項目の順序が、Situation の項目、`titles`、`reply_appeal`、`reply_appeal_reason` の順。
- **SC-005**: プロンプトに、段階の定義（0〜4）と、「すべてを中位にしない」旨と、活動できる住民の数が入る。
- **SC-006**: `grep -rn "JudgeReplyAppeal\|ReplyAppealJudge" apps/server` が、テスト以外で 0 件。
- **SC-007**: `go test ./...` が通る。`go test -race` で `worldrepo`、`bbsengine`、`llm`、`world` が通る。
- **SC-008（デプロイ後に確認）**: Render のログの `header_materialize_total` − `planner_total` が 2 秒未満。`BBS reply appeal:` のログが、これまでどおり出る。

## スコープ外

- 件数への換算式、段階の倍率、活動人数の数え方の変更。
- ルートのヘッダだけを先に返し、返信ヘッダを後から足すこと。
- モデル、推論の重さ、プロンプトの短縮。
- チャンクサイズ、並列度の変更。
- 保存済みの計画の再計算（過去の計画は変えない）。
