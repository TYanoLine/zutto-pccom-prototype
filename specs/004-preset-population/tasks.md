# Tasks: preset population

**Input**: `specs/004-preset-population/` の `spec.md`、`plan.md`
**Prerequisites**: `AGENTS.md`、`plan.md`（特に「生成アルゴリズムの契約」と「金型テスト」）

## この作業のルール（必読）

- 作業ブランチから `main` への PR を 1 本作る。`main` に直接コミットしない。
- **HAKATA の住民は 1 人も変えない**。これがこの作業の合格条件。金型テストが通ることで確認する。
- **T002 で金型ファイルをコミットしたあと、`-update-population-golden` を二度と使わない。**
  金型ファイル（`world/testdata/population_golden.json`）を書き換えない。
  金型テストが失敗したら、実装の誤り。金型ではなく実装を直す。
  2 回直して通らなければ、そこで止めて、どの経路のどの住民が違うかを報告する。
- 乱数の消費順序（`plan.md` の契約）を変えない。コードを「きれいにする」ために、
  呼び出しの順序・回数・評価のタイミングを変えない。
- スコープ外（`spec.md` 末尾）に手を出さない。特に、既存ペルソナの `Interests` を再生成する処理は、
  不要に見えても消さない。
- 既存ファイルに整形だけの差分を作らない。`gofmt -w` を既存ファイル全体にかけない。
- 1 タスク = 1 コミット（メッセージは `T0XX: 内容`）。ただし T009〜T013 は途中でビルドが通らないため、
  1 つのコミットにまとめる（メッセージは `T009-T013: 住民生成を preset の定義で駆動する`）。
- 実行していないコマンドを「成功した」と書かない。実行できなかったものは「未実施」と理由を書く。
- この作業で `.github/` 配下のファイルは変更しない。

---

## Phase 0: 金型（変更前のコードで生成する）

