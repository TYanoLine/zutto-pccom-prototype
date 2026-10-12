# Verification 017: chat-utterances

## T001 ベースライン（変更前、`main` 059582d）

| コマンド | 結果 |
|---|---|
| `go -C apps/server test ./... -count=1` | 全パッケージ ok（失敗 0） |
| `go -C apps/server vet ./...` | エラー 0 |

## T007 `ChatEvents` 1 回分の所要時間

`go -C apps/server test ./internal/worldrepo -run TestChatEventsTiming -count=3 -v`（局に住民を足して 326 人、部屋に人間 1 人と
住民 4 人、試験用の生成器、10 秒ごとに 30 回呼んだ最悪値）:

| 回 | 最悪値 |
|---|---|
| 1 | 84.9 ms |
| 2 | 120.3 ms |
| 3 | 132.3 ms |

spec 016 の WHO 1 回分（約 100 ms）と同じ程度。大半は spec 016 の計画の計算（`gatherCalls`）で、枠の計算は部屋の住民だけを見る。

## T011 全体の確認

| コマンド | 結果 |
|---|---|
| `go -C apps/server test ./... -count=1` | 全パッケージ ok（失敗 0） |
| `go -C apps/server vet ./...` | エラー 0 |
| `go -C apps/server test -race ./internal/ws ./internal/worldrepo -count=1` | ok（ws 1.1 s、worldrepo 175.9 s） |
| `git diff --stat origin/main -- apps/server/internal/hostprogram/erikak/testdata apps/server/internal/world/testdata` | 出力なし（金型の差分なし） |
| `grep -rn "chat on\|chat off\|電報" apps/server/internal/world apps/server/internal/chatslot apps/server/internal/worldrepo apps/server/internal/llm/chat_line*.go` | 6 行。すべて `worldrepo` のテストファイル（下記） |

grep について: tasks.md は「何も出さないこと」としているが、`main` の時点ですでに `worldrepo/callplan_test.go`（spec 016 の
`TestErikaKChatThroughRepository` など、絵理香K版を実際のリポジトリで動かすテスト）が 4 行当たるため、書いてあるとおりの grep は空にならない。
本仕様で足した `chat_talk_test.go` の `TestErikaKChatShowsResidentLines`（同じ形の結合テスト）が 2 行当たる。
テスト以外のファイルに限った `grep -rn --exclude=*_test.go ...` は何も出さない（終了コード 1）。共通の層の実装に絵理香K版の文字列はない。

## 実施しなかった確認

- 実際の LLM（Azure OpenAI）での部屋の会話の確認: この環境に API キーがないため実施していない。`Chat generated content:` の行の実例はない。
  プロンプトと JSON スキーマは `TestWriteChatLineSendsMaterialsAndSchema` で、要求の中身として確認した。
- ローカルでサーバーを起動しての手動確認: 実施していない。絵理香K版の画面は `TestChatRoomShowsLines`（試験用の世界）と
  `TestErikaKChatShowsResidentLines`（実際のリポジトリ + 試験用の生成器）で確認した。

## plan.md と違う点・補ったこと

- **人間の識別子:** 部屋では人間を呼び出し（通話）の ID で記録するが、掲示板の書き込みと関係（`RelationshipEdge`）はハンドルで記録されている
  （絵理香K版の `AuthorMemberID: r.handle`。spec 016 からの状態）。このままでは `news` と関係の分類が人間に一度も当たらないため、
  `worldrepo` で部屋にいる人間について `member:<ハンドル>` を同じ人として扱う対応表（`memberAliases`）を置いた。
- **表示済みの範囲:** `At = max(枠の時刻, 世界の今)` だけでは、同じ呼び出しの中で始めた枠が、その呼び出しの `to` 以下の時刻に記録され、
  表示されないことがあった（`TestResidentsSpeakWhileWatched` で検出）。部屋ごとに最後に返した `to` を覚え、それより後の時刻に記録するようにした（FR-006 の「表示済みの範囲に割り込まない」）。
- **JIS の対応の違い:** モデルがよく書く `〜`（WAVE DASH）などは Go の Shift_JIS の符号化で表せず、端末では `?` になる。
  検証の前に `NormalizeChatLine` で、同じ JIS の文字の表せる形（`～` など）に置き換える。
- **話者の計画の保存:** 人間がいる部屋にいた住民の計画は、入退室の行がなくても観測したものとして保存する（発言がその計画に依るため）。
