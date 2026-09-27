# Tasks: BBS本文の人物別具体化

**Input**: `specs/001-body-concretization/` の設計資料

**Prerequisites**: `plan.md`、`spec.md`、`research.md`、`data-model.md`、`contracts/article-body-read.md`、`quickstart.md`

**Tests**: 仕様が状態遷移、時系列、永続化、ホスト表示の回帰確認を要求しているため、自動テストと人手評価を含める。

**Organization**: 共有基盤を先に整え、その後は各ユーザーストーリーを独立して実装・検証できる単位に分ける。

## Format: `[ID] [P?] [Story] Description`

- **[P]**: 依存タスク完了後、別ファイルで競合せず並行実行できる
- **[Story]**: 対応するユーザーストーリー（`US1`、`US2`、`US3`）
- すべてのタスクに具体的な対象ファイルを記載する

---

## Phase 1: Setup（共有テスト基盤）

**Purpose**: 共有 Article Detail 処理の成功・失敗を、外部 LLM なしで再現できるテスト基盤を用意する。

- [X] T001 Article Detail の0〜2件応答、生成失敗、検証失敗、保存失敗、renderer 呼び出し有無を観測できる fake planner・fake updater・fake renderer を `apps/server/internal/worldrepo/materialization_article_detail_test.go` に作成する

---

## Phase 2: Foundational（全ストーリーのブロッカー）

**Purpose**: 呼び出し元に依存しない記事単位の状態と共有エンジン境界を作る。

**⚠️ CRITICAL**: このフェーズが完了するまでユーザーストーリー実装を開始しない。

- [X] T002 `PostIntent` に欠落時 `false` となる `ArticleDetailsMaterialized` を追加し、記事ローカル詳細とは独立した正本状態として `apps/server/internal/world/models.go` に定義する
- [X] T003 Article Detail planner を renderer の具象型判定から分離して `Repository` の明示的な能力として保持し、通常 runtime と fresh Lab の双方へ同じ planner を配線する変更を `apps/server/internal/worldrepo/repository.go`、`apps/server/cmd/server/main.go`、`apps/server/cmd/server/materialization_lab_fresh.go` に実装する
- [X] T004 title-first／開発フラグ固有の分岐を共有 Article Detail オーケストレーションへ一般化し、通常閲覧・直接確認・Lab がすべて同じ処理を通るよう `apps/server/internal/worldrepo/materialization_interactive_detail.go` と `apps/server/internal/worldrepo/materialization_article_debug.go` を整理する

**Checkpoint**: 本文未確定の記事は入口や title-first 設定に関係なく、同じ planner 能力と記事単位状態を使える。

---

## Phase 3: User Story 1 - 抽象的な件名から具体的な本文を読む（Priority: P1）🎯 MVP

**Goal**: 選択済みの件名・出来事を変えず、根拠の範囲内で0〜2件の局所詳細を先に確定し、同じ本文を再閲覧できるようにする。

**Independent Test**: 抽象的な件名の記事を初回・再度閲覧し、件名が不変で、0〜2件の検証済み詳細と本文が保存され、正常0件では再提案されないことを確認する。

### Tests for User Story 1

- [X] T005 [US1] 0件・1件・2件の正常応答、件名不変、詳細と完了状態の同時保存、再閲覧時の planner 非再実行を `apps/server/internal/worldrepo/materialization_article_detail_test.go` に先行テストとして追加する
- [X] T006 [P] [US1] planner 不在、2回の生成失敗、検証失敗、`UpdatePost` 失敗で本文 renderer が呼ばれず、完了状態が `false` のままになるテストを `apps/server/internal/worldrepo/materialization_failure_test.go` に追加する
- [X] T007 [P] [US1] Detail の0〜2件制限、件名の言い換えだけの詳細、編集指示・内部メタデータ・重複の拒否を `apps/server/internal/llm/bbs_title_article_details_test.go` に追加する

### Implementation for User Story 1

- [X] T008 [US1] 投稿時刻以前の人物事実・人物プロファイル・選択済み記事意図から Detail 要求を作り、正常0件を含む検証済み結果と `ArticleDetailsMaterialized=true` を一度の更新で保存する処理を `apps/server/internal/worldrepo/materialization_interactive_detail.go` に実装する
- [X] T009 [US1] Detail 保存済み記事だけを本文 renderer へ渡し、Detail 失敗時は本文を生成・保存せず、本文失敗時は保存済み Detail を再提案しない制御を `apps/server/internal/worldrepo/materialization_article_debug.go` に実装する
- [X] T010 [US1] 通常閲覧・開発用直接確認・fresh Lab で共有処理の完了条件と失敗条件が一致する統合テストを `apps/server/cmd/server/materialization_lab_fresh_test.go` に追加する

**Checkpoint**: US1 を単独で実行し、抽象件名の記事の初回閲覧、正常0件、再閲覧、Detail 障害を検証できる。

---

## Phase 4: User Story 2 - 返信者ごとの異なる貢献を読む（Priority: P1）

**Goal**: 返信者本人の根拠とスレッド文脈を分離して渡し、既出内容の反復や他人の経験の盗用を避けつつ、ホスト固有のアペンド表示を維持する。

