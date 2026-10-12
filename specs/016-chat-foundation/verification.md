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

## T008 WHO 1 回分（`CallActivitiesNow`）の所要時間

`go -C apps/server test ./internal/worldrepo -run TestCallActivitiesNowTiming -count=3 -v`（チャット有効、部屋 2 室、
`0920000196` の局に住民を足して 326 人、5 つの時刻の最悪値）:

| 回 | 最悪値 |
|---|---|
| 1 | 98.2 ms |
| 2 | 100.2 ms |
| 3 | 96.6 ms |

（同じ環境で最初に 1 回だけ実行したときは 128.5 ms。）いずれも 200 ms 未満なので、`plan.md` §9 のキャッシュは入れていない。
`ChatEvents` も同じ量の計算（10 分ごとの `presenceAt` を 4 時間分と、区間の両端）をするため、チャット室にいる人間 1 人あたり
ポーリング 1 回（1 秒）に約 100 ms かかる見込み（測定はしていない）。

### 追記: 10 分窓のキャッシュ（`plan.md` §9 の対策）を入れた

T008 の時点（上の表）では 200 ms 未満なのでキャッシュを入れなかった。その後、T012 で
`TestErikaKChatThroughRepository`（チャット室に入った人間の `Poll` を世界時刻で 20 秒ずつ 360 回呼ぶ）を書いたところ、
`ChatEvents` が毎回 4 時間分の `presenceAt` をやり直すため、このテスト 1 本に 38.3 s かかった（`Poll` 1 回 約 100 ms）。
ポーリングは 1 秒ごとなので、§9 の対策（局ごとの 10 分窓の `presence.Result` のキャッシュ。保存済みの接続、住民の数、
人間の通話が変わったら捨てる）を `worldrepo/callplan.go` に入れた。

| 測定 | キャッシュなし | キャッシュあり |
|---|---|---|
| `TestErikaKChatThroughRepository`（`Poll` 360 回） | 38.3 s | 4.5 s |
| `TestCallActivitiesNowTiming` の最悪値（その時刻の最初の WHO） | 96.6〜128.5 ms | 81.4〜124.6 ms |
| 同じ時刻の 3 回目の WHO（最初の WHO が保存した後） | 測定なし | 7.7〜13.5 ms |

最初の WHO は、見たものを保存して指紋が変わるため、キャッシュが効くのは次の保存までの呼び出し。
