# Tasks: Erika-K station detail

**Input**: `specs/005-erika-k-station-detail/` の `spec.md`、`plan.md`
**Prerequisites**: `AGENTS.md`、`plan.md`（特に「金型」と「局名から導出する文字列」）

## この作業のルール（必読）

- 作業ブランチから `main` への PR を 1 本作る。`main` に直接コミットしない。
- **HAKATA の画面は 1 バイトも変えない**。これが合格条件。金型テストが通ることで確認する。
- **T003 で金型ファイルをコミットしたあと、`-update-erikak-golden` を二度と使わない。**
  金型ファイル（`erikak/testdata/boards_golden.json`、`erikak/testdata/screens_golden.txt`）を書き換えない。
  金型テストが失敗したら、実装の誤り。金型ではなく実装を直す。
  2 回直して通らなければ、そこで止めて、金型のどの行が違うかを報告する。
- 画面の文字列（空白、改行、全角・半角）を、整えるために変えない。
- スコープ外（`spec.md` 末尾）に手を出さない。特に、`V`、`WHO`、`MEMB`、メール一覧、ファイル一覧、
  `JUNK`、前回アクセスの日時、`Config` は変更しない。
- 既存ファイルに整形だけの差分を作らない。`gofmt -w` を既存ファイル全体にかけない。
- 1 タスク = 1 コミット（メッセージは `T0XX: 内容`）。ただし T011〜T017 は途中でビルドが通らないため、
  1 つのコミットにまとめる（メッセージは `T011-T017: Erika-K の板構成とログイン文言を局の定義で駆動する`）。
- 実行していないコマンドを「成功した」と書かない。実行できなかったものは「未実施」と理由を書く。
- この作業で `.github/` 配下のファイルは変更しない。

---

## Phase 0: 金型（変更前のコードで生成する）

- [ ] **T001** `apps/server/internal/hostprogram/erikak/golden_test.go`（新規）を作る

  次の内容で作る。**この時点では、既存のコードを一切変更しない。**

  ```go
  package erikak

  import (
  	"encoding/json"
  	"flag"
  	"fmt"
  	"os"
  	"path/filepath"
  	"strings"
  	"testing"

  	"zutto-pccom/apps/server/internal/world"
  )

  var updateErikaGolden = flag.Bool("update-erikak-golden", false, "rewrite testdata golden files from the current implementation")

  const (
  	boardsGoldenPath  = "testdata/boards_golden.json"
  	screensGoldenPath = "testdata/screens_golden.txt"
  	goldenHostPhone   = "0920000196"
  )

  type goldenBoard struct {
  	Path                 string  `json:"path"`
  	Key                  string  `json:"key"`
  	Alias                string  `json:"alias,omitempty"`
  	Parent               string  `json:"parent,omitempty"`
  	Name                 string  `json:"name"`
  	Hidden               bool    `json:"hidden,omitempty"`
  	Scope                string  `json:"scope,omitempty"`
  	RootAuthorPolicy     string  `json:"root_author_policy,omitempty"`
  	ActivityWeight       float64 `json:"activity_weight"`
  	ReplyRate            float64 `json:"reply_rate"`
  	RetainedRootCap      int     `json:"retained_root_cap"`
  	VerifiedReferentRate float64 `json:"verified_referent_rate"`
  	Unread               bool    `json:"unread"`
  }

  // goldenBoardTable is the only place this file reads the board definitions, so
  // the refactor changes exactly this function.
  func goldenBoardTable(t *testing.T) []goldenBoard {
  	t.Helper()
  	out := make([]goldenBoard, 0, len(boardTree))
  	for _, n := range boardTree {
  		out = append(out, goldenBoard{
  			Path: n.Path, Key: n.Key, Alias: n.Alias, Parent: n.Parent, Name: n.Name,
  			Hidden: n.Hidden, Scope: n.SemanticScope, RootAuthorPolicy: n.RootAuthorPolicy,
  			ActivityWeight: n.ActivityWeight, ReplyRate: n.ReplyRate,
  			RetainedRootCap: n.RetainedRootCap, VerifiedReferentRate: n.VerifiedReferentRate,
  			Unread: unreadBoard[n.Path],
  		})
  	}
  	return out
  }

  // visible makes control characters readable so the transcript is a stable text file.
  func visible(text string) string {
  	return strings.NewReplacer("\x1b", "<ESC>", "\r\n", "<CRLF>\n", "\r", "<CR>").Replace(text)
  }

  // screenTranscript drives scripted sessions through the host program and records
  // every input and output. The store has no posts, so no output depends on time
  // or randomness.
  func screenTranscript(t *testing.T) string {
  	t.Helper()
  	store := world.NewMemoryStore()
  	host, err := store.HostByPhone(goldenHostPhone)
  	if err != nil {
  		t.Fatal(err)
  	}
  	var out strings.Builder
  	record := func(label, text string) {
  		fmt.Fprintf(&out, "===== %s =====\n%s\n", label, visible(text))
  	}
  	send := func(r *Runtime, input string) {
  		text, disconnect := r.HandleLine(input)
  		record(fmt.Sprintf("input %q disconnect=%t", input, disconnect), text)
  	}

  	guest := New(host, store)
  	record("welcome", guest.Welcome())
  	send(guest, "GUEST")
  	send(guest, "1")
  	send(guest, "/")
  	for _, b := range goldenBoardTable(t) {
  		send(guest, "BJ "+b.Path)
  		send(guest, "/")
  	}
  	for _, in := range []string{"MA", "T", "H", "MEMB", "V", "WHO", "9"} {
  		send(guest, in)
  	}

  	member := New(host, store)
  	send(member, "MIKI")
  	send(member, "dummy")
  	return out.String()
  }

  func firstDiff(got, want string) string {
  	g, w := strings.Split(got, "\n"), strings.Split(want, "\n")
  	for i := 0; i < len(g) && i < len(w); i++ {
  		if g[i] != w[i] {
  			return fmt.Sprintf("line %d:\n got:  %q\n want: %q", i+1, g[i], w[i])
  		}
  	}
  	return fmt.Sprintf("lengths differ: got %d lines, want %d lines", len(g), len(w))
  }

  func TestScreensAndBoardsMatchGolden(t *testing.T) {
  	boardsJSON, err := json.MarshalIndent(goldenBoardTable(t), "", " ")
  	if err != nil {
  		t.Fatal(err)
  	}
  	gotBoards := string(boardsJSON) + "\n"
  	gotScreens := screenTranscript(t)

  	if *updateErikaGolden {
  		if err := os.MkdirAll(filepath.Dir(boardsGoldenPath), 0o755); err != nil {
  			t.Fatal(err)
  		}
  		if err := os.WriteFile(boardsGoldenPath, []byte(gotBoards), 0o644); err != nil {
  			t.Fatal(err)
  		}
  		if err := os.WriteFile(screensGoldenPath, []byte(gotScreens), 0o644); err != nil {
  			t.Fatal(err)
  		}
  		t.Logf("rewrote %s and %s", boardsGoldenPath, screensGoldenPath)
  		return
  	}

  	wantBoards, err := os.ReadFile(boardsGoldenPath)
  	if err != nil {
  		t.Fatalf("read golden (generate it once with -update-erikak-golden on the unchanged implementation): %v", err)
  	}
  	if gotBoards != string(wantBoards) {
  		t.Errorf("board table differs from the golden: %s", firstDiff(gotBoards, string(wantBoards)))
  	}
  	wantScreens, err := os.ReadFile(screensGoldenPath)
  	if err != nil {
  		t.Fatal(err)
  	}
  	if gotScreens != string(wantScreens) {
  		t.Errorf("screen transcript differs from the golden: %s", firstDiff(gotScreens, string(wantScreens)))
  	}
  }

  // A sanity check that does not depend on the golden files.
  func TestGoldenBoardTableShape(t *testing.T) {
  	boards := goldenBoardTable(t)
  	if len(boards) != 26 {
  		t.Fatalf("board table has %d entries, want 26", len(boards))
  	}
  	if boards[0].Path != "1" || boards[len(boards)-1].Path != "80/4" {
  		t.Fatalf("unexpected order: first=%q last=%q", boards[0].Path, boards[len(boards)-1].Path)
  	}
  	unread := 0
  	for _, b := range boards {
  		if b.Unread {
  			unread++
  		}
  		if b.Path == "10/1" && (b.Key != "1" || b.Parent != "10") {
  			t.Fatalf("10/1 key/parent = %q/%q, want 1/10", b.Key, b.Parent)
  		}
  	}
  	if unread != 5 {
  		t.Fatalf("%d boards are marked unread, want 5", unread)
  	}
  }
  ```

