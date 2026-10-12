# Tasks: チャット室での住民の発言（017-chat-utterances）

**Input**: `specs/017-chat-utterances/` の `spec.md`、`plan.md`
**Prerequisites**: `AGENTS.md`、`docs/WORLD_SIMULATION.md`、`docs/LLM_POLICY.md`、`docs/DEBUG_INSPECTION.md`、spec 013、spec 016

## この作業のルール（必読）

- 作業ブランチから `main` への PR を 1 本作る。`main` に直接コミットしない。
- **成果物は Go のコードとテスト、文書の更新、`verification.md`。** spec のファイル（`spec.md`、`plan.md`、`tasks.md`）は変更しない。
- **既存の金型を 1 バイトも変えない。** `apps/server/internal/hostprogram/erikak/testdata/` と `apps/server/internal/world/testdata/`。
  金型の更新用フラグを使わない。金型テストが失敗したら実装の誤り。2 回直して通らなければ止めて、違う行を報告する。
- **誰が・いつ・何に応じて話すかの決定に LLM を使わない。** LLM は `chatslot` が決めた枠の本文だけを作る。
- **テストで実際の LLM を呼ばない。** 試験用の生成器と `roundTripFunc` を使う。
- **失敗したら黙る。** 決まり文句の代わりの本文を作らない。
- 共通の層（`world`、`chatslot`、`worldrepo`、`llm`）に、絵理香K版の画面の文字列を書かない。局の定義（YAML）は変えない。
- 世界の年を定数にしない。プロンプトと検証は世界の日付を入力として受け取る。
- 係数・上限は架空の調整値であることを、コードのコメントに書く。値は `plan.md` §3 から写す。
- スコープ外（`spec.md` §8）に手を出さない。特に、電報の返信、関係の更新、行動の記憶、生成の追跡。
- 既存ファイルに整形だけの差分を作らない。`gofmt -w` を既存ファイル全体にかけない（新規ファイルにはかける）。
  既存ファイルの改行コード（CRLF のファイルがある。例 `docs/host-programs/erika-k.md`）を変えない。
- 1 タスク = 1 コミット（`T0XX: 内容`）。途中でビルドが通らないタスクは、1 つのコミットにまとめてよい。
- 実行していないコマンドを「成功した」と書かない。`.github/` を変更しない。
- spec と実際のコードが食い違ったら、推測で進めず、PR の説明に書いて止まる。

---

## Phase 0: ベースライン

- [ ] **T001** 変更前に、全テストが通ることを確認し、結果を `verification.md` に書く

  ```bash
  go -C apps/server test ./... -count=1
  go -C apps/server vet ./...
  ```

---

## Phase 1: 型（`plan.md` §2）

- [ ] **T002** `world/chat.go` の `ChatUtterance` に `Slot`、`RespondsTo` を、`world/store.go` に `ChatSay` と `ChatEvent` の `Body`、`UtteranceID` を、
  `ChatHost` に `EnterChatRoom`、`LeaveChatRoom` を加える。`worldrepo` に人間の滞在（メモリ、`staleViewer`）を実装する。
  テスト: 古いスナップショット（新しい欄なし）が `worldpersist` で読める。入って出た滞在、`LeaveChatRoom` のない滞在が `staleViewer` で閉じる。

---

## Phase 2: 枠の計算（`plan.md` §3）

- [ ] **T003** `internal/chatslot` を作る（種類、確率、遅れ、鍵、観測区間、深さ）。
  テスト: 同じ入力で同じ枠、観測区間の外に枠がない、`Viewers` が空なら枠なし、`welcome` と `reply` が 1 きっかけにつき多くとも 1 人、
  多数の発言でハンドルを含む発言の応答率が含まない発言より高い、応答のない発言がある、本文のない枠には誰も応じない、深さ 4 を超えない。
- [ ] **T004** 上限（`plan.md` §3.3）を `chatslot` に入れる。
  テスト: 住民 8 人の部屋の 1 時間で、最短間隔、同じ住民の間隔、1 分と 1 時間の行数がすべて上限以内。記録済みの発言は捨てない。

  ```bash
  go -C apps/server test ./internal/chatslot ./internal/world ./internal/worldpersist -count=1
  ```

---

## Phase 3: 1 行の生成（`plan.md` §4）

- [ ] **T005** `llm/chat_line.go` に `ChatLineRequest`、`ChatLineWriter`、`ValidateChatLine` を置く。
  テスト: 検証の各規則（空、改行、長すぎる、Shift_JIS で表せない文字、ハンドルの前置き、未来の語）、全角 2 幅の数え方。
