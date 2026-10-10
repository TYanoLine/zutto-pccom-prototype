# Tasks: ペルソナ特性にもとづくレス・反応の選択

**Input**: `specs/008-persona-reactions/` の `spec.md`、`plan.md`  
**Prerequisites**: `AGENTS.md`、`plan.md`（特に「4. 数式」「5. 実体化」「6. 人間の投稿の取り込み」）

## この作業のルール（必読）

- 作業ブランチから `main` への PR を 1 本作る（draft）。`main` に直接コミットしない。
- **成果物は Go / YAML のコードの変更（実装）と、`verification.md`。** `spec.md`、`plan.md`、`tasks.md` は作らず、変更もしない。
- **既存の金型を 1 バイトも変えない。** 対象は `apps/server/internal/world/population_golden_test.go` の金型、
  `apps/server/internal/hostprogram/erikak/testdata/*`。これらは**変更前から存在する**ので、新しく生成しない。
  更新用のフラグ（`-update-golden` など）を、この作業では一度も使わない。失敗したら、実装を直す。
  2 回直して通らなければ、そこで止めて、どの行が違うかを報告する。
- **人物生成の乱数の消費順序を変えない**（`world/population.go` の `rand.Rand` の呼び出しを、足さず、減らさず、並べ替えない）。
  新しい乱数は、すべて記事・住民の文字列ハッシュから作る。
- **反応の決定に LLM を必須としない。** 計画、担い手の選択、実体化は、数値計算だけで完結する。`llm` パッケージ、`worldengine` の助言（Jev）は、
  `ReactionAdvisor` の差し込み口（`plan.md` 4.7）の**外では呼ばない**。差し込み口は既定で無効で、nil のときは助言なしで動く。
  助言モデル（Jev）自体は改修しない（アダプタを足すだけ）。
- **語彙と時間の知識は、データ。** 語彙表、リズム（時間帯・曜日・類型）、暦、顕著さの表は、YAML のデータとして置く。Go のコードに、固定の一覧や時間帯の表を埋め込まない。
  話題キーを、13 領域に限定する検証や分岐を書かない（開いた語彙）。
- スコープ外（`spec.md` 末尾）に手を出さない。返信本文の生成方法、プロンプト、引用の選び方は変更しない。
- 局固有の内容（板ごとの領域、新人歓迎）は YAML に置く。Go のコードに局名・板番号・電話番号を書かない。
- 数式の係数は、`world/reaction.go` の先頭の定数にまとめ、「架空の調整値。歴史統計ではない」とコメントする。
- 既存ファイルに整形だけの差分を作らない。`gofmt -w` を既存ファイル全体にかけない（新規ファイルには、かけてよい）。
- 1 タスク = 1 コミット（メッセージは `T0XX: 内容`）。途中でビルドが通らないタスクは、1 つにまとめる。
- 実行していないコマンドを「成功した」と書かない。実行できなかったものは「未実施」と理由を書く。
- `.github/` 配下は変更しない。
- spec と実際のコードが食い違っていたら、推測で進めず、PR の説明に書いて止まる。
- この tasks のコードは、コンパイルして確認していない。実際のコードに合わせて直し、直した点を PR に書く。

---

## Phase 0: 前提の確認（コードは書かない）

- [ ] **T000** 前提を確認し、結果を `verification.md` に書く

  ```bash
  git -C . log --oneline -3
  ls apps/server/internal/world/thread_replies.go
  go -C apps/server test ./internal/... -count=1
  grep -rn "AddPost(" apps/server/internal --include=*.go | grep -v _test
  ```

  - `world.ThreadReplyCount` が存在すること。無ければ止まる（先行 PR のマージが必要）。
  - 既存の全テストが、変更前のコードで通ること。通らないものは、名前を記録して続行する（既存の失敗として報告）。
  - 人間の投稿が、どの `AddPost` を通るか（`plan.md` 末尾の確認事項 1）を書く。`worldrepo.Repository.AddPost` を通らない経路があれば、T040 でその経路にも足す。