- [ ] **T002** 生成の前に、金型が決定的であることを確認する

  ```bash
  go -C apps/server test ./internal/hostprogram/erikak -run 'TestGoldenBoardTableShape' -count=1
  ```

  通らなければ、T001 のコードを直す（既存のコードは直さない）。
  `TestGoldenBoardTableShape` の「26 件」「5 件」が、現在のコードと合わない場合も、既存のコードではなく
  テストの数を実際の値に直し、その旨を報告する。

- [ ] **T003** 金型を生成してコミットする（**変更前のコードで**）

  ```bash
  go -C apps/server test ./internal/hostprogram/erikak -run 'TestScreensAndBoardsMatchGolden' -update-erikak-golden -count=1
  go -C apps/server test ./internal/hostprogram/erikak -count=1
  git status --short apps/server/internal/hostprogram/erikak
  ```

  - 生成された `testdata/boards_golden.json` と `testdata/screens_golden.txt` を、T001 のテストと一緒にコミットする。
  - 続けて、フラグなしで次を 3 回実行し、3 回とも通ること（出力が決定的であること）を確認する。
    通らなければ止めて報告する。

    ```bash
    go -C apps/server test ./internal/hostprogram/erikak -run 'TestScreensAndBoardsMatchGolden' -count=3
    ```

  - 画面の金型を目で確認する。`WELCOME TO HAKATA CANAL NET`、メインメニューの見出し
    （全角の `ＨＡＫＡＴＡ ＣＡＮＡＬ ＮＥＴ`）、全 26 板（隠し板を含む）の画面、終了時のあいさつが含まれていること。

**Checkpoint**: 変更前のコードに対する金型がコミットされ、通っている。ここから先、金型ファイルは変更しない。

---

## Phase 1: hostcatalog（定義の読み込み）

