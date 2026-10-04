# Tasks: host capabilities

**Input**: `specs/002-host-capabilities/spec.md`、`plan.md`
**Prerequisites**: `AGENTS.md`、PR #294 がマージ済みの `main`（`world.Host.Debug.GenerationTrace` が存在する）

## この作業のルール（必読）

- 作業ブランチ名は `feature/002-host-capabilities`。`main` に直接コミットしない。
- **このファイルに書いていないファイルは変更しない**。変更が必要に見えたら、変更せずに報告する。
- 1 タスク = 1 コミット。コミットメッセージは `T00X: 内容`。
- 各 Phase の終わりにテストを実行する。**実行していないテストを「通った」と書かない**。
  実行できなかった場合は、できなかった理由とコマンドをそのまま `verification.md` に書く。
- 既存のテストが失敗したら、まず自分の変更が原因かを確認する。原因が自分の変更で、
  この spec の挙動変更に伴う期待値の更新なら直してよい。それ以外のテストは触らない。
  2 回直して通らなければ、そこで止めて状況を報告する。
- スコープ外（`spec.md` 末尾）に手を出さない。特に `CenterDirectory.ts`、
  `VITE_TELEHODAI_NUMBERS` の既定値、ターミナル・モードの例示番号は変更しない。

## Format: `[ID] [P?] Description`

- **[P]**: 他のタスクと別ファイルで、並行して進められる。迷ったら順番どおりに進める。

---

## Phase 1: サーバ（Go）

- [ ] **T001** `apps/server/internal/ws/session.go` に `capabilities` を追加する

  1. `serverMessage` 構造体の `Text` の下に、次のフィールドを追加する。

     ```go
     Capabilities *hostCapabilities `json:"capabilities,omitempty"`
     ```

  2. `serverMessage` の定義の直後に、次の型と関数を追加する。

     ```go
     // hostCapabilities tells the client which optional, host-specific features it
     // may offer for this call. It is derived from the host definition's flags, so
     // the client never needs to know a phone number or a host ID.
     type hostCapabilities struct {
     	GenerationTrace bool `json:"generation_trace"`
     }

     func capabilitiesFor(host world.Host) *hostCapabilities {
     	return &hostCapabilities{GenerationTrace: host.Debug.GenerationTrace}
     }
     ```

  3. `case "dial":` の中で、`sm.Host = &active.Host` の次の行に追加する。

     ```go
     sm.Capabilities = capabilitiesFor(active.Host)
     ```

  4. `case "resume":` の中の、`result` が `ok` のときに送る `serverMessage{...}`
     （`Type: "resume_result", Result: "ok"` を持つもの）に、`Host: &active.Host,` の次の行として追加する。

     ```go
     Capabilities: capabilitiesFor(active.Host),
     ```

  - 他の `serverMessage`（`error`、`terminal`、`carrier`、失敗時の `resume_result`）には付けない。

- [ ] **T002** `apps/server/internal/ws/capabilities_test.go`（新規）を作る

  次の 3 つのテストを書く。パッケージは `ws`。`encoding/json`、`strings`、`testing`、
  `zutto-pccom/apps/server/internal/hostcatalog`、`zutto-pccom/apps/server/internal/world` を使う。

  1. `TestCapabilitiesFollowTheHostFlag`:
     `world.Host{ID: "a", Debug: hostcatalog.DebugFlags{GenerationTrace: true}}` では
     `capabilitiesFor(...).GenerationTrace` が `true`、`world.Host{ID: "b"}` では `false`。
  2. `TestCapabilitiesIgnoreTheRoleAndTheIdentity`:
     `world.Host{ID: "hakata-canal-net", Phone: "0920000196", Role: hostcatalog.RoleExperiment}`
     （フラグなし）では `GenerationTrace` が `false`。
  3. `TestServerMessageEncodesCapabilities`:
     `json.Marshal(serverMessage{Type: "dial_result", Capabilities: &hostCapabilities{GenerationTrace: true}})`
     の結果が `"capabilities":{"generation_trace":true}` を含む。
     `Capabilities` が `nil` のときは `"capabilities"` を含まない。

