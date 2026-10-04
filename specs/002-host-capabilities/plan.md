# Plan: host capabilities

**Spec**: `specs/002-host-capabilities/spec.md`

## 設計の要点

1. **判定の根拠は局定義のフラグ**。サーバは `world.Host.Debug.GenerationTrace`（preset の
   `debug.generation_trace`）をクライアントが使える形に変換して渡す。クライアントはフラグ名を知らず、
   「この通話で使える機能」だけを知る。
2. **`capabilities` は `world.Host` に入れず、メッセージの兄弟フィールドにする**。
   `world.Host` のフラグは `json:"-"`（スナップショットと既存のワイヤ形式を変えないため）なので、
   `host` オブジェクトの中には出ない。`serverMessage` に `capabilities` を追加する。
3. **欠けていれば無効**。古いサーバや、`capabilities` を送らない経路では全機能オフとして扱う。
4. **通話の寿命に合わせて保持する**。`VirtualModem` が接続時に受け取り、`CallState` として
   `App.tsx` に渡す。切断・再発信・回線断で破棄する。

## ワイヤ形式（追加分のみ）

```json
{"type":"dial_result","result":"connect","baud":14400,"line":2,"session_id":"...","host":{...},"capabilities":{"generation_trace":true}}
{"type":"resume_result","result":"ok","session_id":"...","baud":14400,"line":2,"host":{...},"capabilities":{"generation_trace":true}}
```

- `capabilities` は接続成功時に常に付く（全部 false でもオブジェクトを送る）。
- キーは snake_case（既存メッセージに合わせる）。TypeScript 側の型は camelCase に変換する。
- 新しい機能を足すときは、`capabilities` にキーを追加する。既存キーの意味は変えない。

## 変更するファイル

| 層 | ファイル | 内容 |
|---|---|---|
| Go | `apps/server/internal/ws/session.go` | `hostCapabilities` 型、`capabilitiesFor`、`serverMessage.Capabilities`、2 か所でセット |
| Go | `apps/server/internal/ws/capabilities_test.go`（新規） | 変換と JSON 形式のテスト |
| TS | `apps/web/src/modem/HostCapabilities.ts`（新規） | 型、`noCapabilities`、`parseHostCapabilities` |
| TS | `apps/web/src/modem/HostCapabilities.test.ts`（新規） | パーサのテスト |
| TS | `apps/web/src/modem/VirtualModem.ts` | `CallState` に `capabilities` を追加、接続時に保持、切断時に破棄 |
| TS | `apps/web/src/modem/VirtualModem.test.ts`（既存に追記） | `onCallState` に `capabilities` が載るテスト |
| TS | `apps/web/src/App.tsx` | 電話番号の比較を `capabilities` に置き換え、局名を含む文言を中立にする |
| TS | `apps/web/src/debug/GenerationInspector.tsx` と同 `.test.ts` | 利用者に見える文言から局名を除く |
| Doc | `packages/protocol/README.md` | `capabilities` を追記 |
| Doc | `specs/002-host-capabilities/verification.md`（新規） | 実行したコマンドと結果の記録 |

## 検証方針

- 自動: Go のユニットテスト、vitest、`vite build`（型チェックを含む）。
- 手動: ローカルでサーバと Web を起動し、HAKATA（`ATDT0920000196`）でボタンが出ること、
  フラグを持たない局（`ATDT0459999999`、最初の 4 回は BUSY なので AUTO REDIAL で 5 回目に接続）で
  出ないことを確認する。手動確認ができない環境では、`verification.md` に「未実施」と書く。
  実行していないものを実行したと書かない。

## 未決事項（spec 003: センターディレクトリ用）

この spec では扱わないが、003 を書く前に決める必要がある。

1. `CenterDirectory.ts` の `DEFAULT_CENTERS`（HAKATA 固定）は、現在の `App.tsx` では
   サーバのワールド生成の応答（`/api/world/bootstrap`）が成功すると丸ごと置き換えられる。
   プリセット局（HAKATA）をディレクトリに出す経路を、どこに置くか。
2. サーバの `world.Host` には `Listed` と `DialMode` がない（`HostDescriptor` にはある）。
   一覧 API を作るには、`Host` への追加か、`hostcatalog` の descriptor を直接返す経路が必要。
3. ローカル保存の「カスタム局」（`zutto.centers.v1`）との統合方法。
4. サーバに接続できないとき（スタンドアロン）、ディレクトリに何を出すか。