- [ ] **T004** `apps/server/internal/hostcatalog/program_detail.go`（新規）を作る

  ```go
  package hostcatalog

  import (
  	"errors"
  	"fmt"
  	"math"
  	"regexp"
  	"strings"
  )

  const erikaKProgramID = "erika-k"

  // ErikaKDetail is the station-specific data of a host that runs the Erika-K host
  // program: what the station's boards are and what it says at login. The program
  // itself (screens, commands, state machine) lives in the erikak package. The
  // slices are read-only: callers must not modify them.
  type ErikaKDetail struct {
  	Login  ErikaKLogin   `yaml:"login"`
  	Boards []ErikaKBoard `yaml:"boards"`
  }

  // ErikaKLogin is what the station says after login.
  type ErikaKLogin struct {
  	// StationMessage lines are shown boxed in the banner. Empty means no box.
  	StationMessage []string `yaml:"station_message"`
  	// MemberGreeting follows the banner; "{handle}" is replaced by the user's
  	// handle. Empty means no greeting.
  	MemberGreeting string `yaml:"member_greeting"`
  }

  // ErikaKBoard is one entry of the station's board tree. The tree is given in
  // display order. Key and parent are derived from Path ("10/1" is board 1 under
  // forum 10).
  type ErikaKBoard struct {
  	Path                 string  `yaml:"path"`
  	Alias                string  `yaml:"alias"`
  	Name                 string  `yaml:"name"`
  	Hidden               bool    `yaml:"hidden"`
  	Scope                string  `yaml:"scope"`
  	RootAuthorPolicy     string  `yaml:"root_author_policy"`
  	ActivityWeight       float64 `yaml:"activity_weight"`
  	ReplyRate            float64 `yaml:"reply_rate"`
  	RetainedRootCap      int     `yaml:"retained_root_cap"`
  	VerifiedReferentRate float64 `yaml:"verified_referent_rate"`
  	// Unread marks the board as having unread messages in the station's fixed
  	// prototype display, until real read state exists.
  	Unread bool `yaml:"unread"`
  }

  var (
  	boardPathPattern   = regexp.MustCompile(`^[0-9]+(/[0-9]+)*$`)
  	placeholderPattern = regexp.MustCompile(`\{[^}]*\}`)
  )

  // validateErikaKDetail checks the structure of an Erika-K detail. How wide a
  // line may be on screen is checked by the erikak package (ValidateDetail).
  func validateErikaKDetail(d *ErikaKDetail) error {
  	if d == nil {
  		return nil
  	}
  	var errs []error
  	add := func(field string, err error) {
  		if err != nil {
  			errs = append(errs, fmt.Errorf("%s: %w", field, err))
  		}
  	}

  	for i, line := range d.Login.StationMessage {
  		if strings.TrimSpace(line) == "" {
  			add(fmt.Sprintf("login.station_message[%d]", i), errors.New("must not be empty"))
  		}
  	}
  	for _, found := range placeholderPattern.FindAllString(d.Login.MemberGreeting, -1) {
  		if found != "{handle}" {
  			add("login.member_greeting", fmt.Errorf("unknown placeholder %s (only {handle} is supported)", found))
  		}
  	}

  	defined := make(map[string]bool, len(d.Boards))
  	for _, b := range d.Boards {
  		if boardPathPattern.MatchString(b.Path) {
  			defined[b.Path] = true
  		}
  	}
  	seen := make(map[string]bool, len(d.Boards))
  	for i, b := range d.Boards {
  		field := fmt.Sprintf("boards[%d]", i)
  		if strings.TrimSpace(b.Path) == "" {
  			add(field+".path", errors.New("is required"))
  			continue
  		}
  		if !boardPathPattern.MatchString(b.Path) {
  			add(field+".path", fmt.Errorf("%q must be numbers separated by \"/\" (for example \"10/1\")", b.Path))
  			continue
  		}
  		if seen[b.Path] {
  			add(field+".path", fmt.Errorf("%q is listed twice", b.Path))
  			continue
  		}
  		seen[b.Path] = true
  		if cut := strings.LastIndex(b.Path, "/"); cut >= 0 && !defined[b.Path[:cut]] {
  			add(field+".path", fmt.Errorf("the parent board %q of %q is not defined", b.Path[:cut], b.Path))
  		}
  		if strings.TrimSpace(b.Name) == "" {
  			add(field+".name", errors.New("is required"))
  		}
  		if b.RootAuthorPolicy != "" && b.RootAuthorPolicy != "sysop_only" {
  			add(field+".root_author_policy", fmt.Errorf("%q must be empty or \"sysop_only\"", b.RootAuthorPolicy))
  		}
  		if math.IsNaN(b.ActivityWeight) || b.ActivityWeight < 0 {
  			add(field+".activity_weight", fmt.Errorf("%v must not be negative", b.ActivityWeight))
  		}
  		if math.IsNaN(b.ReplyRate) || b.ReplyRate < 0 {
  			add(field+".reply_rate", fmt.Errorf("%v must not be negative", b.ReplyRate))
  		}
  		if b.RetainedRootCap < 0 {
  			add(field+".retained_root_cap", fmt.Errorf("%d must not be negative", b.RetainedRootCap))
  		}
  		if math.IsNaN(b.VerifiedReferentRate) || b.VerifiedReferentRate < 0 || b.VerifiedReferentRate > 1 {
  			add(field+".verified_referent_rate", fmt.Errorf("%v must be between 0 and 1", b.VerifiedReferentRate))
  		}
  	}
  	return errors.Join(errs...)
  }
  ```