- [ ] **T001** `apps/server/internal/world/population_golden_test.go`（新規）を作る

  次の内容で作る。**この時点では、既存のコードを一切変更しない。**

  ```go
  package world

  import (
  	"crypto/sha256"
  	"encoding/hex"
  	"encoding/json"
  	"flag"
  	"os"
  	"path/filepath"
  	"testing"
  )

  var updatePopulationGolden = flag.Bool("update-population-golden", false, "rewrite testdata/population_golden.json from the current implementation")

  const (
  	populationGoldenPath  = "testdata/population_golden.json"
  	populationGoldenPhone = "0920000196"
  )

  type goldenEntry struct {
  	ID     string `json:"id"`
  	Handle string `json:"handle"`
  	SHA256 string `json:"sha256"`
  }

  // ensureResidents is the only place this file calls the population generator,
  // so a later rename of that API changes exactly this one line.
  func ensureResidents(s *MemoryStore, phone string) int {
  	return s.EnsureHakataExperimentPopulation(phone)
  }

  func goldenEntries(t *testing.T, personas []Persona) []goldenEntry {
  	t.Helper()
  	out := make([]goldenEntry, 0, len(personas))
  	for _, p := range personas {
  		data, err := json.Marshal(p)
  		if err != nil {
  			t.Fatal(err)
  		}
  		sum := sha256.Sum256(data)
  		out = append(out, goldenEntry{ID: p.ID, Handle: p.Handle, SHA256: hex.EncodeToString(sum[:])})
  	}
  	return out
  }

  func goldenHost(t *testing.T, s *MemoryStore) Host {
  	t.Helper()
  	h, err := s.HostByPhone(populationGoldenPhone)
  	if err != nil {
  		t.Fatal(err)
  	}
  	return h
  }

  // populationScenarios runs the four paths that matter in production (see plan.md).
  func populationScenarios(t *testing.T) map[string][]goldenEntry {
  	t.Helper()
  	out := map[string][]goldenEntry{}

  	// 1 and 2: a fresh process, then a restart over the existing population.
  	fresh := NewMemoryStore()
  	host := goldenHost(t, fresh)
  	out["fresh_constructor"] = goldenEntries(t, fresh.ListHostPersonas(host.ID))
  	ensureResidents(fresh, populationGoldenPhone)
  	out["after_re_ensure"] = goldenEntries(t, fresh.ListHostPersonas(host.ID))

  	// 3: older snapshots held handle-only skeletons.
  	handleOnly := NewMemoryStore()
  	for _, id := range append([]string(nil), handleOnly.memberships[host.ID]...) {
  		p := handleOnly.personas[id]
  		handleOnly.personas[id] = Persona{ID: p.ID, Handle: p.Handle}
  	}
  	ensureResidents(handleOnly, populationGoldenPhone)
  	out["from_handle_only"] = goldenEntries(t, handleOnly.ListHostPersonas(host.ID))

  	// 4: a snapshot taken when the station had fewer residents.
  	partial := NewMemoryStore()
  	keep := append([]string(nil), partial.memberships[host.ID][:20]...)
  	keepSet := make(map[string]bool, len(keep))
  	for _, id := range keep {
  		keepSet[id] = true
  	}
  	for id := range partial.personas {
  		if !keepSet[id] {
  			delete(partial.personas, id)
  		}
  	}
  	partial.memberships[host.ID] = keep
  	ensureResidents(partial, populationGoldenPhone)
  	out["after_partial_restore"] = goldenEntries(t, partial.ListHostPersonas(host.ID))
  	return out
  }

  func TestPopulationMatchesGolden(t *testing.T) {
  	got := populationScenarios(t)
  	if *updatePopulationGolden {
  		data, err := json.MarshalIndent(got, "", " ")
  		if err != nil {
  			t.Fatal(err)
  		}
  		if err := os.MkdirAll(filepath.Dir(populationGoldenPath), 0o755); err != nil {
  			t.Fatal(err)
  		}
  		if err := os.WriteFile(populationGoldenPath, append(data, '\n'), 0o644); err != nil {
  			t.Fatal(err)
  		}
  		t.Logf("rewrote %s", populationGoldenPath)
  		return
  	}

  	data, err := os.ReadFile(populationGoldenPath)
  	if err != nil {
  		t.Fatalf("read golden (generate it once with -update-population-golden on the unchanged implementation): %v", err)
  	}
  	var want map[string][]goldenEntry
  	if err := json.Unmarshal(data, &want); err != nil {
  		t.Fatal(err)
  	}
  	if len(want) != len(got) {
  		t.Fatalf("golden has %d scenarios, the test produces %d", len(want), len(got))
  	}
  	for name, w := range want {
  		g, ok := got[name]
  		if !ok {
  			t.Errorf("scenario %q is missing", name)
  			continue
  		}
  		if len(g) != len(w) {
  			t.Errorf("%s: %d residents, golden has %d", name, len(g), len(w))
  			continue
  		}
  		mismatches := 0
  		for i := range w {
  			if g[i] == w[i] {
  				continue
  			}
  			mismatches++
  			if mismatches <= 5 {
  				t.Errorf("%s[%d]: got id=%s handle=%s sha=%s, want id=%s handle=%s sha=%s",
  					name, i, g[i].ID, g[i].Handle, g[i].SHA256, w[i].ID, w[i].Handle, w[i].SHA256)
  			}
  		}
  		if mismatches > 5 {
  			t.Errorf("%s: %d residents differ in total", name, mismatches)
  		}
  	}
  }

  // A sanity check that does not depend on the golden file: the shape of the
  // population is what the persisted data refers to.
  func TestPopulationShape(t *testing.T) {
  	for name, entries := range populationScenarios(t) {
  		if len(entries) != 326 {
  			t.Errorf("%s: %d residents, want 326", name, len(entries))
  			continue
  		}
  		if entries[0].ID != "hakata-mari" || entries[0].Handle != "MARI" {
  			t.Errorf("%s: first resident = %+v, want hakata-mari / MARI", name, entries[0])
  		}
  		if entries[14].ID != "hakata-midnight" {
  			t.Errorf("%s: 15th resident id = %q, want hakata-midnight", name, entries[14].ID)
  		}
  		if entries[15].ID != "hakata-member-016" {
  			t.Errorf("%s: 16th resident id = %q, want hakata-member-016", name, entries[15].ID)
  		}
  	}
  }
  ```

- [ ] **T002** 金型を生成してコミットする（**変更前のコードで**）

  ```bash
  go -C apps/server test ./internal/world -run 'TestPopulationShape' -count=1
  go -C apps/server test ./internal/world -run 'TestPopulationMatchesGolden' -update-population-golden -count=1
  go -C apps/server test ./internal/world -count=1
  git status --short apps/server/internal/world
  ```

  - `TestPopulationShape` が通ることを、生成の**前に**確認する（通らなければ、T001 のコードを直す。既存のコードは直さない）。
  - 生成された `apps/server/internal/world/testdata/population_golden.json` を、T001 のテストと一緒にコミットする。
  - 続けて、`go -C apps/server test ./internal/world -run 'TestPopulationMatchesGolden' -count=3` が、
    フラグなしで 3 回とも通ることを確認する（生成が決定的であることの確認）。通らなければ止めて報告する。

**Checkpoint**: 変更前のコードに対する金型がコミットされ、通っている。ここから先、金型ファイルは変更しない。

---

## Phase 1: hostcatalog（定義の読み込み）

