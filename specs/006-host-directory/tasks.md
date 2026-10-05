# Tasks: host directory

**Input**: `specs/006-host-directory/` の `spec.md`、`plan.md`
**Prerequisites**: `AGENTS.md`（特に「Implementing a spec」）、`plan.md`（特に「2 つの PR に分ける」）

この spec は **2 つの PR** に分けて実装する。**依頼ごとに、Part 1 か Part 2 のどちらか 1 つだけ**を実装する。

| Part | 内容 | タスク | 前提 |
|---|---|---|---|
| **Part 1（サーバ）** | `GET /api/directory` の追加 | T001〜T009 | なし |
| **Part 2（Web）** | Web を `/api/directory` に切り替え、旧コードを削除 | T010〜T018 | Part 1 がマージされ、Render にデプロイされ、本番で `/api/directory` が応答していること |

## この作業のルール（必読）

- 作業ブランチから `main` への PR を 1 本作る。`main` に直接コミットしない。
- **成果物はコードの変更（実装）。** spec のファイルを作る・変更することは、この作業の仕事ではない
  （この spec のフォルダに追加してよいのは、`verification.md` だけ）。
- **契約テストの期待値（HAKATA の要素の値）を、実装に合わせて書き換えない。** 実装が合わなければ、実装を直す。
  期待値は `spec.md` の SC-002 に書かれている。
- スコープ外（`spec.md` 末尾）に手を出さない。特に、`/api/world/bootstrap`、`worldcatalog`、`VITE_TELEHODAI_NUMBERS`、
  ターミナル・モードの案内文の `ATDT0920000196` は変更しない。
- 既存ファイルに整形だけの差分を作らない。`gofmt -w` を既存ファイル全体にかけない。
- 1 タスク = 1 コミット（メッセージは `T0XX: 内容`）。ただし、途中でビルドが通らないタスクは、1 つのコミットにまとめる。
- 実行していないコマンドを「成功した」と書かない。実行できなかったものは「未実施」と理由を書く。
  Agent の PR は、ワークフローが自動で動かないことがある。CI の結果が無いものを、成功とみなさない。
- この作業で `.github/` 配下のファイルは変更しない。
- spec と実際のコードが食い違っていたら、推測で進めず、PR の説明に書いて止まる。

---

# Part 1: サーバ（`GET /api/directory`）

## Phase 1: world と worldrepo

- [ ] **T001** `apps/server/internal/world/directory.go`（新規）を作る

  ```go
  package world

  // DirectoryEntry is what the dialing directory shows about one host. It is
  // derived from the host definition, so it is as immutable as the definition.
  type DirectoryEntry struct {
  	ID       string `json:"id"`
  	Name     string `json:"name"`
  	Software string `json:"software,omitempty"`
  	Phone    string `json:"phone"`
  	DialMode string `json:"dialMode"`
  	MaxBaud  int    `json:"maxBaud"`
  }

  // HostDirectoryStore lists the hosts shown in the dialing directory: the hosts
  // whose preset says listed: true. A host that is not listed can still be dialed.
  type HostDirectoryStore interface {
  	ListedHosts() []DirectoryEntry
  }

  // ListedHosts returns the directory entries in preset key order. The returned
  // slice is a copy.
  func (s *MemoryStore) ListedHosts() []DirectoryEntry {
  	s.mu.RLock()
  	defer s.mu.RUnlock()
  	return append([]DirectoryEntry(nil), s.directory...)
  }
  ```

- [ ] **T002** `apps/server/internal/world/preset_hosts.go` に、電話帳の一覧を追加する

  1. `presetData` 構造体に追加する。

     ```go
     directory []DirectoryEntry
     ```

  2. `loadPresetData` の中で、`descriptors` から `hosts` を作るループの**直後**に追加する。

     ```go
     		// The dialing directory lists the hosts whose preset says listed: true, in
     		// the order the presets are loaded (by key). Hosts that are not listed can
     		// still be dialed; they are just not shown.
     		for _, d := range descriptors {
     			if !d.Listed {
     				continue
     			}
     			presetDataVal.directory = append(presetDataVal.directory, DirectoryEntry{
     				ID: d.Key, Name: d.Name, Software: d.SoftwareLabel,
     				Phone: d.Phone, DialMode: d.DialMode, MaxBaud: d.MaxBaud,
     			})
     		}
     ```

  3. `presetDetails()` の近くに、アクセサを追加する。

     ```go
     func presetDirectory() []DirectoryEntry {
     	return append([]DirectoryEntry(nil), loadPresetData().directory...)
     }
     ```