- [ ] **T005** `apps/server/internal/hostcatalog/preset.go` に `detail.erika_k` を追加する

  1. `PresetDetail` 構造体に、`Welcome` の次の行として追加する。

     ```go
     // ErikaK is the Erika-K host program's station data; nil when the preset has none.
     ErikaK *ErikaKDetail `yaml:"erika_k"`
     ```

  2. `ParsePreset` の中で、`population` の検査の**直後**、`if len(errs) > 0 {` の**直前**に追加する。

     ```go
     if f.Detail.ErikaK != nil {
     	if h.Program != erikaKProgramID {
     		add("detail.erika_k", fmt.Errorf("is only valid for host.program %q", erikaKProgramID))
     	}
     	add("detail.erika_k", validateErikaKDetail(f.Detail.ErikaK))
     }
     ```

     （`h` は `ParsePreset` の中で `f.Host` を指す変数。`add` の使い方は、周囲のコードに合わせる。）

  3. `Preset.Detail` は、これまでどおり `PresetDetail` の値のまま（`ErikaK` が含まれる）。他の変更は不要。

- [ ] **T006** `apps/server/internal/hostcatalog/program_detail_test.go`（新規）にテストを書く

  既存の `preset_test.go` の `samplePreset` と、`ParsePreset` の呼び方を先に読み、同じ形式で書く。
  `samplePreset` の `host.program` が `erika-k` でない場合は、このテスト用に `erika-k` へ置き換える
  （`strings.Replace`）か、`erika-k` の preset を組み立てるヘルパーを作る。次を確認する。

  | テスト | 内容 |
  |---|---|
  | 正常系 | 有効な `detail.erika_k` が `Preset.Detail.ErikaK` に入る。板の `path`、`unread`、`hidden`、数値が読める |
  | 省略 | `detail.erika_k` が無ければ `Preset.Detail.ErikaK == nil` |
  | プログラム | `host.program` が `erika-k` 以外の preset に `detail.erika_k` があるとエラー |
  | `path` | 空、不正な形式（`"a"`、`"1//2"`、`"1/"`）、重複、親が無い（`"10/1"` だけ）でエラー |
  | `name` | 空でエラー |
  | 方針 | `root_author_policy: other` でエラー。`sysop_only` と省略は有効 |
  | 数値 | 負の `activity_weight`、`reply_rate`、`retained_root_cap`、範囲外の `verified_referent_rate` でエラー |
  | ログイン | `station_message` に空の行、`member_greeting` に `{name}` のような未知の置換でエラー。`{handle}` は有効 |
  | 複数のエラー | 複数の違反が、1 回のエラーにまとまって報告される |
  | キーの typo | `detail.erika_k.boards[]` の下の未知のキー（例 `activity_wieght`）がエラー |
  | 親が後ろにある | 子が親より前に書かれていても、親が存在すれば有効 |

- [ ] **T007** `apps/server/internal/hostcatalog/presets/hakata-canal-net.yaml` に `detail.erika_k` を追加する

  1. `revision: 3` を `revision: 4` にする。
  2. `plan.md` の「スキーマ」と「板の項目と、現在の Go の項目の対応」に従い、ファイルの末尾に
     `detail:` ブロックを追加する。`detail:` が既にあれば、その下に `erika_k:` を追加する。
  3. `login.station_message`、`login.member_greeting` を、`erikak/runtime.go` の `finishLogin` の文字列から
     **そのまま**写す（`%s` は `{handle}` にする）。
  4. `boards` に、`boardTree` の**全 26 件を現在の順序のまま**写す。
     - 文字列は、すべて二重引用符で囲む。
     - ゼロ値の項目（`0`、空、`false`）は書かない。`Key` と `Parent` は書かない。
     - `unreadBoard` の対象（`1`、`4`、`10/2`、`60/1`、`60/3`）に `unread: true` を付ける。
     - 数値は、値が変わらないように写す（`.10` → `0.10`、`1.25` → `1.25`）。
     - `boardTree` の上の 2 つのコメントを、YAML のコメントとして、対応する板の近くに移す。
  5. 冒頭のコメントに、「Erika-K の板構成とログイン文言は `detail.erika_k` にある」と 1 行追記する。

- [ ] **T008** 確認

  ```bash
  go -C apps/server build ./... && go -C apps/server test ./internal/hostcatalog/... ./internal/hostprogram/erikak/... -count=1
  ```

  この時点では、`erikak` はまだ古い板の表を使っているので、**金型テストは通る**はず。
  通らなければ、T004〜T007 のどこかが `erikak` の挙動に影響している。止めて報告する。

**Checkpoint**: preset が Erika-K の局の定義を読める。`erikak` の挙動は変わっていない。

---

## Phase 2: world / worldrepo（定義の受け渡し）

- [ ] **T009** `apps/server/internal/world` に `HostDetailStore` を追加する

  1. `store.go` の、`PersonaFactStore` などの任意機能のインターフェースの近くに追加する。
     `store.go` が `hostcatalog` を import していなければ追加する。

     ```go
     // HostDetailStore exposes the immutable, preset-defined detail of a host:
     // program-specific data such as an Erika-K station's boards and login
     // messages. The detail is part of the host definition, so it is never
     // changed at runtime and is not stored in snapshots. The returned value is
     // shared and read-only; callers copy what they keep.
     type HostDetailStore interface {
     	HostDetail(hostID string) (hostcatalog.PresetDetail, bool)
     }
     ```

  2. `MemoryStore` 構造体に、`populations` の次の行として追加する。

     ```go
     details map[string]hostcatalog.PresetDetail
     ```

  3. `NewMemoryStore` の初期化に `details: map[string]hostcatalog.PresetDetail{},` を追加し、
     `presetPopulations()` を使っているループの前後で、次のとおりに詳細を登録する。

     ```go
     	for key, detail := range presetDetails() {
     		s.details[key] = detail
     	}
     ```

  4. `MemoryStore` にメソッドを追加する（`store.go` か、新しいファイル `host_detail.go`）。

     ```go
     // HostDetail returns the preset-defined detail of the host with the given ID.
     func (s *MemoryStore) HostDetail(hostID string) (hostcatalog.PresetDetail, bool) {
     	s.mu.RLock()
     	defer s.mu.RUnlock()
     	d, ok := s.details[hostID]
     	return d, ok
     }
     ```

  5. `preset_hosts.go` の `presetData` に `details map[string]hostcatalog.PresetDetail` を追加する。
     `loadPresetData` の中で、`presets` のすべての要素について `details[p.Key] = p.Detail` を設定する。
     `presetDetails()` を、`presetPopulations()` と同じ形（マップのコピーを返す）で追加する。

