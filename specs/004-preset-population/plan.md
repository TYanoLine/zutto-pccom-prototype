# Plan: preset population

**Spec**: `specs/004-preset-population/spec.md`

## 設計の要点

1. **局のデータは YAML、アルゴリズムは Go**。現在の `hakata_cast.go` を、次の 2 つに分ける。

   | 局のデータ（preset の `population:`） | 共通のアルゴリズム（`world/population.go`） |
   |---|---|
   | `seed`（乱数のシード） | メンバーごとの乱数の作り方 `seed + (i+1)*7919` |
   | `id_prefix`（ID の接頭辞） | ID の組み立て（`<prefix>-<slug>`、`<prefix>-member-NNN`） |
   | `core_handles`（コア住民 15 名） | 活動傾向の分布（regular 10% など） |
   | `handle_bases`（ハンドルの素材 43 語） | 興味ドメインの表（`local` 72% など） |
   | `handle_prefixes`（接頭辞 5 語） | ハンドルの 6 つの書式と、衝突時のフォールバック |

2. **`Host` には入れない**。`world.Host` は値として `==` で比較される（`preset_hosts_test.go` など）。
   スライスを持つ構造体を入れると比較できなくなる。住民の定義は、`MemoryStore` が
   ホスト ID をキーに別のマップ（`populations`）で持つ。`HostDescriptor`（局の骨格）にも入れない。
3. **`role` は関与しない**。住民を生成するかは、`population:` の有無だけで決まる。
4. **乱数の消費順序が結果を決める**。移行で最も壊れやすいのはここ。次節の契約を守り、金型テストで確認する。

## スキーマ

`presets/hakata-canal-net.yaml` に追加する値（現在の Go のソースと同一）。**すべての文字列を引用符で囲む**。
特に `"98"` は、引用符が無いと YAML が数値として扱う。

```yaml
population:
  seed: 199608260920
  id_prefix: "hakata"
  core_handles: ["MARI", "YUKI", "NORI", "KAZU", "TAKU", "NEKO", "KEN", "MAKO", "TOMO", "AKI", "RYO", "HIRO", "SACHI", "JUN", "MIDNIGHT"]
  handle_bases: ["AKI", "AYA", "EMI", "HIDE", "HIRO", "JUN", "KAZU", "KEN", "KOJI", "MAKO", "MARI", "MASA", "MIKI", "NAO", "NORI", "REI", "RYO", "SHIN", "TAKA", "TOMO", "YUKI", "YUJI", "SATOSHI", "TAKESHI", "KENTA", "MEG", "MAYU", "RINA", "ERI", "MINT", "WOLF", "RABBIT", "JOKER", "NOVA", "LUNA", "MARU", "KERO", "N88", "V30", "X68", "PC98", "COM", "MODEM"]
  handle_prefixes: ["N88", "V30", "X68", "98", "COM"]
```

- 配列の**順序**が結果を決める（乱数の添字で選ぶため）。並べ替えない。
- preset の `revision` は 2 から 3 に上げる。

## 検証規則

読み込み時（`ParsePreset`）に構造を、`Populations()` で `host.members` との整合を検査する。
すべての違反を一度に報告し、エラーには `population.<フィールド>` を含める。

| 規則 | 検査する場所 |
|---|---|
| `seed` は必須（0 も有効な値なので、「未指定」と区別できるように `*int64` で受ける） | `ParsePreset` |
| `id_prefix` は必須で、局の key と同じ形式（小文字の英数字をハイフンで連ねる） | `ParsePreset` |
| `core_handles` の各要素は、前後の空白を除いて空でない | `ParsePreset` |
| `core_handles` は、大文字小文字を区別せず重複しない | `ParsePreset` |
| `core_handles` の各要素の `HandleSlug` は空でなく、互いに重複しない（ID が衝突するため） | `ParsePreset` |
| `handle_bases`、`handle_prefixes` の各要素は、前後の空白を除いて空でない | `ParsePreset` |
| `len(core_handles)` は `host.members` 以下 | `Populations` |
| `host.members` が `len(core_handles)` より大きいとき、`handle_bases` は 1 つ以上 | `Populations` |
| `id_prefix` は、preset 間で一意 | `LoadPresetsFS`（key・電話番号の重複検査と同じ場所） |

## 生成アルゴリズムの契約（乱数の消費順序）

現在の `ensureHakataExperimentPopulationLocked` と同じ結果を出すための、守るべき点。
**これらを変えると、金型テストが失敗する**。

1. メンバー `i`（0 始まり）ごとに、**新しい**乱数 `rand.New(rand.NewSource(seed + int64(i+1)*7919))` を作る。
2. ID: `i < len(core_handles)` なら `id_prefix + "-" + HandleSlug(core_handles[i])`、
   それ以外は `fmt.Sprintf("%s-member-%03d", id_prefix, i+1)`。
3. コア住民のハンドルは乱数を使わない（`core_handles[i]` をそのまま使う）。
   それ以外は `nextResidentHandle` が乱数から決める。