- [ ] **T003** `apps/server/internal/hostcatalog/population.go`（新規）を作る

  ```go
  package hostcatalog

  import (
  	"errors"
  	"fmt"
  	"strings"
  )

  // Population describes the resident members of a host: the data the world's
  // population generator needs. The member records themselves are world state;
  // they are generated deterministically from this description. The generator
  // lives in the world package; this package only carries and validates data.
  // The slices are read-only: callers must not modify them.
  type Population struct {
  	// Seed makes generation reproducible: member i uses Seed + (i+1)*7919.
  	Seed int64
  	// IDPrefix is the persona ID prefix. Core residents get
  	// "<prefix>-<HandleSlug(handle)>", the others "<prefix>-member-NNN". It must be
  	// unique across presets.
  	IDPrefix string
  	// CoreHandles are the first residents, in order. Their IDs are stable so that
  	// persisted data and other definitions can refer to them.
  	CoreHandles []string
  	// HandleBases and HandlePrefixes are the vocabulary for generated handles.
  	// Their order matters: they are indexed by a seeded random number.
  	HandleBases    []string
  	HandlePrefixes []string
  }

  type presetPopulation struct {
  	Seed           *int64   `yaml:"seed"`
  	IDPrefix       string   `yaml:"id_prefix"`
  	CoreHandles    []string `yaml:"core_handles"`
  	HandleBases    []string `yaml:"handle_bases"`
  	HandlePrefixes []string `yaml:"handle_prefixes"`
  }

  // HandleSlug lower-cases a handle and keeps only a-z, 0-9 and "-". It is the
  // part of a core resident's persona ID that comes from the handle.
  func HandleSlug(handle string) string {
  	out := make([]byte, 0, len(handle))
  	for i := 0; i < len(handle); i++ {
  		c := handle[i]
  		if c >= 'A' && c <= 'Z' {
  			c = c - 'A' + 'a'
  		}
  		if (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '-' {
  			out = append(out, c)
  		}
  	}
  	return string(out)
  }

  // parsePopulation validates the structure of a preset's population block.
  // Rules that need the member count are checked by Populations.
  func parsePopulation(f *presetPopulation) (*Population, error) {
  	if f == nil {
  		return nil, nil
  	}
  	var errs []error
  	add := func(field string, err error) {
  		if err != nil {
  			errs = append(errs, fmt.Errorf("%s: %w", field, err))
  		}
  	}

  	pop := &Population{IDPrefix: f.IDPrefix}
  	if f.Seed == nil {
  		add("seed", errors.New("is required"))
  	} else {
  		pop.Seed = *f.Seed
  	}
  	if strings.TrimSpace(f.IDPrefix) == "" {
  		add("id_prefix", errors.New("is required"))
  	} else {
  		add("id_prefix", checkKey(f.IDPrefix))
  	}

  	seenHandles := map[string]bool{}
  	seenSlugs := map[string]string{}
  	for i, raw := range f.CoreHandles {
  		field := fmt.Sprintf("core_handles[%d]", i)
  		h := strings.TrimSpace(raw)
  		if h == "" {
  			add(field, errors.New("must not be empty"))
  			continue
  		}
  		lower := strings.ToLower(h)
  		if seenHandles[lower] {
  			add(field, fmt.Errorf("%q is listed twice", h))
  			continue
  		}
  		seenHandles[lower] = true
  		slug := HandleSlug(h)
  		if slug == "" {
  			add(field, fmt.Errorf("%q has no letters or digits to build a persona ID from", h))
  			continue
  		}
  		if prev, dup := seenSlugs[slug]; dup {
  			add(field, fmt.Errorf("%q would get the same persona ID as %q", h, prev))
  			continue
  		}
  		seenSlugs[slug] = h
  		pop.CoreHandles = append(pop.CoreHandles, h)
  	}
  	pop.HandleBases = cleanWords(f.HandleBases, "handle_bases", add)
  	pop.HandlePrefixes = cleanWords(f.HandlePrefixes, "handle_prefixes", add)

  	if len(errs) > 0 {
  		return nil, errors.Join(errs...)
  	}
  	return pop, nil
  }

  func cleanWords(in []string, field string, add func(string, error)) []string {
  	var out []string
  	for i, raw := range in {
  		w := strings.TrimSpace(raw)
  		if w == "" {
  			add(fmt.Sprintf("%s[%d]", field, i), errors.New("must not be empty"))
  			continue
  		}
  		out = append(out, w)
  	}
  	return out
  }

  // Populations returns the resident-population definitions by preset key, after
  // checking them against each preset's member count. A preset without a
  // population block has no entry.
  func Populations(presets []Preset) (map[string]Population, error) {
  	out := map[string]Population{}
  	var errs []error
  	for _, p := range presets {
  		if p.Population == nil {
  			continue
  		}
  		pop := *p.Population
  		if p.Members == nil {
  			errs = append(errs, fmt.Errorf("%s: population: host.members is required", p.Source))
  			continue
  		}
  		members := *p.Members
  		if len(pop.CoreHandles) > members {
  			errs = append(errs, fmt.Errorf("%s: population: core_handles has %d entries but host.members is %d", p.Source, len(pop.CoreHandles), members))
  		}
  		if members > len(pop.CoreHandles) && len(pop.HandleBases) == 0 {
  			errs = append(errs, fmt.Errorf("%s: population: handle_bases is required because host.members (%d) exceeds core_handles (%d)", p.Source, members, len(pop.CoreHandles)))
  		}
  		out[p.Key] = pop
  	}
  	if len(errs) > 0 {
  		return nil, errors.Join(errs...)
  	}
  	return out, nil
  }
  ```

