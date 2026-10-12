# Verification 017: chat-utterances

## T001 ベースライン（変更前、`main` 059582d）

| コマンド | 結果 |
|---|---|
| `go -C apps/server test ./... -count=1` | 全パッケージ ok（失敗 0） |
| `go -C apps/server vet ./...` | エラー 0 |