- [ ] **T003** `apps/server/internal/world/store.go` に、一覧を持たせる

  1. `MemoryStore` 構造体の `details` の次の行に追加する。

     ```go
     // directory is the dialing directory: the listed hosts, from the presets.
     directory []DirectoryEntry
     ```

  2. `NewMemoryStore` の、`details` を登録するループの**直後**に追加する。

     ```go
     	s.directory = presetDirectory()
     ```

- [ ] **T004** `apps/server/internal/worldrepo/host_directory.go`（新規）を作る

  ```go
  package worldrepo

  import "zutto-pccom/apps/server/internal/world"

  // ListedHosts returns the dialing directory of the underlying store, so the
  // HTTP layer can read it through the repository. A store without the capability
  // has an empty directory.
  func (r *Repository) ListedHosts() []world.DirectoryEntry {
  	if p, ok := r.Base.(world.HostDirectoryStore); ok {
  		return p.ListedHosts()
  	}
  	return nil
  }
  ```

- [ ] **T005** `world` と `worldrepo` のテストを書く

  1. `apps/server/internal/world/directory_test.go`（新規）:

     ```go
     package world

     import "testing"

     // The contract values of the sample station. They are the values the web client
     // used to hard-code as DEFAULT_CENTERS, so a change here is a visible change to
     // the directory players see.
     func TestListedHostsIsTheSampleStation(t *testing.T) {
     	got := NewMemoryStore().ListedHosts()
     	want := DirectoryEntry{
     		ID: "hakata-canal-net", Name: "HAKATA CANAL NET", Software: "絵理香K版",
     		Phone: "0920000196", DialMode: "tone", MaxBaud: 14400,
     	}
     	if len(got) != 1 || got[0] != want {
     		t.Fatalf("directory = %+v, want exactly [%+v]", got, want)
     	}
     }

     // Listed and dialable are separate: a host that is not listed (busy-test) is
     // missing from the directory but can still be reached by number.
     func TestUnlistedHostsAreNotInTheDirectoryButCanBeDialed(t *testing.T) {
     	s := NewMemoryStore()
     	for _, e := range s.ListedHosts() {
     		if e.ID == "busy-test" || e.Phone == "0459999999" {
     			t.Fatalf("an unlisted host is in the directory: %+v", e)
     		}
     	}
     	if h, err := s.HostByPhone("0459999999"); err != nil || h.ID != "busy-test" {
     		t.Fatalf("the unlisted host cannot be reached by number: %v %+v", err, h)
     	}
     }

     // The directory comes from the presets. Registering a host later (as tests do)
     // does not put it in the directory.
     func TestRegisteringAHostDoesNotChangeTheDirectory(t *testing.T) {
     	s := NewMemoryStore()
     	s.SaveHost(Host{ID: "added-later", Phone: "0450000097", Name: "ADDED LATER"})
     	if n := len(s.ListedHosts()); n != 1 {
     		t.Fatalf("directory has %d entries, want 1", n)
     	}
     }

     func TestListedHostsReturnsACopy(t *testing.T) {
     	s := NewMemoryStore()
     	first := s.ListedHosts()
     	first[0].Name = "CHANGED"
     	if got := s.ListedHosts()[0].Name; got != "HAKATA CANAL NET" {
     		t.Fatalf("a caller modified the store's directory: %q", got)
     	}
     }

     func TestMemoryStoreIsAHostDirectoryStore(t *testing.T) {
     	var _ HostDirectoryStore = NewMemoryStore()
     }
     ```

  2. `apps/server/internal/worldrepo/host_directory_test.go`（新規）:

     ```go
     package worldrepo

     import (
     	"testing"

     	"zutto-pccom/apps/server/internal/world"
     )

     func TestRepositoryServesTheDirectoryOfItsBaseStore(t *testing.T) {
     	r := &Repository{Base: world.NewMemoryStore()}
     	got := r.ListedHosts()
     	if len(got) != 1 || got[0].ID != "hakata-canal-net" {
     		t.Fatalf("directory = %+v", got)
     	}
     }

     func TestRepositoryHasAnEmptyDirectoryWhenItsBaseStoreHasNone(t *testing.T) {
     	r := &Repository{Base: storeWithoutDetail{world.NewMemoryStore()}}
     	if got := r.ListedHosts(); len(got) != 0 {
     		t.Fatalf("a base store without the capability yielded %+v", got)
     	}
     }

     func TestRepositoryIsAHostDirectoryStore(t *testing.T) {
     	var _ world.HostDirectoryStore = &Repository{}
     }
     ```

     （`storeWithoutDetail` は、`worldrepo/host_detail_test.go` に既にあるテスト用の型。
     `world.Store` だけを見せる包み。別の名前で重複して定義しない。）

