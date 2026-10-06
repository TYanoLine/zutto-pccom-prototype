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
| 助言モデル | `worldengine/jev.go` の `BehaviorAdvisor` | 実装はあるが、記事生成からの呼び出し元がない。**任意の差し込み口（4.7）からだけ使う。既定は無効** |

ペルソナの項目（`world/models.go` の `Persona`）: `ActivityPattern`、`ReplyTendency`、`ThreadStartTendency`、`LurkerTendency`、`NewcomerOpenness`、`Argumentativeness`、`Interests`（13 領域: local, chat, daily_life, food, shopping, transport, offline_meetings, music, games, anime_manga, communications, software, hardware）、`Opinions`。

## 2. 全体の流れ

```text
記事の保存（人間 / NPC）
   │  Repository.AddPost（人間）、bbsengine（NPC のルート）
   ▼
ReactionPlan を作る（純関数・LLM なし）
   ├─ 本数 n     = world.ThreadReplyCount(host, board, ordinal, reply_rate)
   ├─ 話題キー  = 確定データ ⊕ 語彙表の照合 ⊕ 板の事前分布（取れなければ「不明」。任意でタグ付け）
   ├─ 候補       = 活動度と時間帯で上位 K 人
   └─ k = 1..n について、重みつきサンプリングで担い手、リズム（時代・曜日・局・板・類型）に沿った遅延で予定時刻
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
    Advice    float64   // 助言を使った場合の量子化済みの値（なければ 0）
    // 実体化済みなら、その投稿 ID。0 は未実体化。
    MaterializedPostID int64
}
```

- 保存は `ReactionPlanStore`（`BoardActivityStateStore` と同じ形のインターフェース）。メモリ実装と永続ストア（`worldpersist`）の両方に足す。
- 実体化した返信の `Intent.ProducerEventID` は `react:<TargetPostID>:<Index>` とする。実体化の冪等性は、この ID の存在確認で担保する。

### 3.2 話題キー（開いた語彙）

`PostIntent.Topics []string`（新規、omitempty）: 記事に付く話題キー。**閉じた一覧ではない**。

- 形式: 小文字・空白なしの文字列。`games` のような粗い基本層のキー（13 領域。`randomResidentInterests` と同じ語彙）と、
  `ff7`、`telehodai`、`hakata_event` のような具体キーが同居できる。未知のキーを拒否しない。
- 住民側のキーは、既存のデータから作る（新しい項目は足さない）:
  - `Persona.Interests`（キーは任意の文字列を許す。現状の生成は 13 領域）
  - `Persona.Opinions`（キー → 重みのマップ）
  - 確定した `PersonaFact.Topic`
- 記事側のキーは、次の順で取り出す。LLM は使わない:
  1. `Intent.AnchorKey`、`ProducerReferents`、`Topic`、`SituationFacts` の参照対象（`referent:` で始まるもの）
  2. 件名・本文から、**語彙表**に載っている語を拾う（人間の投稿はこれが主）
  3. 板の事前分布（局の定義）
  4. 取れなかったとき: 「不明」。`ε` と想定外の参加が働く（4.2）
- 領域キーとの対応（具体キー → 粗い基本層）は、語彙表の `broader` で持つ。例: `ff7` → `games`。これで、具体キーを知らない住民も、
  基本層を通して部分的に結びつく。

### 3.3 語彙表（データ。コードに固定しない）

時代・局・板ごとの語彙は YAML で与える。局の定義と同じ仕組みで読み込み、層として重ねる（時代 → 局 → 板の順に上書き）。

```yaml
# 例: 時代の語彙（apps/server/internal/world/data/lexicon-1996.yaml のような場所。場所は T012 で決める）
topics:
  games:        { words: ["セーブ", "ステージ", "クリア"], broader: [] }
  ff7:          { words: ["FF7", "ファイナルファンタジー"], broader: [games] }
  telehodai:    { words: ["テレホーダイ", "テレホ"], broader: [communications] }
```