- [ ] **T010** `apps/server/internal/worldrepo/repository.go` に `HostDetail` を追加し、テストを書く

  ```go
  // HostDetail returns the preset-defined detail of a host from the underlying
  // store, so host programs can read it through the repository.
  func (r *Repository) HostDetail(hostID string) (hostcatalog.PresetDetail, bool) {
  	if p, ok := r.Base.(world.HostDetailStore); ok {
  		return p.HostDetail(hostID)
  	}
  	return hostcatalog.PresetDetail{}, false
  }
  ```

  - `repository.go` が `hostcatalog` を import していなければ追加する。
  - テスト（`worldrepo` の既存のテストファイルの形式に合わせて、新しいテストファイルに書く）:
    `world.NewMemoryStore()` を `Base` にした `Repository` が、HAKATA の詳細（`ErikaK != nil`、板が 26 件）を返すこと。
    詳細を持たないストア（`world.Store` だけを満たす最小の型）を `Base` にすると、`ok == false` になること。
  - `world` 側にも、`MemoryStore.HostDetail` のテスト（`world/host_detail_test.go`、新規）を書く:
    HAKATA は詳細を持ち、`busy-test` は持たない（`ok == false`）。

---

## Phase 3: erikak（板のカタログと局名の導出）

T011〜T017 は 1 つのコミットにまとめる（途中でビルドが通らないため）。

- [ ] **T011** `erikak/catalog.go`（新規）を作る

  `runtime.go` の `boardNode` 構造体の定義を、ここに移す。次を追加する。

  ```go
  package erikak

  import (
  	"strings"

  	"zutto-pccom/apps/server/internal/hostcatalog"
  )

  // boardCatalog is one station's board tree and unread marks, built from its
  // preset detail. The zero value is an empty catalog.
  type boardCatalog struct {
  	nodes  []boardNode
  	unread map[string]bool
  }

  func newBoardCatalog(boards []hostcatalog.ErikaKBoard) boardCatalog {
  	c := boardCatalog{nodes: make([]boardNode, 0, len(boards)), unread: map[string]bool{}}
  	for _, b := range boards {
  		path := normalizePath(b.Path)
  		key := path
  		if cut := strings.LastIndex(path, "/"); cut >= 0 {
  			key = path[cut+1:]
  		}
  		c.nodes = append(c.nodes, boardNode{
  			Path: path, Key: key, Alias: b.Alias, Parent: parentPath(path), Name: b.Name,
  			Hidden: b.Hidden, SemanticScope: b.Scope, RootAuthorPolicy: b.RootAuthorPolicy,
  			ActivityWeight: b.ActivityWeight, ReplyRate: b.ReplyRate,
  			RetainedRootCap: b.RetainedRootCap, VerifiedReferentRate: b.VerifiedReferentRate,
  		})
  		if b.Unread {
  			c.unread[path] = true
  		}
  	}
  	return c
  }

  func (c boardCatalog) find(path string) (boardNode, bool) {
  	path = normalizePath(path)
  	for _, node := range c.nodes {
  		if node.Path == path {
  			return node, true
  		}
  	}
  	return boardNode{}, false
  }

  func (c boardCatalog) visibleChildren(parent string) []boardNode {
  	out := make([]boardNode, 0)
  	for _, node := range c.nodes {
  		if node.Parent == parent && !node.Hidden {
  			out = append(out, node)
  		}
  	}
  	return out
  }

  func (c boardCatalog) child(parent, key string) (string, bool) {
  	for _, node := range c.nodes {
  		if node.Parent == parent && node.Key == strings.TrimSpace(key) {
  			return node.Path, true
  		}
  	}
  	return "", false
  }

  func (c boardCatalog) isForum(path string) bool {
  	for _, node := range c.nodes {
  		if node.Parent == path {
  			return true
  		}
  	}
  	return false
  }
  ```

  - `normalizePath` と `parentPath` は `runtime.go` の既存の関数を使う（移さない）。
  - 元の `findNode`、`visibleChildren`、`childSelection`、`isForum` と、**同じ結果**になること
    （検索の順序、`Hidden` の扱い、`Key` の比較）。