- [ ] **T006** 確認

  ```bash
  go -C apps/server build ./... && go -C apps/server test ./internal/world/... ./internal/worldrepo/... -count=1
  ```

## Phase 2: HTTP

- [ ] **T007** `apps/server/cmd/server/directory_handler.go`（新規）を作る

  ```go
  package main

  import (
  	"encoding/json"
  	"net/http"

  	"zutto-pccom/apps/server/internal/world"
  )

  // directoryLister is the part of the store the directory handler needs.
  type directoryLister interface {
  	ListedHosts() []world.DirectoryEntry
  }

  // newDirectoryHandler serves the dialing directory: the hosts whose preset says
  // listed: true. The directory is part of the immutable host definitions, so it
  // needs no authentication and nothing about the caller.
  func newDirectoryHandler(dir directoryLister) http.HandlerFunc {
  	return func(w http.ResponseWriter, r *http.Request) {
  		if r.Method != http.MethodGet && r.Method != http.MethodHead {
  			w.Header().Set("Allow", "GET, HEAD")
  			w.WriteHeader(http.StatusMethodNotAllowed)
  			return
  		}
  		centers := dir.ListedHosts()
  		if centers == nil {
  			centers = []world.DirectoryEntry{}
  		}
  		w.Header().Set("Content-Type", "application/json")
  		w.Header().Set("Cache-Control", "no-cache")
  		if r.Method == http.MethodHead {
  			return
  		}
  		_ = json.NewEncoder(w).Encode(map[string][]world.DirectoryEntry{"centers": centers})
  	}
  }
  ```

