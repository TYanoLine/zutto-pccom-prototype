# Verification 016: chat-foundation

## T001 ベースライン（変更前、`main` a3ecc15）

| コマンド | 結果 |
|---|---|
| `go -C apps/server test ./... -count=1` | 全パッケージ ok（失敗 0） |
| `go -C apps/server vet ./...` | エラー 0 |

## T006 `chat_rooms` の置き場所

`world.Host` ではなく、局の定義の `detail.chat_rooms`（`hostcatalog.PresetDetail.ChatRooms`）に置いた。
`world.ChatRoom` は `world.ChatRoomsOf(detail)` で作る。

理由: `world.Host` にスライスの項目を足すと、`internal/world/preset_hosts_test.go:34` の `got != w` が
コンパイルできなくなる（試しに `[]int` の項目を足して `go vet ./...` で確認した:
`invalid operation: got != w (struct containing []int cannot be compared)`）。`HostDetail` は不変の局の定義の一部で、
`erika_k` の節もここにあるため、共通の項目（`chat_rooms`）と具現化の節（`erika_k.chat`）が同じ場所に並ぶ。
