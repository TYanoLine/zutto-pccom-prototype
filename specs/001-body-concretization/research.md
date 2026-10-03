# 調査と設計判断: BBS本文の人物別具体化

## 既存の phase 1

**Decision**: 実装済みの人物履歴と Article Detail の入力を基線とし、状態遷移と品質を検証する。

**Rationale**: `materialization_worker_context.go` は同一ホスト・同一人物の先行投稿から最大6件を選び、同一スレッドを除外する。未生成本文は確定済み意味情報で表す。`materialization_interactive_detail.go` は人物情報、時点有効な人物事実、スレッド、人物履歴を渡す。SDD-001 は phase 1 実装済み、品質サンプリング待ちと記す。

**Alternatives considered**: phase 1 全体の再実装は既存の状態遷移と重複する。

## Detail 0件と保存失敗

**Decision**: `PostIntent` に `ArticleDetailsMaterialized bool` を追加し、Detail の完了状態を件数と独立して保存する。正常な0件も `true`、未実行または失敗は `false` とする。本文生成前に詳細と完了状態の保存成功を確認する。

**Rationale**: 現行の `hasInteractiveArticleDetails` は `article_detail=` の有無だけを見る。0件後、本文保存前の再呼び出しでは再提案され得る。`UpdatePost` が失敗しても呼び出し元が成功扱いになり得るため、未保存の詳細を本文へ渡す危険がある。完了状態を独立させれば、0件と未実行を区別できる。

**Alternatives considered**: 必ず1件を強制すると短い相づちを許す SDD に反する。`article_detail_contract=` を完了マーカーとして流用すると、生成向け指示と永続状態が混ざる。複数値の状態列挙は失敗を正本化しない現要件には過剰なので、真偽値を選ぶ。

## Detail 能力と失敗境界

**Decision**: 共有記事具体化処理は明示的な `BBSTitleArticleDetailPlanner` 能力を要求する。planner が未構成、呼び出しに失敗、出力検証に失敗、または記事更新に失敗した場合は Detail 失敗として本文生成を中断する。正常な空配列だけを0件完了として受理する。

**Rationale**: 「能力がない」「provider が失敗した」を0件成功へ変換すると、clarify で選択した失敗時中断を破り、呼び出し環境によって本文品質も変わる。AIを使わないホスト操作は引き続き動作できるが、AI本文を具体化する処理に入った場合は必要能力の欠如を明示的な失敗として扱う。

**Alternatives considered**: planner 不在時に暗黙の0件を返す案は、障害と意図した0件を区別できないため採用しない。本文 renderer の具象型から毎回 planner を推測する現状は入口依存を残すため、依存能力を明示する。

## 人物履歴と時系列

**Decision**: `AuthorPersonaID`、時刻と記事番号、返信スレッド除外による既存の取得境界を維持する。人物事実は `MaterializedAt <= Post.CreatedAt` のものだけ使う。

**Rationale**: SDD の時間境界に一致し、主要なケースは既存テストで確認できる。過去発言は本人の発言記録であり恒久的な人物事実ではない。

**Alternatives considered**: ハンドルだけで人物を照合すると同名を混同する。全履歴を渡すと無関係な話題が支配しやすい。

## ホスト表示と評価

**Decision**: 世界サービスは確定記事を返し、ホスト runtime が固有の件名・アペンド・失敗表示を行う。fresh Lab の読み取り専用ビューで利用可能な複数返信スレッドを評価し、20〜30本は十分な標本がある場合の目安とする。

**Rationale**: `WaitForArticleBody` はスレッド内の同時読みを調整する。Erika-K は独立件名のないアペンドと失敗文言を持つ。SDD は品質の数値閾値を安定した基線まで保留する。通常世界に十分な返信数がなければ実数を報告し、必要に応じて Lab を補助標本に使う。Lab アーカイブは世界正本とは別の評価証拠である。

**Alternatives considered**: 共通ホスト UI や単一生成例の主観評価は採用しない。

## 適用範囲

**Decision**: Article Detail と本文作成を共有記事具体化エンジンの処理にする。通常閲覧、開発用の直接確認、Lab は同じ処理を呼び出し、Lab は隔離データを供給する。

**Rationale**: 現在は `developmentInteractiveTitleFirstEnabled` が呼び出しを制限しているが、これは入口に結び付いた実装上のフラグである。記事本文具体化の可否を入口で変えると、通常閲覧で別の意味・品質規則になり、Lab の結果も対象挙動を代表できない。入口の違いはデータ隔離・操作・表示にとどめ、選択済み記事から本文を確定する処理は共有する。

**Alternatives considered**: Lab 専用の本文具体化ロジックや、通常閲覧だけで Detail を省略する構成は採用しない。ホスト固有の操作・件名・返信形式は各 HostProgram が引き続き扱う。

## 旧記事との互換性

**Decision**: `ArticleDetailsMaterialized` が欠落または `false` で本文が空の記事は未処理として初回閲覧時に具体化する。本文が既に保存された記事は状態が欠落していても再生成しない。

**Rationale**: JSON の新しい真偽値は旧スナップショットでゼロ値になる。本文済み記事を遡って再処理すると、確定済み本文と別の詳細を作る危険がある。本文未確定記事だけを新しい共有処理へ移行すれば、既存の正本を変えず段階的に適用できる。

**Alternatives considered**: 起動時の一括移行は未観察記事を広く処理し、遅延具体化に反する。本文済み記事への後付け完了マーカー書き込みも本機能には不要。

技術上の未解決事項はない。生成品質は実測待ちとして扱う。
