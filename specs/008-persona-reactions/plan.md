# Plan: ペルソナ特性にもとづくレス・反応の選択

`spec.md` の設計。数や名前は、読んだコードで確認したものだけを書いている。確認できていない点は、末尾「実装時に確認すること」にある。

## 1. 現状の分類（実コードにもとづく）

| 項目 | 場所 | 現状 |
|---|---|---|
| レスの本数（スレッド単位） | `world/thread_replies.go` の `ThreadReplyCount` | ゼロ膨張・重い裾の幾何分布。板の `reply_rate` が平均。**本 spec では変更しない** |
| 担い手の選定 | `bbsengine/engine.go` の `actorRoster` | `personaActivityWeight`（パターン 5 種: regular/active/occasional/lurker/dormant、`lurker_tendency` で減衰）と、時間帯ごとの揺らぎ。上位 `activeActorTarget` 人（12〜72）を残す |
| 返信先の選定 | `bbsengine/engine.go` の `planSlots` / `addThreadReplies` | 一様ランダム（定期更新）、または親ルート固定（初回バッチ）。担い手はハッシュで割り当て |
| 人間の投稿 | `erikak/runtime.go`、`turbobbs/runtime.go` の `Store.AddPost` | 保存のみ。`Intent` は空 |
| 返信の言語化 | `worldrepo/llm_materializer.go`（`personaSummary`）、`bbs_article_worker_context.go` | 実装済み。**本 spec では変更しない** |
| 永続化の前例 | `world/store.go` の `BoardActivityStateStore` | 計画をプロセスと再起動をまたいで保存する仕組みがある。計画の保存に流用する |
| 助言モデル | `worldengine/jev.go` の `BehaviorAdvisor` | 実装はあるが、記事生成からの呼び出し元がない。**この経路では使わない** |

ペルソナの項目（`world/models.go` の `Persona`）: `ActivityPattern`、`ReplyTendency`、`ThreadStartTendency`、`LurkerTendency`、`NewcomerOpenness`、`Argumentativeness`、`Interests`（13 領域: local, chat, daily_life, food, shopping, transport, offline_meetings, music, games, anime_manga, communications, software, hardware）、`Opinions`。

## 2. 全体の流れ

```text
記事の保存（人間 / NPC）
   │  Repository.AddPost（人間）、bbsengine（NPC のルート）
   ▼
ReactionPlan を作る（純関数・LLM なし）
   ├─ 本数 n     = world.ThreadReplyCount(host, board, ordinal, reply_rate)
   ├─ 領域タグ t = 板の事前分布 ⊕ 記事タグ ⊕（人間なら辞書照合）
   ├─ 候補       = 活動度と時間帯で上位 K 人
   └─ k = 1..n について、重みつきサンプリングで担い手、遅延分布で予定時刻
   │  保存（担い手と予定時刻を含む）
   ▼
観測（板の一覧、スレッド表示、定期更新）で、予定時刻 ≤ 世界の現在 の返信を実体化
   │  既存の「既存記事への返信スロット」（ReplyToPostID）を使う。ヘッダの生成に LLM は不要
   ▼
本文は、読まれたときに遅延生成（既存どおり。ここで初めて LLM）
```

## 3. データ

### 3.1 `world.ReactionPlan`（新規）

```go
type ReactionPlan struct {
    TargetPostID int64
    Ordinal      int       // 板内のルート通し番号（本数の抽選キー）
    PlannedAt    time.Time
    Replies      []PlannedReaction
}
type PlannedReaction struct {
    Index     int       // 1..n、同じ計画内で一意
    PersonaID string
    DueAt     time.Time
    // 実体化済みなら、その投稿 ID。0 は未実体化。
    MaterializedPostID int64
}
```

- 保存は `ReactionPlanStore`（`BoardActivityStateStore` と同じ形のインターフェース）。メモリ実装と永続ストア（`worldpersist`）の両方に足す。
- 実体化した返信の `Intent.ProducerEventID` は `react:<TargetPostID>:<Index>` とする。実体化の冪等性は、この ID の存在確認で担保する。

### 3.2 `PostIntent.Domains []string`（新規フィールド、omitempty）

NPC の記事に付く、興味領域のタグ（13 領域の部分集合、最大 3 個）。既存の `PostIntent` の JSON に omitempty で足すので、既存データは変わらない。

### 3.3 局の定義（YAML。`hostcatalog/presets/*.yaml`）

板ごとの任意項目:

