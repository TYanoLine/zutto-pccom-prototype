# Tasks: 人間の識別子（019-human-identity）

**Input**: `specs/019-human-identity/` の `spec.md`、`plan.md`
**Prerequisites**: `AGENTS.md`、`docs/WORLD_SIMULATION.md`、`docs/research/ACCOUNT_ID_HANDLE_EVIDENCE.md`、spec 010、spec 011、spec 016、spec 017

## この作業のルール（必読）

- spec 017 の実装（PR #360）が `main` に入ってから始める。入っていなければ止めて、そう報告する。
- 作業ブランチから `main` への PR を 1 本作る。`main` に直接コミットしない。
- **成果物は Go と TypeScript のコードとテスト、文書の更新、`verification.md`。** spec のファイル（`spec.md`、`plan.md`、`tasks.md`）は変更しない。
- **既存の金型を 1 バイトも変えない。** `apps/server/internal/hostprogram/erikak/testdata/` と `apps/server/internal/world/testdata/`。
  金型の更新用フラグを使わない。金型テストが失敗したら実装の誤り。2 回直して通らなければ止めて、違う行を報告する。
- **画面の文字列を変えない。** 人間 ID を端末に出さない。
- **人間 ID を LLM の材料と住民の判断に使わない。** 住民の判断は `world.KnownAs` を通す。
- **呼び出し元の鍵を保存しない、ログに出さない。**
- **古い記録を書き換えない、移し替えない。**
- テストで実際の LLM を呼ばない。
- スコープ外（`spec.md` §8）に手を出さない。特に、入会登録、パスワード、ログイン ID の検査、台帳と関係の永続化。
- 既存ファイルに整形だけの差分を作らない。`gofmt -w` を既存ファイル全体にかけない（新規ファイルにはかける）。
  既存ファイルの改行コードを変えない。
- 1 タスク = 1 コミット（`T0XX: 内容`）。途中でビルドが通らないタスクは、1 つのコミットにまとめてよい。
- 実行していないコマンドを「成功した」と書かない。`.github/` を変更しない。
- spec と実際のコードが食い違ったら、推測で進めず、PR の説明に書いて止まる。

---

## Phase 0: ベースライン

- [ ] **T001** 変更前に、全テストが通ることを確認し、結果を `verification.md` に書く

  ```bash
  go -C apps/server test ./... -count=1
  go -C apps/server vet ./...
  npm --prefix apps/web test
  npm --prefix apps/web run build
  ```

  `memberAliases` の位置と、`plan.md` §1 の行番号を実際の値で確かめ、違えば `verification.md` に書く。

---

## Phase 1: 型と住民から見た人（`plan.md` §2）

- [ ] **T002** `world` に欄と種類を加える（`ActorKindLogin`、`HumanCall.HumanID`、`PresenceSession.CallID`、`Post.AuthorHumanID`、
  `InteractionEvent.ActorLogin` / `AddressedLogins`、`LineMessage.FromHandle`）。`ActorKindMember` と `ActorRef` のコメントを直す。
  テスト: 新しい欄のない古いスナップショットの JSON が `worldpersist` で読める。`HumanCall.Session()` が `member:<HumanID>` と `CallID` を持つ
  （`HumanID` が空なら `member:<CallID>`）。
- [ ] **T003** `world/known.go` に `GuestLoginID`、`KnownAs`、`PostActor`、`PostKnownAs`、`InteractionEvent` の `KnownActor` / `KnownAddressed` を作る。
  テスト: 住民はそのまま、人間はログイン ID、`GUEST` と空はゼロ、古い書き込み（`AuthorHumanID` なし）の `PostActor` が従来と同じ、
  `ActorLogin` が空の古い事象の `KnownActor` が `login:<ID>`。

  ```bash
  go -C apps/server test ./internal/world ./internal/worldpersist -count=1
  ```

---

## Phase 2: 人間 ID（`plan.md` §3）

- [ ] **T004** `ws/human.go` に `humanID` と `isCallerKey` を作り、`clientMessage.CallerKey`、`SessionManager.Create` の人間 ID、
  `CallSession.HumanID`、`LineOccupancy.BeginHumanCall` の人間 ID、`hostprogram.CallInfo` と `SetCallInfo(info)` を入れる。
  絵理香K版の `SetCallInfo` は、この時点では受け取るだけ（`r.humanID` を持つ）。
  テスト（ストーリー 1）: 同じ鍵の 2 回の `dial` で `HumanID` が等しく通話 ID が異なる、別の鍵で異なる、鍵なし・形の正しくない鍵で通話ごとに異なる、
  `HumanID` に鍵の文字列が含まれない、ログ（`log.SetOutput`）に鍵が出ない。

  ```bash
  go -C apps/server test ./internal/ws ./internal/worldrepo ./internal/hostprogram/... -count=1
  ```

---

## Phase 3: 記録と住民から見た人（`plan.md` §4、§5）

