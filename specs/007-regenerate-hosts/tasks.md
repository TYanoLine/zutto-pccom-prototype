# Tasks: 生成ホストの再生成と電話帳の堅牢化

**Input**: `specs/007-regenerate-hosts/spec.md`、`plan.md`  
**Prerequisites**: `AGENTS.md`、`plan.md`

## この作業のルール（必読）

- 2 つの PR に分ける（**Part 1: サーバ**、**Part 2: Web**）。それぞれ作業ブランチから `main` への PR を作る。
- **成果物は Go / TypeScript のコードの変更（実装）とテスト。** spec のファイルを作る・変更することは、この作業の仕事ではない。
- **既存の動作（HAKATA の表示、ダイヤル、電話番号の割り当て）を壊さないこと。**
- スコープ外（テレホーダイ番号の既定値、生成局への通話接続、preset の変更など）に手を出さない。
- 既存ファイルに整形だけの差分を作らない。`gofmt -w` を既存ファイル全体にかけない。
- 実行していないコマンドを「成功した」と書かない。実行できなかったものは「未実施」と理由を書く。
- この作業で `.github/` 配下のファイルは変更しない。
- spec と実際のコードが食い違っていたら、推測で進めず、PR の説明に書いて止まる。

---

# Part 1: サーバ（PR 1）

## Phase 1: `worldcatalog.Store.ResetHosts` の実装

- [ ] **T001** `apps/server/internal/worldcatalog/postgres.go` に `ResetHosts` を追加する

  `ResetHost`（237行目）の直後に、次のメソッドを挿入する。

  ```go
  // ResetHosts regenerates all directory entries for a world while preserving
  // the world, its seed, and the phone numbers. A fresh generation number gives
  // the skeletons new entropy.
  func (s *Store) ResetHosts(ctx context.Context, worldKey string, count int, generateNames func(context.Context, int) ([]string, error)) (Catalog, error) {
  	if !ValidWorldKey(worldKey) {
  		return Catalog{}, errors.New("invalid world key")
  	}
  	if count <= 0 {
  		return Catalog{}, errors.New("center count must be positive")
  	}
  	conn, err := s.pool.Acquire(ctx)
  	if err != nil {
  		return Catalog{}, fmt.Errorf("acquire postgres connection: %w", err)
  	}
  	defer conn.Release()
  	if err := lockWorld(ctx, conn, worldKey); err != nil {
  		return Catalog{}, err
  	}
  	defer unlockWorld(conn, worldKey)

  	var worldID string
  	var seed int64
  	err = conn.QueryRow(ctx, `SELECT id::text,seed FROM worlds WHERE world_key=$1`, worldKey).Scan(&worldID, &seed)
  	if errors.Is(err, pgx.ErrNoRows) {
  		return Catalog{}, errors.New("world not found")
  	}
  	if err != nil {
  		return Catalog{}, fmt.Errorf("load world: %w", err)
  	}

  	var currentGen int
  	err = conn.QueryRow(ctx, `SELECT COALESCE(MAX(generation), 0) FROM hosts WHERE world_id=$1::uuid`, worldID).Scan(&currentGen)
  	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
  		return Catalog{}, fmt.Errorf("query max generation: %w", err)
  	}
  	newGen := currentGen + 1

  	names, err := generateNames(ctx, count)
  	if err != nil {
  		return Catalog{}, err
  	}
  	if len(names) != count {
  		return Catalog{}, fmt.Errorf("expected %d generated names, got %d", count, len(names))
  	}

  	centers := make([]Center, count)
  	for i, name := range names {
  		centers[i] = makeCenter(name, seed, i, newGen)
  	}

  	tx, err := conn.Begin(ctx)
  	if err != nil {
  		return Catalog{}, fmt.Errorf("begin transaction: %w", err)
  	}
  	defer func() { _ = tx.Rollback(context.Background()) }()

  	if _, err := tx.Exec(ctx, `DELETE FROM hosts WHERE world_id=$1::uuid`, worldID); err != nil {
  		return Catalog{}, fmt.Errorf("delete old hosts: %w", err)
  	}
  	for i, center := range centers {
  		if err := insertCenter(ctx, tx, worldID, i, newGen, center); err != nil {
  			return Catalog{}, err
  		}
  	}
  	if err := tx.Commit(ctx); err != nil {
  		return Catalog{}, fmt.Errorf("commit hosts: %w", err)
  	}

  	return Catalog{WorldID: worldID, Seed: seed, Centers: centers, Created: false}, nil
  }
  ```