4. `nextResidentHandle` の各試行（最大 80 回）で、次の 6 つの候補を**この順序で、毎回すべて評価する**
   （使わない候補の分も乱数を消費する）。

   | 順 | 候補 | 乱数の使い方 |
   |---|---|---|
   | 0 | `base` | `base` は `handle_bases[rng.Intn(len(handle_bases))]`（試行の最初に 1 回） |
   | 1 | `base.X` | `'A'+rune(rng.Intn(26))` |
   | 2 | `base-X` | `'A'+rune(rng.Intn(26))` |
   | 3 | `baseNN` | `1+rng.Intn(99)`（`%02d`） |
   | 4 | `base_NN` | `1+rng.Intn(99)`（`%02d`） |
   | 5 | `PREFIX-base` | `handle_prefixes[rng.Intn(len(handle_prefixes))]` |

   最初に未使用の候補を返す。`handle_prefixes` が空のときだけ、候補 5 を作らない（乱数も消費しない）。
   80 回で見つからなければ、`USER.<A-Z><NNN>` のフォールバック（現在のコードのまま）。
5. 「使用済みのハンドル」の集合は、`membership` の既存ペルソナのハンドル（小文字）で初期化し、
   生成のたびに追加する。**既存のペルソナがあっても、そのメンバーのハンドルを乱数から計算して集合に加える**
   （計算結果は、既存ペルソナには使われないが、後続の乱数の消費と集合の内容に影響する）。
6. ペルソナが無ければ、`newPersonaSkeleton`（活動傾向 → 興味の順に乱数を消費）で作る。
7. ペルソナが既にあれば（FR-010）:
   - `Handle` が空なら補う。
   - `ActivityPattern` が空なら、活動傾向を埋める（乱数を消費）。
   - **そのあと必ず `Interests` を再生成する**（既存の値は捨てる）。
8. membership に無ければ末尾に追加し、追加した数を返す。

## 金型テスト（4 つの経路）

変更前のコードで記録し、変更後に同じ値が出ることを確認する。各経路で、HAKATA の全住民について
`{id, handle, sha256(json.Marshal(Persona))}` を記録する（`Persona` の全フィールドを含む）。

| 経路 | 状況 | 本番での対応 |
|---|---|---|
| `fresh_constructor` | `NewMemoryStore()` 直後 | スナップショットなしの起動 |
| `after_re_ensure` | 上の状態でもう一度補完を呼ぶ | スナップショットあり（行が無い初回、または復元後） |
| `from_handle_only` | 全住民を `{ID, Handle}` だけのペルソナに置き換えて補完 | 古いスナップショット（活動傾向なし） |
| `after_partial_restore` | 先頭 20 人だけを残して補完 | 住民数が少なかった時点のスナップショット |

金型ファイルは `apps/server/internal/world/testdata/population_golden.json`。
**変更前のコードで生成し、最初のコミットに含める。以降、このファイルを書き換えない。**
書き換えが必要に見えたら、生成結果が変わったということなので、実装の誤りを疑う。

## 変更するファイル

| 層 | ファイル | 内容 |
|---|---|---|
| 金型 | `world/population_golden_test.go`（新規）、`world/testdata/population_golden.json`（新規） | 変更前のコードで生成 |
| hostcatalog | `population.go`（新規） | `Population` 型、`HandleSlug`、`parsePopulation`、`Populations` |
| hostcatalog | `preset.go` | `presetFile.Population`、`Preset.Population` |
| hostcatalog | `loader.go` | `id_prefix` の一意性検査 |
| hostcatalog | `population_test.go`（新規） | 検証規則のテスト |
| hostcatalog | `presets/hakata-canal-net.yaml` | `population:` を追加、`revision: 3` |
| world | `hakata_cast.go` → `population.go`（`git mv` して書き換え） | 共通の生成アルゴリズム |
| world | `store.go` | `populations` マップ、コンストラクタの配線 |
| world | `preset_hosts.go` | 1 回の読み込みで host と population を返す |
| world | `host_role.go`、`host_role_test.go` | `checkSingleExperiment` と、そのテストの置き換え |
| world | `hakata_cast_test.go` → `population_test.go`（`git mv`） | 中立な名前に |
| server | `cmd/server/runtime_store.go` | `EnsurePopulation` を、スナップショット対象のすべての局に対して呼ぶ |
| Doc | `hostcatalog/README.md` | `population:` の書式、`role` の説明の修正 |

## 残してよい `hakata` の記述（SC-003）

`apps/server/internal/world/` のテスト以外のファイルでは、コメントでの局の key の例示
（`hakata-canal-net` など）だけ。それ以外は中立な表現にする。

## リスクと対策

| リスク | 対策 |
|---|---|
| 乱数の消費順序のずれ | 「生成アルゴリズムの契約」と金型テスト。変更前に金型を生成する |
| YAML の `98` が数値になる | 引用符で囲む。`handle_prefixes` に `"98"` が含まれることを検査するテストを追加する |
| 既存ペルソナの `Interests` 再生成を、整理のつもりで消してしまう | FR-010。`TestEnsure…RefreshesOldBiasedRoutingInterests` が検出する |
| `Host` の比較が壊れる | 住民の定義を `Host` に入れない（設計の要点 2） |
| 既存のスナップショットが読めなくなる | ペルソナ ID の形式を変えない（FR-005）。金型テストで ID も固定 |
| 複数の局が同じ ID 接頭辞を使う | `id_prefix` の一意性を起動時に検査する |
