# Plan: 生成ホストの再生成と電話帳の堅牢化

**Spec**: `specs/007-regenerate-hosts/spec.md`

## 現状（調べた事実と、推測の区別）

| 項目 | 内容 | 確かさ |
|---|---|---|
| 生成局の保存構造 | Postgres の `worlds`（id, world_key, seed, generation_version）と `hosts`（world_id, phone_number, name, facts, generation, directory_order） | コードで確認 |
| 電話番号の決定方式 | `postgres.go` の `makeCenter`: `identityZ := mix(seed, index, 0)`、`area := 3 + int(identityZ%7)`、`Phone: fmt.Sprintf("0%d%08d", area, 10000000+index)`。generation が変わっても、第3引数に 0 を渡すため**電話番号と ID は完全に不変** | コードで確認 |
| 既存のリセット実装 | `ResetWorld`（世界削除）、`ResetHost`（1 局のみ再生成）はあるが、世界全体のホスト再生成（`ResetHosts`）はなく、HTTP エンドポイントもない | コードで確認 |
| キャンセルの伝播 | `main.go` の `bootstrapWorld` で `context.WithTimeout(r.Context(), 75*time.Second)` としており、クライアント切断時に `r.Context()` がキャンセルされ、LLM や DB 処理が中断される | コードで確認 |
| 競合制御 | `pg_advisory_lock(hashtextextended(worldKey, 0))` により、同じ世界の鍵に対する生成処理は直列化されている | コードで確認 |
| Web のエラー表示 | `App.tsx` で `fetchDirectory`（プリセット局）が失敗した際、`openDirectoryWhenReadyRef` が true でも `showDirectoryError()` を呼んでおらず、読み込み中画面のまま止まる | コードで確認 |
| Web の再試行 | `fetchWorldCenters` の catch で空配列 `[]` を返して終了しており、再試行や失敗時の notice 表示がない | コードで確認 |

## 設計

```text
【サーバ側: 生成の切り離しと再生成】
Client Request ──▶ r.Context() (切断されてもキャンセルされない)
                     │
                     ▼ context.WithoutCancel(r.Context()) + 75s timeout
           ┌────────────────────────────────────────┐
           │ lockWorld(worldKey)                    │
           │ LLM で名前生成 (失敗時は DB に触らない)  │
           │ BEGIN TX                               │
           │   DELETE FROM hosts WHERE world_id=$1  │
           │   INSERT NEW HOSTS (generation + 1)    │
           │ COMMIT                                 │
           └────────────────────────────────────────┘
                     │
                     ▼
Response 200 OK (クライアント接続中なら返信、切断済なら破棄)

【Web側: 自動再試行と 1r フロー】
メインメニュー ──[ 1r ]──▶ 再生成画面 (showRegeneratingHosts)
                             │
                             ├─▶ POST /api/world/reset-hosts?key=...
                             │     │
                             │     ▼ 成功
                             ├─▶ centersRef 更新 (プリセット + 新生成局)
                             │   センター・リストを自動表示 (局数更新)
                             │
                             ▼ 失敗
                           エラー表示 (ESC で戻る)

電話帳バックグラウンド取得 ──▶ 失敗 ──▶ 2秒・10秒・30秒で再試行 (または online イベント)
                                    │
                                    └─▶ notice: "ほかのセンターの読み込みに失敗しました（再試行中...）"
```

## API の契約

### `POST /api/world/reset-hosts`

```http
POST /api/world/reset-hosts?key=0123456789abcdef0123456789abcdef
Content-Type: application/json

200 OK
Content-Type: application/json

{
  "worldId": "3fa85f64-5717-4562-b3fc-2c963f66afa6",
  "centers": [
    {
      "id": "world-001",
      "name": "新ホスト名",
      "phone": "0310000000",
      "dialMode": "tone",
      "maxBaud": 28800,
      "softwareFamily": "erika-k",
      "lineCount": 1,
      "foundedOn": "1992-05-12",
      "popularity": 0.65,
      "memberCount": 420
    }
  ],
  "source": "azure_openai",
  "model": "zutto-pccom-gpt-6-luna"
}
```

- クエリパラメータ `key` が無効（正規表現 `^[0-9a-f]{32}$` に不一致）な場合は `400 Bad Request`。
- DB 未設定の場合は `503 Service Unavailable`。
- 世界が存在しない場合は `404 Not Found`。
- LLM 生成または DB コミット失敗時は `502 Bad Gateway`。

## Go の設計

### 1. `apps/server/internal/worldcatalog/postgres.go`

`Store` に以下のメソッドを追加する：

```go
func (s *Store) ResetHosts(ctx context.Context, worldKey string, count int, generateNames func(context.Context, int) ([]string, error)) (Catalog, error)
```

