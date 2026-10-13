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

## T013 全体の確認（実装後）

| コマンド | 結果 |
|---|---|
| `go -C apps/server test ./... -count=1` | 全パッケージ ok（29 行、FAIL なし） |
| `go -C apps/server vet ./...` | エラー 0 |
| `go -C apps/server test -race ./internal/worldrepo ./internal/ws -count=1` | ok（データ競合の警告なし） |
| `npm --prefix apps/web test` | 23 ファイル、136 テスト ok |
| `npm --prefix apps/web run build` | ok |
| `git diff --stat origin/main -- apps/server/internal/hostprogram/erikak/testdata apps/server/internal/world/testdata` | 差分なし（金型は 1 バイトも変わっていない） |
| `grep -rn memberAliases apps/server/internal` | 何も出ない |

## ストーリーとテスト

| ストーリー | テスト |
|---|---|
| 1 かけ直しても同じ人 | `ws` の `TestHumanIDFollowsTheBrowserKey`、`TestCallsOfOneUserShareTheHumanID` |
| 2 ゲストも同じ人 | `erikak` の `TestAppendKeepsHumanIDAndLoginID`（2 人のゲストの `AuthorHumanID` が異なり、`Author` は `GUEST`） |
| 3 住民はログイン ID でしか知らない | `worldrepo` の `TestResidentsKnowCallersByLoginID`、`TestHumanPostsAreFiledByHumanIDAndKnownByLoginID` |
| 4 出来事の話題もログイン ID | `TestResidentsKnowCallersByLoginID`（枠の入力になる出来事の数）、`TestResidentBringsUpACallersPost` |
| 5 人間 ID は材料に出ない | `TestHumanIDNeverReachesTheWriters`、`actormemory` の `TestHumansAreKnownByLoginIDOnly` |
| 6 自分の判定は通話で | `worldrepo` の `TestOneUserOnTwoCalls` |
| 7 古い記録が読める | `world` の `TestPostActorKeepsOldRecords`、`TestEventKnownActors`、`TestHumanCallSessionNamesTheUser`。新しい欄は `omitempty` で、既存の `worldpersist` の古いスナップショットのテストが通る |

## plan.md と違う点・補った点

- **T005 と T006 は 1 コミット。** 台帳の記録の変更と、チャット室が読む側の変更が、同じ型にまたがり、分けるとビルドが通らない。
- **同じ人が同じ部屋に 2 つの通話で入った場合。** `EnterChatRoom` / `LeaveChatRoom` は `member:<人間 ID>` を渡す（`plan.md` §4 の「小さい方」）。そのため、同じ人の 2 つの通話が同じ部屋にいると、一方の入室がもう一方の滞在を閉じる。
  発言の表示は通話ごとに正しい（`TestOneUserOnTwoCalls`）。住民から見た在室の扱いだけが、同じ人で 1 つにまとまる。通話ごとの在室は、必要になれば別の変更にする。
- **行動の記憶の検索結果に出る GUEST。** `spec.md` FR-010 は「ゲストは Author が空」と書いたが、検索結果の著者が空になり、読む側が誰の書き込みか分からなくなる。
  掲示板に見える名前（`login:GUEST`）をそのまま著者として出した（人間 ID は出ない）。関係と出来事の話題からは、spec どおり GUEST を外している。
- **`CallInfo` は `world` に置いた。** `plan.md` §3 は `hostprogram` に置くとしたが、`hostprogram` が `erikak` を import しているため、`erikak` が使うと import が循環する。
- **`parseActor` は `member` も受け続ける。** `spec.md` は `login` を加えると書いたのみで、`member` を禁じてはいない。ツールを実行する主体は住民だけなので、挙動は変わらない。
  相手の指定（`matchesCounterpart`）は、`member:<ID>` を何にも当てない。
- **既存テストの期待を 2 か所変えた。** `TestAddPost_UpdatesRelationshipEdge`（関係の鍵が `member:<ID>` から `login:<ID>` に変わった）と、`actormemory` の `taro`（`login:TARO`）。いずれも本仕様が変える点で、金型ではない。
  `TestResidentBringsUpACallersPost` は、GUEST では話題にならなくなったため、TARO で書き込んだ場合のテストに書き直した。

## 未実施・限界

- **呼び出し元の鍵がログとスナップショットに出ないことの専用テストはない。** 鍵を読むのは `ws/human.go` の `humanID` だけで、人間 ID が鍵を含まないことは `TestHumanIDFollowsTheBrowserKey` で確かめている。`ws/session.go` は鍵をログに出さない（`log.Printf` は鍵を使わない）が、この点をテストでは確かめていない。
- **ブラウザでの手動確認はしていない。** この環境にブラウザはあるが、実際のサーバ（WebSocket）と端末をつないだ確認はしていない。`VirtualModem` が `caller_key` を送ることは単体テストだけで確かめた。
- **実機の LLM は呼んでいない。**
- 入会登録、パスワード、台帳と関係の永続化は、スコープ外のまま。