**Independent Test**: 複数返信スレッドを時系列に読み、返信要求に先行記事と返信者本人の履歴が重複なく含まれ、別人の経験が本人の事実として扱われず、Erika-K の返信に独立件名が追加されないことを確認する。

### Tests for User Story 2

- [X] T011 [P] [US2] 返信時の親記事・先行返信、返信者本人の最大6件の履歴、同一スレッド除外、未具体化履歴の意味情報利用を `apps/server/internal/worldrepo/materialization_worker_context_test.go` に先行テストとして追加する
- [X] T012 [P] [US2] 返信 Detail 要求でスレッド文脈と返信者本人の履歴を分け、他人の発言が本人の履歴へ混ざらないことを `apps/server/internal/worldrepo/materialization_interactive_latency_test.go` で検証する
- [X] T013 [P] [US2] Detail または本文の最終失敗時の `(アペンドの読み込みに失敗しました)` と、成功時に独立件名を表示しない回帰テストを `apps/server/internal/hostprogram/erikak/runtime_test.go` に追加する

### Implementation for User Story 2

- [X] T014 [US2] 返信先・先行返信と返信者本人の履歴を重複しない別領域として Detail 要求へ渡し、本文未生成の履歴を連鎖生成しない文脈構築を `apps/server/internal/worldrepo/materialization_worker_context.go` と `apps/server/internal/worldrepo/materialization_interactive_detail.go` に実装する
- [X] T015 [US2] 共有エンジンの成功・失敗を Erika-K の既存アペンド意味論へ接続し、返信の件名・親子関係・失敗文言を共通層へ移さず `apps/server/internal/hostprogram/erikak/runtime.go` に維持する

**Checkpoint**: US2 を複数返信スレッドだけで検証でき、US1 の単記事閲覧を変更せずに返信者固有の差分とホスト表示を確認できる。

---

## Phase 5: User Story 3 - 人物の連続性と時系列を守って読む（Priority: P1）

**Goal**: 投稿時点の正本情報だけを使い、永続化・再起動・並行閲覧でも本文と Detail を一度だけ確定する。

**Independent Test**: 投稿時刻の前後に人物事実を配置し、旧形式の記事を含む世界を保存・復元して同じ記事を並行閲覧する。未来事実が使われず、正常0件も再提案されず、本文済み旧記事が再処理されないことを確認する。

### Tests for User Story 3

- [X] T016 [P] [US3] 投稿時刻以後の人物事実除外、現在の正本事実の優先、履歴最大6件、同順位の決定的順序、同一スレッド除外を `apps/server/internal/worldrepo/materialization_persona_facts_test.go` と `apps/server/internal/worldrepo/materialization_worker_context_test.go` に先行テストとして追加する
- [X] T017 [P] [US3] `ArticleDetailsMaterialized` の保存・復元、欠落した旧スナップショットの `false` 読み込み、本文済み旧記事の非再処理、正常0件の再起動後非再提案を `apps/server/internal/world/development_snapshot_test.go` に追加する
- [X] T018 [P] [US3] 同じ記事の並行閲覧で planner・本文生成が重複確定せず、全閲覧が同じ本文と Detail を得るテストを `apps/server/internal/worldrepo/observation_test.go` に追加する

### Implementation for User Story 3

- [X] T019 [US3] 投稿時刻を上限にした人物事実と同一ホスト・本人・過去のみの履歴選択を維持し、最大6件を関連性・板・参照対象・新しさ・記事番号で決定的に選ぶ処理を `apps/server/internal/worldrepo/materialization_worker_context.go` に実装する
- [X] T020 [US3] 旧記事を「本文なし・未完了なら初回処理、本文ありなら再処理なし」と判定し、正常0件の完了状態をスナップショット複製・保存・復元で保持する変更を `apps/server/internal/world/development_snapshot.go` と `apps/server/internal/worldrepo/materialization_article_debug.go` に実装する
- [X] T021 [US3] 既存の観察 singleflight と因果順序の内側で Detail 保存から本文保存までを直列化し、競合結果を正本へ上書きしない処理を `apps/server/internal/worldrepo/observation.go` に実装する

**Checkpoint**: US3 の時系列、永続化、旧データ互換、並行閲覧を単独で再現でき、すべての閲覧が同じ正本結果を返す。

---

## Phase 6: Polish & Cross-Cutting Concerns

**Purpose**: 共有エンジン化の文書反映、全回帰確認、利用可能な標本による品質評価を行う。

- [X] T022 [P] 共有エンジン、記事単位の完了状態、Lab は隔離入口であること、Detail 失敗時の中断規則を `docs/sdd/SDD_001_BODY_CONCRETIZATION.md`、`docs/ARCHITECTURE.md`、`docs/LLM_POLICY.md`、`docs/MATERIALIZATION_LAB.md` に反映する
- [X] T023 `specs/001-body-concretization/quickstart.md` のコマンドで `worldrepo`、`llm`、`world`、Erika-K、server Lab の対象テストを実行し、失敗があれば対象実装またはテストを修正して実測結果を `specs/001-body-concretization/verification.md` に記録する
- [X] T024 通常閲覧と同じ共有処理を使う fresh Lab から取得できた抽象件名記事と複数返信スレッドをレビューし、実数・不足理由・意味的反復・一人称事実の根拠・背景の話題化・未来漏洩・1996年境界を `specs/001-body-concretization/quality-sampling.md` に記録する（20件／20〜30本は標本が十分な場合の目安であり最低条件にしない）

