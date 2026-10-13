# Plan: 人間の識別子（019-human-identity）

本計画は spec 017 の実装（PR #360）が `main` に入った後のコードを前提にする。`memberAliases` などの位置は、その時点の `main` で確かめる。

## 0. 層の分け方

| 層 | 置くもの | 置かないもの |
|---|---|---|
| Web（`apps/web`） | 呼び出し元の鍵の生成と保存、`dial` での送信 | 人間 ID の導出 |
| `ws` | 鍵から人間 ID を求める、通話に人間 ID を持たせる | ログイン ID |
| `world` | 人間 ID とログイン ID の欄、`login` の種類、住民から見た人の関数 | 画面の文字列 |
| `worldrepo` | 通話・台帳・関係・チャット室での使い分け | 画面 |
| `actormemory` | 検索結果での人間の表し方（ログイン ID） | 人間 ID |
| `hostprogram/erikak`、`turbobbs` | 書き込みへの人間 ID、自分の判定 | 人間 ID の導出、住民の判断 |

## 1. 現状（読んだコード）

- `ws/sessions.go:91` `Create(host, baud, line, runtime)` が `newSessionID()`（16 byte の乱数の 16 進）で通話 ID を作り、
  `BeginHumanCall(host, id, baud)` を呼ぶ。`ws/session.go:179` が `SetCallInfo(active.ID, active.Line, active.Baud)` を呼ぶ。
  `clientMessage` は `type`、`phone`、`line`、`attempt`、`session_id` だけ。
- `world/presence.go:47` `HumanCall{CallID, HostID, Handle, Line, Baud, Start}`、`Session()` が主体を `member:<CallID>` にする。
  `worldrepo/presence.go:115` `BeginHumanCall`、`:134` `SetHumanCallHandle`。
- `hostprogram/runtime.go:32` `CallInfoReceiver{SetCallInfo(callID string, line, baud int)}`。実装は絵理香K版だけ。
- 絵理香K版: `runtime.go:87-101` ログイン ID（空なら `GUEST`、`guest` は大文字に）、書き込みは `Author` と `AuthorMemberID` にハンドル。
  自分の判定は `chat.go:85`、`runtime.go:1223` の `s.Actor.ID == r.callID`、部屋の発言は `chat.go` の `e.Actor != r.self()`。
  `self()` は `member:<callID>`。電報は `From: member:<callID>`（`chat.go:213`）。
- TurboBBS: 局内のアカウント表（呼び出しの間だけ）、書き込みは `Author/AuthorMemberID: r.caller`。`CallInfoReceiver` を実装しない。
- 台帳と関係: `worldrepo/repository.go:253` `recordPostInteraction` が主体を `AuthorPersonaID`、`AuthorMemberID`、`Author` の順で求める。
  親の書き込みの主体も同じ。関係は `member:<ハンドル>` で記録される。同じ求め方が `worldrepo/bbs_article_body.go`（`actorRefFromPost`）と
  `actormemory/executor.go`（`postActor`）にもある。
- `actormemory/executor.go`: `parseActor` は `persona` と `member` だけを受ける。`matchesCounterpart` も同じ。
- spec 017: `worldrepo/chat_talk.go` の `memberAliases`、`chatRelation(host, alias, a, b)`、`chatWorldEvents(host, alias, now)`。
- Web: `CenterDirectory.ts` の `getOrCreateWorldKey` が `localStorage` の鍵を作る（同じ形の鍵の作り方の手本）。
  `VirtualModem.ts:372` が `{type:'dial', phone, attempt}` を送る。`App.tsx:220` が `VirtualModem` を作る。
  `VirtualModem.test.ts` は `dial` の JSON を文字列で比べている（3 か所）。
- 台帳と関係はスナップショットに入っていない（`worldpersist`）。古い台帳の移し替えは要らない。

## 2. 型（`world`）