**Checkpoint**: 前提が確認され、既存の失敗が記録されている。

---

## Phase 1: 純関数（計画の数式）

- [ ] **T010** `apps/server/internal/world/reaction.go`（新規）に、型と定数を書く

  `plan.md` の 3.1 の `ReactionPlan`、`PlannedReaction`、4 章の定数（`ε`、`α`、`δ`、`K`、`wildcard`、`β`、遅延の尺度）。時間帯の表は、ここに置かない（T013 のデータ）。

- [ ] **T011** 同ファイルに、純関数を書く（LLM と外部状態を使わない）

  ```go
  // 話題の重み: 板の事前分布 × 記事の話題キー × 顕著さ（plan 4.1）。キーは任意の文字列
  func ArticleTopicWeights(boardPrior map[string]float64, topics []string, lex *Lexicon, salience []Salience, on time.Time) map[string]float64
  // 関連度 R(p,a)（plan 4.2）。住民のキーは Interests、Opinions、確定した PersonaFact.Topic
  func Relevance(p Persona, facts []PersonaFact, weights map[string]float64) float64
  // 担い手の重み w(p)（plan 4.3）。rhythm は plan 4.5 の合成値、advice は助言（なければ 1）
  func ReactionWeight(p Persona, rel, rhythm, advice float64, st ThreadState, k int) float64
  // 決定論的な非復元抽出で k 番目の担い手を選ぶ（plan 4.4）
  func PickReactor(seed string, candidates []Weighted) (personaID string, ok bool)
  // 遅延（plan 4.5）
  func ReactionDueAt(seed string, from time.Time, r RhythmFunc, pattern string, k int) time.Time // 時間の再スケーリング（plan 4.5）
  // 計画全体
  func PlanReactions(in PlanInput) ReactionPlan
  ```

  - 乱数は `hash/fnv` で `host|board|post|k|persona|"react-v1"` から作る（`thread_replies.go` の `threadUnit` と同じ流儀）。
  - `ThreadState` は、同じスレッドの既往の返信者（回数）、スレ主の ID、直前の返信者、親記事の著者が新人か、を持つ。

- [ ] **T012** `apps/server/internal/world/lexicon.go`（新規）に、語彙表の型と読み込み、照合を書く

  - 構造は `plan.md` 3.3。`topics`（`words`、`broader`）と `salience`（`topic`、`from`、`to`、`boost`）。
  - 照合: 件名・本文から語を拾い、話題キーの集合を返す。`broader` を通して、具体キーを基本層に伝える。
  - 未知のキーを拒否しない。語彙表にない話題は、そのまま使える。
  - 語彙表は、時代 → 局 → 板の順に重ねて読み込む（後のものが上書き）。
  - データファイル（`lexicon-1996.yaml`。場所は `hostcatalog` のデータの置き場に合わせて決める）に、13 の基本層の語を、1996 年の語彙で書く。
    現代語（SNS 用語、`草`、`ググる` など）を入れない（`AGENTS.md` の In-world writing）。「架空の再構成。網羅を目指さない」とデータ先頭に書く。

- [ ] **T013** `apps/server/internal/world/rhythm.go`（新規）に、リズムの層と合成を書く

  - `RhythmLayer`、`RhythmContext`、合成（対数の和、`clamp(…, 0.03, 4)`）は `plan.md` 4.5。
  - 初期の層（それぞれ、データを読み、データがなければ 1）: 時代の通信料金、曜日・祝日、世界の出来事、生活類型 × 曜日 × 時刻、局の混雑、住民の `ActivityPattern`。
  - 時間帯の表は、時代のデータ（`rhythm-1996.yaml`）に置く。テレホーダイの夜間割引は `docs/research/NTT_DIAL_TARIFF_1996.md` を根拠にし、
    根拠が推定であること（Likely / inferred）をデータに書く。
  - 住民の生活類型（`Archetype`）は、人物 ID のハッシュと `member_mix` から決める（`plan.md` 4.6）。人物生成の乱数を使わない。
  - `ReactionDueAt`（時間の再スケーリング）を、リズムの累積で実装する。

