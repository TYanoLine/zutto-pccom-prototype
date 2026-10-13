# Verification: 電報への返信と住民からの電報（018-line-message-replies）

## 実行したコマンドと結果

変更前（`main` の dff0a5e）:

- `go -C apps/server test ./... -count=1` 全 package 成功。
- `go -C apps/server vet ./...` 出力なし。

変更後:

- `go -C apps/server test ./... -count=1` 全 package 成功。
- `go -C apps/server vet ./...` 出力なし。
- `go -C apps/server test -race ./internal/worldrepo ./internal/ws ./internal/lineslot ./internal/hostprogram/... -count=1` 成功。
- `npm --prefix apps/web test` 136 件成功（Web は変更していない）。
- `git diff --stat origin/main -- apps/server/internal/hostprogram/erikak/testdata apps/server/internal/world/testdata` 差分なし（金型は 1 バイトも変えていない）。
- 共通の層（`world`、`worldrepo`、`llm`、`lineslot`）に `CALL>`、`#%02d` の画面の文字列がないことを grep で確認（テスト以外）。

## ストーリーとテスト

| ストーリー | テスト |
|---|---|
| 1 電報に返事が来る | `worldrepo.TestResidentsAnswerAHumansTelegram`、`TestErikaKCallerIsAnsweredOnTheScreen` |
| 2 返信しないこともある | `lineslot.TestAnswerRateIsBelowOneAndRisesWithRelation`、`TestAtMostOneAnswerPerMessage` |
| 3 部屋にいる相手は部屋を出てから | `lineslot.TestAnswerComesAfterDelivery`、`TestAnswerNeedsTheResidentStillOnTheLine` |
| 4 知り合いから声がかかる | `lineslot.TestGuestIsNeverHailed`、`TestStrangersDoNotHail`、`TestAResidentHailsAtMostTwicePerCall`、`TestRecordedHailsCountTowardTheCap`、`worldrepo.TestAGuestIsAnsweredButNeverHailed` |
| 5 同じ時刻の同じ事実 | `lineslot.TestSlotsAreDeterministic`、`TestRecordedSlotsAreNotRepeated`、`worldrepo.TestLineMessageEventsReturnTheSameMessagesForTheSameWindow` |
| 6 接続していない間は作らない | `lineslot.TestNothingWhileTheCallIsNotLive`、`worldrepo.TestNothingIsWrittenWhenTheCallIsNotLive` |
| 7 うるさくならない | `lineslot.TestLimitsHoldInABusyStation`、`TestRecordedMessagesCountTowardTheHourlyLimit` |
| 8 LLM がない・失敗 | `worldrepo.TestNoLineMessagesWithoutAWriter`、`TestResidentsKeepQuietWhenWritingFails`（エラー、改行、長すぎ、Shift_JIS 外、未来の語、ハンドルの前置き） |
| 9 本文に従わない | `lineslot.TestMessageBodyDoesNotChangeTheSlots`、`worldrepo.TestLineMessageSlotsDoNotDependOnTheMessageBody`、`TestTheWritersMaterialsQuoteTheHumansWords`、`llm.TestLineMessagePromptQuotesTheIncomingMessage` |
| 10 届いた電報が画面に出る | `erikak.TestIncomingTelegramInterruptsTheCallPrompt`、`TestSeveralTelegramsComeOneByOneAndWhoOnce`、`TestTelegramsWaitWhileTheCallerWritesOrChats`、`TestEnterAtTheCallPromptShowsWaitingTelegramsBeforeWho`、`TestIncomingTelegramInTheRoomMenuIsShownBeforeItsPrompt`、`TestNoTelegramsBeforeLogin` |
| 11 画面の金型が変わらない | 金型の差分が空、`TestEnterWithoutTelegramsIsUnchanged` |

人間 ID が LLM の材料に出ないことは、`worldrepo.TestResidentsAnswerAHumansTelegram`（要求を JSON にして検索）、
`llm.TestLineMessageRequestCarriesNoHumanID`、`worldrepo.TestErikaKCallerIsAnsweredOnTheScreen`（画面）で確かめた。

## `plan.md` と違う点

- **`LineMessageHost` の引数。** 計画の `(host, human, line, from, to)` の代わりに、`world.LineMessageCall{CallID, Human, Line, Login}` を取る。
  通話が生きているかの確認（通話 ID）と、住民から見た人（ログイン ID）が要るため。
- **`LineMessage.FromLine` を足した。** 計画は「話者の回線番号は住民の滞在から求める」だったが、住民が回線を離れた後に表示される電報で
  `#NN` が求められなくなるため、記録に回線番号を持たせた。住民の電報では `FromHandle` に住民のハンドルも入れる
  （`FromHandle` の意味を「送り手が表示される名前」に広げた）。古い記録は欄が空のまま読める。
- **`Poll` の呼び出し。** 画面に出せない状態（電報の入力中、チャット室、掲示板など）でも `LineMessageEvents` を呼び、枠の計算と生成を続ける
  （出せない間は `lineMark` を進めず、画面に戻ったときに出る）。ログイン前は呼ばない。
- **`chat_menu` の配達の表示。** 割り込みの後、WHO の代わりにルーム番号のプロンプトを再表示する（接続記録にない推定）。
- `Poll` の `from` は、`lineMark` が空のときは最初の呼び出しで初期化する（通話の開始時刻ではない）。
- **コミット。** T003 と T004、T006 と T007 は同じファイルにまたがるため 1 つのコミットにまとめた。T008 のコミットに
  `world.LineMessage.FromLine` と `line_talk.go` の `FromLine` / `FromHandle` の記録が含まれる。

## 実行していないもの

- **実際の LLM での確認。** テストはすべて試験用の生成器。本物のモデルが出す電報の質、検証の不合格率、遅延は未確認。
- **ブラウザからの実機での確認。** 画面の出力は `Runtime` の単体と、実際の `Repository` をつないだテストまで。WebSocket とブラウザを通した表示は確かめていない。
- **`cmd/server` での有効化のテスト。** `SetLineMessageWriter` の呼び出しは `main.go` に 1 行足しただけで、設定の有無で呼ばれ方が変わることを確かめるテストはない（`cmd/server` に既存の同種のテストがないため）。
- **`observePresence`。** 電報を送った住民の通話を「観測済み」として保存する処理は、チャット（`ChatEvents`）と違い、電報では行っていない。
  住民の通話は、WHO で見えた時点で保存される前提。
- **同じ人が 2 つの通話で電報を送ったとき。** 返信は、先に問い合わせた通話の回線に宛てられる（spec 019 の近道と同じ種類の制限）。
- 頻度・上限は debug 用の暫定値のまま。
