# Tasks: 題名の書き方（書き手の声）の改善と、題名の形の診断

**Input**: `specs/009-title-voice/` の `spec.md`、`plan.md`、`baseline/root-titles-2026-10-02_06.jsonl`  
**Prerequisites**: `AGENTS.md`、`docs/LLM_POLICY.md`（特に「Prompt construction」「Period-native conversational economy」）、`plan.md` の 2 章

## この作業のルール（必読）

- 作業ブランチから `main` への PR を 1 本作る（draft）。`main` に直接コミットしない。
- **成果物は Go のコードの変更（実装）、開発用ツール、`docs/LLM_POLICY.md` の追記、`verification.md`。** `spec.md`、`plan.md`、`tasks.md`、
  `baseline/` のファイルを作る・変更することは、この作業の仕事ではない。
- **ベースライン（`baseline/*.jsonl`）は、変更前のコードが本番で生成した証拠である。書き換えない。** 実装後のコードから作り直さない。
  診断の指標は、このファイルに対して**範囲で**検証する（実装後の出力から期待値を作らない）。
- **確定した事実を変えない。** 出来事（Situation）の要約・事実・`Intent`・本文の生成に、変更を加えない。変えるのは、題名の文字列と、
  その生成のプロンプト・入力だけである。
- **題名の型にノルマ、割合、テンプレートを作らない。** 世界側の処理は、素材の用意、決定論的な検証、複数案からの決定論的な選択、観測だけ。
  診断の指標を、生成の拒否条件にしない。
- **禁止事項を並べて足さない。** `docs/LLM_POLICY.md` の「目的・素材・出力形式」に従い、プロンプトに足す指示は、plan 2.1 の範囲にとどめる。
- 題名の例示に、ゲーム・地域の題名と、ベースラインの題名を使わない。現代語を入れない（`AGENTS.md` の In-world writing）。
- スコープ外（`spec.md` 末尾）に手を出さない。特に `openai_world_situation_proposer.go`、本文生成、`bbs_title_candidates.go` は変更しない。
- ホストの ID・電話番号・役割で分岐しない。評価用の振る舞いは、既存のホストの明示的なフラグ（`debug.content_log` など）で切り替える。
- 既存ファイルに整形だけの差分を作らない。`gofmt -w` を既存ファイル全体にかけない（新規ファイルには、かけてよい）。
- 1 タスク = 1 コミット（メッセージは `T0XX: 内容`）。途中でビルドが通らないタスクは、1 つにまとめる。
- 実行していないコマンドを「成功した」と書かない。実 LLM を使うもの（リプレイ）は、キーがなければ「未実施」と理由を書く。
- `.github/` 配下は変更しない。
- spec と実際のコードが食い違っていたら、推測で進めず、PR の説明に書いて止まる。同じテストの失敗が、2 回の修正で直らなければ止まる。
- この tasks のコードは、コンパイルして確認していない。実際のコードに合わせて直し、直した点を PR に書く。

---

## Phase 0: 前提の確認（コードは書かない）

- [ ] **T000** 前提を確認し、結果を `verification.md` に書く

  ```bash
  wc -l specs/009-title-voice/baseline/root-titles-2026-10-02_06.jsonl     # 179 のはず
  go -C apps/server test ./internal/... -count=1
  grep -n "recentSubjects\|productionTitleChunkSize" apps/server/internal/worldrepo/bbs_situation_first_planner.go
  grep -n "func.*responseTextWithJSONSchema" -r apps/server/internal/llm
  ```

  - ベースラインが 179 件であること。違えば止まる。
  - 既存の全テストが、変更前に通ること。通らないものは、名前を記録して続行する。
  - plan 末尾の「実装時に確認すること」の 4 点を調べ、結果を書く。

**Checkpoint**: 前提が確認され、既存の失敗が記録されている。

---

## Phase 1: 診断（純関数と開発用ツール）