- [ ] **T008** `apps/server/cmd/server/main.go` にルートを登録し、テストを書く

  1. `mux.HandleFunc("/api/centers", bootstrapWorld)` の**次の行**に追加する。

     ```go
     	mux.HandleFunc("/api/directory", newDirectoryHandler(runtimeStore))
     ```

     （既存の `/api/world/bootstrap` と `/api/centers` は変更しない。）
  2. `apps/server/cmd/server/directory_handler_test.go`（新規）:

     ```go
     package main

     import (
     	"encoding/json"
     	"net/http"
     	"net/http/httptest"
     	"strings"
     	"testing"

     	"zutto-pccom/apps/server/internal/world"
     )

     type fakeDirectory []world.DirectoryEntry

     func (d fakeDirectory) ListedHosts() []world.DirectoryEntry { return d }

     func TestDirectoryHandlerServesTheSampleStationExactly(t *testing.T) {
     	rec := httptest.NewRecorder()
     	newDirectoryHandler(world.NewMemoryStore())(rec, httptest.NewRequest(http.MethodGet, "/api/directory", nil))

     	if rec.Code != http.StatusOK {
     		t.Fatalf("status = %d", rec.Code)
     	}
     	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
     		t.Fatalf("content type = %q", ct)
     	}
     	if cc := rec.Header().Get("Cache-Control"); cc != "no-cache" {
     		t.Fatalf("cache control = %q", cc)
     	}
     	// The exact body: these are the values the web client used to hard-code.
     	want := `{"centers":[{"id":"hakata-canal-net","name":"HAKATA CANAL NET","software":"絵理香K版","phone":"0920000196","dialMode":"tone","maxBaud":14400}]}`
     	if got := strings.TrimSpace(rec.Body.String()); got != want {
     		t.Fatalf("body =\n%s\nwant\n%s", got, want)
     	}
     }

     func TestDirectoryHandlerNeverReturnsNull(t *testing.T) {
     	rec := httptest.NewRecorder()
     	newDirectoryHandler(fakeDirectory(nil))(rec, httptest.NewRequest(http.MethodGet, "/api/directory", nil))
     	if got := strings.TrimSpace(rec.Body.String()); got != `{"centers":[]}` {
     		t.Fatalf("body = %s", got)
     	}
     }

     func TestDirectoryHandlerOmitsAnEmptySoftwareLabel(t *testing.T) {
     	rec := httptest.NewRecorder()
     	dir := fakeDirectory{{ID: "x", Name: "X", Phone: "0312345678", DialMode: "pulse", MaxBaud: 9600}}
     	newDirectoryHandler(dir)(rec, httptest.NewRequest(http.MethodGet, "/api/directory", nil))
     	var payload struct {
     		Centers []map[string]any `json:"centers"`
     	}
     	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
     		t.Fatal(err)
     	}
     	if _, present := payload.Centers[0]["software"]; present {
     		t.Fatalf("an empty software label must be omitted: %v", payload.Centers[0])
     	}
     }

     func TestDirectoryHandlerMethods(t *testing.T) {
     	handler := newDirectoryHandler(world.NewMemoryStore())

     	head := httptest.NewRecorder()
     	handler(head, httptest.NewRequest(http.MethodHead, "/api/directory", nil))
     	if head.Code != http.StatusOK || head.Body.Len() != 0 {
     		t.Fatalf("HEAD = %d with %d body bytes", head.Code, head.Body.Len())
     	}

     	post := httptest.NewRecorder()
     	handler(post, httptest.NewRequest(http.MethodPost, "/api/directory", nil))
     	if post.Code != http.StatusMethodNotAllowed || post.Header().Get("Allow") != "GET, HEAD" {
     		t.Fatalf("POST = %d, Allow = %q", post.Code, post.Header().Get("Allow"))
     	}
     }
     ```

- [ ] **T009** ドキュメントを更新する

  1. `packages/protocol/README.md` に、HTTP の節（無ければ、WebSocket の説明の後に追加）として、`GET /api/directory` の説明を追加する:
     応答の形（`plan.md` の「API の契約」のとおり）、`listed: true` の局だけであること、
     `listed: false` の局は、載らないがダイヤルできること。
  2. `apps/server/internal/hostcatalog/README.md` の「Listed vs. dialable」に、1 段落追加する:
     「`listed: true` の局は、ダイヤルの電話帳（`GET /api/directory`、`world.HostDirectoryStore`）に載る。
     `listed: false` の局は、載らないが、番号を直接ダイヤルすれば繋がる。」
  3. `specs/README.md` の番号の表に、`| 006 | host-directory（電話帳の一覧をサーバが返す） |` を追加する。

**Part 1 の完了条件**:

```bash
go -C apps/server build ./...
go -C apps/server vet ./...
go -C apps/server test ./... -count=1
```

結果を `specs/006-host-directory/verification.md`（新規）の「Part 1」の節に記録する（形式は、末尾の「verification.md の形式」）。
PR は **draft** で作成する。タイトル: `電話帳の一覧をサーバが返す（GET /api/directory）`。
説明に含める: 目的、変更の要約、**既存の API を変えていないこと（追加だけ）**、
`verification.md` の結果（未実施を含めて正直に）、Part 2 が別の PR であること。

**Part 1 のマージ後**（人間が行う）: Render のデプロイ完了後に、
`https://zutto-pccom-prototype.onrender.com/api/directory` が、`plan.md` の契約どおりの JSON を返すことを確認する。
確認できてから、Part 2 を依頼する。

---

# Part 2: Web（`/api/directory` への切り替え）