- [ ] **T012** `erikak/detail.go`（新規）を作る

  ```go
  package erikak

  import (
  	"fmt"
  	"strings"

  	"zutto-pccom/apps/server/internal/hostcatalog"
  	"zutto-pccom/apps/server/internal/world"
  )

  // detailFor returns the Erika-K detail of host from store, or an empty detail
  // when the store has none: the runtime then has no boards and says nothing
  // station-specific at login.
  func detailFor(host world.Host, store world.Store) hostcatalog.ErikaKDetail {
  	if p, ok := store.(world.HostDetailStore); ok {
  		if d, ok := p.HostDetail(host.ID); ok && d.ErikaK != nil {
  			return *d.ErikaK
  		}
  	}
  	return hostcatalog.ErikaKDetail{}
  }

  // fullWidthASCII converts ASCII 0x21-0x7E to the full-width forms used in
  // Erika-K's menu title. Other characters, including spaces, are unchanged.
  func fullWidthASCII(s string) string {
  	var b strings.Builder
  	for _, r := range s {
  		if r >= 0x21 && r <= 0x7E {
  			r += 0xFEE0
  		}
  		b.WriteRune(r)
  	}
  	return b.String()
  }

  // ValidateDetail checks the parts of a station's detail that depend on how this
  // program lays out its screens. hostcatalog validates the structure; the width of
  // a boxed line cannot be checked there because it needs the program's metrics.
  func ValidateDetail(d hostcatalog.ErikaKDetail) error {
  	inner := boxedInnerWidth(80)
  	var errs []string
  	for i, line := range d.Login.StationMessage {
  		if w := displayCellWidth(line); w > inner {
  			errs = append(errs, fmt.Sprintf("login.station_message[%d] is %d cells wide, the box holds %d", i, w, inner))
  		}
  	}
  	if len(errs) > 0 {
  		return fmt.Errorf("erika-k detail: %s", strings.Join(errs, "; "))
  	}
  	return nil
  }
  ```

  - 空白の扱い（`fullWidthASCII`）は、**金型と一致するかで決める**。T003 の画面の金型で、現在の見出しの区切りが
    半角空白か全角空白かを確認し、全角空白（U+3000）なら、空白も U+3000 に変換する。金型は変更しない。

- [ ] **T013** `erikak/runtime.go` を変更する

  1. `boardNode` の定義と、`boardTree`、`unreadBoard` の変数を削除する（`boardNode` は T011 で移した）。
     `boardTree` の上のコメントも削除する（YAML に移した）。
  2. `Runtime` 構造体に追加する。

     ```go
     detail  hostcatalog.ErikaKDetail
     catalog boardCatalog
     ```

     `hostcatalog` を import する。
  3. `NewWithConfig` を、次の形にする（`New` はこれを呼ぶままで変更しない）。

     ```go
     func NewWithConfig(host world.Host, store world.Store, cfg Config) *Runtime {
     	d := detailFor(host, store)
     	return &Runtime{Host: host, Store: store, Config: cfg, state: "login_id", handle: "GUEST",
     		detail: d, catalog: newBoardCatalog(d.Boards)}
     }
     ```

  4. 板の表を使っているすべての箇所を、`r.catalog` に置き換える。

     | 元 | 新 |
     |---|---|
     | `findNode(path)`（関数） | `r.catalog.find(path)` |
     | `visibleChildren(parent)`（関数） | `r.catalog.visibleChildren(parent)` |
     | `r.childSelection(key)` の本体 | `r.catalog.child(r.boardPath, key)` を呼ぶ |
     | `r.isForum(path)` の本体 | `r.catalog.isForum(path)` を呼ぶ |
     | `for _, node := range boardTree`（`planBoardActivity`、`renderBoardMap`、`renderUnreadSummary`） | `for _, node := range r.catalog.nodes` |
     | `unreadBoard[path]` | `r.catalog.unread[path]` |
     | `cachedBoardPosts` の中の `findNode` | `r.catalog.find` |

     元の関数（`findNode`、`visibleChildren`）は削除する。**表示の順序と、`Hidden` の扱いを変えない。**
  5. `BoardByPath` を、板の定義を引数に取る形にする。

     ```go
     // BoardByPath resolves a board of the given station definition for shared
     // debug/observation tooling. Hidden boards are not exposed.
     func BoardByPath(boards []hostcatalog.ErikaKBoard, path string) (world.Board, bool) {
     	node, ok := newBoardCatalog(boards).find(strings.TrimSpace(path))
     	if !ok || node.Hidden {
     		return world.Board{}, false
     	}
     	return worldBoard(node), true
     }
     ```

  6. `boxedLine` の `prefix`、`suffix` の定数を、パッケージレベルに出し、内側の幅を返す関数を作る
     （`ValidateDetail` が使う。値は変えない）。

     ```go
     const (
     	boxedLinePrefix = "■  "
     	boxedLineSuffix = "■"
     )

     func boxedInnerWidth(width int) int {
     	return width - displayCellWidth(boxedLinePrefix) - displayCellWidth(boxedLineSuffix)
     }
     ```

     `boxedLine` は、これらを使うように直す（出力は同じ）。

