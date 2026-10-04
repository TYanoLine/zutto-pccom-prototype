# Verification: generic debug names

| コマンド | 結果 | 備考 |
|---|---|---|
| `go -C apps/server build ./...` | 成功 | 2026-10-04 実行 |
| `go -C apps/server vet ./...` | 成功 | 2026-10-04 実行 |
| `go -C apps/server test ./...` | 失敗 | `internal/worldrepo/TestGeneratedContentAuditLogsOnlyNewCommittedWorldHeaders` が `audit leaked to another host` で失敗。ほかのパッケージは成功 |

## 残った hakata を含む Go ファイル（テストを除く）

grep の結果は次のとおり。いずれも `plan.md` の許可リストに含まれる。

- `apps/server/internal/config/config.go` — 旧環境変数の互換読み込み
- `apps/server/internal/hostcatalog/descriptor.go` — key の例示
- `apps/server/internal/hostcatalog/preset.go` — key の例示
- `apps/server/internal/hostprogram/erikak/runtime.go` — Erika-K の局データ
- `apps/server/internal/world/hakata_cast.go` — 住民生成
- `apps/server/internal/world/host_role.go` — 住民生成に関する ID
- `apps/server/internal/world/store.go` — 住民生成の呼び出し
- `apps/server/cmd/server/runtime_store.go` — 住民生成の呼び出しと復元ログ

## 環境変数の互換（config_test.go）

T002 の各設定について、新名のみ、旧名のみ、両方（新名優先）、どちらも無しの既定値を確認するテーブルテストは成功した。`go -C apps/server test ./internal/config/...` は成功。

## 手動確認

- 旧名（`DEBUG_HAKATA_LLM_TRACE=1` など）でサーバが起動する: 未実施（外部サービス・起動環境を用意していない）
- 新名（`DEBUG_GENERATION_TRACE=1` など）でサーバが起動する: 未実施（外部サービス・起動環境を用意していない）

## その他

`git diff --stat main...HEAD` は未実施。ローカル clone に `main` ref がなく、`main` を解決できなかった。