```go
// world/presence.go
const (
	ActorKindPersona = "persona"
	// ActorKindMember is a human user. Its ID is the human ID (spec 019), the
	// same across calls, login IDs and worlds. Records made before spec 019
	// hold a call ID or a handle here.
	ActorKindMember = "member"
	// ActorKindLogin is a person as residents know them: by the login ID they
	// showed on the record's host. Residents never see a human ID.
	ActorKindLogin = "login"
)

type HumanCall struct {
	CallID  string `json:"call_id"`
	HumanID string `json:"human_id,omitempty"`
	// 既存: HostID, Handle, Line, Baud, Start
}
// Session: Actor = member:<HumanID>（HumanID が空なら従来どおり member:<CallID>）、CallID = c.CallID

type PresenceSession struct {
	// 既存の欄に加える。住民の通話では空。
	CallID string `json:"call_id,omitempty"`
}

// world/models.go
type Post struct {
	// 既存の欄に加える。AuthorMemberID は「局のログイン ID」のまま。
	AuthorHumanID string `json:"author_human_id,omitempty"`
}
type InteractionEvent struct {
	// 既存の欄に加える。人間の主体・宛先のログイン ID。AddressedLogins は AddressedTo と同じ順で、住民の宛先は空文字。
	ActorLogin      string   `json:"actor_login,omitempty"`
	AddressedLogins []string `json:"addressed_logins,omitempty"`
}

// world/chat.go
type LineMessage struct {
	// 既存の欄に加える。人間の送り手のログイン ID。
	FromHandle string `json:"from_handle,omitempty"`
}
```

### 2.1 住民から見た人（`world/known.go`、新規）

```go
// GuestLoginID is the shared login ID of guests. Residents cannot tell guests
// apart, so a guest is never someone they know.
const GuestLoginID = "GUEST"

// KnownAs is a person as residents know them (spec 019): a resident as
// itself, a human by the login ID shown on the record. A guest, or a human
// without a login ID, is the zero ActorRef: nobody residents can know.
func KnownAs(actor ActorRef, login string) ActorRef {
	switch actor.Kind {
	case ActorKindPersona, ActorKindLogin:
		return actor
	case ActorKindMember:
		if login == "" || login == GuestLoginID {
			return ActorRef{}
		}
		return ActorRef{Kind: ActorKindLogin, ID: login}
	}
	return ActorRef{}
}

// PostKnownAs / PostActor: 書き込みの主体（システム）と住民から見た人。
// PostActor: AuthorPersonaID → persona、AuthorHumanID → member:<AuthorHumanID>、
//            それ以外は従来どおり member:<AuthorMemberID or Author>（古い記録）。
// PostKnownAs: persona はそのまま、人間は KnownAs(member, AuthorMemberID or Author)。
// InteractionEvent.KnownActor() / KnownAddressed(i): ActorLogin / AddressedLogins を使う。
//   ActorLogin が空の人間の主体（古い記録）は、ID をログイン ID とみなす（古い記録の ID はハンドルなので）。
```

- 3 か所に重複している書き込みの主体の求め方（`recordPostInteraction`、`actorRefFromPost`、`postActor`）を `world.PostActor` と
  `world.PostKnownAs` にまとめる。住民の判断に使う側（`actorRefFromPost`、`postActor`）は `PostKnownAs`、台帳の主体は `PostActor`。
- 新しい欄はすべて `omitempty`。古いスナップショットはそのまま読める。

## 3. 人間 ID（`ws`）

```go
// ws/human.go（新規）
// humanID derives a user's human ID from the key their browser keeps
// (spec 019). The key is a secret-ish token and is never stored or logged; only
// the one-way digest is. Without a well-formed key the call gets an ID of its own.
func humanID(callerKey string) string {
	if isCallerKey(callerKey) { // 32 桁の小文字 16 進
		sum := sha256.Sum256([]byte("zutto-human:" + callerKey))
		return "h" + hex.EncodeToString(sum[:8])
	}
	id, _ := newSessionID()
	return "h" + id[:16]
}
```