- [ ] **T004** `apps/server/internal/hostcatalog/preset.go` に `population` を追加する

  1. `presetFile` に、`Generation` の次の行として追加する。

     ```go
     Population *presetPopulation `yaml:"population"`
     ```

  2. `Preset` 構造体の `GenerationFlags GenerationFlags` の次の行に追加する。

     ```go
     // Population describes the resident members; nil means the host has none.
     Population *Population
     ```

  3. `ParsePreset` の中で、`dialMode` の検査のあと、`if len(errs) > 0 {` の**直前**に追加する。

     ```go
     population, err := parsePopulation(f.Population)
     add("population", err)
     ```

     （`err` が既に同じスコープで宣言されていてコンパイルエラーになる場合は、`:=` を `=` に直すか、別の変数名を使う。）

  4. `ParsePreset` の最後の `Preset{...}` に `Population: population,` を追加する。
  5. `Descriptor()` は変更しない（住民の定義は `HostDescriptor` に入れない）。

- [ ] **T005** `apps/server/internal/hostcatalog/loader.go` に `id_prefix` の一意性検査を追加する

  `LoadPresetsFS` の中で、`keys`、`phones` と並べて `prefixes := map[string]string{}` を作り、
  電話番号の重複検査の**直後**（`presets = append(presets, p)` の前）に追加する。

  ```go
  		if p.Population != nil {
  			if prev, dup := prefixes[p.Population.IDPrefix]; dup {
  				errs = append(errs, fmt.Errorf("%s: population.id_prefix %q is already used by %s", name, p.Population.IDPrefix, prev))
  				continue
  			}
  			prefixes[p.Population.IDPrefix] = name
  		}
  ```

- [ ] **T006** `apps/server/internal/hostcatalog/population_test.go`（新規）にテストを書く

  既存の `preset_test.go` の `samplePreset` と `ParsePreset` の呼び方、`loader_test.go` の
  `fstest.MapFS` の使い方を先に読み、同じ形式で書く。次を確認する。

  | テスト | 内容 |
  |---|---|
  | 正常系 | 有効な `population:` が `Preset.Population` に入る。`handle_prefixes: ["98"]` が文字列 `"98"` として読める |
  | 省略 | `population:` が無ければ `Preset.Population == nil` |
  | 必須 | `seed` が無い、`id_prefix` が無い、`id_prefix` が不正な形式（大文字、空白）でエラー。`seed: 0` は有効 |
  | コア住民 | 空、大文字小文字違いの重複、slug が空（例 `"!!"`）、slug の衝突（例 `"A.B"` と `"AB"`）でエラー |
  | 素材 | `handle_bases` または `handle_prefixes` に空の要素があるとエラー |
  | 複数のエラー | 複数の違反が、1 回のエラーにまとまって報告される |
  | キーの typo | `population:` の下の未知のキー（例 `seeds:`）がエラー |
  | `Populations` | `core_handles` が `members` より多い、`members` が `core_handles` より多いのに `handle_bases` が空、でエラー。`members == len(core_handles)` で `handle_bases` が空は有効 |
  | `id_prefix` の重複 | 2 つの preset が同じ `id_prefix` を持つと `LoadPresetsFS` がエラー |
  | `HandleSlug` | `"MARI"` → `"mari"`、`"N88"` → `"n88"`、`"A.B_C-D"` → `"abc-d"`、`"MIDNIGHT"` → `"midnight"` |
  | 埋め込みの HAKATA | `LoadPresets` で読んだ `hakata-canal-net` が、シード `199608260920`、`id_prefix` `hakata`、コア住民 15 名（先頭 `MARI`、末尾 `MIDNIGHT`）、素材 43 語、接頭辞 5 語（`"98"` を含む）を持つ |

  最後の「埋め込みの HAKATA」のテストは、T007 のあとで通る。

- [ ] **T007** `apps/server/internal/hostcatalog/presets/hakata-canal-net.yaml` に `population:` を追加する

  1. `revision: 2` を `revision: 3` にする。
  2. ファイル末尾（`generation:` ブロックの後）に、`plan.md` の「スキーマ」のブロックをそのまま追加する。
     **値は 1 文字も変えない。順序も変えない。すべての文字列に引用符を付ける。**
  3. 冒頭のコメントに、「住民の定義は `population:` ブロックにある」と 1 行追記する。

- [ ] **T008** 確認

  ```bash
  go -C apps/server build ./... && go -C apps/server test ./internal/hostcatalog/... ./internal/world/... -count=1
  ```

  この時点では、`world` はまだ古い生成コードを使っているので、**金型テストは通る**はず。
  通らなければ、T003〜T007 のどこかが `world` の挙動に影響している。止めて報告する。

**Checkpoint**: preset が住民の定義を読める。`world` の挙動は変わっていない。

---

## Phase 2: world（定義で生成を駆動する）