---

## Dependencies & Execution Order

### Phase Dependencies

- **Phase 1（Setup）**: 依存なし
- **Phase 2（Foundational）**: Phase 1 に依存し、すべてのユーザーストーリーをブロックする
- **Phase 3（US1）**: Phase 2 完了後に開始できる。共有 Detail 状態遷移を完成させる MVP
- **Phase 4（US2）**: Phase 2 完了後に開始できる。共有処理を使うが、返信文脈とホスト表示は US1 と独立して検証する
- **Phase 5（US3）**: Phase 2 完了後に開始できる。永続化・時系列・並行性は US1/US2 と独立して先行テストを作成できる
- **Phase 6（Polish）**: 採用する全ユーザーストーリーと自動テスト完了後に実施する

### User Story Dependencies

```text
Phase 1 Setup
  └─ Phase 2 Foundational
       ├─ US1 単記事の具体化（MVP）
       ├─ US2 返信者ごとの具体化
       └─ US3 時系列・永続化・並行性
            └─ Phase 6 文書・全回帰・品質評価
```

- **US1**: Phase 2 のみが前提。正常0件と失敗を区別する最小の価値単位
- **US2**: Phase 2 のみが前提。US1 と同じ共有処理を使うが、返信文脈と Erika-K 表示だけで独立検証可能
- **US3**: Phase 2 のみが前提。スナップショット・時系列・singleflight のテストは他ストーリーと並行作成可能
- 統合時は状態遷移の中心を先に固めるため、推奨順序を **US1 → US2 → US3** とする

### Within Each User Story

1. 先行テストを追加し、対象ケースが未実装のため失敗することを確認する
2. 文脈・モデルなど入力側を実装する
3. 共有サービスの状態遷移を実装する
4. ホスト／Lab 境界を統合する
5. ストーリー単独のテストを通してから次へ進む

### Parallel Opportunities

- T006 と T007 は T005 のテスト helper が使える状態になれば別パッケージ／別ファイルで並行可能
- T011、T012、T013 は Phase 2 後に別ファイルで並行可能
- T016、T017、T018 は Phase 2 後に別の責務・ファイルで並行可能
- T022 は実装結果が確定した後、T023 のテスト実行準備と並行可能
- US2 と US3 は Phase 2 後に別担当で進められるが、共有ファイル `materialization_interactive_detail.go` の変更は順番に統合する

---

## Parallel Examples

### User Story 2

```text
Task T011: worldrepo の返信文脈テスト
Task T012: llm の Detail 契約テスト
Task T013: Erika-K の表示回帰テスト
```

### User Story 3

```text
Task T016: 人物事実・履歴の時系列テスト
Task T017: スナップショット互換テスト
Task T018: 並行閲覧テスト
```

---

## Implementation Strategy

### MVP First（User Story 1）

1. T001〜T004 で共有テスト基盤、記事単位状態、明示 planner、共通呼び出し経路を作る
2. T005〜T010 で正常0件、成功、失敗、再閲覧、入口間の同一規則を完成させる
3. US1 の独立テストを実行し、件名不変・詳細保存後の本文生成・失敗時の本文非生成を確認する
4. ここで止めても、抽象件名の記事を安全に具体化する最小機能として評価できる

### Incremental Delivery

1. **Foundation + US1**: 単記事の共通具体化と正しい完了状態
2. **+ US2**: 返信文脈とホスト固有アペンド表示
3. **+ US3**: 時系列、再起動、旧データ、並行閲覧の保証
4. **Polish**: 文書・全回帰・取得可能な標本での品質記録

### Quality Sampling Policy

- 評価対象は「別投稿」ではなく、取得できた複数返信スレッド内の各返信と抽象件名の記事
- 20件／20〜30本は標本が十分に存在する場合の目安で、タスク完了の最低件数ではない
- 標本が少ない場合は不足を生成で埋めたことにせず、実数、取得条件、制約を記録する
- Lab は別エンジンではなく、隔離データで同じ共有処理を再現する評価入口として使う

---

## Notes

- `[P]` は依存タスク完了後に異なるファイルで競合なく進められるタスクのみへ付与した
- Article Detail は世界行動、件名、返信先、恒久的な人物事実を新設しない
- 正常0件は成功として完了状態を保存し、planner・検証・保存の失敗は未完了のまま再試行可能にする
- `PostUpdater.UpdatePost` 成功前の提案を本文 renderer や閲覧者へ成功結果として渡さない
- 既存本文のある旧記事は再具体化せず、本文のない旧記事だけを初回閲覧で処理する
- 操作系と失敗表示は各 host program に残し、世界データと記事具体化規則だけを共有する