**前提**: Part 1 がマージされ、本番で `/api/directory` が応答していること。

## Phase 3: `CenterDirectory.ts`

- [ ] **T010** `apps/web/src/modem/CenterDirectory.ts` を、次の内容に書き換える

  ```ts
  import type { DialMode } from '../audio/dialLineAudio';

  // One entry of the dialing directory. `name` is the display name: the station's
  // name, followed by its software in brackets when it has one.
  export type RegisteredCenter = {
    id: string;
    name: string;
    phone: string;
    dialMode: DialMode;
    maxBaud?: number;
  };

  const PRODUCTION_API_ORIGIN = 'https://zutto-pccom-prototype.onrender.com';
  const DIRECTORY_PATH = '/api/directory';
  // A first visit may wake the server (a free Render instance sleeps).
  const DIRECTORY_FETCH_TIMEOUT_MS = 120_000;
  // Keys of the browser-local directory that no longer exists.
  const LEGACY_STORAGE_KEYS = ['zutto.centers.v1', 'zutto.worldKey.v1'];

  function digitsOnly(value: unknown): string {
    return typeof value === 'string' ? value.replace(/\D/g, '').slice(0, 20) : '';
  }

  function normalizeCenter(value: unknown, index: number): RegisteredCenter | null {
    if (!value || typeof value !== 'object') return null;
    const raw = value as Record<string, unknown>;
    const phone = digitsOnly(raw.phone);
    if (!phone) return null;
    const baseName = typeof raw.name === 'string' && raw.name.trim() ? raw.name.trim() : `CENTER ${index + 1}`;
    const software = typeof raw.software === 'string' ? raw.software.trim() : '';
    const name = (software ? `${baseName} [${software}]` : baseName).slice(0, 64);
    const dialMode: DialMode = raw.dialMode === 'pulse' ? 'pulse' : 'tone';
    const id = typeof raw.id === 'string' && raw.id.trim() ? raw.id.trim() : `center-${phone}`;
    const maxBaud = typeof raw.maxBaud === 'number' && Number.isFinite(raw.maxBaud) ? raw.maxBaud : undefined;
    return { id, name, phone, dialMode, maxBaud };
  }

  // parseDirectory turns the server's response into directory entries. Entries
  // without a phone number are dropped, and the first entry wins when a number
  // appears twice.
  export function parseDirectory(payload: unknown): RegisteredCenter[] {
    const centers = (payload as { centers?: unknown } | null)?.centers;
    if (!Array.isArray(centers)) throw new Error('center directory: invalid response');
    const seen = new Set<string>();
    const out: RegisteredCenter[] = [];
    centers.forEach((value, index) => {
      const center = normalizeCenter(value, index);
      if (!center || seen.has(center.phone)) return;
      seen.add(center.phone);
      out.push(center);
    });
    return out;
  }

  // directoryEndpoint is the URL of the directory API: on the origin of the
  // WebSocket server when there is one, otherwise on this page (local
  // development) or on the production server.
  export function directoryEndpoint(wsURL: string, pageURL: string, localPage: boolean): string {
    if (wsURL) {
      const url = new URL(wsURL, pageURL);
      url.protocol = url.protocol === 'wss:' ? 'https:' : 'http:';
      url.pathname = DIRECTORY_PATH;
      url.search = '';
      return url.toString();
    }
    return localPage ? DIRECTORY_PATH : `${PRODUCTION_API_ORIGIN}${DIRECTORY_PATH}`;
  }

  // clearLegacyDirectoryStorage removes the keys of the old browser-local
  // directory. Cleanup is optional: blocked storage must not break the directory.
  export function clearLegacyDirectoryStorage(storage?: Pick<Storage, 'removeItem'>): void {
    try {
      const target = storage ?? (typeof window === 'undefined' ? undefined : window.localStorage);
      if (!target) return;
      for (const key of LEGACY_STORAGE_KEYS) target.removeItem(key);
    } catch {
      // Ignore.
    }
  }

  export async function fetchDirectory(wsURL = ''): Promise<RegisteredCenter[]> {
    const endpoint = directoryEndpoint(wsURL, window.location.href, isLocalPage());
    const controller = new AbortController();
    const timeout = window.setTimeout(() => controller.abort(), DIRECTORY_FETCH_TIMEOUT_MS);
    try {
      const response = await fetch(new URL(endpoint, window.location.href).toString(), { signal: controller.signal });
      if (!response.ok) throw new Error(`center directory: ${response.status}`);
      return parseDirectory(await response.json());
    } finally {
      window.clearTimeout(timeout);
    }
  }

  function isLocalPage(): boolean {
    return typeof window !== 'undefined'
      && (window.location.hostname === 'localhost' || window.location.hostname === '127.0.0.1');
  }
  ```

  - 削除されるもの: `DEFAULT_CENTERS`、`CENTER_STORAGE_KEY`、`WORLD_KEY_STORAGE_KEY`、`getOrCreateWorldKey`、
    `loadCenters`、`saveCenters`、`fetchWorldCenters`、`RegisteredCenter.builtIn`。

