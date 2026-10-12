# Verification: 人間の識別子（019-human-identity）

## T001 ベースライン（変更前、`main` 777c04f）

| コマンド | 結果 |
|---|---|
| `go -C apps/server test ./... -count=1` | 全パッケージ ok |
| `go -C apps/server vet ./...` | エラー 0 |
| `npm --prefix apps/web test` | 22 ファイル、131 テスト ok |
| `npm --prefix apps/web run build` | ok |

`plan.md` §1 の位置は、`main` の実際の値と一致した（`ws/sessions.go:90` の `Create` は 91 と書いたが 1 行ずれのみ）。
`memberAliases` は `worldrepo/chat_talk.go:452`、呼び出しは `:247`。
