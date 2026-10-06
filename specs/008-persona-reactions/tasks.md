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
- **LLM を呼ばない。** 反応の計画、担い手の選択、実体化で、`llm` パッケージと `worldengine` の助言（Jev）を呼ばない。
  助言モデルのコードは削除も改修もしない（スコープ外）。
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

  `plan.md` の 3.1 の `ReactionPlan`、`PlannedReaction`、4 章の定数（`ε`、`α`、`δ`、`K`、時間帯係数、遅延の尺度）。

- [ ] **T011** 同ファイルに、純関数を書く（LLM と外部状態を使わない）

  ```go
  // 領域タグ: board_prior ⊕ article_tag を正規化（plan 4.1）
  func ArticleDomainWeights(boardPrior map[string]float64, articleTags []string, lexiconHits map[string]float64) map[string]float64
  // 関連度 R(p,a)（plan 4.2）
  func Relevance(p Persona, weights map[string]float64) float64
  // 担い手の重み w(p)（plan 4.3）
  func ReactionWeight(p Persona, rel float64, t time.Time, st ThreadState, k int) float64
  // 決定論的な非復元抽出で k 番目の担い手を選ぶ（plan 4.4）
  func PickReactor(seed string, candidates []Weighted) (personaID string, ok bool)
  // 遅延（plan 4.5）
  func ReactionDelay(seed string, pattern string, k int) time.Duration
  // 計画全体
  func PlanReactions(in PlanInput) ReactionPlan
  ```

  - 乱数は `hash/fnv` で `host|board|post|k|persona|"react-v1"` から作る（`thread_replies.go` の `threadUnit` と同じ流儀）。
  - `ThreadState` は、同じスレッドの既往の返信者（回数）、スレ主の ID、直前の返信者、親記事の著者が新人か、を持つ。

- [ ] **T012** `apps/server/internal/world/domain_lexicon.go`（新規）に、13 領域の 1996 年の語彙辞書と、照合関数を書く

  - 領域キーは、`randomResidentInterests` の 13 個と、完全に同じ。
  - 現代語（SNS 用語、`草`、`ググる` など）を入れない（`AGENTS.md` の In-world writing）。
  - 先頭に「架空の再構成。網羅を目指さない。Fictional reconstruction」とコメントする。

- [ ] **T013** `apps/server/internal/world/reaction_test.go`（新規）に、統計テストと性質テストを書く

  固定シード、各 5,000 回以上で、次を確認する（しきい値は幅を持たせる）:
  1. 決定論: 同じ入力 → 同じ計画。
  2. `ReplyTendency` が高い住民ほど、選ばれる確率が高い。lurker / dormant はほとんど選ばれない。
  3. 関連領域に強い関心を持つ住民が、そうでない住民より有意に多く選ばれる。無関係な話題にも、最小確率（`ε`）で選ばれる。
  4. 2 件目以降で `Argumentativeness` が高い住民が選ばれやすい。
  5. 新人の投稿で `NewcomerOpenness` が高い住民が選ばれやすい。
  6. スレ主の戻り返信確率が、非スレ主より高い。
  7. 親記事の著者本人と、直前の返信者は、選ばれない。
  8. 同じ住民は、同じスレッドで 3 回以上続けて選ばれにくい（`0.35^n`）。
  9. 本数 0（`ThreadReplyCount` が 0）のとき、計画の返信は 0 件。
  10. 予定時刻が、親記事より後になる。

  ```bash
  go -C apps/server test ./internal/world -run 'Reaction|DomainLexicon' -count=1
  go -C apps/server test ./internal/world -count=1
  ```

**Checkpoint**: 純関数と統計テストが通り、`world` パッケージの既存テスト（人物生成の金型を含む）が、変更なしで通る。

---

## Phase 2: 保存

- [ ] **T020** `world/store.go` に `ReactionPlanStore` のインターフェースと、メモリ実装を足す

  `BoardActivityStateStore`（`world/store.go` の同名）と同じ形にする: 保存、取得（対象記事 ID）、ホストの一覧、実体化済みの印の更新。

- [ ] **T021** `worldpersist` に、同じ保存を足す

  既存のスナップショットとの互換を確認する（空のスナップショットを読み込んでも失敗しない）。互換のためのテストを足す。

- [ ] **T022** `world/models.go` に `PostIntent.Domains []string`（`json:"domains,omitempty"`）を足す

  既存の JSON のシリアライズ結果が、`Domains` が空のとき変わらないことを、テストで確認する。

**Checkpoint**: 保存と読み込みが往復して一致する。既存のスナップショットが読み込める。

---

## Phase 3: 局の定義（YAML）

- [ ] **T030** `hostcatalog` に、板の任意項目 `domains`（領域 → 重み）と `welcomes_newcomers` を足す

  - 未知の領域キー、負の重みは、検証エラーにする（エラーメッセージに、板の path を含める）。
  - 省略した場合の既定: 全領域が同じ重み、`welcomes_newcomers: false`。
  - 既存のプリセットのロードと、`hostcatalog/README.md` の記述を更新する。

- [ ] **T031** 評価用のプリセットの 1 つ（`hakata-canal-net.yaml`）の数枚の板に、`domains` を足す（任意）

  足す場合は、その板の `scope` の文面に沿った領域だけにする。プリセットの他の項目は変えない。
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

- [ ] **T052** NPC の記事の領域タグを、Situation の提案に足す（`llm` の既存の構造化出力に `domains` を足す）

  - 追加の LLM 呼び出しは行わない。既存の 1 回の出力のスキーマに、13 領域から最大 3 個を選ぶフィールドを足す。
  - 検証: 13 領域の外の値は捨てる。出力が空でも、生成を失敗させない（板の事前分布に落ちる）。
  - 既存の LLM 関連のテスト（`llm/*_test.go`）が、通ること。

**Checkpoint**: NPC の記事のレスが、同じ基準で選ばれる。`go test ./internal/...` が通る。

---

## Phase 6: 文書と完了

- [ ] **T060** `docs/WORLD_SIMULATION.md` の Jev の節を直す

  「この経路では使わない。接続されていない任意の助言機構」と明記し、反応の決定が統計・確率モデルだけで行われることを書く。

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
`world/reaction.go`、`world/reaction_test.go`、`world/domain_lexicon.go`、保存の実装（`world/store.go`、`worldpersist`）、
`hostcatalog` の変更、`worldrepo/repository.go` と観測の変更、`bbsengine/engine.go` の変更、`docs/` の 2 ファイル、
`specs/008-persona-reactions/verification.md`。金型ファイルは、差分に含まれない。