- [ ] **T011** `apps/web/src/modem/CenterDirectory.test.ts`（新規）を作る

  既存のテスト（例 `VirtualModem.test.ts`）の import の形式（`vitest`）に合わせる。次を確認する。

  | テスト | 内容 |
  |---|---|
  | HAKATA の要素 | `parseDirectory({centers:[{id:'hakata-canal-net',name:'HAKATA CANAL NET',software:'絵理香K版',phone:'0920000196',dialMode:'tone',maxBaud:14400}]})` が、`toEqual` で `[{id:'hakata-canal-net',name:'HAKATA CANAL NET [絵理香K版]',phone:'0920000196',dialMode:'tone',maxBaud:14400}]`（**従来の `DEFAULT_CENTERS` と同じ表示名**） |
  | `software` なし | 括弧が付かない（`name` がそのまま） |
  | 形が不正 | `centers` が配列でない（`undefined`、`null`、`{}`、文字列）と、`center directory: invalid response` を投げる |
  | 電話番号 | 無い要素は捨てる。`092-000-0196` は `0920000196` になる（数字のみ）。重複は先のものが残る |
  | ダイヤル方式 | `pulse` は `pulse`。それ以外（`undefined`、`'x'`）は `tone` |
  | 最大速度 | 数値は保たれる。数値でない（文字列、`NaN`）と `undefined` |
  | 空の一覧 | `{centers: []}` は `[]`（エラーにしない。「空」を失敗とみなすのは `App.tsx`） |
  | `directoryEndpoint` | `('ws://localhost:8080/ws', 'http://localhost:5173/', true)` → `http://localhost:8080/api/directory`。`('wss://api.example.com/ws', …)` → `https://api.example.com/api/directory`。`('', …, true)` → `/api/directory`。`('', …, false)` → `https://zutto-pccom-prototype.onrender.com/api/directory`。`wsURL` の `?query` は捨てられる |
  | 旧キーの掃除 | 3 つのキー（`zutto.centers.v1`、`zutto.worldKey.v1`、`other`）を持つ偽の storage で、前の 2 つだけが消え、`other` が残る。`removeItem` が例外を投げる storage でも、例外が外に出ない |

## Phase 4: `App.tsx` と旧コードの削除

- [ ] **T012** `apps/web/src/App.tsx` を変更する

  1. import を置き換える。

     ```ts
     import { clearLegacyDirectoryStorage, fetchDirectory } from './modem/CenterDirectory';
     ```

     （`fetchWorldCenters`、`loadCenters` の import を削除する。`import type { RegisteredCenter }` は残す。）
  2. 次の行

     ```ts
     const centersRef = useRef<RegisteredCenter[]>(loadCenters());
     ```

     を、次に置き換える。

     ```ts
     const centersRef = useRef<RegisteredCenter[]>([]);
     ```

  3. `fetchWorldCenters(wsURL).then(centers => {` で始まる `useEffect` の先頭を、次の形にする
     （`.then(...)` 以降の中身は、変更しない）。

     ```ts
       useEffect(() => {
         clearLegacyDirectoryStorage();
         fetchDirectory(wsURL).then(centers => {
     ```

  - それ以外の行（特に `telehodaiNumbers`、ターミナル・モードの `ATDT0920000196` の案内文）は変更しない。

