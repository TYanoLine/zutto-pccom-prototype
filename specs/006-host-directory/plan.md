# Plan: host directory

**Spec**: `specs/006-host-directory/spec.md`

## 現状（調べた事実と、推測の区別）

| 項目 | 内容 | 確かさ |
|---|---|---|
| Web の一覧の出所 | `DEFAULT_CENTERS`（HAKATA 固定）、`localStorage` のカスタム局、`/api/world/bootstrap` の生成局 | コードで確認 |
| 一覧の置き換え | `App.tsx` が、`fetchWorldCenters` の成功時に、`centersRef.current = centers` で全体を置き換える | コードで確認 |
| 生成局の保存先 | Postgres の `worlds`・`hosts`。ブラウザごとの `worldKey` 単位で 100 局 | コードで確認 |
| 生成局がダイヤルできない | `telephone.Network.Dial` は `store.HostByPhone` を使い、未知の番号は `NoAnswer`。`Repository.HostByPhone` は `Base`（preset の局だけ）に委譲する | コードで確認（実機では未確認） |
| 本番で、HAKATA が一覧から消えている | 上の 2 点からの推測。Render のリクエストログに `/api/world/bootstrap` の呼び出しは無く（過去 1 日）、確認できていない | **推測** |
| `saveCenters`、`CenterDirectoryPanel.tsx` | 定義があるだけで、どこからも使われていない | コードで確認 |
| `world.Host` | `Listed`、`DialMode` を持たない。`HostDescriptor` は持つ | コードで確認 |

## 設計

```text
presets/*.yaml ──▶ hostcatalog.Descriptors ──▶ world.presetData.directory（listed: true だけ）
                                                         │
                                  MemoryStore.ListedHosts() ◀── world.HostDirectoryStore
                                                         ▲
                                   worldrepo.Repository.ListedHosts （Base に委譲）
                                                         │
                          GET /api/directory  ◀─ cmd/server の handler
                                                         │
                          Web: fetchDirectory ─▶ parseDirectory ─▶ centersRef ─▶ TerminalCenterDirectory
```

- 一覧は、局の定義から導出する、**不変の読み取り専用のデータ**。`world.Host` には、`Listed` や `DialMode` を足さない
  （`Host` は `==` で比較される。局の骨格は `HostDescriptor` が持つ）。
- 電話帳は、`listed: true` の局だけ。`listed: false` の局は、ダイヤルできるが、載らない。
- 表示名の組み立て（`<name> [<software>]`）は、Web が行う。`software` が無ければ、括弧を付けない。

## API の契約

```http
GET /api/directory
200 OK
Content-Type: application/json
Cache-Control: no-cache

{"centers":[{"id":"hakata-canal-net","name":"HAKATA CANAL NET","software":"絵理香K版","phone":"0920000196","dialMode":"tone","maxBaud":14400}]}
```

- 局が 0 件のときは `{"centers":[]}`（`null` にしない）。
- `software` は、`software_label` が空なら、キーごと省略する。
- 並びは preset の key の昇順（`LoadPresets` が返す順。ファイル名の昇順）。
- 既存の `/api/world/bootstrap`（`centers` 配列を返す）と、要素の形が似ているが、別のエンドポイント。

## Go の型

```go
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
```

## Web の設計

| 項目 | 内容 |
|---|---|
| `RegisteredCenter` | `{ id, name, phone, dialMode, maxBaud? }`。`builtIn` を削除。`name` は、表示名（`<name> [<software>]`）にして、利用側（`TerminalCenterDirectory`、`App.tsx`）を変えない |
| `directoryEndpoint(wsURL, pageURL, localPage)` | 純粋関数。`wsURL` があれば、その origin の `/api/directory`。無ければ、ローカルのページは `/api/directory`、それ以外は本番の origin + `/api/directory` |
| `parseDirectory(payload)` | 純粋関数。`centers` が配列でなければ、`center directory: invalid response`。電話番号が無い要素は捨てる。電話番号が重複する要素は、先のものを残す |
| `fetchDirectory(wsURL)` | `fetch` の薄い包み。タイムアウトは従来どおり 120 秒（Render の無料プランの起動待ちのため） |
| `clearLegacyDirectoryStorage()` | 旧キー 2 つを削除する。例外を握りつぶす |
| `App.tsx` | `centersRef` の初期値を `[]` にする。起動時に `clearLegacyDirectoryStorage()` を 1 回呼び、`fetchDirectory(wsURL)` で取得する。それ以外の流れ（`directoryLoadStateRef`、エラー表示）は変えない |