T009〜T013 は 1 つのコミットにまとめる（途中でビルドが通らないため）。

- [ ] **T009** `world/hakata_cast.go` を `world/population.go` に移して書き換える

  ```bash
  git mv apps/server/internal/world/hakata_cast.go apps/server/internal/world/population.go
  ```

  ファイルの中身を、次のとおりにする。**計算の順序と回数は、元のコードと同一にする**
  （元のコードの `hakataCoreHandles`、`hakataHandleBases` の定数、`normalizeFixtureID` は削除する）。
  `randomResidentInterests` の表と、`fillActivitySkeleton` の分布は、元のコードの値と式をそのまま写す
  （名前の `Hakata` だけを除く）。

  ```go
  package world

  import (
  	"fmt"
  	"math/rand"
  	"strings"

  	"zutto-pccom/apps/server/internal/hostcatalog"
  )

  // ensurePopulationLocked creates the resident members that spec describes for
  // host, up to host.Members, and attaches them to the host. It never creates
  // posts or expensive persona details: detailed life facts and prose traits stay
  // lazy world state. It is deterministic for a given spec and store content, and
  // the order and number of random draws is part of that contract (see the
  // golden test).
  func ensurePopulationLocked(s *MemoryStore, host Host, spec hostcatalog.Population) int {
  	if s == nil || host.ID == "" {
  		return 0
  	}
  	target := host.Members
  	if target <= 0 {
  		return 0
  	}

  	existingMembership := make(map[string]bool, len(s.memberships[host.ID]))
  	usedHandles := map[string]bool{}
  	for _, id := range s.memberships[host.ID] {
  		existingMembership[id] = true
  		if p, ok := s.personas[id]; ok && strings.TrimSpace(p.Handle) != "" {
  			usedHandles[strings.ToLower(strings.TrimSpace(p.Handle))] = true
  		}
  	}

  	added := 0
  	for i := 0; i < target; i++ {
  		rng := rand.New(rand.NewSource(spec.Seed + int64(i+1)*7919))
  		id := fmt.Sprintf("%s-member-%03d", spec.IDPrefix, i+1)
  		handle := ""
  		if i < len(spec.CoreHandles) {
  			handle = spec.CoreHandles[i]
  			// Core residents keep stable, handle-derived IDs so older snapshots and
  			// facts continue to refer to the same residents.
  			id = spec.IDPrefix + "-" + hostcatalog.HandleSlug(handle)
  		} else {
  			handle = nextResidentHandle(rng, spec, usedHandles)
  		}
  		usedHandles[strings.ToLower(handle)] = true

  		if _, ok := s.personas[id]; !ok {
  			s.personas[id] = newPersonaSkeleton(rng, id, handle)
  		} else {
  			// Older debug snapshots contained handle-only skeletons. Enrich only
  			// missing activity state without replacing any already materialized
  			// persona details.
  			p := s.personas[id]
  			if strings.TrimSpace(p.Handle) == "" {
  				p.Handle = handle
  			}
  			if strings.TrimSpace(p.ActivityPattern) == "" {
  				fillActivitySkeleton(rng, &p)
  			}
  			// Refresh only the cheap routing affinities on startup so older
  			// snapshots do not preserve a known computer/game-heavy distribution;
  			// detailed persona facts remain intact.
  			p.Interests = randomResidentInterests(rng)
  			s.personas[id] = p
  		}

  		if existingMembership[id] {
  			continue
  		}
  		s.memberships[host.ID] = append(s.memberships[host.ID], id)
  		existingMembership[id] = true
  		added++
  	}
  	return added
  }

  // EnsurePopulation restores the resident population that the host's preset
  // declares, up to Host.Members. It returns 0 for a host without a population
  // definition. It never creates posts or expensive persona details.
  func (s *MemoryStore) EnsurePopulation(phone string) int {
  	s.mu.Lock()
  	defer s.mu.Unlock()
  	host, ok := s.hosts[phone]
  	if !ok || host.ID == "" {
  		return 0
  	}
  	spec, ok := s.populations[host.ID]
  	if !ok {
  		return 0
  	}
  	return ensurePopulationLocked(s, host, spec)
  }

  func newPersonaSkeleton(rng *rand.Rand, id, handle string) Persona {
  	p := Persona{ID: id, Handle: handle}
  	fillActivitySkeleton(rng, &p)
  	p.Interests = randomResidentInterests(rng)
  	return p
  }

  // fillActivitySkeleton, randomResidentInterests: copy the bodies of
  // fillHakataActivitySkeleton and randomHakataInterests from the previous
  // hakata_cast.go EXACTLY (same thresholds, same order of rng calls), changing
  // only the function names.

  // nextResidentHandle draws a handle from the spec's vocabulary that is not in
  // used. Every attempt evaluates all candidate formats in a fixed order, whether
  // or not they are used, because each evaluation consumes random numbers.
  func nextResidentHandle(rng *rand.Rand, spec hostcatalog.Population, used map[string]bool) string {
  	if len(spec.HandleBases) > 0 {
  		for attempt := 0; attempt < 80; attempt++ {
  			base := spec.HandleBases[rng.Intn(len(spec.HandleBases))]
  			variants := []string{
  				base,
  				fmt.Sprintf("%s.%c", base, 'A'+rune(rng.Intn(26))),
  				fmt.Sprintf("%s-%c", base, 'A'+rune(rng.Intn(26))),
  				fmt.Sprintf("%s%02d", base, 1+rng.Intn(99)),
  				fmt.Sprintf("%s_%02d", base, 1+rng.Intn(99)),
  			}
  			if len(spec.HandlePrefixes) > 0 {
  				variants = append(variants, fmt.Sprintf("%s-%s", spec.HandlePrefixes[rng.Intn(len(spec.HandlePrefixes))], base))
  			}
  			for _, candidate := range variants {
  				key := strings.ToLower(candidate)
  				if !used[key] {
  					return candidate
  				}
  			}
  		}
  	}
  	for n := 1; ; n++ {
  		candidate := fmt.Sprintf("USER.%c%03d", 'A'+rune(rng.Intn(26)), n)
  		if !used[strings.ToLower(candidate)] {
  			return candidate
  		}
  	}
  }
  ```

  - 上のコメント「copy the bodies … EXACTLY」の箇所には、元の `fillHakataActivitySkeleton` と
    `randomHakataInterests` の本体を、**関数名以外を変えずに**写す（`git show HEAD:...` で元のファイルを見る）。
    コメント自体は、コピー後に削除する。
  - 元のコードの先頭にあったコメント（「HAKATA CANAL NET is currently a generator-evaluation station…」）は、
    局に依存しない説明に置き換えた（上のコード）。残さない。