- [ ] **T013** `apps/web/src/modem/CenterDirectoryPanel.tsx` を削除する

  どこからも使われていない（`grep -rn CenterDirectoryPanel apps/web/src` が、このファイル自身だけを返す）。
  使われていれば、削除せずに報告する。関連する CSS（`.center-directory-panel` など）が、他で使われていなければ、
  CSS の削除は、この作業では**しない**（別の機会）。

- [ ] **T014** 確認

  ```bash
  npm --prefix apps/web test
  npm --prefix apps/web run build
  grep -rnE "DEFAULT_CENTERS|loadCenters|saveCenters|fetchWorldCenters|getOrCreateWorldKey|builtIn|bootstrap" apps/web/src --include=*.ts --include=*.tsx | grep -v "\.test\."
  grep -rn "0920000196" apps/web/src | grep -v "\.test\."
  ```

  - 最初の `grep` が、何も返さないこと。
  - 2 番目の `grep` の結果が、`App.tsx` の 2 か所（`VITE_TELEHODAI_NUMBERS` の既定値と、ターミナル・モードの案内文）
    だけであること。それ以外が残っていれば、報告する。

## Phase 5: ドキュメントと PR

- [ ] **T015** `packages/protocol/README.md` に、Web が `/api/directory` を使うことを 1 文で追記する（Part 1 で追加した節に）。

- [ ] **T016** `specs/006-host-directory/verification.md` の「Part 2」の節に、結果を記録する

- [ ] **T017** 手動確認の項目を、PR の説明に書く（実施は人間が行う）

  - ブラウザで、メインメニューの `1` を開くと、HAKATA が `HAKATA CANAL NET [絵理香K版]` で表示される。
  - CALL で、`ATDT0920000196` が発信され、接続できる。
  - 開発者ツールの Network に、`/api/world/bootstrap` が出ない（`/api/directory` が出る）。
  - 起動後に、`localStorage` から `zutto.centers.v1` と `zutto.worldKey.v1` が消えている。

- [ ] **T018** PR を作る

  - `main` への PR。**draft** で作成する。タイトル: `電話帳の一覧をサーバから取得する（Web から局の固定値を削除）`。
  - 説明に含める: 目的、変更の要約、**Part 1 がデプロイ済みで、本番の `/api/directory` が応答すること（確認した日時）**、
    削除したもの（`DEFAULT_CENTERS` ほか）、`verification.md` の結果（未実施を含めて正直に）、スコープ外の項目。

---

## verification.md の形式

```markdown
# Verification: host directory

## Part 1（サーバ）
| コマンド | 結果 | 備考 |
|---|---|---|
| `go -C apps/server build ./...` | 成功 / 失敗 / 未実施 | |
| `go -C apps/server vet ./...` | 〃 | |
| `go -C apps/server test ./...` | 〃 | |

本番の `/api/directory`: 確認済み（日時）/ 未確認

## Part 2（Web）
| コマンド | 結果 | 備考 |
|---|---|---|
| `npm --prefix apps/web test` | 成功 / 失敗 / 未実施 | |
| `npm --prefix apps/web run build` | 〃 | |
| grep（旧コードの残り） | 空 / 残りあり | |

## 手動確認
- 電話帳に HAKATA が表示され、発信できる: 確認済み / 未実施
- `/api/world/bootstrap` が呼ばれない: 確認済み / 未実施
```

## 実行順序

```text
Part 1:  T001 → T002 → T003 → T004 → T005 → T006 → T007 → T008 → T009
            （マージ → Render にデプロイ → 本番の /api/directory を確認）
Part 2:  T010 → T011 → T012 → T013 → T014 → T015 → T016 → T017 → T018
```

## 完了の定義

- `spec.md` の FR と SC を満たす（手動確認が未実施なら、その旨を明記する）。
- 契約テストの期待値が、`spec.md` の SC-002 の値と一致している。
- 変更が `plan.md` の「変更するファイル」の表の範囲に収まっている。