- [ ] **T003** Phase 1 の確認

  ```bash
  go -C apps/server build ./... && go -C apps/server test ./internal/ws/...
  ```

  失敗したら修正する。結果を後で `verification.md` に書くので、コマンドと出力の要点を控える。

**Checkpoint**: サーバが `capabilities` を送る。Web 側はまだ使っていない。

---

## Phase 2: Web の土台

- [ ] **T004** `apps/web/src/modem/HostCapabilities.ts`（新規）を作る

  ```ts
  // What the connected host lets this client offer. It comes from the server in
  // `dial_result` / `resume_result` (`capabilities`), never from a phone number.
  export type HostCapabilities = {
    generationTrace: boolean;
  };

  export function noCapabilities(): HostCapabilities {
    return { generationTrace: false };
  }

  // Anything that is not an object, or a flag that is not exactly `true`, is off.
  // An older server that sends no `capabilities` therefore enables nothing.
  export function parseHostCapabilities(value: unknown): HostCapabilities {
    if (!value || typeof value !== 'object') return noCapabilities();
    const raw = value as { generation_trace?: unknown };
    return { generationTrace: raw.generation_trace === true };
  }
  ```

- [ ] **T005** [P] `apps/web/src/modem/HostCapabilities.test.ts`（新規）を作る

  vitest（`import { describe, expect, it } from 'vitest';`）で次を確認する。

  - `parseHostCapabilities(undefined)`、`null`、`'x'`、`42` は `{ generationTrace: false }`。
  - `parseHostCapabilities({ generation_trace: true })` は `{ generationTrace: true }`。
  - `parseHostCapabilities({ generation_trace: 'true' })` と `{ generation_trace: 1 }` は `false`。
  - `parseHostCapabilities({})` は `false`。
  - `noCapabilities()` を 2 回呼んで、別のオブジェクトが返る（`not.toBe`）。

- [ ] **T006** `apps/web/src/modem/VirtualModem.ts` を変更する

  1. import を追加する。

     ```ts
     import { noCapabilities, parseHostCapabilities } from './HostCapabilities';
     import type { HostCapabilities } from './HostCapabilities';
     ```

  2. `type ServerMessage` に `capabilities?: unknown;` を追加する。

  3. `CallState` を次に変更する。

     ```ts
     export type CallState = { phone: string; baud: number; capabilities: HostCapabilities } | null;
     ```

  4. クラスのフィールド `private sessionID = '';` の次の行に追加する。

     ```ts
     private capabilities: HostCapabilities = noCapabilities();
     ```

  5. `this.sessionID = '';` と書かれている箇所は 4 つある（`hangup()`、`dispose()`、`dial()`、
     `finishCarrierLoss()`）。**4 つすべて**で、その次の行に追加する。

     ```ts
     this.capabilities = noCapabilities();
     ```

     （`this.sessionID = msg.session_id ?? ...` のような別の代入には付けない。）

  6. `handleServer` の `if (msg.result === 'connect') {` の中で、
     `this.sessionID = msg.session_id ?? '';` の次の行に追加する。

     ```ts
     this.capabilities = parseHostCapabilities(msg.capabilities);
     ```

  7. `finishRemoteConnect` の `this.onCallState?.({ phone, baud });` を次に変更する。

     ```ts
     this.onCallState?.({ phone, baud, capabilities: this.capabilities });
     ```

  - `resume_result` の処理では `capabilities` を読まない（接続中の値を保つ）。