- `clientMessage` に `CallerKey string \`json:"caller_key,omitempty"\`` を加える。`dial` のときだけ読む。
- `SessionManager.Create(host, baud, line, runtime, humanID)`、`CallSession.HumanID`。`BeginHumanCall(host, callID, humanID, baud)`。
- `hostprogram.CallInfoReceiver` を `SetCallInfo(info CallInfo)` に変える。

  ```go
  // CallInfo is what a host program learns about its own caller's call.
  type CallInfo struct {
  	CallID  string // this connection
  	HumanID string // the user, across calls (spec 019)
  	Line    int
  	Baud    int
  }
  ```

- ログに鍵を出さない。既存のログに人間 ID を出すのはかまわない（世界の外の記録）。

## 4. 記録（`worldrepo`）

- `BeginHumanCall` が `HumanCall.HumanID` を保存する。`ObservePresenceNow` などが返す人間のセッションは `member:<HumanID>` と `CallID` を持つ。
- `recordPostInteraction`: 主体は `world.PostActor(p)`、`ActorLogin` は人間の主体のときの `p.AuthorMemberID`（空なら `p.Author`）。
  親の書き込みも同じで、`AddressedLogins` を埋める。**関係は `world.PostKnownAs` で記録する**。どちらかがゼロ（ゲストなど）なら記録しない。
- `chat_records.go`: `RecordChatUtterance` と `SendLineMessage` の台帳の記録に、人間の主体のログイン ID（発言の `Handle`、電報の `FromHandle`）を入れる。
- `chat_talk.go`（spec 017）:
  - `memberAliases` を消す。`chatRelation(host, a, b)` と `chatWorldEvents(host, now)` から `alias` の引数を消す。
  - 部屋の人間の在室は通話 ID で数える（`EnterChatRoom` / `LeaveChatRoom` の鍵に通話 ID を使う）。枠の計算に渡す `Viewers` は
    `member:<人間 ID>` ごとにまとめる（同じ人の 2 つの通話は区間の和、ログイン ID は後の通話のもの）。
  - 関係の値: 枠の計算が渡す 2 人を、住民はそのまま、人間は在室のログイン ID で `KnownAs` に直してから引く。ゼロなら 0。
  - 出来事: 台帳の事象の主体・宛先を `KnownActor` / `KnownAddressed` に直し、部屋にいる人間のうち同じ「住民から見た人」を持つ人の
    `member:<人間 ID>` を `Subject` にする。ゼロ（ゲスト）の事象は捨てる。
- `EnterChatRoom` の引数: 現在の `(host, room, actor, handle, at)` に通話 ID を加えるか、絵理香K版が通話ごとの鍵を渡すかは、
  `main` のコードを読んで小さい方を選び、PR に書く（spec の FR-012 を満たせばよい）。

## 5. 行動の記憶（`actormemory`）

- 検索の材料（`Item.Author`、`Item.Addressed`）は、人間を `world.PostKnownAs` / `KnownActor` / `KnownAddressed` で表す。ゼロ（ゲスト）は
  `login:GUEST` ではなく「表さない」（`Author` が空の項目）。
- `parseActor` と `matchesCounterpart` が `login` を受ける。`member:<人間 ID>` を相手に指定しても何も当たらない（人間 ID は住民の世界にない）。
- 検索する主体は住民（`persona`）だけなので、`visibleTo` は変えない。

## 6. ホストプログラム

- 絵理香K版（`runtime.go`、`chat.go`）:
  - `SetCallInfo(info)` で `r.callID`、`r.humanID`、`r.line`、`r.baud` を持つ。
  - 書き込み（新規 `:124` 付近、追記 `:165` 付近）に `AuthorHumanID: r.humanID`。
  - 自分の判定: WHO（`runtime.go:1223`）と回線の一覧（`chat.go:85`）は `s.CallID == r.callID`。
    部屋の発言は、この通話で `RecordChatUtterance` が返した ID の集合で判定する（`e.UtteranceID`）。`self()` は電報と在室の主体
    `member:<人間 ID>` を返す。
  - 電報: `From: member:<人間 ID>`、`FromHandle: r.handle`。