- [ ] **T014** `apps/server/internal/world/reaction_test.go`、`rhythm_test.go`、`lexicon_test.go`（新規）に、統計テストと性質テストを書く

  固定シード、各 5,000 回以上で、次を確認する（しきい値は幅を持たせる）:
  1. 決定論: 同じ入力 → 同じ計画。
  2. `ReplyTendency` が高い住民ほど選ばれやすい。lurker / dormant はほとんど選ばれない。
  3. 関連する話題に強い関心を持つ住民が、そうでない住民より有意に多く選ばれる。無関係な住民も、`ε` と `wildcard` で、ときどき選ばれる。
  4. 2 件目以降で `Argumentativeness` が高い住民が選ばれやすい。
  5. 新人の投稿で `NewcomerOpenness` が高い住民が選ばれやすい。
  6. スレ主の戻り返信確率が、非スレ主より高い。
  7. 親記事の著者本人と、直前の返信者は、選ばれない。
  8. 同じ住民は、同じスレッドで 3 回以上続けて選ばれにくい（`0.35^n`）。
  9. 本数 0 のとき、計画の返信は 0 件。
  10. 予定時刻が、親記事より後になる。
  11. **開いた語彙（SC-006）**: 語彙表にも 13 領域にもないキー（例: `x_unlisted`）を、記事と住民の両方に付けると、そのキーで関連度が上がる。コードの変更なしで、テストデータの追加だけで成り立つ。
  12. **リズムの層（SC-007）**: ある層を足しても、既存の層の出力が変わらない。夜型が多い会員構成では、予定時刻が夜に偏る。平日と休日で、学生の予定時刻の分布が異なる。
  13. **層の独立**: 層をデータで差し替えると（例: テレホーダイの時間帯を変える）、その層の寄与だけが変わる。
  14. リズムのデータが空（すべて中立）でも、計画が作れる。

  ```bash
  go -C apps/server test ./internal/world -run 'Reaction|Rhythm|Lexicon' -count=1
  go -C apps/server test ./internal/world -count=1
  ```

**Checkpoint**: 純関数と統計テストが通り、`world` パッケージの既存テスト（人物生成の金型を含む）が、変更なしで通る。

---

## Phase 2: 保存

- [ ] **T020** `world/store.go` に `ReactionPlanStore` のインターフェースと、メモリ実装を足す

  `BoardActivityStateStore`（`world/store.go` の同名）と同じ形にする: 保存、取得（対象記事 ID）、ホストの一覧、実体化済みの印の更新。

- [ ] **T021** `worldpersist` に、同じ保存を足す

  既存のスナップショットとの互換を確認する（空のスナップショットを読み込んでも失敗しない）。互換のためのテストを足す。

- [ ] **T022** `world/models.go` に `PostIntent.Topics []string`（`json:"topics,omitempty"`）を足す

  既存の JSON のシリアライズ結果が、`Topics` が空のとき変わらないことを、テストで確認する。

**Checkpoint**: 保存と読み込みが往復して一致する。既存のスナップショットが読み込める。

---

## Phase 3: 局の定義（YAML）

- [ ] **T030** `hostcatalog` に、局と板の任意項目を足す（`plan.md` 3.4）

  - 局: `rhythm.member_mix`、`rhythm.line_capacity`。板: `topics`（話題 → 重み。キーは自由）、`welcomes_newcomers`、`rhythm.member_mix`（上書き）。
  - 検証: 負の重み、割合の合計が 0 はエラー（板の path を含める）。**話題キーは検証しない**（開いた語彙）。
  - 省略時の既定は、全話題が同じ重み、`welcomes_newcomers: false`、会員構成は「一般」。
  - 既存のプリセットのロードと、`hostcatalog/README.md` の記述を更新する。