- [ ] **T007** `apps/web/src/modem/VirtualModem.test.ts` にテストを追記する

  1. 先に `VirtualModem.test.ts` と `VirtualModem.telemetry.test.ts` を読み、`connect` の
     `dial_result` を受けて `onCallState` が呼ばれるまでの既存の準備（fake socket、fake timer、
     audio の指定）をそのまま真似る。新しい準備方法を作らない。
  2. 次の 3 つのテストを追加する。
     - `capabilities` に `{ generation_trace: true }` を載せた `connect` を受けると、
       `onCallState` が `capabilities.generationTrace === true` で呼ばれる。
     - `capabilities` を載せない `connect` では `generationTrace === false`。
     - `generation_trace: true` で接続して `hangup()` した後、`capabilities` なしで再発信して
       接続すると `generationTrace === false`（前の通話の値を引きずらない）。
  3. 既存のテストに `onCallState` の引数を `{ phone, baud }` ちょうどで比較しているものがあれば、
     `capabilities: { generationTrace: false }` を加えた期待値に更新する。

- [ ] **T008** Phase 2 の確認

  ```bash
  npm --prefix apps/web test
  ```

  `App.tsx` はまだ古い `CallState` を前提にしているので、型エラーが出る場合は T009 まで進めてよい
  （vitest は型を検査しない）。

**Checkpoint**: `VirtualModem` が通話ごとの `capabilities` を持つ。

---

## Phase 3: `App.tsx` と文言

- [ ] **T009** `apps/web/src/App.tsx` を変更する

  1. import を追加する。

     ```ts
     import type { HostCapabilities } from './modem/HostCapabilities';
     ```

  2. `type ActiveCall = { phone: string; connectedAt: Date };` を次に変更する。

     ```ts
     type ActiveCall = { phone: string; connectedAt: Date; capabilities: HostCapabilities };
     ```

  3. `modem.onCallState = call => {...}` の中の
     `return call ? { phone: call.phone, connectedAt: now } : null;` を次に変更する。

     ```ts
     return call ? { phone: call.phone, connectedAt: now, capabilities: call.capabilities } : null;
     ```

  4. 次の 1 行

     ```ts
     const hakataTraceVisible = activeCall?.phone === '0920000196' && !standaloneLine;
     ```

     を次に置き換える。

     ```ts
     const generationTraceVisible = activeCall?.capabilities.generationTrace === true && !standaloneLine;
     ```

  5. ファイル内の `hakataTraceVisible` の使用箇所（デスクトップのボタン、モバイルのボタン、
     `<GenerationInspector ... active={...} />` の 3 か所）をすべて `generationTraceVisible` に変える。

  6. モバイルのボタンの `aria-label="HAKATA生成ログを開く"` を `aria-label="生成ログを開く"` にする。

  - これ以外の行（特に `telehodaiNumbers` の既定値と、ターミナル・モードの `ATDT0920000196` の
    案内文）は変更しない。

- [ ] **T010** [P] `apps/web/src/debug/GenerationInspector.tsx` と `GenerationInspector.test.ts` の文言を直す

  利用者に見える文言とコメントから局名を除く。ロジックは変えない。

  | 変更前 | 変更後 |
  |---|---|
  | `サーバー側でHAKATA生成ログが無効になっています。` | `サーバー側で生成ログが無効になっています。` |
  | `aria-label="HAKATA 生成デバッグ"` | `aria-label="生成デバッグ"` |
  | `<strong>HAKATA 生成デバッグ</strong>` | `<strong>生成デバッグ</strong>` |
  | `HAKATAの板を開くか記事を読むと` | `対象局の板を開くか記事を読むと` |
  | `// This is a modern HAKATA evaluation overlay.` | `// This is a modern evaluation overlay.` |

  `GenerationInspector.test.ts` に上の文言を検査している箇所があれば、同じ文言に合わせて更新する。
  `describe('HAKATA generation trace API', ...)` は `describe('generation trace API', ...)` にする。

