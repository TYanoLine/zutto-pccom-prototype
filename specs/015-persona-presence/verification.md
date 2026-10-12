# Verification 015: persona-presence

## T001 ベースライン（変更前、`main` d5a36ff）

| コマンド | 結果 |
|---|---|
| `go -C apps/server test ./... -count=1` | 全パッケージ ok（失敗 0） |
| `go -C apps/server vet ./...` | エラー 0 |
| `npm --prefix apps/web test` | 22 ファイル、123 テスト、すべて passed |

## 実装後（ブランチ `claude/project-thread-76jmo1`）

| タスク | コマンド | 結果 |
|---|---|---|
| T002 | `go -C apps/server test ./internal/worldclock -count=1` | ok（§3.3 の表、1996 年の曜日、基準年 1990、基準年が開始年より後でエラー） |
| T003 | `go -C apps/server test ./internal/config ./cmd/server -count=1` | ok |
| T004 | `npm --prefix apps/web test`、`npx tsc -b`（`apps/web`） | 22 ファイル、131 テスト passed（+8）。型検査 ok |
| T006 | `go -C apps/server test ./internal/worldcalendar -count=1` | ok（祝日 19 日がクライアントの集合と一致、振替休日、期間、データのない年） |
| T008–T010 | `go -C apps/server test ./internal/presence -count=1 -v` | 全テスト PASS（決定性、1,000 時刻で回線数以下、テレホタイム、平日昼、保存済みの接続の優先、層ごとの係数） |
| T011 | `go -C apps/server test ./internal/world -run 'TestPopulation' -count=3` | ok（人物生成の金型は無変更） |
| T012 | `go -C apps/server test ./internal/worldrepo -count=1` | ok（観測の保存と不変、同時観測で保存 1 回、人間の通話の占有と解放、Repository 経由の WHO） |
| T013 | `go -C apps/server test ./internal/worldpersist -count=1` | ok（接続の保存と復元、古いスナップショットの読み込み） |
| T014 | `go -C apps/server test ./internal/telephone -count=1` | ok |
| T015 | `go -C apps/server test ./internal/ws -count=1` | ok |
| T016–T017 | `go -C apps/server test ./internal/hostprogram/... -count=1` | ok（画面金型は無変更で一致） |
| T019 | `go -C apps/server test ./... -count=1` | 全パッケージ ok |
| T019 | `go -C apps/server vet ./...` | エラー 0 |
| T019 | `git diff --stat origin/main -- apps/server/internal/hostprogram/erikak/testdata apps/server/internal/world/testdata` | 出力なし（金型ファイルの差分なし） |

### T019 の grep について（tasks.md との食い違い）

`grep -n '"MARI\|MARI ' apps/server/internal/hostprogram/erikak/runtime.go` は 2 行を返した。
メンバーリスト（`MEMB`）と、別の固定文面（870 行目付近）である。アクセス記録（`T`）の固定の名前も、
grep のパターンには掛からないが残っている。tasks.md は「アクセス記録の 1 行だけが残る」と書いていたが、実際には WHO 以外の固定の名前が 3 か所ある。
いずれも spec の範囲外（WHO だけを差し替える）なので変更していない。

### TestPresenceHistogram（300 人、8 回線、TelehoFriendly の試験局。各時刻 :30 の接続数）

`go -C apps/server test ./internal/presence -run TestPresenceHistogram -presence-histogram -v -count=1`

```text
1996-10-15（火）: 00=8 01=8 02=6 03=3 04=1 05=1 06=4 07=3 08=0 09=0 10=0 11=1 12=1 13=1 14=0 15=0 16=1 17=0 18=0 19=1 20=1 21=0 22=0 23=7
1996-10-19（土）: 00=8 01=8 02=8 03=4 04=1 05=1 06=1 07=1 08=1 09=0 10=1 11=0 12=0 13=0 14=2 15=2 16=0 17=0 18=1 19=0 20=2 21=0 22=1 23=8
1996-12-24（火）: 00=8 01=7 02=5 03=3 04=1 05=0 06=2 07=2 08=0 09=0 10=1 11=0 12=0 13=1 14=0 15=0 16=2 17=0 18=1 19=1 20=2 21=2 22=1 23=8
1996-12-31（火）: 00=8 01=7 02=6 03=3 04=0 05=0 06=3 07=3 08=0 09=0 10=0 11=0 12=1 13=0 14=1 15=0 16=1 17=0 18=0 19=0 20=0 21=2 22=1 23=8
```

23 時から 2 時ごろまで 8 回線が埋まり、昼と 19〜22 時はほぼ空く。19〜22 時が静かすぎる可能性がある
（テレホ待ちと共有回線の係数が重なるため）。係数の調整は spec のとおり別 PR とする。

### 実施しなかった確認

- 実際のサーバーを起動しての WHO / BUSY の手動確認: 未実施。この環境ではブラウザから端末を操作していない。
- plan.md §11 の「起動時、保存済みの最新の記事が世界の現在より後なら警告を出す」: 実装していない（リスク対策として書かれていたもので、タスクには含まれていなかった）。
- 1996 年の `events`（接続に影響する史実の出来事）: 一次資料で日時を確認できたものがなく、空のまま（`docs/research/CALENDAR_1996.md`）。

### plan.md からの設計上の差分

- `world.LineOccupancy` は plan の `BeginHumanCall(call HumanCall) (line, ok)` ではなく、`DialLine(host, at)`、`BeginHumanCall(host, callID, baud)`、
  `SetHumanCallHandle`、`EndHumanCall(callID)` とした。空き回線の判定に住民の在席の計算が必要で、計算は `world` より上の `worldrepo` にしか置けないため
  （`presence` は `world` を参照する）。保存だけを `world.PresenceStore`（`MemoryStore` が実装）に置いた。
- `operating_hours` は `world.Host` / `hostcatalog.Traits` で `[2]string` とした（`world.Host` を `==` で比較しているテストがあり、スライスにできないため）。
- `worldpersist.Store.SavePersonaFact` を、バッチ中なら書き込みを最後にまとめるようにした（外の挙動は同じ）。WHO 1 回で住民ごとに 5 件の人物事実を保存するため。