- [ ] **T010** `world/store.go` に住民の定義を持たせる

  1. import に `"zutto-pccom/apps/server/internal/hostcatalog"` を追加する。
  2. `MemoryStore` 構造体の `memberships map[string][]string` の次の行に追加する。

     ```go
     // populations holds each host's resident-population definition (from its
     // preset), keyed by host ID. Hosts without an entry have no residents.
     populations map[string]hostcatalog.Population
     ```

  3. `NewMemoryStore` の初期化に `populations: map[string]hostcatalog.Population{},` を追加する。
  4. `NewMemoryStore` の `presetHosts()` のループを、次のとおりに置き換える。

     ```go
     	// Host definitions and resident populations come from the embedded YAML
     	// presets (internal/hostcatalog/presets). A host gets residents only when its
     	// preset has a population block; the role plays no part.
     	populations := presetPopulations()
     	for _, h := range presetHosts() {
     		s.hosts[h.Phone] = h
     		if h.IsExperiment() {
     			s.posts[h.ID] = nil
     		}
     		if spec, ok := populations[h.ID]; ok {
     			s.populations[h.ID] = spec
     			ensurePopulationLocked(s, h, spec)
     		}
     	}
     ```

     （`if h.IsExperiment() { s.posts[h.ID] = nil }` は、元の挙動の維持のために残す。変更しない。）

- [ ] **T011** `world/preset_hosts.go` で、host と population を 1 回の読み込みで返す

  `presetHostsOnce`、`presetHostList`、`presetHostErr` を、次の形に置き換える。

  ```go
  type presetData struct {
  	hosts       []Host
  	populations map[string]hostcatalog.Population
  }

  var (
  	presetDataOnce sync.Once
  	presetDataVal  presetData
  	presetDataErr  error
  )

  // loadPresetData loads the embedded presets once. The files are compiled into
  // the binary and covered by tests, so a broken definition is a programming
  // error: it panics instead of letting the process start without its hosts.
  func loadPresetData() presetData {
  	presetDataOnce.Do(func() {
  		presets, err := hostcatalog.LoadPresets(hostcatalog.Options{})
  		if err != nil {
  			presetDataErr = err
  			return
  		}
  		descriptors, err := hostcatalog.Descriptors(presets)
  		if err != nil {
  			presetDataErr = err
  			return
  		}
  		populations, err := hostcatalog.Populations(presets)
  		if err != nil {
  			presetDataErr = err
  			return
  		}
  		for _, d := range descriptors {
  			presetDataVal.hosts = append(presetDataVal.hosts, HostFromDescriptor(d))
  		}
  		presetDataVal.populations = populations
  	})
  	if presetDataErr != nil {
  		panic(fmt.Sprintf("world: invalid host presets: %v", presetDataErr))
  	}
  	return presetDataVal
  }

  func presetHosts() []Host {
  	return append([]Host(nil), loadPresetData().hosts...)
  }

  func presetPopulations() map[string]hostcatalog.Population {
  	src := loadPresetData().populations
  	out := make(map[string]hostcatalog.Population, len(src))
  	for key, spec := range src {
  		out[key] = spec
  	}
  	return out
  }
  ```

  - `checkSingleExperiment` の呼び出しは削除する（T012 で関数も削除する）。
  - `HostFromDescriptor` は変更しない。