- 語彙表にない話題も、記事の `Topics` にそのまま入れられる（表は「拾うための手がかり」であり、許可リストではない）。
- 顕著さ（流行）は、日付範囲つきの上げ下げで与える（4.1 の `salience`）:

```yaml
salience:
  - { topic: ff7, from: "1996-09-01", to: "1997-03-31", boost: 1.8 }
```

### 3.4 局の定義（YAML。`hostcatalog/presets/*.yaml`）

板の任意項目と、局の任意項目:

```yaml
rhythm:                       # 局（省略可）
  member_mix: { student: 0.35, office: 0.40, homemaker: 0.10, night: 0.15 }   # 会員の生活類型の割合
  line_capacity: 4            # 回線数（省略可。混雑の層で使う）
boards:
  - path: "3"
    topics: { games: 1.0, software: 0.4 }     # 板の話題の事前分布（省略可。キーは自由）
    welcomes_newcomers: true
    rhythm: { member_mix: { student: 0.6, office: 0.2, night: 0.2 } }   # 板ごとの会員構成の上書き（省略可）
```

- 省略時の既定: 全話題を同じ重み、`welcomes_newcomers: false`、会員構成は「一般」（学生・会社員・家事・夜型を、時代のデータの既定比で混ぜる）。
- 検証: 負の重み、割合の合計が 0 は、エラー（板の path を含める）。話題キーは**検証しない**（開いた語彙）。
- コードに局名・板番号は書かない。

## 4. 数式（すべて架空の調整値。歴史統計ではない）

記号: 記事 a、住民 p、返信番号 k、予定時刻 t。

### 4.1 話題の重み（記事側）

`T_a(x) = normalize( board_prior(x) × (1 + 2·tag(x)) × salience(x, 日付) )`、`x` は話題キー。
`tag(x)` は、記事の `Topics` に含まれれば 1。含まれないキーは 0。`broader` を通して、具体キーは基本層にも半分の重みで伝える。

### 4.2 関連度と想定外の参加

`R(p,a) = Σ_x T_a(x) × I_p(x)`、`I_p(x)` は、住民のキー重み（`Interests`、`Opinions`、確定した `PersonaFact.Topic`。なければ 0）。範囲 0〜1。

- 記事側が「不明」（`T_a` が平らな事前分布のみ）のときは、`R` が全員ほぼ同じ値になり、話題による差が出ない。
- **想定外の参加**: 重みの式に `(ε + R)^α` を使うので、`R = 0` の住民にも、`ε^α` の最小確率が残る。さらに、
  `wildcard` 確率（初期値 0.08）で、`R` を無視して、活動度だけで選ぶ。話題の合う人だけが返信する、不自然な世界にならないための設計。

### 4.3 担い手の重み

```text
w(p) = A(p,t) × (ε + R(p,a))^α × (δ + ReplyTendency(p)) × S(p,thread,k) × Advice(p)
```

- `A(p,t) = personaActivityWeight(pattern, lurker) × Rhythm(p, t)`。`Rhythm` は 4.5。
- 初期値: `ε = 0.15`、`α = 1.5`、`δ = 0.10`、`K = 64`。
- `S`（スレッド状態の補正。積）:
  - 同じスレッドで既に n 回返信済み: `0.35^n`
  - k ≥ 2 で、`Argumentativeness` が高い: `1 + 0.8 × Argumentativeness`
  - 親記事の著者が新人（人間の初投稿、または `welcomes_newcomers` の板）: `1 + 1.0 × NewcomerOpenness`
  - k ≥ 2 で p がスレ主: `2.5`
  - 親記事の著者本人、または直前の返信者: `0`
  - 住民どうしの関係の項: 常に 1（将来の差し込み口）
- `Advice(p)` は 4.7。助言がなければ 1。

### 4.4 サンプリング

