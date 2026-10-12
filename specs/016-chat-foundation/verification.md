# Verification 016: chat-foundation

## T001 ベースライン（変更前、`main` a3ecc15）

| コマンド | 結果 |
|---|---|
| `go -C apps/server test ./... -count=1` | 全パッケージ ok（失敗 0） |
| `go -C apps/server vet ./...` | エラー 0 |