- [ ] **T014** `erikak/runtime.go` の局名とログイン文言を、局の定義と `Host` から作る

  1. `finishLogin` の、ログイン後のバナーを次の形にする（出力は、HAKATA では現在と**同一**）。

     ```go
     	var b strings.Builder
     	fmt.Fprintf(&b, "\r\n前回アクセス %s\r\n\r\n", last)
     	b.WriteString(decorativeLine("WELCOME TO "+strings.ToUpper(r.Host.Name), "#", 80) + "\r\n")
     	if len(r.detail.Login.StationMessage) > 0 {
     		b.WriteString(doubleCellRule("■", 80) + "\r\n")
     		for _, line := range r.detail.Login.StationMessage {
     			b.WriteString(boxedLine(line, 80) + "\r\n")
     		}
     		b.WriteString(doubleCellRule("■", 80) + "\r\n")
     	}
     	b.WriteString(decorativeLine("ERIKA-K", "#", 80) + "\r\n")
     	if greeting := r.detail.Login.MemberGreeting; greeting != "" {
     		b.WriteString("\r\n" + strings.ReplaceAll(greeting, "{handle}", r.handle) + "\r\n")
     	}
     	b.WriteString(r.renderMainMenu())
     	return b.String()
     ```

     `last` の決め方（`96/08/25 23:41` を含む）は変更しない。
  2. `renderMainMenu` の見出しの行を、次のとおりに導出する。

     ```go
     "\r\n-" + fullWidthASCII(r.Host.Name) + "-  〖Ｍain Ｍenu〗  " + fullWidthASCII(r.Host.Software) + "\r\n"
     ```

     見出しより下の行は変更しない。
  3. `handleMain` の `case "9", "BYE", "QUIT", "GOODBYE":` の戻り値を、次にする。

     ```go
     "\r\nご利用ありがとうございました。\r\nまた " + r.Host.Name + " でお会いしましょう。\r\n"
     ```

  4. 修正後、`runtime.go` に `HAKATA` や `ＨＡＫＡＴＡ` が残っていないこと。

- [ ] **T015** `apps/server/cmd/server/main.go` の `BoardByPath` の呼び出しを直す

  デバッグ用サンプルの、`erikak.BoardByPath(boardID)` を呼んでいる箇所を、次の形にする。

  ```go
  		if board.ID == "" && host.SoftwareID == "erika-k" {
  			if detail, ok := runtimeStore.HostDetail(host.ID); ok && detail.ErikaK != nil {
  				if resolved, ok := erikak.BoardByPath(detail.ErikaK.Boards, boardID); ok {
  					board = resolved
  				}
  			}
  		}
  ```

- [ ] **T016** `erikak` の既存テストを更新する

  1. `runtime_test.go` に、テスト用ヘルパーを追加する。

     ```go
     // sampleDetail returns the Erika-K detail of the sample station (HAKATA) from
     // its preset.
     func sampleDetail(t *testing.T) hostcatalog.ErikaKDetail {
     	t.Helper()
     	d, ok := world.NewMemoryStore().HostDetail("hakata-canal-net")
     	if !ok || d.ErikaK == nil {
     		t.Fatal("the sample station has no Erika-K detail")
     	}
     	return *d.ErikaK
     }
     ```

  2. `BoardByPath(...)` を直接呼んでいるテストを、`BoardByPath(sampleDetail(t).Boards, ...)` に直す
     （`TestBoardByPathResolvesCanonicalLeaf`、`TestExistingPC98HeadersDoNotLaunchMoreGenerationOnIndexReturn`、
     `TestHakataNoticeBoardIsReadOnly…`、`TestHakataBoardPurposesDoNotLeakHistoricalUncertainty`）。
     テストの検査内容は変えない。テスト名の `Hakata` は、局に依存しない名前にする
     （`TestNoticeBoardIsReadOnly…`、`TestBoardPurposesDoNotLeakHistoricalUncertainty`）。
  3. `width_test.go` の `TestLoginBannerUsesExactDisplayCells` は、`&Runtime{handle: "GUEST"}` では
     局の定義が無いので、`sampleRuntime(t)` で作った `Runtime` の `handle` を `"GUEST"` にして、
     `finishLogin()` を呼ぶ形に直す。検査内容は変えない。
  4. `golden_test.go` の `goldenBoardTable` の本体**だけ**を、新しいカタログから作る形に直す。
     **このファイルの他の部分と、`testdata/*` は変更しない。**

     ```go
     func goldenBoardTable(t *testing.T) []goldenBoard {
     	t.Helper()
     	catalog := newBoardCatalog(sampleDetail(t).Boards)
     	out := make([]goldenBoard, 0, len(catalog.nodes))
     	for _, n := range catalog.nodes {
     		out = append(out, goldenBoard{
     			Path: n.Path, Key: n.Key, Alias: n.Alias, Parent: n.Parent, Name: n.Name,
     			Hidden: n.Hidden, Scope: n.SemanticScope, RootAuthorPolicy: n.RootAuthorPolicy,
     			ActivityWeight: n.ActivityWeight, ReplyRate: n.ReplyRate,
     			RetainedRootCap: n.RetainedRootCap, VerifiedReferentRate: n.VerifiedReferentRate,
     			Unread: catalog.unread[n.Path],
     		})
     	}
     	return out
     }
     ```

  5. 新しいテスト（`erikak/detail_test.go`、新規）を追加する。

     | テスト | 内容 |
     |---|---|
     | 別の局 | `*world.MemoryStore` を埋め込み、`HostDetail` を上書きして別の板構成を返すテスト用ストアで `Runtime` を作ると、その板だけがメニューに出る。HAKATA の板（例 `博多・天神広場`）は出ない |
     | 詳細なし | `HostDetail` が `ok == false` を返すストアで `Runtime` を作っても panic せず、ログインでき、`BM` で板が 0 件。局のメッセージとあいさつが出ない |
     | ゼロ値 | `&Runtime{}` で `renderMainMenu()`、`renderBoardMap()`、`renderUnreadSummary()` が panic しない |
     | 局名の導出 | `Host{Name: "SAMPLE BBS", Software: "テストＫ版"}` の `Runtime` で、ログインの出力に `WELCOME TO SAMPLE BBS`、メインメニューに `ＳＡＭＰＬＥ ＢＢＳ`（区切りの空白は金型と同じ扱い）、`9` の出力に `また SAMPLE BBS でお会いしましょう。` が含まれる |
     | 局のメッセージ | `station_message` を 1 行にすると枠の行が 1 行になる。空にすると枠（`■` の罫線）が出ない。`member_greeting` の `{handle}` がハンドルに置き換わる。空にするとあいさつの行が出ない |
     | `fullWidthASCII` | `"ABC xyz-09"` → 全角化（空白は金型と同じ扱い）、日本語は変わらない、空文字列は空 |
     | `ValidateDetail` | 幅が 74 セル以下の行は有効。75 セル以上の行はエラー（全角 38 文字の行など） |
     | 埋め込み preset | `hostcatalog.LoadPresets` で読んだ、`detail.erika_k` を持つすべての preset が、`ValidateDetail` を通る |
     | `BoardByPath` | 隠し板（`99`）と存在しない板は `false`。`70/1` は解決できる |