```yaml
boards:
  - path: "3"
    domains: { games: 1.0, software: 0.4 }   # 板の領域の事前分布（省略可）
    welcomes_newcomers: true                  # 新人の投稿に反応しやすい板（省略可）
```

- 省略時は、全領域を同じ重みとし、`welcomes_newcomers` は false。
- 項目は `hostcatalog` のローダと検証（未知の領域キー、負の重みはエラー）に追加する。コードに局名・板番号は書かない。

## 4. 数式（すべて架空の調整値。歴史統計ではない）

記号: 記事 a、住民 p、返信番号 k、予定時刻 t。

### 4.1 領域タグ

`t_a(d) = normalize( board_prior(d) × (1 + 2 × article_tag(d)) )`  
`article_tag(d)` は、NPC の記事なら `Intent.Domains` に含まれれば 1、人間の投稿なら辞書照合のヒット比、それ以外は 0。

人間の投稿の辞書は、`world/domain_lexicon.go` に、13 領域それぞれの 1996 年の語彙リストとして持つ（Go のデータ表）。
現代語を含めない。辞書は**架空の再構成**で、網羅を目指さない。ヒットが 0 のときは、板の事前分布だけを使う。

### 4.2 関連度

`R(p,a) = Σ_d t_a(d) × I_p(d)`、`I_p(d)` は `Persona.Interests[d]`（なければ 0）。範囲は 0〜1。

### 4.3 担い手の重み

```text
w(p) = A(p,t) × (ε + R(p,a))^α × (δ + ReplyTendency(p)) × S(p,thread,k)
```

- `A(p,t) = personaActivityWeight(pattern, lurker) × H(pattern, hour(t))`  
  `H` は時間帯係数。ダイヤルアップの夜間割引（`docs/research/NTT_DIAL_TARIFF_1996.md`）を参考にした、夜に高い曲線。
  **Likely / inferred**（根拠は推定）。初期値は、日中 0.6、夕方 0.9、夜（21 時〜翌 2 時）1.0、未明 0.5。
- 初期値: `ε = 0.15`、`α = 1.5`、`δ = 0.10`。
- `S`（スレッド状態の補正。積）:
  - 同じスレッドで既に n 回返信済み: `0.35^n`
  - k ≥ 2 で、`Argumentativeness` が高い: `1 + 0.8 × Argumentativeness`
  - 親記事の著者が新人（人間の初投稿、または `welcomes_newcomers` の板）: `1 + 1.0 × NewcomerOpenness`
  - k ≥ 2 で p がスレ主: `2.5`（戻り返信）
  - 親記事の著者本人、または直前の返信者: `0`
  - 住民どうしの関係の項: 常に 1（将来の差し込み口）
- 候補は、`A` の上位 `K = 64` 人（`activeActorTarget` の方式と整合させる）。

### 4.4 サンプリング

決定論的な重みつき非復元抽出（Efraimidis–Spirakis）: `key(p) = u(p)^(1/w(p))`、`u(p)` は
`hash(host|board|post|k|persona|"react-v1")` から作る (0,1) の値。key の大きい順に、`S` の制約を満たす最初の 1 人を選ぶ。
同じ入力なら、同じ人が選ばれる。

### 4.5 遅延

`gap_k = 15分 × u^-0.9 × scale(pattern)`（k=1 は親からの遅延、k≥2 は直前の返信からの遅延）。
`scale`: regular 0.6、active 0.8、occasional 1.5、lurker 3.0、dormant 6.0（初期値）。
予定時刻が世界の現在を超える返信は、現在までに収まる範囲に押し込むのではなく、**そのまま未来の予定として保存**する
（観測時刻が来るまで実体化しない）。人間にとっては、返信が少し遅れて現れることになる。

## 5. 実体化

- 観測の入口（板の一覧、スレッド表示、`CatchUp`）で、保存済みの計画のうち `DueAt ≤ 世界の現在` かつ `MaterializedPostID == 0` のものを実体化する。
- 返信記事は、既存の既存記事への返信（`ReplyToPostID`）の経路で作る。この経路は、ヘッダ（件名と意図）の生成に LLM を使わない
  （`bbs_situation_first_planner.go` の `reply_to_existing`）。
- 保存後に `MaterializedPostID` を更新する。更新に失敗しても返信の二重作成が起きないよう、`ProducerEventID` の存在確認を先に行う。

## 6. 人間の投稿の取り込み

- `worldrepo.Repository.AddPost` で、`AuthorPersonaID` が空で、`Intent.Action` が世界生成のもの（`ActionWorldCatchup`）でない投稿を、人間の投稿とみなす。
  ルート記事（`IsSemanticRoot`）とアペ（`ParentID != 0`）の両方に、計画を作る。