## 2 つの PR に分ける（デプロイの順序）

Web（Vercel）とサーバ（Render）は、別々にデプロイされる。Web が先に切り替わって、サーバに `/api/directory` が無いと、
電話帳が一時的にエラーになる。そのため、次の順で、2 つの PR に分ける。

| PR | 内容 | 順序の条件 |
|---|---|---|
| **PR 1（サーバ）** | `/api/directory` の追加。既存の API は変えない（追加だけ。古い Web が動き続ける） | 先にマージして、Render にデプロイされる |
| **PR 2（Web）** | Web を `/api/directory` に切り替え、旧コードを削除 | PR 1 のデプロイ後に、本番で `/api/directory` が応答することを確認してから、作業を始める |

## 変更するファイル

| PR | 層 | ファイル | 内容 |
|---|---|---|---|
| 1 | world | `directory.go`（新規） | `DirectoryEntry`、`HostDirectoryStore`、`MemoryStore.ListedHosts` |
| 1 | world | `preset_hosts.go` | `presetData.directory`、`presetDirectory()` |
| 1 | world | `store.go` | `MemoryStore.directory` の初期化 |
| 1 | world | `directory_test.go`（新規） | 一覧の内容と、`listed` とダイヤルの分離 |
| 1 | worldrepo | `host_directory.go`（新規）、同テスト | `Repository.ListedHosts`（`Base` に委譲） |
| 1 | server | `cmd/server/directory_handler.go`（新規）、同テスト、`main.go` | `GET /api/directory` |
| 1 | Doc | `packages/protocol/README.md`、`hostcatalog/README.md` | API と、電話帳の説明 |
| 2 | web | `src/modem/CenterDirectory.ts` | 書き換え（`fetchDirectory` ほか） |
| 2 | web | `src/modem/CenterDirectory.test.ts`（新規） | 純粋関数のテスト |
| 2 | web | `src/App.tsx` | 取得の切り替え、旧キーの掃除 |
| 2 | web | `src/modem/CenterDirectoryPanel.tsx` | 削除 |
| 共通 | Doc | `specs/README.md` | 番号の表に 006 を追加 |

## 検証

- 契約テスト（サーバ）: HAKATA の要素を、従来の `DEFAULT_CENTERS` の値と、完全に一致する値で検査する（SC-002）。
  これが、「電話帳の表示が変わらない」ことの根拠になる（金型の代わり）。
- 本番での確認: PR 1 のデプロイ後に、`https://zutto-pccom-prototype.onrender.com/api/directory` が、上の契約の JSON を返すこと。
- 手動確認（PR 2 のあと）: ブラウザで、メインメニューの `1` を開き、HAKATA が `HAKATA CANAL NET [絵理香K版]` で表示され、
  CALL で発信できること。ブラウザの開発者ツールの Network に、`/api/world/bootstrap` が出ないこと。

## リスクと対策

| リスク | 対策 |
|---|---|
| Web が先に切り替わり、サーバに API が無い | 2 つの PR に分ける。PR 1 を先にデプロイして確認する |
| 古い Web（キャッシュ）が、`/api/world/bootstrap` を呼ぶ | サーバの既存の API を変えない（FR-009）。古い Web も動き続ける |
| Render の無料プランの起動待ちで、取得が遅い | タイムアウトを 120 秒のままにする。従来の「読み込み中」の表示も、そのまま |
| 一覧が空になる（preset が `listed: false` だけ） | 従来の「empty center directory」のエラー表示になる（FR-008） |
| `RegisteredCenter` の変更が、利用側を壊す | `name` を表示名にして、利用側を変えない。`TerminalCenterDirectory` は `builtIn` を使っていない |
| 旧キーの掃除で、他のキーを消す | 削除するキーを、定数の 2 つに限る。テストで、他のキーが残ることを確認する |
| 本番の origin の直書き（`https://zutto-pccom-prototype.onrender.com`） | 従来どおり（既存の挙動）。変えない |
