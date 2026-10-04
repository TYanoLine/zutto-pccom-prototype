# Spec: 住民の定義を preset 局のデータに外部化する

**Spec ID**: 004-preset-population
**Status**: Draft
**Depends on**: PR #294（局ごとのフラグ）、spec 003（名前の中立化）。いずれもマージ済み

## 背景

局の「住民」（会員のペルソナ骨格）の定義と生成が、HAKATA 専用のコードとして
`apps/server/internal/world/hakata_cast.go` に固定されている。

- 局のデータ（コア住民 15 名のハンドル、ハンドルの素材 43 語と接頭辞 5 語、乱数のシード、
  ID の接頭辞 `hakata`）が Go のソースに直接書かれている。
- 生成を呼ぶ条件が、局の `role: experiment` になっている（`world/store.go`）。
  別の局に住民を持たせるには、コードを書き換えるしかない。
- 「実験ホストは 1 つまで」という制約（`checkSingleExperiment`）が、固定の ID（`hakata-member-001` など）の
  衝突を避けるために存在する。

局の定義は preset（YAML）に置き、共通コードは「定義を受け取って住民を生成する」ことだけを担当する形にする。
**HAKATA の住民は、移行の前後で完全に同一でなければならない**（永続化済みのスナップショットが、
ペルソナ ID と membership で住民を参照しているため）。

## ユーザーストーリー

### US1: 住民の定義を preset に書けば、その局に住民が生成される（Priority: P1）

運営者として、局の preset に `population:` を書くだけで、その局に決定的な住民が生成されるようにしたい。
局の key、電話番号、`role` をコードに書かずに、局の追加に追従してほしい。

**Independent Test**: テスト用の preset（または store 上の仕様）に `population:` を与えた局に住民が生成され、
`population:` を持たない局（`role: experiment` の局を含む）には生成されない。

### US2: HAKATA の住民は移行前後で同一（Priority: P1）

運営者として、この変更で HAKATA の住民（ID、ハンドル、活動傾向、興味、membership の順序）が
1 つも変わらないことを、自動テストで確認したい。

**Independent Test**: 変更前のコードで記録した「金型（golden）」と、変更後の生成結果が、4 つの経路で一致する。

## 要件

- **FR-001（スキーマ）**: preset に `population:` ブロックを追加できる（`plan.md` のスキーマ）。
  省略した局には住民を生成しない。
- **FR-002（生成の条件）**: 住民を生成するかは、その局の preset に `population:` があるかだけで決まる。
  `role`、局の key、電話番号、ID で決めない。
- **FR-003（データとアルゴリズムの分離）**: 局のデータ（シード、ID 接頭辞、コア住民、ハンドルの素材）は YAML に置く。
  生成のアルゴリズム（活動傾向の分布、興味ドメインの表、ハンドルの組み立て方）は共通の Go コードに置く。
  どの局の名前もコードに現れない。
- **FR-004（完全な同一性）**: HAKATA の住民は、`plan.md` の 4 つの経路すべてで、
  全ペルソナの全フィールドと membership の順序が、変更前と一致する。乱数の消費順序を変えない。
- **FR-005（ID の互換）**: ペルソナ ID の形式を変えない。コア住民は `<id_prefix>-<ハンドルの slug>`、
  それ以外は `<id_prefix>-member-NNN`（NNN は 1 始まりの通し番号 3 桁）。
- **FR-006（人数）**: 生成する人数は `host.members`。コード上の既定値 326 は廃止する
  （`members` が 0 以下なら住民を生成しない）。
- **FR-007（API 名）**: `MemoryStore.EnsureHakataExperimentPopulation` を `EnsurePopulation` に改名する。
  スナップショット復元後に住民を補完する呼び出し（`cmd/server/runtime_store.go`）は、
  `role` ではなく、スナップショット対象のすべての局に対して行う（`population:` が無ければ 0 を返すだけ）。
- **FR-008（制約の置き換え）**: 「実験ホストは 1 つまで」の制約を廃止し、
  「`population.id_prefix` は preset 間で一意」に置き換える。違反は起動時のエラーにする。
- **FR-009（検証）**: preset の読み込み時に `population:` を検証し、すべての問題を一度に報告する
  （`plan.md` の検証規則）。
- **FR-010（既存の挙動の維持）**: 既存のペルソナの `Interests` を、起動時に再生成する現在の挙動
  （`TestEnsure…RefreshesOldBiasedRoutingInterests` が検証しているもの）を、そのまま維持する。
  この挙動の見直しはこの spec の範囲に含めない。
- **FR-011（ドキュメント）**: `hostcatalog/README.md` に `population:` の書式と検証規則を記載する。
  「`role: experiment` は住民生成を意味する」という記述を除く。

## 成功基準

- **SC-001**: 金型テスト（`TestPopulationMatchesGolden`）が、変更前に生成した金型ファイルを**変更せずに**通る。
- **SC-002**: `go -C apps/server test ./...` と `go -C apps/server vet ./...` が通る。
- **SC-003**: `apps/server/internal/world/` のテスト以外のファイルに、`hakata`（大文字小文字を区別しない）が
  残っていない。残ってよいのは、コメントでの key の例示だけ（`plan.md` に列挙）。
- **SC-004**: `population:` を持たない局に、`role: experiment` を付けても住民が生成されないテストがある。
  `role` を持たない局に `population:` を与えると住民が生成されるテストがある。

## スコープ外（変更しない）

- 活動傾向の分布（regular 10% など）と、興味ドメインの表（`local` 72% など）の外部化。
  局ごとの調整が必要になった時点で、`population:` に任意項目として追加する（別 spec）。
  追加するときは、この spec で作る金型テストが回帰を検出する。
- 既存ペルソナの `Interests` 再生成（FR-010 の挙動）の廃止。永続化済みの状態に依存して内容が変わるため、
  別途、影響を確認してから判断する。
- Erika-K の板構成と Welcome 文言（`hostprogram/erikak/runtime.go`）、SYSOP ペルソナ。
- `role` の廃止。`role: experiment` は、ラベルとして残す。
- 局のフラグ（`debug.*`、`generation.*`）、Web 側、センターディレクトリ。
- 永続化（スナップショット）の形式。ペルソナ ID が変わらないので、スナップショットは変更しない。