- アペに対する反応は、スレッドのルートの `ordinal` ではなく、アペ自体の通し番号（`ParentID` のスレッド内の順位）をキーに、
  本数を抽選する。同じ `ThreadReplyCount` を使う。
- 計画を作る処理は同期で軽い（数値計算のみ）。失敗しても、投稿の保存は成功させる（ログに残す）。

## 7. NPC の記事の取り込み（段階 2）

- `bbsengine.planSlots` / `addThreadReplies` の「担い手の割り当て」を、4.3〜4.4 のサンプリングに差し替える。
  本数は、これまでどおり `ThreadReplyCount`。
- 定期更新の「4 件に 1 件が返信」（`(i+1)%4 == 0`）と、返信先の一様ランダム選択は、廃止してこの計画に置き換える
  （段階 2 で、既存テストを新しい挙動に合わせて更新する）。
- 人物生成の乱数の消費順序は変えない（`population_golden_test.go` を変更しない）。

## 8. 検証の方針

- **変えてはいけないもの**: 人物生成の金型（`world/population_golden_test.go`）と Erika-K の画面（`erikak/testdata/screens_golden.txt`、`boards_golden.json`）。
  これらは**変更前から存在する金型**なので、新規に生成せず、実装後もそのまま通ることだけを確認する。
- **統計テスト**: 固定シード、十分な回数（各 5,000〜20,000）で、US2 と US4 の向きの差を検証する。しきい値は、
  期待比の許容幅を持たせる（厳密な値の一致は求めない）。
- **LLM 非使用**: テスト用の偽の `llm` プロバイダに呼び出し回数を数えさせ、計画から実体化までで 0 であること。
- **冪等性**: 同じ計画を 3 回実体化しても、返信数が増えない。
- **人間の投稿**: 人間の投稿を保存 → 計画が保存 → 時刻を進めて観測 → 返信が現れ、`ReplyToPostID` が人間の投稿になる。
  本数 0 の場合は、返信が作られない。

## 9. 変更するファイル（予定）

新規: `world/reaction.go`（計画・純関数）、`world/reaction_test.go`、`world/domain_lexicon.go`、`world/reaction_store.go`（インターフェース）
変更: `world/models.go`（`PostIntent.Domains`）、`world/store.go`（メモリ実装）、`worldpersist/*`（保存）、
`hostcatalog/*`（YAML の項目と検証）、`hostcatalog/presets/*.yaml`（任意の板定義）、`worldrepo/repository.go`（`AddPost` と観測）、
`bbsengine/engine.go`（段階 2）、`llm/*` の Situation 提案の出力に `domains`（段階 2。既存の呼び出しのスキーマに足す）、
`docs/WORLD_SIMULATION.md`、`docs/BOARD_ACTIVITY_PLANNING.md`。

## 10. リスクと対策

| リスク | 対策 |
|---|---|
| 係数が不自然な分布を作る | 統計テストと、固定シードの分布の目視（`verification.md` に分布を載せる）。係数は定数としてまとめ、調整を 1 か所にする |
| 返信が現れるまで人間に見えず「反応がない」ように感じる | 予定時刻は分〜時間単位。観測のたびに実体化するので、再入室や板の再表示で現れる。通知（プッシュ）は、スコープ外 |
| 計画の保存が肥大化する | 1 記事あたり最大 `MaxThreadReplies`（30）。実体化済みで古いものは、記事の保持期限に合わせて削除（段階 2 で検討） |
| 人間の短文アペの領域照合が弱い | 板の事前分布に落ちる設計。辞書は後から拡張できる |
| 乱数の消費順序の変更で、既存の人物が変わる | 新しい乱数は、すべて記事・住民のハッシュから作り、人物生成の `rand.Rand` を使わない。金型で確認 |

## 実装時に確認すること

- 人間の投稿が、すべて `worldrepo.Repository.AddPost` を通るか（ホストプログラムの `Store` が `Repository` か `Base` か）。通らない経路があれば、その経路にも同じ取り込みを足す。
- `worldpersist` が、`ReactionPlanStore` の保存で、既存のスナップショットと互換か（空のスナップショットの読み込みで失敗しない）。
- `BoardActivityState` が数える返信数と、実際に実体化された返信数の整合（初回バッチの上限との関係）。
- `ThreadReplyCount` の通し番号（`ordinal`）の定義を、人間の投稿にも一貫して使えるか。
