# Verification 015: persona-presence

## T001 ベースライン（変更前、`main` d5a36ff）

| コマンド | 結果 |
|---|---|
| `go -C apps/server test ./... -count=1` | 全パッケージ ok（失敗 0） |
| `go -C apps/server vet ./...` | エラー 0 |
| `npm --prefix apps/web test` | 22 ファイル、123 テスト、すべて passed |