- [ ] **T002** `apps/server/internal/worldcatalog/postgres_test.go` に確認テストを追加する

  ファイルの末尾に次のテストを追加する。

  ```go
  func TestMakeCenterMaintainsPhoneAcrossGenerations(t *testing.T) {
  	seed := int64(987654321)
  	for index := 0; index < 10; index++ {
  		c0 := makeCenter("GEN0", seed, index, 0)
  		c1 := makeCenter("GEN1", seed, index, 1)
  		c2 := makeCenter("GEN2", seed, index, 2)
  		if c0.Phone != c1.Phone || c1.Phone != c2.Phone {
  			t.Fatalf("index %d phone changed across generations: %s, %s, %s", index, c0.Phone, c1.Phone, c2.Phone)
  		}
  		if c0.ID != c1.ID || c1.ID != c2.ID {
  			t.Fatalf("index %d id changed across generations: %s, %s, %s", index, c0.ID, c1.ID, c2.ID)
  		}
  		if c0.Name == c1.Name || c1.Name == c2.Name {
  			t.Fatalf("index %d name unexpectedly identical: %s", index, c0.Name)
  		}
  	}
  }
  ```

- [ ] **T003** ビルドとテストの確認

  ```bash
  go -C apps/server test ./internal/worldcatalog/... -count=1
  ```

---

## Phase 2: ルート追加と生成処理の非同期切り離し

- [ ] **T004** `apps/server/cmd/server/main.go` の `bootstrapWorld` で `context.WithoutCancel` を適用する

  118行目の：
  ```go
  ctx, cancel := context.WithTimeout(r.Context(), 75*time.Second)
  ```
  を次のように変更する：
  ```go
  ctx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), 75*time.Second)
  ```

- [ ] **T005** `apps/server/cmd/server/main.go` に `resetHosts` ハンドラを定義する

  `bootstrapWorld`（133行目付近）の直後に次のハンドラを追加する：

  ```go
  	resetHosts := func(w http.ResponseWriter, r *http.Request) {
  		w.Header().Set("Content-Type", "application/json")
  		if r.Method != http.MethodPost {
  			w.Header().Set("Allow", "POST")
  			w.WriteHeader(http.StatusMethodNotAllowed)
  			_ = json.NewEncoder(w).Encode(map[string]any{"error": "POST only"})
  			return
  		}
  		if catalogStore == nil {
  			http.Error(w, `{"error":"persistent world database is not configured"}`, http.StatusServiceUnavailable)
  			return
  		}
  		worldKey := r.URL.Query().Get("key")
  		if !worldcatalog.ValidWorldKey(worldKey) {
  			http.Error(w, `{"error":"invalid world key"}`, http.StatusBadRequest)
  			return
  		}
  		ctx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), 75*time.Second)
  		defer cancel()
  		catalog, err := catalogStore.ResetHosts(ctx, worldKey, generatedCenterCount, generateNames)
  		if err != nil {
  			log.Printf("world reset hosts failed: %v", err)
  			w.WriteHeader(http.StatusBadGateway)
  			_ = json.NewEncoder(w).Encode(map[string]any{"error": err.Error()})
  			return
  		}
  		_ = json.NewEncoder(w).Encode(map[string]any{
  			"worldId": catalog.WorldID,
  			"centers": catalog.Centers,
  			"source":  "azure_openai",
  			"model":   cfg.AzureOpenAIModel,
  		})
  	}
  ```

- [ ] **T006** `main.go` のルーティングに登録する

  464行目（`mux.HandleFunc("/api/centers", bootstrapWorld)`）の直後に以下を追加する：
  ```go
  	mux.HandleFunc("/api/world/reset-hosts", resetHosts)
  ```

- [ ] **T007** プロトコル仕様書 `packages/protocol/README.md` に `/api/world/reset-hosts` を追記する

  ファイルの末尾に `POST /api/world/reset-hosts` の説明、メソッド、クエリパラメータ `key`、応答形式を記載する。

- [ ] **T008** サーバ全体のテストとビルド確認

  ```bash
  go -C apps/server build ./...
  go -C apps/server test ./... -count=1
  ```

- [ ] **T009** Part 1 のコミットと PR の作成

  ブランチ `server/007-regenerate-hosts` を作成し、コミットして PR を作成する。
  `go-test` CI が通過し、マージされて Render にデプロイされたことを確認してから Part 2 へ進む。

---

# Part 2: Web（PR 2）

## Phase 3: エンドポイント関数とテスト