- TurboBBS（`turbobbs/runtime.go`）: `SetCallInfo` を実装して人間 ID を持ち、書き込み（`:540` 付近）に `AuthorHumanID`。

## 7. Web

- `apps/web/src/modem/CallerKey.ts`（新規）: `getOrCreateCallerKey(storage?)`。鍵の名前は `zutto.callerKey.v1`。`getOrCreateWorldKey` と同じ形
  （32 桁の 16 進、保存に失敗したらそのページの間だけ）。
- `VirtualModemOptions.callerKey?: string`。与えられたときだけ `dial` に `caller_key` を付ける。既存の `VirtualModem.test.ts` の JSON は変わらない。
- `App.tsx` が `getOrCreateCallerKey()` を渡す。

## 8. 検証の方針

- 金型: 絵理香K版の画面（`erikak/testdata`）と人物生成（`world/testdata`）を変えない。画面の文字列は変えないので、既存の金型テストがそのまま証拠になる。
  新しい金型は作らない。
- 人間 ID が LLM に渡らないこと: 試験用の生成器（チャットの 1 行、記事の本文）が受け取ったリクエストを `json.Marshal` し、人間 ID の文字列を含まないことを確かめる。
- 鍵が残らないこと: 通話の後のスナップショットの JSON とログ（`log.SetOutput` で受ける）に鍵の文字列がない。
- 古いスナップショット: 新しい欄のない JSON を手で書いて読み込む。

## 9. 変更するファイル

| ファイル | 変更 |
|---|---|
| `apps/server/internal/world/presence.go`、`models.go`、`chat.go` | 欄と種類、コメント |
| `apps/server/internal/world/known.go`（新規）、`known_test.go`（新規） | 住民から見た人、書き込みの主体 |
| `apps/server/internal/world/store.go` | `BeginHumanCall` の引数（`LineOccupancy`） |
| `apps/server/internal/hostprogram/runtime.go` | `CallInfo` |
| `apps/server/internal/ws/human.go`（新規）、`session.go`、`sessions.go` とテスト | 鍵、人間 ID |
| `apps/server/internal/worldrepo/presence.go`、`repository.go`、`chat_records.go`、`chat_talk.go`、`bbs_article_body.go` とテスト | 記録と住民から見た人 |
| `apps/server/internal/actormemory/executor.go` とテスト | 検索での人間 |
| `apps/server/internal/hostprogram/erikak/runtime.go`、`chat.go` とテスト | 人間 ID、自分の判定 |
| `apps/server/internal/hostprogram/turbobbs/runtime.go` とテスト | 人間 ID |
| `apps/web/src/modem/CallerKey.ts`（新規）とテスト、`VirtualModem.ts`、`App.tsx` | 鍵 |
| `docs/WORLD_SIMULATION.md`、`docs/ARCHITECTURE.md`、`docs/research/ACCOUNT_ID_HANDLE_EVIDENCE.md` | 文書 |
| `specs/019-human-identity/verification.md`（新規） | 記録 |

## 10. リスクと対策

| リスク | 対策 |
|---|---|
| `member` の ID の意味が変わり、古い記録と新しい記録が混ざる | 住民の判断は `KnownAs` を通すので、古い記録もログイン ID（記録されたハンドル）で知られる。システムの集計は本仕様の範囲にない |
| 人間 ID が LLM の材料に漏れる | 材料はハンドルだけで組み立てる。リクエスト全体に人間 ID がないことをテストで確かめる |
| 同じ人の 2 つの通話が、自分の判定で混ざる | 自分の判定を通話 ID と記録した発言の ID で行う。ストーリー 6 のテスト |
| 鍵がログや記録に残り、他人がなりすませる | 鍵は導出にだけ使い、保存しない。テストで確かめる。本人確認は本仕様の範囲外と書く |
| ゲストどうしが住民に同じ人と見られる | `KnownAs` が `GUEST` をゼロにし、関係と出来事から外す |
| `CallInfoReceiver` の変更で、他のホストプログラムがコンパイルできない | 実装は絵理香K版と TurboBBS だけ（`grep -rn SetCallInfo` で確かめる） |