- [ ] **T010** `apps/server/internal/titleshape/`（新規パッケージ）に、診断の純関数を書く（plan 2.4）

  ```go
  package titleshape

  func Normalize(s string) string                       // NFKC、空白・中点・記号の除去、小文字化
  func Referents(summary string) []string               // 『…』の語。なければ先頭のカタカナ・英数字の連続（2 文字以上）
  type Lead string // "referent" | "place" | "handle" | "other"
  func ClassifyLead(title string, referents, places []string) Lead
  func SuffixKey(title string) string                   // 終端記号を除いた末尾 4 文字
  func IsDuplicate(a, b string) bool                    // Normalize の完全一致
  func Similarity(a, b string) float64                  // 文字 2-gram の Jaccard
  type Report struct { /* 先頭の型の率、末尾の反復（最大シェア、上位）、重複、長さ、保持率 */ }
  func Measure(items []Item, places []string) Report    // Item: Subject と Summary
  ```

  - 外部パッケージへの依存を足さない（`golang.org/x/text` は、`go.mod` に既にあるときだけ使う。なければ、標準ライブラリで正規化する）。
  - LLM、ネットワーク、時刻、乱数を使わない。

- [ ] **T011** `titleshape` のテストを書く

  1. 小さな手書きの例で、`Normalize`、`Referents`、`ClassifyLead`、`SuffixKey`、`IsDuplicate`、`Similarity` の性質を確認する。
  2. **ベースライン**を読み、範囲で確認する（実装後の出力を金型にしない）:
     - `20/1` で、`referent` 先頭の率が、0.75 以上 0.95 以下。
     - `10/1` で、`place` 先頭の率が、`--places 天神,地下街` のとき 0.4 以上 0.8 以下。
     - `3` に、完全一致の重複が 1 組ある。
     - `20/1` の対象名の保持率が、0.9 以上。
  3. 範囲がこの目視の概数と合わないときは、**実装を直す前に**、ベースラインを数え直し、`verification.md` に差を書く。

  ```bash
  go -C apps/server test ./internal/titleshape -count=1
  ```

- [ ] **T012** `apps/server/cmd/titlestats/main.go`（新規）を書く

  - 入力: JSONL（ベースラインと同じ形式。`board`、`subject`、`situation_summary`）。標準入力またはファイル引数。
  - 出力: 板ごとに、`Report` の指標を、読みやすい表で。`--json` で JSON。`--places` で地名の語。
  - 例: `go -C apps/server run ./cmd/titlestats -- ../../specs/009-title-voice/baseline/root-titles-2026-10-02_06.jsonl`

- [ ] **T013** ベースラインの指標を、ツールで出し、`verification.md` に記録する

  この値が、SC-001〜SC-005 の比較の基準になる。目視の概数と大きく違う場合は、差を書く。

**Checkpoint**: `go test ./internal/titleshape` が通り、ベースラインの指標が記録されている。ここまでは、本番の動作を変えていない。

---

## Phase 2: プロンプトの構成（FR-001〜FR-003）

- [ ] **T020** `llm/provider.go` の `BBSSituationTitleSeed` に、後方互換の変更を入れる

  `situation_summary` の JSON の名前を `background` に出力する別の型（または `MarshalJSON`）を用意する。既存のフィールドは残す。
  既存のテストが、変更なしで通ること。

- [ ] **T021** `llm/situation_titles.go` のプロンプトを、目的・素材・出力形式に組み直す（plan 2.1）

  - 目的: 「この人物が、自分の投稿として、板の一覧に付ける題名」（FR-001）。
  - 素材: `board`、`world_date`、局・地域、`author`（ハンドル、`persona_profile`）、`background`（要約と事実）、
    `already_covered`、`form_notes`。`background` は「書き手が何をしたかを知る背景。語順や言い回しの手本ではない」と 1 回だけ書く（FR-002）。
  - 出力形式: 36 文字以内、1 行、`Re:` なし。検証は、既存のままにする。
  - 禁止事項を足さない。現在の指示にある条件（36 文字、1 行、Re: なし）は残す。

- [ ] **T022** 題名の型の例のデータを足す（plan 2.2）

  - データ表（Go のデータ、または YAML）に、8〜12 個。ゲーム・地域・ベースラインの題名を含めない。1996 年前後の日常（園芸、料理、鉄道、読書など）。
  - 呼び出しごとに、`hash(host|board|batch)` で 3 個を選ぶ純関数。

- [ ] **T023** プロンプトのテストを書く（偽のプロバイダが受け取る文字列を検査）

  1. 素材（`author`、`background`、`already_covered`）が入っている。
  2. `background` の位置づけの文が、ちょうど 1 回だけある。
  3. **ベースラインの題名が、プロンプトに 1 つも含まれない**（全 179 件を検査）。
  4. 例が 3 個で、呼び出しごとに選択が変わりうる（シードを変えると別の組が出る）。同じシードなら同じ。
  5. 禁止の語（「〜しないこと」「〜禁止」）が、新しく足した部分に増えていない（旧プロンプトの条件を除く）。
  6. 検証（36 文字超過など）が、従来どおり失敗する。

  ```bash
  go -C apps/server test ./internal/llm -run 'SituationTitle' -count=1
  go -C apps/server test ./internal/... -count=1
  ```