- [ ] **T006** `StructuredOpenAIProvider.WriteChatLine` を実装する（JSON スキーマ `chat_line`、プロンプトは目的 + 材料 + 時代の枠）。
  テスト（`roundTripFunc`）: 要求に JSON スキーマと材料（話者、種類、宛先、関係、部屋の人、直近の発言、世界の日付）が入る、
  「ユーザー」「お客様」など人間を特別扱いする語がプロンプトにない、応答の `line` を返す、壊れた JSON はエラー。

  ```bash
  go -C apps/server test ./internal/llm -count=1
  ```

---

## Phase 4: リポジトリと記録（`plan.md` §5、§6）

- [ ] **T007** `worldrepo/chat_talk.go` に `SetChatLineWriter`、`ChatEvents` の発言の部分、生成の仕事、記録、関係の分類を実装する。
  `worldrepo/chat_content_log.go` にログを置く。
  テスト（試験用の生成器、`chatTestRepo` を使う）:
  - 人間が部屋に入って世界時刻を進めると発言が記録され、`ChatEvents` が発言と入退室を時刻順に返す。各発言の時刻にその話者が部屋にいる。
  - 人間が部屋にいない、または生成器がないとき、生成器の呼び出しが 0 回で発言がない。
  - 人間の発言に応じる住民が 1 発言につき多くとも 1 人。
  - 失敗する生成器、検証に落ちる生成器で発言が記録されない。
  - 同じ枠が 2 回記録されない（同じ時刻の `ChatEvents` を並行に呼ぶ）。
  - 記録された発言の `At` が、記録の前に返した `ChatEvents` の `to` 以下にならない。
  - 台帳に `chat_utterance`、`npc_world_selected`、応答なら `RespondsToEventID` が入る。
  - 1 時間の滞在で、部屋の住民の発言が上限以内。
  - `ChatEvents` 1 回の所要時間を住民 300 人の局で測り、`verification.md` に書く。
- [ ] **T008** `cmd/server/main.go` で、LLM の設定があるとき `SetChatLineWriter(azureOpenAIRenderer)` を呼ぶ。

  ```bash
  go -C apps/server test ./internal/worldrepo ./cmd/server -count=1
  go -C apps/server test -race ./internal/worldrepo -count=1
  ```

---

## Phase 5: 絵理香K版での具現化（`plan.md` §7）

- [ ] **T009** `erikak/chat.go` で、部屋に入ったとき `EnterChatRoom`、`..` で `LeaveChatRoom` を呼び、`roomEvents` で発言の行を出す（自分の発言は出さない）。
  テスト: 実際のリポジトリと試験用の生成器で、部屋の行が入退室か `HANDLE  :本文` だけで時刻順、自分の発言が `Poll` に再び現れない、
  生成器なしの出力が spec 016 と同じ（`TestErikaKChatThroughRepository` が変わらず通る）。

  ```bash
  go -C apps/server test ./internal/hostprogram/... -count=1
  git diff --stat origin/main -- apps/server/internal/hostprogram/erikak/testdata apps/server/internal/world/testdata
  ```

  2 つ目のコマンドの出力が空であること。

---

## Phase 6: 文書・全体の確認・PR

- [ ] **T010** `docs/WORLD_SIMULATION.md`、`docs/LLM_POLICY.md`、`docs/host-programs/erika-k.md`、`docs/DEBUG_INSPECTION.md` を更新する（`plan.md` §8）。
- [ ] **T011** 全体の確認

  ```bash
  go -C apps/server test ./... -count=1
  go -C apps/server vet ./...
  go -C apps/server test -race ./internal/ws ./internal/worldrepo -count=1
  grep -rn "chat on\|chat off\|電報" apps/server/internal/world apps/server/internal/chatslot apps/server/internal/worldrepo apps/server/internal/llm/chat_line*.go
  ```

  最後の grep は何も出さないこと。

- [ ] **T012** `specs/017-chat-utterances/verification.md` に、実行したコマンドと結果、T007 の所要時間、実施しなかった確認と理由を書く。
  実際の LLM で部屋の会話を確かめた場合は、その記録（`Chat generated content:` の行の抜粋）も書く。
- [ ] **T013** draft の PR を開く。説明に、Fictional reconstruction とした係数・上限、spec と食い違った点を書く。

## 完了の定義

PR の差分に次が含まれ、T011 のコマンドがすべて成功している:
`internal/world/`、`internal/chatslot/`、`internal/llm/chat_line*.go`、`internal/worldrepo/`、`internal/worldpersist/`（テスト）、
`internal/hostprogram/erikak/`、`cmd/server/main.go`、更新した `docs/`、`specs/017-chat-utterances/verification.md`。金型ファイルの差分がない。