- [ ] **T031** 時代のデータ（語彙表、リズム、暦、顕著さ）を足し、評価用プリセット（`hakata-canal-net.yaml`）の数枚の板に `topics` と `rhythm` を足す（任意）

  板に足す場合は、その板の `scope` の文面に沿った内容だけにする。プリセットの他の項目は変えない。
  足さなくても、完了の条件は満たす（既定の動作で通る）。

**Checkpoint**: プリセットのロードが通り、`hostcatalog` のテストが通る。

---

## Phase 4: 人間の投稿への反応（段階 1）

- [ ] **T040** `worldrepo/repository.go` の `AddPost` に、人間の投稿の取り込みを足す

  ```go
  func (r *Repository) AddPost(hostID string, p world.Post) world.Post {
      saved := r.Base.AddPost(hostID, p)
      r.planReactionsForHumanPost(hostID, saved) // 失敗しても投稿は成功。ログのみ。
      return saved
  }
  ```

  - 人間の投稿の判定: `AuthorPersonaID == ""` かつ `Intent.Action != bbsengine.ActionWorldCatchup`（`bbsengine` を import できない場合は、`Intent` の判定を `world` に置く）。
  - ルートもアペも対象（`plan.md` の 6 章）。T000 で見つけた、`Repository.AddPost` を通らない経路にも、同じ関数を呼ぶ。

- [ ] **T041** 観測の入口に、期日の来た計画の実体化を足す

  対象: `worldrepo/observation.go` の `WaitForBoardHeaders`、スレッド表示の入口、`bbsengine.CatchUp`。
  - `DueAt ≤ 世界の現在`（`r.currentWorldTime()`）かつ未実体化の返信を、`ReplyToPostID` の経路で追加する。
  - 実体化の前に、`ProducerEventID = "react:<post>:<index>"` の既存投稿の有無を確認する（冪等）。
  - ヘッダの生成に LLM を使わない。

- [ ] **T042** 統合テストを書く

  1. 人間のルート記事を保存 → 計画が保存される（本数 0 の場合は、返信が作られないことを確認する別のケースも用意）。
  2. 世界の時計を進めて観測 → 返信が現れ、`ResponseTargetID` が人間の投稿になる。担い手は、人間の投稿の著者ではない。
  3. 同じ観測を 3 回繰り返しても、返信が増えない。
  4. 人間のアペにも、計画が作られる。
  5. テスト用の偽の LLM プロバイダの呼び出し回数が 0（SC-002）。
  6. 住民を増やしたあとでも、保存済みの計画の担い手が変わらない（FR-007）。

  ```bash
  go -C apps/server test ./internal/worldrepo -run 'Reaction' -count=1
  go -C apps/server test ./internal/... -count=1
  ```

**Checkpoint**: 人間の投稿への反応が、LLM なしで、冪等に、決定論的に動く。既存の金型が、変更なしで通る。

---

## Phase 5: NPC の記事の担い手を、同じ基準にする（段階 2）

- [ ] **T050** `bbsengine/engine.go` の `addThreadReplies` の担い手の割り当てを、`PickReactor` に差し替える

  - 本数は、これまでどおり `ThreadReplyCount`。
  - 担い手の候補に、`Persona`（`PersonaStore`）を使う。ペルソナが取れないホストでは、これまでの割り当てに戻す（後退を許す条件をコメントで明記）。

- [ ] **T051** 定期更新の返信先の選択（`planSlots` の `(i+1)%4` と一様ランダム）を、同じ計画に置き換える

  `CatchUp` で、新しいルート記事に `PlanReactions` を作り、期日の来た返信を実体化する。
  既存のテスト（`engine_test.go`）のうち、旧仕様（固定の 4 件に 1 件、一様な返信先）に依存するものを、新しい性質（本数、決定論、特性の傾向）に合わせて書き直す。
  書き直した理由を、`verification.md` に書く。