**Checkpoint**: プロンプトの組み立てが、決定論的に検証されている。既存のテストが通る。

---

## Phase 3: 直近題名・重複・診断のログ（FR-004、FR-006、FR-007）

- [ ] **T030** `worldrepo/bbs_situation_first_planner.go` で、`already_covered` の渡し方を変える（plan 2.3）

  - `recentSubjects` を、シード付きで入れ替え、最大 20 件にして渡す（シードは、`host|board|チャンク番号`）。
  - `titleshape` で、直近の形を測り、**しきい値を超えたときだけ**、事実の文（`recent_form_facts`）を作る。ノルマ・禁止の語を含めない。
    しきい値は、定数にまとめ、「調整値」とコメントする。
  - `BBSSituationTitleRequest` に、後方互換のフィールドを足す（`RecentFormFacts []string`）。

- [ ] **T031** 重複の検査と、1 回の再生成を足す（plan 2.6）

  - チャンクの題名を受け取った直後に、`titleshape.IsDuplicate` で、`already_covered` と、そのバッチの題名に対して調べる。
  - 重複した記事だけを、1 回再生成する（`already_covered` に重複した題名を足す）。それでも重複したら採用し、診断に記録する。失敗にしない。

- [ ] **T032** 診断のログを足す（plan 2.4）

  - バッチの終わりに、`BBS title shape: {JSON}` を 1 行出力する。条件は、既存の `shouldLogGeneratedContent(host)` と同じ。
  - 人間の投稿・オプトインしていないホストの内容は出さない（既存の注意を守る）。

- [ ] **T033** テストを書く（偽の題名プロバイダを使う）

  1. 偽のプロバイダが、同じ題名を 2 件返す → 重複した記事だけが再生成される。再生成でも重複 → 採用され、失敗しない。
  2. 偏りの大きい直近題名 → `recent_form_facts` が入る。偏りが小さい → 入らない。事実の文に、禁止・ノルマの語がない。
  3. `already_covered` の順序が、シードで決定論的。件数が 20 を超えない。
  4. 診断のログが、オプトインしていないホストでは出ない。
  5. 既存の `worldrepo` のテストが、変更なしで通る。

  ```bash
  go -C apps/server test ./internal/worldrepo -count=1
  go -C apps/server test ./internal/... -count=1
  ```

**Checkpoint**: 重複と直近題名の扱いが、LLM なしで検証されている。

---

## Phase 4: 複数案と決定論的な選択（FR-005。既定は無効）

- [ ] **T040** 設定と、スキーマを足す（plan 2.5、2.7）

  - 既存の `config` の流儀で、環境変数 1 つ（既定は無効）。無効のとき、挙動もプロンプトも、Phase 3 までと**同一**であること（テストで確認）。
  - 有効のとき、出力スキーマを `titles: {event_id: [string, …（最大 3）]}` にする。スキーマが文字列と配列の併用を許さない場合は、2 つのスキーマを持つ。

- [ ] **T041** `titleshape.PickVariant` を書く（plan 2.5）

  ```go
  // 各案の「形の距離」の最小値が最大のものを選ぶ。同点は seed のハッシュ。
  func PickVariant(seed string, variants []string, covered []string) int
  ```

  - 距離 = `1 − (0.4·先頭3文字の一致 + 0.4·末尾4文字の一致 + 0.2·Similarity)`。重みは定数にまとめ、「調整値」とコメントする。
  - 36 文字超過などの、検証を通らない案を除く処理は、呼び出し側で行う。全案が通らなければ、従来のエラー。

- [ ] **T042** テストを書く

  1. 同じ入力 → 同じ選択（決定論）。シードを変えると、同点のとき選択が変わりうる。
  2. 直近に似た案がある → 似ていない案が選ばれる。
  3. 1 案だけでも動く。案が 3 を超える応答は、先頭の 3 案だけを使う。
  4. 無効のとき、LLM の呼び出し回数とプロンプトが、有効にする前と同一。

  ```bash
  go -C apps/server test ./internal/titleshape ./internal/llm ./internal/worldrepo -count=1
  ```