- [ ] **T005** `worldrepo/presence.go` の `BeginHumanCall` が `HumanID` を保存する。`recordPostInteraction` を `world.PostActor` と
  `ActorLogin` / `AddressedLogins` に、関係を `world.PostKnownAs` に変える（ゼロなら関係を作らない）。`bbs_article_body.go` の `actorRefFromPost` を
  `world.PostKnownAs` に置き換える。`chat_records.go` の台帳に人間のログイン ID を入れる。
  テスト: 人間の書き込みの台帳の主体が `member:<人間 ID>`、`ActorLogin` がログイン ID。人間が住民にレスをすると関係が `login:<ログイン ID>` で
  記録され、`GUEST` では記録されない。住民同士の関係は変わらない。
- [ ] **T006** `chat_talk.go` の `memberAliases` を消し、関係の値と出来事を住民から見た人で求める（`plan.md` §4）。在室を通話ごとに数え、
  `Viewers` を人間 ID ごとにまとめる。
  テスト（ストーリー 3、4）: 人間 A が TARO で住民にレスをしたあと、GUEST で入ると関係 0、TARO で入ると 1 以上、人間 B が TARO で入ると 1 以上。
  A が TARO で書き込み、GUEST で入ると `news` の出来事 0 件、TARO で入ると 1 件以上。GUEST の書き込みは出来事にならない。
- [ ] **T007** `actormemory` で人間を `login:<ログイン ID>` で表し、`parseActor` と `matchesCounterpart` が `login` を受ける（`plan.md` §5）。
  テスト: 人間の書き込みが検索結果に `login:<ログイン ID>` で現れ、人間 ID が現れない。`login:TARO` を相手に指定すると当たる。
- [ ] **T008** 人間 ID が LLM の材料に出ないことを確かめるテストを書く（ストーリー 5）。試験用のチャットの 1 行の生成器と記事の本文の生成器が受け取った
  リクエストを `json.Marshal` し、人間 ID の文字列を含まない。

  ```bash
  go -C apps/server test ./internal/world ./internal/worldrepo ./internal/actormemory -count=1
  ```

---

## Phase 4: ホストプログラム（`plan.md` §6）

- [ ] **T009** 絵理香K版: 書き込みに `AuthorHumanID`、電報に `member:<人間 ID>` と `FromHandle`、自分の判定を通話 ID と記録した発言の ID に変える。
  テスト（ストーリー 2、6）: 2 つの鍵の GUEST の書き込みで `AuthorHumanID` が異なり `Author` はともに `GUEST`。同じ人間 ID の 2 つの通話で、
  WHO の自分の印が 1 行、一方の発言がもう一方の画面に出て、発言した側には二重に出ない。
- [ ] **T010** TurboBBS: `SetCallInfo` を実装し、書き込みに `AuthorHumanID`。テスト: 書き込みに人間 ID が入る。画面の出力は変わらない。

  ```bash
  go -C apps/server test ./internal/hostprogram/... -count=1
  ```

---

## Phase 5: Web（`plan.md` §7）

- [ ] **T011** `CallerKey.ts` の `getOrCreateCallerKey`、`VirtualModemOptions.callerKey`、`App.tsx` での受け渡し。
  テスト: 2 回呼ぶと同じ鍵、保存できないときもその呼び出しでは鍵を返す、`callerKey` を与えると `dial` に `caller_key` が付き、与えないと既存の JSON のまま。

  ```bash
  npm --prefix apps/web test
  npm --prefix apps/web run build
  ```

---

## Phase 6: 文書と仕上げ

- [ ] **T012** 文書（spec FR-014）: `docs/WORLD_SIMULATION.md`、`docs/ARCHITECTURE.md` の既知の近道、`docs/research/ACCOUNT_ID_HANDLE_EVIDENCE.md`。
- [ ] **T013** 全体の確認をし、`verification.md` に結果を書く。

  ```bash
  go -C apps/server test ./... -count=1
  go -C apps/server vet ./...
  go -C apps/server test -race ./internal/worldrepo ./internal/ws -count=1
  npm --prefix apps/web test
  npm --prefix apps/web run build
  git diff --stat origin/main -- apps/server/internal/hostprogram/erikak/testdata apps/server/internal/world/testdata
  grep -rn "memberAliases" apps/server/internal
  ```

  金型の差分と `memberAliases` の grep は空であること。
- [ ] **T014** `main` への draft PR を開く。説明に、実行したコマンドと結果、未実施のもの（実機のブラウザでの確認など）と理由、
  `plan.md` と違う点を書く。

## 完了の定義

- PR の差分に、`world/known.go`、`ws/human.go`、`apps/web/src/modem/CallerKey.ts` とそれぞれのテスト、T002〜T012 の変更、`verification.md` が含まれる。
- T013 のコマンドがすべて通り、金型の差分がない。
- draft PR が開いている。