- [ ] **T012** `world/host_role.go` と `host_role_test.go` を更新する

  - `checkSingleExperiment` を削除する。`fmt` の import が不要になれば削除する。
  - `IsExperiment` のコメントの最後の文（「The only remaining role-based behavior is …」）を、次に置き換える。

    ```go
    // The role is only a label: the resident population is defined by the preset's
    // population block, not by the role.
    ```

  - `host_role_test.go`:
    - `TestPresetsAllowAtMostOneExperimentHost` を削除する。
    - `TestOnlyExperimentHostsGetTheResidentPopulation` を、次のテストに置き換える
      （`hostcatalog` と `strings` は、このファイルで既に import されている）。

      ```go
      func TestResidentsFollowThePopulationDefinitionNotTheRole(t *testing.T) {
      	s := NewMemoryStore()
      	if n := len(s.ListHostPersonas("hakata-canal-net")); n == 0 {
      		t.Fatal("a host whose preset has a population got no residents")
      	}
      	if n := len(s.ListHostPersonas("busy-test")); n != 0 {
      		t.Fatalf("a host without a population got %d residents", n)
      	}

      	// The experiment role alone creates nobody.
      	s.SaveHost(Host{ID: "exp-no-population", Phone: "0910000001", Role: hostcatalog.RoleExperiment, Members: 50})
      	if added := s.EnsurePopulation("0910000001"); added != 0 {
      		t.Fatalf("a role-only host got %d residents", added)
      	}
      	if n := len(s.ListHostPersonas("exp-no-population")); n != 0 {
      		t.Fatalf("a role-only host has %d residents", n)
      	}

      	// A population definition alone creates residents, without any role.
      	s.populations["pop-only"] = hostcatalog.Population{
      		Seed: 1, IDPrefix: "pop-only",
      		CoreHandles: []string{"ALPHA"}, HandleBases: []string{"BETA", "GAMMA"},
      	}
      	s.SaveHost(Host{ID: "pop-only", Phone: "0990000002", Members: 30})
      	if added := s.EnsurePopulation("0990000002"); added != 30 {
      		t.Fatalf("added = %d, want 30", added)
      	}
      	residents := s.ListHostPersonas("pop-only")
      	if len(residents) != 30 {
      		t.Fatalf("residents = %d, want 30", len(residents))
      	}
      	if residents[0].ID != "pop-only-alpha" || residents[1].ID != "pop-only-member-002" {
      		t.Fatalf("unexpected IDs: %q, %q", residents[0].ID, residents[1].ID)
      	}

      	// Handles are unique within the host, and persona IDs never collide with
      	// another host's (each host's IDs start with its own id_prefix).
      	handles := map[string]bool{}
      	for _, r := range residents {
      		key := strings.ToLower(r.Handle)
      		if handles[key] {
      			t.Fatalf("duplicate handle %q", r.Handle)
      		}
      		handles[key] = true
      		if !strings.HasPrefix(r.ID, "pop-only-") {
      			t.Fatalf("persona ID %q does not use the host's id_prefix", r.ID)
      		}
      	}

      	// A second call adds nobody.
      	if added := s.EnsurePopulation("0990000002"); added != 0 {
      		t.Fatalf("second call added %d", added)
      	}
      }
      ```

- [ ] **T013** 金型テストの呼び出しを新しい API に合わせる

  `world/population_golden_test.go` の `ensureResidents` の本体**だけ**を、次の 1 行に変える。
  **このファイルの他の部分と、`testdata/population_golden.json` は変更しない。**

  ```go
  return s.EnsurePopulation(phone)
  ```

  続けて、次を実行する。

  ```bash
  go -C apps/server build ./...
  go -C apps/server test ./internal/world/... ./internal/hostcatalog/... -count=1
  git diff --stat <T002 のコミット>..HEAD -- apps/server/internal/world/testdata
  ```

  - **`TestPopulationMatchesGolden` が通ること。** 通らなければ、`plan.md` の契約を、コードと 1 行ずつ照合する。
  - 最後の `git diff` が空（金型ファイルが変わっていない）であること。
  - T009〜T013 をまとめた 1 つのコミットを作る。

---

## Phase 3: 呼び出し側とテスト名

- [ ] **T014** `world/hakata_cast_test.go` を `world/population_test.go` に改名して直す

  ```bash
  git mv apps/server/internal/world/hakata_cast_test.go apps/server/internal/world/population_test.go
  ```

  - テスト関数名を中立にする。
    `TestHakataExperimentInterestsAreSparseAndNotComputerUniversal` → `TestResidentInterestsAreSparseAndNotComputerUniversal`、
    `TestEnsureHakataExperimentPopulationRefreshesOldBiasedRoutingInterests` → `TestEnsurePopulationRefreshesOldBiasedRoutingInterests`。
  - `store.EnsureHakataExperimentPopulation(host.Phone)` を `store.EnsurePopulation(host.Phone)` にする。
  - メッセージの「HAKATA」を除く（`is still a universal HAKATA interest` → `is still a universal interest`、
    `missing HAKATA personas` → `missing resident personas`）。
  - テストの検査内容は変えない。局の電話番号（`0920000196`）は、テストの fixture として残してよい。