**処理フロー**:
1. `ValidWorldKey(worldKey)` チェック。
2. `s.pool.Acquire(ctx)` で接続取得、`lockWorld(ctx, conn, worldKey)` でアドバイザリロック取得（`defer unlockWorld`）。
3. `SELECT id::text, seed FROM worlds WHERE world_key=$1` で世界存在確認（無ければ `ErrNoRows` -> エラー）。
4. 現在の最大世代番号を取得：
   `SELECT COALESCE(MAX(generation), 0) FROM hosts WHERE world_id=$1::uuid`
   新世代: `newGeneration := currentGen + 1`
5. トランザクションに入る前に、`generateNames(ctx, count)` を実行。失敗した場合は DB を一切変更せずエラーを返す。
6. `conn.Begin(ctx)` でトランザクション開始。
7. `DELETE FROM hosts WHERE world_id=$1::uuid`
8. `makeCenters` に相当するループで、`makeCenter(names[i], seed, i, newGeneration)` を呼び出し、各局を `insertCenter(ctx, tx, worldID, i, newGeneration, center)` で挿入。
9. `tx.Commit(ctx)`。
10. `Catalog{WorldID: worldID, Seed: seed, Centers: centers, Created: false}` を返す。

### 2. `apps/server/cmd/server/main.go`

`bootstrapWorld` および新設する `resetHosts` で、リクエストから生成コンテキストを切り離す：

```go
// クライアント切断に影響されないコンテキストを作成 (75秒制限)
bgCtx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), 75*time.Second)
defer cancel()
```

- `bootstrapWorld`: `GetOrCreate` に `bgCtx` を渡す。
- `resetHosts`: `ResetHosts` に `bgCtx` を渡す。
- ルーティング: `mux.HandleFunc("/api/world/reset-hosts", resetHosts)`

## Web の設計

### 1. `apps/web/src/modem/CenterDirectory.ts`

- `WORLD_RESET_HOSTS_PATH = '/api/world/reset-hosts'`
- `worldResetHostsEndpoint(wsURL: string, pageURL: string, localPage: boolean): string`
- `resetWorldHosts(wsURL = ''): Promise<RegisteredCenter[]>`
  - `POST` メソッドで `worldResetHostsEndpoint`（`?key=<worldKey>`）を呼び出す。
  - レスポンスを `parseDirectory` でパースして返す。

### 2. `apps/web/src/App.tsx`

#### (a) プリセット局読み込みエラーの修正
```typescript
fetchDirectory(wsURL).then(presetCenters => {
  // 正常系
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

#### (b) 生成局取得の自動再試行と notice 表示
- 生成局の取得と再試行を行うヘルパー関数 `loadWorldCentersWithRetry` を設置：
  - 再試行間隔: `[2000, 10000, 30000]`（3回目以降は 30000ms で維持、コンポーネント生存中または最大試行回数まで）。
  - 試行失敗時:
    - `directoryRef.current?.setNotice('ほかのセンターの読み込みに失敗しました（再試行中...）')`
    - `directoryRef.current?.refresh()`
  - 成功時:
    - 取得した局をプリセット局とマージ（`mergeCenters`）。
    - `directoryRef.current?.setNotice('')`
    - `directoryRef.current?.refresh()`
  - `window.addEventListener('online', onOnline)` で、オンライン復帰時に直ちに再試行を発火。

#### (c) メインメニュー `1r` コマンド
- `submitInput()` で `command.toLowerCase() === '1r'` を処理：
  - 画面を `showRegeneratingHosts()` に切り替え。
  - `resetWorldHosts(wsURL)` を実行。
  - 完了時:
    - 成功: `centersRef.current` を更新、局数更新、画面が再生成画面ならセンター・リスト（電話帳）を開く。
    - 失敗: 画面に「ホスト情報の再生成に失敗しました。\r\nESCキーでメイン・メニューに戻ってください。」を表示。

## 2 つの PR に分ける（デプロイ順序）

| PR | 内容 | 順序の条件 |
|---|---|---|
| **PR 1（サーバ）** | `worldcatalog.Store.ResetHosts`、`POST /api/world/reset-hosts`、コンテキスト切り離し | 先にマージして Render にデプロイされる |
| **PR 2（Web）** | `1r` コマンド、生成局再試行、notice 表示、プリセット局エラー画面修正 | PR 1 のデプロイ後に作業・マージ |

## 変更するファイル

### PR 1（サーバ）
- `apps/server/internal/worldcatalog/postgres.go`: `ResetHosts` メソッド追加
- `apps/server/internal/worldcatalog/postgres_test.go`: `ResetHosts` または純粋関数のテスト
- `apps/server/cmd/server/main.go`: `context.WithoutCancel` の適用、`resetHosts` ハンドラ追加とルーティング
- `packages/protocol/README.md`: `/api/world/reset-hosts` の仕様追記

### PR 2（Web）
- `apps/web/src/modem/CenterDirectory.ts`: `worldResetHostsEndpoint`, `resetWorldHosts`
- `apps/web/src/modem/CenterDirectory.test.ts`: 新関数のテスト
- `apps/web/src/App.tsx`: `1r` コマンド処理、再生成画面、再試行ロジック、エラー画面修正
- `apps/web/src/App.test.tsx` または関連テスト: 動作検証