**Checkpoint**: 複数案が、既定の無効のまま入り、有効にしても決定論的に動く。

---

## Phase 5: リプレイ、文書、完了

- [ ] **T050** `apps/server/cmd/titlereplay/main.go`（新規）を書く

  - ベースラインの出来事（`situation_summary`、`situation_facts`、板）を入力に、`BBSSituationTitleRequest` を組み、題名生成を `--runs N`（既定 3）回実行する。
  - 20 件ずつチャンクで呼び、`already_covered` は、リプレイ内で積み上げた題名で代替する。`persona_profile` は空（ベースラインに無い）。
  - 出力は、ベースラインと同じ形式の JSONL（`run` を足す）。`titlestats` にそのまま渡せる。
  - 実プロバイダは、既存の設定（環境変数）から作る。キーがなければ、明確なエラーで終了する（`--fake` は、テスト用の偽の応答）。

- [ ] **T051** リプレイを実行し、`titlestats` で比較する（SC-001〜SC-007）

  ```bash
  go -C apps/server run ./cmd/titlereplay -- --runs 3 ../../specs/009-title-voice/baseline/root-titles-2026-10-02_06.jsonl > /tmp/replay.jsonl
  go -C apps/server run ./cmd/titlestats -- /tmp/replay.jsonl
  ```

  - 実 LLM のキーがなく実行できなければ、「未実施」と理由を、`verification.md` に書く。**偽の応答で成功とみなさない。**
  - 実行できたら、ベースラインとの比較表（板ごと）を `verification.md` に載せる。SC を満たさない項目は、そのまま書く。
  - 複数案（無効・有効）、`recent_form_facts`（あり・なし）の比較も、可能なら載せる。

- [ ] **T052** `docs/LLM_POLICY.md` に、節を足す（FR-009）

  見出し: 「Reference without limiting」。内容は、次の趣旨（文言は、既存の文書の文体に合わせて整える）:

  - 過去の事実、時代の傾向、既存の投稿は、LLM が使う素材であり、書き方・話題の上限ではない。
  - 多様性は、禁止・ノルマ・テンプレートではなく、素材の質（書き手の声、文脈、観測された偏りの事実）と、
    決定論的な選択（複数案からの選択）で得る。
  - 世界側は、素材の用意、検証、選択、観測を行う。診断の指標は、生成の拒否条件にしない。
  - 題名では、対象名は識別に必要なとき入れ、位置は自由（既存の「Period-native conversational economy」と整合）。

  `AGENTS.md` は変更しない。AGENTS.md の「Non-negotiable design rules」へ、1 行足す提案文を、PR の説明に書く。

- [ ] **T053** `specs/009-title-voice/verification.md`（新規）を書く

  - 実行したコマンドと結果（未実施は理由つき）。
  - ベースラインの指標（T013）と、リプレイの比較（T051）。
  - ベースラインの指標が、目視の概数と違った点。
  - リプレイと本番の入力の違い（persona なし、直近題名の代替）。
  - 複数案・形の事実の有効・無効の結果と、有効にするかの推奨（実測にもとづく）。
  - コンパイルして確認していなかったコードを直した点。

- [ ] **T054** `go -C apps/server vet ./...` と `go -C apps/server test ./... -count=1` を実行し、結果を記録する

- [ ] **T055** draft の PR を開く

  PR の説明に書く: 実行したコマンドと結果、ベースラインを変更していないこと、未実施のもの（実 LLM のリプレイなど）、
  AGENTS.md への提案文（T052）、フォローアップ（plan 6 章: Situation 提案側、Lab 用プロンプトの例示、返信への展開）。
  ワークフロー（`go-test`）が自動で動かない場合は、そう書く。

**完了の定義**: PR の差分に、次が含まれる。
`internal/titleshape/`（とテスト）、`cmd/titlestats`、`cmd/titlereplay`、`llm/situation_titles.go`、`llm/provider.go`、
`worldrepo/bbs_situation_first_planner.go`、`config` の変更、題名の型の例のデータ、`docs/LLM_POLICY.md`、
`specs/009-title-voice/verification.md`。差分に含まれない: `baseline/` の変更、`openai_world_situation_proposer.go`、`bbs_title_candidates.go`、`AGENTS.md`。
