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

## T004 ポーリングのテスト

| コマンド | 結果 |
|---|---|
| `go -C apps/server test ./internal/ws -count=1` | ok |
| `go -C apps/server test -race ./internal/ws -count=1` | ok |

## T009 / T013 の確認

| コマンド | 結果 |
|---|---|
| `go -C apps/server test ./internal/world ./internal/callplan ./internal/worldrepo ./internal/worldpersist ./internal/hostcatalog -count=1` | すべて ok |
| `go -C apps/server test ./internal/hostprogram/... ./internal/hostcatalog ./cmd/server -count=1` | すべて ok |
| `git diff --stat origin/main -- apps/server/internal/hostprogram/erikak/testdata apps/server/internal/world/testdata` | 出力なし（金型の差分なし） |

## T015 全体の確認（HEAD fba7330）

| コマンド | 結果 |
|---|---|
| `go -C apps/server test ./... -count=1` | 26 パッケージ ok、失敗 0 |
| `go -C apps/server vet ./...` | エラー 0 |
| `go -C apps/server test -race ./internal/ws ./internal/worldrepo -count=1` | ok（ws 1.2 s、worldrepo 57.6 s。T015 の時点の HEAD c052160 で実行。その後の変更は YAML の 1 文字だけ） |
| `grep -rn "電報\|\"CALL\"\|\"CHAT\"" apps/server/internal/ws apps/server/internal/world apps/server/internal/callplan apps/server/internal/worldrepo` | 下の 2 行 |

```text
apps/server/internal/world/presence.go:63:// paging -> "CALL", chat -> "CHAT"). See spec 016.
apps/server/internal/worldrepo/bbs_situation_open_topics_test.go:81:        {"CHAT", world.Board{ID: "68/1", Name: "深夜雑談", SemanticScope: "深夜に接続している会員の雑談。"}},
```

1 行目は `world/presence.go` の状態名の対応表のコメント（許される行）。2 行目は `main` にすでにある掲示板の題名のテスト
（`0892531` で追加。チャットとは関係がなく、本仕様では変更していない）。

## 既存のテストの変更

`erikak/runtime_test.go` の `TestWHOShowsWorldPresence` は、HAKATA CANAL NET（`0920000196`）で旧スタブのチャット画面の `WHO` を
確かめていた。T012 で HAKATA に `erika_k.chat` を加えたので、`4` / `C` は電報の画面に進み、このテストは失敗した。
テストの意図（`erika_k.chat` のない局のスタブの `WHO` が世界の接続を出す）を保つため、このテストの `whoStore` に、局の定義から
`erika_k.chat` を外す `HostDetail` を加えた。アサーションは変えていない。

## ローカルのサーバーでの手動確認

`ADDR=127.0.0.1:18080` でサーバーを起動し（Azure OpenAI のキーなし。したがって住民はチャット室に入らない）、Node の WebSocket で
`0920000196` に接続して、`""`（GUEST）、`4`、`""`、`1`、`2`、`テストです(^^)`、`X`、`0`、`こんばんは`、`..`、`""` を送った。
世界時刻は 1996-10-12 の午後で、回線 2、3 は空いていた。出力の抜粋（BEL は `\a` と表記）:

```text
#01 GUEST     WHO       14K-...
#02 ring
#03 ring

[回線番号]=電報 [ﾘﾀｰﾝ]=WHO [X]=CHAT [.]=戻る
CALL> ->? COMMAND ERROR
...
ルーム番号をどうぞ  （中止はﾘﾀｰﾝｷｰ） -->

  チャット終了は 「..」 または 「CTRL+B」 を入力して下さい
------------------------------------------------------------------------------
  ロビーはどなたでもどうぞ。はじめての方もお気軽に(^^)
------------------------------------------------------------------------------
------------------------------------CHAT ON-----------------------------------
\a\a## chat on  GUEST ##

GUEST   :こんばんは

GUEST   :..

\a\a## chat off GUEST ##
------------------------------------CHAT OFF----------------------------------

#01 GUEST     WHO       14K-...
#02 ring
#03 ring
```

`1`（自分の回線）と `2`（空き回線）は `? COMMAND ERROR` になり、電報の本文には進まなかった。
このとき電報の画面の見出しの `〖〗` が PC-98 の Shift_JIS に変換できず `?` と表示されたので、`【】` に直した（`fba7330`）。

## 実施しなかった確認

- Web 端末（`apps/web`）での表示は確かめていない。とくに、入力した行が端末にどう残るか（各応答の先頭の改行と、チャット室での自分の行の表示の重なり）は、
  上の生の出力でしか見ていない。
- 住民がチャット室に出入りする様子は、Azure OpenAI のキーがない手元のサーバーでは出ないため、手では確かめていない。
  `worldrepo` の `TestErikaKChatThroughRepository`（実際の局と住民、チャット有効、`Poll` を 2 時間分）で確かめた。
- `ChatEvents` 1 回の所要時間は単独では測っていない（上の `Poll` 360 回の合計からの平均だけ）。
- 対話の台帳（`InteractionEvent`）は、これまでと同じくスナップショットに保存されない。保存・復元するのは発言と回線宛メッセージの記録（`chat_utterances`、`line_messages`）だけ。