決定論的な重みつき非復元抽出（Efraimidis–Spirakis）: `key(p) = u(p)^(1/w(p))`、`u(p)` は
`hash(host|board|post|k|persona|"react-v1")` から作る (0,1) の値。key の大きい順に、`S` の制約を満たす最初の 1 人を選ぶ。

### 4.5 活動のリズム（層の合成）

```go
// 1 つの層。他の層を知らない。
type RhythmLayer interface {
    Name() string
    // t の時点で、p（と、その属する局・板）が活動しやすい度合い。1 が中立。
    Factor(ctx RhythmContext) float64
}
type RhythmContext struct {
    Host world.Host; Board world.Board; Persona world.Persona
    Archetype string        // 住民の生活類型（4.6）
    At time.Time            // 世界の時刻（曜日・日付を含む）
}
```

合成: `Rhythm(p,t) = clamp( exp( Σ_層 ln Factor_層(ctx) ), 0.03, 4 )`。対数で足すので、層の順序に依存せず、層の追加は他に影響しない。
層は、データ（YAML）と小さな Go の実装で、次のように与える（初期の層。後から足せる）:

| 層 | 見るもの | データの出どころ |
|---|---|---|
| 時代の通信料金 | 時刻（テレホーダイの夜間割引の時間帯など） | 時代のデータ（`docs/research/NTT_DIAL_TARIFF_1996.md` を根拠にした表。**Likely / inferred**） |
| 曜日・祝日 | 曜日、祝日 | 世界の暦。時代のデータ |
| 世界の出来事 | 日付 | 時代・局のデータ（イベントの日） |
| 生活類型 | 類型（学生、会社員、家事、夜型）× 曜日 × 時刻 | 時代のデータ（類型ごとの 7×24 の相対重み） |
| 局の混雑 | 回線数、時間帯の人気 | 局の定義（省略可） |
| 住民の個性 | `ActivityPattern` | 既存 |

- 各層の既定は、データがなければ 1（中立）。データを足すだけで、挙動が変わる。
- 時間帯の表は、**Go のコードに埋めず**、時代のデータ（YAML）に置く。コードは、読み込んで合成するだけ。
- 計算は、時刻（時間単位）ごとにキャッシュしてよい。

**遅延への使い方（時間の再スケーリング）**: 返信の遅延は、「時計の時間」ではなく「活動しやすさで重みづけした時間」で数える。
`gap_eff = 15分 × u^-0.9 × scale(pattern)` を、リズムの累積 `Λ(t)=∫Rhythm(p,s)ds` の上で進め、実時刻に戻す。
活動しにくい時間帯（平日の昼の会社員など）は、飛ばされ、活動しやすい時間帯（夜）に返信が現れる。予定時刻は計画時に確定し、保存する。

### 4.6 住民の生活類型（`Archetype`）

- 類型は、人物 ID のハッシュと、その板（または局）の `member_mix` から、決定論的に割り当てる（人物生成の乱数を使わない）。
- 確定した人物事実（職業など）が後から現れたら、その事実の類型へ移す。それは、計画の**新規作成**にだけ影響し、保存済みの計画は変えない。
- 類型の名前は閉じた一覧ではなく、時代のデータで与える（未知の類型は、中立の 7×24 を持つ）。

### 4.7 任意の助言（Jev など）の差し込み口

```go
// 実装は任意。nil なら、助言なし（Advice = 1）。
type ReactionAdvisor interface {
    // 話題タグ付け: 取れなかった人間の投稿の話題キーを抽出する。決定には使わない。
    TagTopics(ctx context.Context, post world.Post) ([]string, error)
    // 事前確率: 候補ごとの「反応しやすさ」の助言（0〜1）。
    AdviseReactors(ctx context.Context, req AdviceRequest) (map[string]float64, error)
}
```

- **話題タグ付け**: 人間の投稿の保存時に、辞書で取れなかった場合だけ、1 投稿 1 回。複数投稿を窓でまとめてもよい。結果は、記事の `Topics` に保存する。
  失敗・無効なら、`Topics` は空のまま（4.2 の「不明」に落ちる）。**誰が反応するかを決めさせない。**