- [ ] **T011** Phase 3 の確認

  ```bash
  npm --prefix apps/web test
  npm --prefix apps/web run build
  grep -rn "0920000196\|HAKATA\|hakata" apps/web/src
  ```

  `grep` の結果に残ってよいのは次だけ。これ以外が残っていたら、スコープ外かどうかを判断して報告する。

  - `apps/web/src/App.tsx` の `VITE_TELEHODAI_NUMBERS` の既定値と、ターミナル・モードの案内文の例示番号。
  - `apps/web/src/modem/CenterDirectory.ts`（`DEFAULT_CENTERS`。スコープ外）。
  - `apps/web/src/billing/PseudoTariffService.test.ts`（テスト用の番号）。

**Checkpoint**: 「生成ログ」の可否が `capabilities` だけで決まる。

---

## Phase 4: ドキュメントと検証

- [ ] **T012** [P] `packages/protocol/README.md` に `capabilities` を追記する

  `Server -> client:` のコード例の `dial_result` の `connect` の行を次に変更する。

  ```json
  {"type":"dial_result","result":"connect","baud":14400,"line":2,"host":{...},"capabilities":{"generation_trace":false}}
  ```

  コード例の後に、次の段落を追加する。

  ```markdown
  `capabilities` is sent with every successful `dial_result` (`connect`) and `resume_result`
  (`ok`). It lists the optional, host-specific features the client may offer for this call; every
  key is `false` unless the host definition turns it on. A client must treat a missing
  `capabilities` (an older server) as all features off. New features add keys; existing keys keep
  their meaning. `generation_trace` mirrors the host's `debug.generation_trace` flag.
  ```

- [ ] **T013** 全体の確認と記録

  ```bash
  npm test
  npm --prefix apps/web run build
  ```

  結果を `specs/002-host-capabilities/verification.md`（新規）に、次の形式で記録する。

  ```markdown
  # Verification: host capabilities

  | コマンド | 結果 | 備考 |
  |---|---|---|
  | `go -C apps/server test ./...` | 成功 / 失敗 / 未実施 | 失敗・未実施なら理由 |
  | `npm --prefix apps/web test` | 〃 | |
  | `npm --prefix apps/web run build` | 〃 | |

  ## 手動確認
  - HAKATA（`ATDT0920000196`）で「生成ログ」ボタンが出る: 確認済み / 未実施
  - フラグなしの局（`ATDT0459999999`、AUTO REDIAL で 5 回目に接続）でボタンが出ない: 確認済み / 未実施
  - 切断後にボタンが消える: 確認済み / 未実施

  ## 残った HAKATA / 0920000196 の参照（grep 結果）
  （T011 の grep の結果を貼り、スコープ外であることを注記する）
  ```

  実行できなかった項目は「未実施」と書き、理由を添える。成功と書けるのは、実際に実行して成功を確認したものだけ。

- [ ] **T014** PR を作る

  - ブランチ: `feature/002-host-capabilities` → `main`。**draft** で作成する。
  - タイトル: `Web: 生成ログの表示可否をサーバの capabilities で決める`
  - 説明に含める: 目的（PR #294 の続き。Web に残る電話番号比較の除去）、変更の要約、
    `verification.md` の結果（未実施の項目も含めて正直に）、スコープ外の項目、
    互換性（`capabilities` が無い古いサーバでは機能が無効になる）。

---

## Dependencies & Execution Order

```text
Phase 1 (T001 → T002 → T003)
  └─ Phase 2 (T004 → T005[P], T006 → T007 → T008)
       └─ Phase 3 (T009, T010[P] → T011)
            └─ Phase 4 (T012[P] → T013 → T014)
```

- Phase 1 と Phase 2 の T004〜T005 は互いに独立しているが、順番どおりに進めてよい。
- T006 は T004 に依存する。T009 は T006 に依存する。
- T013 は T011 の結果を使う。

## 完了の定義

- `spec.md` の FR-001〜FR-007 と SC-001〜SC-004 を満たす（手動確認が未実施なら、その旨を明記する）。
- 変更したファイルが `plan.md` の「変更するファイル」の表の範囲に収まっている（`git diff --stat` で確認）。
