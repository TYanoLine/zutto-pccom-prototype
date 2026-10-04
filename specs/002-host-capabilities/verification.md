# Verification: host capabilities

実行日: 2026-10-04。結果は実際に実行して観測したもの。

## 自動テスト

| コマンド | 結果 | 備考 |
|---|---|---|
| `go -C apps/server build ./...` | 成功 | 下の「実行環境の注記」を参照 |
| `go -C apps/server test ./internal/ws/...` | 成功 | `capabilities_test.go` の 3 テストを含む |
| `go -C apps/server test ./...` | **一部失敗** | `internal/worldrepo` の `TestHAKATAAuditLogsOnlyNewCommittedWorldHeaders` のみ失敗。他のパッケージはすべて成功。下の「既存の失敗」を参照 |
| `npm --prefix apps/web test` | 成功 | 18 ファイル / 82 テスト |
| `npm --prefix apps/web run build` | 成功 | `vite build`（vercel.json チェック込み）。型エラーなし |

## 実行環境の注記

- 実行環境では `proxy.golang.org` と `golang.org` に接続できないため、Go の依存
  （`golang.org/x/*`、`gopkg.in/yaml.v3`）は GitHub のミラー
  （`github.com/golang/*`、`github.com/go-yaml/yaml/v3`）に一時的に差し替えて取得した。
  差し替えは `-modfile` で指定した一時ファイル（リポジトリ外）で行い、`go.mod` / `go.sum` は変更していない。
- Go は 1.24.13 を使用（`go.mod` の指定は 1.23.0）。

## 既存の失敗（この変更とは無関係）

- `internal/worldrepo` の `TestHAKATAAuditLogsOnlyNewCommittedWorldHeaders`
  （`bbs_content_log_test.go:87`: `audit leaked to another host`）。
- この変更を入れる前の `main` でも、同じテストを単独で実行して同じ失敗を再現した
  （この変更を入れた状態で 3 回連続、変更を退避した `main` で 1 回、いずれも失敗）。
  Go のコードは `ws` 以外を変更していないため、この spec の挙動変更とは関係しない。
  spec のルールに従い、このテストには触れていない。別途調査が必要。

## 手動確認

実行環境にブラウザと PC-98 端末の実機がないため、次は未実施。

- HAKATA（`ATDT0920000196`）で「生成ログ」ボタンが出る: 未実施
- フラグなしの局（`ATDT0459999999`、AUTO REDIAL で 5 回目に接続）でボタンが出ない: 未実施
- 切断後にボタンが消える: 未実施

挙動は `VirtualModem.test.ts` の 3 テスト（flag あり・なし・再発信で持ち越さない）と
`HostCapabilities.test.ts` で自動的に検証している。

## 残った HAKATA / 0920000196 の参照（grep 結果）

`grep -rn "0920000196\|HAKATA\|hakata" apps/web/src` の結果は次のとおり。すべてスコープ外。

- `apps/web/src/App.tsx:57` — `VITE_TELEHODAI_NUMBERS` の既定値（スコープ外）
- `apps/web/src/App.tsx:212` — ターミナル・モードの案内文 `ATDT0920000196`（スコープ外・例示番号）
- `apps/web/src/modem/CenterDirectory.ts:14-16` — `DEFAULT_CENTERS`（スコープ外）
- `apps/web/src/billing/PseudoTariffService.test.ts`、`apps/web/src/billing/CallerLocation.test.ts` — テスト用の番号
- `apps/web/src/modem/VirtualModem.test.ts`、`VirtualModem.telemetry.test.ts` — テスト用のダイヤル番号とホスト名

`App.tsx` に電話番号による生成ログの表示判定は残っていない。