- **事前確率**: 上位 `K` 人の候補を 1 回の呼び出しにまとめる。結果は `0.05` 刻みに量子化し、`Advice(p) = 1 + β·(advice − 0.5)·2`、`β ≤ 0.4`（少数派）。
  結果は計画に保存する（再計算で変わらない）。助言モデルは、返信先・話題・事実を作らない。
- 既定は無効。`worldengine` の既存の助言機構（`BehaviorAdvisor`）への薄いアダプタを実装し、設定で有効にする。偽の実装を使ったテストで、有効・無効のどちらでも決定論性が保たれることを確認する。
- コストの目安: 無効なら 0 回。有効でも、人間の投稿 1 件あたり最大 2 回（タグ付けは辞書で取れなかったときだけ）。

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

新規: `world/reaction.go`（計画・純関数）、`world/reaction_test.go`、`world/rhythm.go`（層の合成）、`world/lexicon.go`（語彙表の読み込みと照合）、
`world/reaction_store.go`（インターフェース）、時代のデータ（語彙・リズム・暦。場所は T012 で決める）、`worldrepo/reaction_advisor.go`（助言のアダプタ）
変更: `world/models.go`（`PostIntent.Topics`）、`world/store.go`（メモリ実装）、`worldpersist/*`（保存）、
`hostcatalog/*`（YAML の項目と検証）、`hostcatalog/presets/*.yaml`（任意の板定義）、`worldrepo/repository.go`（`AddPost` と観測）、
`bbsengine/engine.go`（段階 2）、`llm/*` の Situation 提案の出力に `topics`（段階 2。既存の呼び出しのスキーマに足す。13 領域に限らない）、
`docs/WORLD_SIMULATION.md`、`docs/BOARD_ACTIVITY_PLANNING.md`。

## 10. リスクと対策

| リスク | 対策 |
|---|---|
| 係数が不自然な分布を作る | 統計テストと、固定シードの分布の目視（`verification.md` に分布を載せる）。係数は定数としてまとめ、調整を 1 か所にする |
| 返信が現れるまで人間に見えず「反応がない」ように感じる | 予定時刻は分〜時間単位。観測のたびに実体化するので、再入室や板の再表示で現れる。通知（プッシュ）は、スコープ外 |
| 計画の保存が肥大化する | 1 記事あたり最大 `MaxThreadReplies`（30）。実体化済みで古いものは、記事の保持期限に合わせて削除（段階 2 で検討） |
| 人間の短文アペの話題抽出が弱い | 板の事前分布と「不明」に落ちる設計。語彙表はデータで拡張でき、任意の助言でタグ付けもできる |
| 層が増えて、挙動の理由が追えなくなる | 層ごとの寄与（`Factor`）を、デバッグ用に出力できるようにする。層は単体でテストする |
| 助言モデルの結果で、再現性が失われる | 量子化して計画に保存。無効・失敗で決定論的な処理に戻る。偽の助言でテスト |
| 乱数の消費順序の変更で、既存の人物が変わる | 新しい乱数は、すべて記事・住民のハッシュから作り、人物生成の `rand.Rand` を使わない。金型で確認 |

## 実装時に確認すること

- 人間の投稿が、すべて `worldrepo.Repository.AddPost` を通るか（ホストプログラムの `Store` が `Repository` か `Base` か）。通らない経路があれば、その経路にも同じ取り込みを足す。
- `worldpersist` が、`ReactionPlanStore` の保存で、既存のスナップショットと互換か（空のスナップショットの読み込みで失敗しない）。
- `BoardActivityState` が数える返信数と、実際に実体化された返信数の整合（初回バッチの上限との関係）。
- `ThreadReplyCount` の通し番号（`ordinal`）の定義を、人間の投稿にも一貫して使えるか。
