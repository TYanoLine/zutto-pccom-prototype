# Spec: Web 側の局固有判定をサーバ提供の capabilities に置き換える

**Spec ID**: 002-host-capabilities
**Status**: Draft
**Depends on**: PR #294（`debug.*` / `generation.*` フラグ。マージ済み）

## 背景

PR #294 で、サーバ側の「HAKATA だけが持つデバッグ挙動」は、局定義（preset YAML）の
フラグ（`debug.generation_trace` など）で切り替わるようになった。しかし Web クライアントには
HAKATA の電話番号を直接比較する判定が残っている。

```ts
// apps/web/src/App.tsx
const hakataTraceVisible = activeCall?.phone === '0920000196' && !standaloneLine;
```

この比較のせいで、別の局が `debug.generation_trace: true` を持っても「生成ログ」ボタンが出ず、
逆にサーバ側でフラグを外してもボタンが出続ける。判定の根拠をサーバ側のフラグに一本化する。

## ユーザーストーリー

### US1: 生成ログのボタンは、接続先の局がフラグを持つときだけ出る（Priority: P1）

運営者として、`debug.generation_trace: true` の局に接続したときだけ「生成ログ」ボタンを見たい。
局の電話番号や ID をクライアントのコードに書かずに、局の追加・変更に追従してほしい。

**Independent Test**: HAKATA に接続するとボタンが出る。`debug.generation_trace` を持たない局
（`ATDT0459999999`）に接続するとボタンが出ない。クライアントのコードに電話番号の比較がない。

## 要件

- **FR-001**: サーバは、接続成功時の `dial_result` メッセージに `capabilities` を含める。
  `capabilities.generation_trace` は、その局の `debug.generation_trace` と一致する。
- **FR-002**: サーバは、`resume_result`（`result` が `ok`）にも同じ `capabilities` を含める。
- **FR-003**: `capabilities` が欠けている場合（古いサーバ）、クライアントは全機能を無効として扱う。
- **FR-004**: クライアントは、接続ごとに受け取った `capabilities` を接続状態（`CallState`）に保持し、
  切断・再発信・回線断のたびに無効状態へ戻す。
- **FR-005**: 「生成ログ」ボタン（デスクトップ・モバイル）と `GenerationInspector` の `active` は、
  `capabilities.generation_trace` が `true` で、かつスタンドアロン（サーバなし）でないときだけ有効にする。
- **FR-006**: 共通のクライアントコードに、`0920000196`・`hakata`・`HAKATA` を条件とする判定を置かない。
  利用者に見える文言（ボタンの aria-label、ダイアログの見出し、エラーメッセージ）も局名を含めない。
- **FR-007**: `capabilities` の追加は後方互換にする。既存フィールド（`host` を含む）は変更しない。

## 成功基準

- **SC-001**: `go -C apps/server test ./...` と `npm --prefix apps/web test` が通る。
- **SC-002**: `npm --prefix apps/web run build` が通る（型エラーなし）。
- **SC-003**: `apps/web/src` の App.tsx 内に、生成ログ表示の可否を電話番号で決める記述が残っていない。
- **SC-004**: 手動確認で、HAKATA ではボタンが出て、`0459999999`（フラグなしの局）では出ない。

## スコープ外（この spec では変更しない）

- **センターディレクトリ**（`apps/web/src/modem/CenterDirectory.ts` の `DEFAULT_CENTERS`）。
  サーバから局の一覧を返す API が必要で、ワールド生成の局やローカル保存の局との関係を
  先に決める必要がある。別 spec（003）で扱う。未決事項は `plan.md` の末尾に記録した。
- **テレホーダイ番号の既定値**（`App.tsx` の `VITE_TELEHODAI_NUMBERS` の既定値 `0920000196`）。
  料金計算に使われるため、別途判断する。
- **ターミナル・モードの案内文の例示番号**（`ATDT0920000196`）。実在する番号の例示であり、判定ではない。
- 環境変数名（`DEBUG_HAKATA_LLM_TRACE` など）と、Go 側の `HAKATA` を含む識別子の変更。
- 住民生成、Erika-K の板構成の外部化。