- [ ] **T010** `apps/web/src/modem/CenterDirectory.ts` にリセット用関数を追加する

  18行目付近（`WORLD_CENTERS_PATH` 定義の後）：
  ```typescript
  const WORLD_RESET_HOSTS_PATH = '/api/world/reset-hosts';
  ```

  100行目付近（`worldCentersEndpoint` の後）：
  ```typescript
  export function worldResetHostsEndpoint(wsURL: string, pageURL: string, localPage: boolean): string {
    return apiEndpoint(WORLD_RESET_HOSTS_PATH, wsURL, pageURL, localPage);
  }
  ```

  167行目付近（`fetchWorldCenters` の後）：
  ```typescript
  export async function resetWorldHosts(wsURL = ''): Promise<RegisteredCenter[]> {
    const endpoint = worldResetHostsEndpoint(wsURL, window.location.href, isLocalPage());
    const url = new URL(endpoint, window.location.href);
    url.searchParams.set('key', getOrCreateWorldKey());
    const controller = new AbortController();
    const timeout = window.setTimeout(() => controller.abort(), FETCH_TIMEOUT_MS);
    try {
      const response = await fetch(url.toString(), { method: 'POST', signal: controller.signal });
      if (!response.ok) {
        let detail = '';
        try { detail = ((await response.json()) as { error?: string }).error ?? ''; } catch { /* ignore */ }
        throw new Error(detail || `reset hosts: ${response.status}`);
      }
      return parseDirectory(await response.json());
    } finally {
      window.clearTimeout(timeout);
    }
  }
  ```

- [ ] **T011** `apps/web/src/modem/CenterDirectory.test.ts` にテストを追加する

  `worldResetHostsEndpoint` の URL 生成、`resetWorldHosts` のテストを追加する。

- [ ] **T012** テスト実行

  ```bash
  npm --prefix apps/web test
  ```

---

## Phase 4: `App.tsx` の UI と堅牢化

- [ ] **T013** プリセット局読み込みエラー時の画面遷移を修正する

  `App.tsx` の `fetchDirectory` の catch 節（182〜187行目）で、`openDirectoryWhenReadyRef.current` が true だった場合に `showDirectoryError()` を呼び出す：
  ```typescript
      }).catch(error => {
        directoryLoadStateRef.current = 'error';
        const message = error instanceof Error ? error.message : String(error);
        setDirectoryStatus(`CENTER API ERROR: ${message}`);
        if (openDirectoryWhenReadyRef.current) {
          openDirectoryWhenReadyRef.current = false;
          showDirectoryError();
        }
      });
  ```

- [ ] **T014** 生成局の自動再試行と失敗時 notice を実装する

  生成局の取得を関数化し、指数バックオフ（2秒、10秒、30秒）と `online` イベントによる再試行を実装する。
  - 読み込み中: `ほかのセンターを読み込み中...`
  - 失敗時: `ほかのセンターの読み込みに失敗しました（再試行中...）`
  - 成功時: notice を空文字にしてクリアし、`mergeCenters` で一覧を更新して `directoryRef.current?.refresh()` を呼ぶ。

- [ ] **T015** メインメニューでの `1r` 入力と再生成画面を実装する

  1. `showRegeneratingHosts()` 関数を追加：
     ```typescript
     function showRegeneratingHosts() {
       screenModeRef.current = 'regenerating';
       terminal.clear();
       terminal.write(`\x1b[37;44m ずっとパソコン通信　ホスト情報再生成                         Ver ${APP_VERSION} \x1b[0m\r\n\r\n`);
       terminal.write('                     \x1b[30;46m　ホスト情報の再生成　\x1b[0m\r\n\r\n');
       terminal.write(' 生成ホスト情報を再生成しています。\r\n');
       terminal.write(' しばらくお待ちください...\r\n\r\n');
       terminal.write(' ※ 完了すると、自動的にセンター・リストを表示します。\r\n');
       terminal.write('    ESCキーでメイン・メニューに戻ることができます（再生成は継続します）。');
     }
     ```
  2. `submitInput()` で `command.toLowerCase() === '1r'` の分岐を追加し、再生成処理を実行する。
  3. `keyDown` で ESC キー入力時、再生成画面であればメインメニューへ戻れるようにする。

- [ ] **T016** ビルドとテストの確認

  ```bash
  npm --prefix apps/web test
  npm --prefix apps/web run build
  ```

- [ ] **T017** Part 2 のコミットと PR の作成

  ブランチ `web/007-regenerate-hosts` を作成し、コミットして PR を作成する。
  Vercel のプレビュー環境で動作を確認する。