- [ ] **T015** `apps/server/cmd/server/runtime_store.go` を直す

  スナップショット復元後の補完を、次のとおりにする（`IsExperiment()` の条件を外す）。

  ```go
  	for _, h := range snapshotHosts {
  		// Restore the resident population after the snapshot so an older snapshot
  		// does not shrink it. Hosts without a population definition add nobody.
  		if added := store.EnsurePopulation(h.Phone); added > 0 {
  			log.Printf("resident population restored: host=%s added_members=%d", h.ID, added)
  		}
  	}
  ```

  `go -C apps/server build ./...` が通ること。

- [ ] **T016** 確認

  ```bash
  go -C apps/server build ./...
  go -C apps/server vet ./...
  go -C apps/server test ./... -count=1
  grep -rniE "hakata" apps/server/internal/world --include=*.go | grep -v _test.go
  ```

  最後の `grep` に残ってよいのは、コメントでの局の key の例示だけ。それ以外は直す。

---

## Phase 4: ドキュメントと PR

- [ ] **T017** `apps/server/internal/hostcatalog/README.md` を更新する

  1. 「Preset files」のスキーマの例に、`population:` ブロックを追加する（`plan.md` のスキーマの形式で、
     HAKATA の値の一部を省略した例でよい。`seed`、`id_prefix`、`core_handles`、`handle_bases`、`handle_prefixes`）。
  2. 「Population」の節を新設し、次を説明する: 住民は `population:` があるときだけ生成される（`role` と無関係）、
     局のデータ（YAML）と生成アルゴリズム（Go）の分担、配列の順序が結果を決めること、
     `id_prefix` が preset 間で一意であること、`host.members` が人数であること、
     検証規則（`plan.md` の表の要約）。
  3. 「Changing presets safely」の、「`role: experiment` still marks the evaluation station for one thing only: its
     resident population …」の項目を削除し、「`population:` の `seed`、`id_prefix`、配列の順序を変えると、
     既存の住民が変わる（永続化済みのデータと食い違う）」という注意に置き換える。
  4. 冒頭の「Status」に、住民の定義を読むようになったことを 1 文で追記する。

- [ ] **T018** 全体の確認と記録

  ```bash
  go -C apps/server build ./...
  go -C apps/server vet ./...
  go -C apps/server test ./... -count=1
  go -C apps/server test ./internal/world -run 'TestPopulationMatchesGolden|TestPopulationShape' -count=3 -v
  git diff --stat main...HEAD
  ```

  結果を `specs/004-preset-population/verification.md`（新規）に、次の形式で記録する。

  ```markdown
  # Verification: preset population

  | コマンド | 結果 | 備考 |
  |---|---|---|
  | `go -C apps/server build ./...` | 成功 / 失敗 / 未実施 | |
  | `go -C apps/server vet ./...` | 〃 | |
  | `go -C apps/server test ./...` | 〃 | |
  | 金型テスト（3 回連続） | 〃 | 4 つの経路すべて |

  ## 金型ファイルが変わっていないこと
  （`git diff <T002 のコミット>..HEAD -- apps/server/internal/world/testdata` の結果。空であること）

  ## 残った hakata を含むファイル（world のテスト以外）
  （grep の結果と、許可リストに入っているか）

  ## 手動確認
  - サーバを起動して HAKATA に接続し、板の一覧が従来どおり表示される: 確認済み / 未実施
  ```

- [ ] **T019** PR を作る

  - `main` への PR。**draft** で作成する。
  - タイトル: `住民の定義を preset の population に外部化する（HAKATA の住民は不変）`
  - 説明に含める: 目的、変更の要約、**HAKATA の住民が変わっていないことの根拠（金型テストの 4 経路と、
    金型ファイルが変更前に生成され、以降変更されていないこと）**、
    `verification.md` の結果（未実施を含めて正直に）、スコープ外の項目
    （`Interests` の再生成を維持していること、活動傾向・興味ドメインは Go に残していること）。

---

## 実行順序

```text
Phase 0 (T001 → T002)
  └─ Phase 1 (T003 → T004 → T005 → T006, T007 → T008)
       └─ Phase 2 (T009〜T013 を 1 コミット)
            └─ Phase 3 (T014, T015 → T016) → Phase 4 (T017 → T018 → T019)
```

## 完了の定義

- `spec.md` の FR-001〜FR-011 と SC-001〜SC-004 を満たす（手動確認が未実施なら、その旨を明記する）。
- 金型テストが、変更前に生成した金型ファイルを変更せずに通る。
- 変更が `plan.md` の「変更するファイル」の表の範囲に収まっている。