- [ ] **T017** 確認とコミット

  ```bash
  go -C apps/server build ./...
  go -C apps/server vet ./...
  go -C apps/server test ./internal/hostprogram/... ./internal/hostcatalog/... ./internal/world/... ./internal/worldrepo/... -count=1
  git diff --stat <T003 のコミット>..HEAD -- apps/server/internal/hostprogram/erikak/testdata
  grep -rniE "hakata" apps/server/internal/hostprogram --include=*.go | grep -v _test.go
  ```

  - **`TestScreensAndBoardsMatchGolden` が通ること。** 通らなければ、`firstDiff` の出力を手がかりに、
    実装（特に `fullWidthASCII` の空白の扱い、YAML への移し間違い、板の順序）を直す。金型は変えない。
  - 金型ファイルの `git diff --stat` が空であること。
  - 最後の `grep` の結果が空であること。
  - T011〜T017 をまとめた 1 つのコミットを作る。

---

## Phase 4: ドキュメントと PR

- [ ] **T018** `apps/server/internal/hostcatalog/README.md` を更新する

  1. 「Preset files」のスキーマの例に、`detail.erika_k`（`login` と `boards` の一部）を追加する。
  2. 「Erika-K detail」の節を新設し、次を説明する: `host.program: erika-k` の局だけが書けること、
     板は表示順に並べて `path` で階層を表すこと（`key` と `parent` は導出）、
     `login.station_message` と `member_greeting`（`{handle}`）、局名・ソフト名から導出される画面の文字列
     （メインメニューの見出し、ログイン見出し、終了のあいさつ）は書かなくてよいこと、
     検証規則の要約（`plan.md` の表）、プロトタイプ用の固定画面はまだコードにあること。
  3. 「Changing presets safely」に、「`detail.erika_k` の板の順序と `path` を変えると、局の画面が変わる」
     という注意を追記する。
  4. 冒頭の「Status」の、「Not driven by these files yet」の記述から、
     Erika-K の板構成とログイン文言を除く（残りの項目は変えない）。

- [ ] **T019** 全体の確認と記録

  ```bash
  go -C apps/server build ./...
  go -C apps/server vet ./...
  go -C apps/server test ./... -count=1
  go -C apps/server test ./internal/hostprogram/erikak -run 'TestScreensAndBoardsMatchGolden|TestGoldenBoardTableShape' -count=3 -v
  git diff --stat main...HEAD
  ```

  結果を `specs/005-erika-k-station-detail/verification.md`（新規）に、次の形式で記録する。

  ```markdown
  # Verification: Erika-K station detail

  | コマンド | 結果 | 備考 |
  |---|---|---|
  | `go -C apps/server build ./...` | 成功 / 失敗 / 未実施 | |
  | `go -C apps/server vet ./...` | 〃 | |
  | `go -C apps/server test ./...` | 〃 | |
  | 金型テスト（3 回連続） | 〃 | 板の表と画面の出力 |

  ## 金型ファイルが変わっていないこと
  （`git diff <T003 のコミット>..HEAD -- apps/server/internal/hostprogram/erikak/testdata` の結果。空であること）

  ## 残った hakata を含むファイル（hostprogram のテスト以外）
  （grep の結果。空であること）

  ## 手動確認
  - サーバを起動して HAKATA に接続し、ログイン画面、メインメニュー、板の一覧が従来どおり表示される: 確認済み / 未実施
  ```

- [ ] **T020** PR を作る

  - `main` への PR。**draft** で作成する。
  - タイトル: `Erika-K の板構成とログイン文言を局の定義に外部化する（HAKATA の画面は不変）`
  - 説明に含める: 目的、変更の要約、**HAKATA の画面が変わっていないことの根拠（板の表と画面出力の金型。
    金型は変更前に生成し、以降変更していない）**、`verification.md` の結果（未実施を含めて正直に）、
    スコープ外の項目（プロトタイプ用の固定画面、`Config`、前回アクセスの日時）。

---

## 実行順序

```text
Phase 0 (T001 → T002 → T003)
  └─ Phase 1 (T004 → T005 → T006, T007 → T008)
       └─ Phase 2 (T009 → T010)
            └─ Phase 3 (T011〜T017 を 1 コミット)
                 └─ Phase 4 (T018 → T019 → T020)
```

## 完了の定義

- `spec.md` の FR-001〜FR-010 と SC-001〜SC-004 を満たす（手動確認が未実施なら、その旨を明記する）。
- 金型テストが、変更前に生成した金型ファイルを変更せずに通る。
- 変更が `plan.md` の「変更するファイル」の表の範囲に収まっている。