- [ ] **T052** NPC の記事の話題キーを付ける

  - まず、既存の確定データ（`AnchorKey`、`ProducerReferents`、`Topic`、`SituationFacts` の参照対象）から取り出す（LLM なし）。これだけで足りる範囲で完成とする。
  - 追加の LLM 呼び出しは行わない。Situation 提案の既存の構造化出力に、任意の `topics`（自由な文字列、最大 5 個）を足す場合も、
    1 回の出力に含めるだけにする。**13 領域に限定しない**。出力が空でも、生成を失敗させない。
  - 既存の LLM 関連のテスト（`llm/*_test.go`）が通ること。

- [ ] **T053** 任意の助言の差し込み口を実装する（`plan.md` 4.7）

  - `world` に `ReactionAdvisor` のインターフェース。`worldrepo/reaction_advisor.go` に、`worldengine.BehaviorAdvisor`（Jev）への薄いアダプタ。
  - 既定は無効（nil）。設定で有効にする（`config` の既存の設定の流儀に合わせる。キーが未設定なら無効）。
  - 話題タグ付け: 辞書・確定データで話題キーが取れなかった人間の投稿にだけ。結果は `Topics` に保存。失敗しても投稿の保存と計画は成功する。
  - 事前確率: 上位 `K` 人を 1 回にまとめて問い合わせ、0.05 刻みに量子化し、`Advice` を `1 + β·(advice − 0.5)·2`、`β ≤ 0.4` で混ぜる。結果は計画に保存。
  - テスト（偽の助言を使う）: 有効・無効・失敗の 3 通りで、計画が決定論的で、冪等で、助言が `β` の範囲を超えて決定を覆さないこと（SC-008）。

**Checkpoint**: NPC の記事のレスが、同じ基準で選ばれる。`go test ./internal/...` が通る。

---

## Phase 6: 文書と完了

- [ ] **T060** `docs/WORLD_SIMULATION.md` の Jev の節を直す

  反応の決定は統計・確率モデルだけで完結し、助言モデルは `ReactionAdvisor` の差し込み口（話題タグ付けと事前確率の少数派の重み）からだけ、任意で使うことを書く。既定は無効であることも書く。

- [ ] **T061** `docs/BOARD_ACTIVITY_PLANNING.md` に、「本数（`ThreadReplyCount`）と担い手（本 spec）」の関係を書く

- [ ] **T062** `specs/008-persona-reactions/verification.md`（新規）を書く

  - 実行したコマンドと結果（未実施は理由つき）。
  - 統計テストの分布の抜粋（例: 領域に強い関心がある住民と、ない住民の選択率）。
  - 既存の金型が、変更されていないこと（`git diff origin/main -- <金型のパス>` の出力が空）。

- [ ] **T063** `go -C apps/server vet ./...` と `go -C apps/server test ./... -count=1` を実行し、結果を記録する

- [ ] **T064** draft の PR を開く

  PR の説明に、次を書く: 実行したコマンドと結果、金型が変わっていないこと、旧仕様のテストを書き直した理由（T051）、
  コンパイルして確認していなかったコードを直した点、未実施のもの。ワークフロー（`go-test`）が自動で動かない場合は、そう書く。

**完了の定義**: PR の差分に、次が含まれる。
`world/reaction.go`、`world/rhythm.go`、`world/lexicon.go` と各テスト、時代のデータ（語彙・リズム・暦）、`worldrepo/reaction_advisor.go`、保存の実装（`world/store.go`、`worldpersist`）、
`hostcatalog` の変更、`worldrepo/repository.go` と観測の変更、`bbsengine/engine.go` の変更、`docs/` の 2 ファイル、
`specs/008-persona-reactions/verification.md`。金型ファイルは、差分に含まれない。
