# Plan: チャット室での住民の発言（017-chat-utterances）

## 0. 層の分け方

spec 016 と同じ。

| 層 | 置くもの | 置かないもの |
|---|---|---|
| `world` | 発言の記録の欄、`ChatEvent` の発言の種類、`ChatHost` の人間の入退室 | 画面の文字列、行の形 |
| `chatslot`（新規） | 発言の枠の決定論的な計算と上限（保存しない、LLM を使わない） | LLM、保存、画面 |
| `llm` | チャットの 1 行の生成のインターフェース、プロンプト、JSON スキーマ、検証 | 誰が話すかの決定 |
| `worldrepo` | 人間の滞在、枠の計算の呼び出し、生成の開始と記録、台帳、ログ | 画面 |
| `hostprogram/erikak` | 発言の行の形、自分の発言を出さないこと、入退室の通知 | 枠の計算、生成 |

## 1. 現状（読んだコード）

- `worldrepo/callplan.go:142` `ChatEvents(host, room, from, to)` が入退室だけを返す。計画は `gatherCalls` と `plans` で求め、
  表示したものを `observePresence` で保存する（presence lock の中）。
- `worldrepo/chat_records.go:19` `RecordChatUtterance` が、`u.At` に部屋にいる住民を宛先にして発言を保存し、台帳に記録する。
  住民の話者にも使えるが、どの発言に応じたかを持たない。
- `world/chat.go:26` `ChatUtterance{ID, HostID, Room, Actor, Handle, Body, At}`。`MemoryStore.SaveChatUtterance` が `"<host>:<n>"` の ID を付ける。
- `erikak/chat.go:297` `roomEvents` が `ChatEvents` の入退室を BEL 2 回の行で出す。`handleChatRoom` は自分の発言を
  `padRunes(handle, 8) + ":" + line` で即座に出し、`RecordChatUtterance` する。人間の入室・退室は世界に伝えていない。
- LLM: `StructuredOpenAIProvider.responseTextWithJSONSchema`（`llm/openai_structured_timeline.go:257`）が厳密な JSON スキーマの呼び出し。
  時代の枠は `withDiegeticWorldFrame`（`llm/diegetic_frame.go:38`）、未来の語は `forbiddenFutureMetaTerm`（`llm/openai.go:358`）。
- 人物像は `personaSummary`（`worldrepo/llm_materializer.go:224`）、関係の分類は `materializationRelationshipContext`
  （`worldrepo/bbs_article_body.go:359`、記事に結びついている。0 = 初対面、1 = 以前にもやり取り、2 以上 = 常連仲間）。
- 生成内容のログは `worldrepo/bbs_content_log.go` の `shouldLogGeneratedContent`（`DEBUG_LOG_GENERATED_CONTENT` と局の `debug.content_log`）。
- 非同期の生成は `worldrepo/observation.go` の `observationJob{done, err}` と、鍵ごとに 1 つだけ動かすやり方。
- spec 008（掲示板での反応の選択）は未実装。本仕様の選び方は `chatslot` に閉じる。

## 2. 型（`world`）

```go
// world/chat.go
type ChatUtterance struct {
	ID, HostID string; Room int; Actor ActorRef; Handle, Body string; At time.Time
	// Slot is the World's decision a resident's line verbalizes; empty for a
	// human's line. RespondsTo is the ID of the utterance it answers.
	Slot       string `json:"slot,omitempty"`
	RespondsTo string `json:"responds_to,omitempty"`
}

// world/store.go
const ChatSay ChatEventKind = "say" // 既存の ChatJoin、ChatLeave に加える
type ChatEvent struct { At; Kind; Room; Actor; Handle; Body string; UtteranceID string } // Body、UtteranceID を加える

type ChatHost interface {
	// 既存: CallActivitiesNow, ChatEvents, RecordChatUtterance, SendLineMessage
	// EnterChatRoom / LeaveChatRoom tell the world a caller is in a room. A
	// caller's stay is when resident lines are written (spec 017 FR-002).
	EnterChatRoom(host Host, room int, actor ActorRef, handle string, at time.Time)
	LeaveChatRoom(host Host, room int, actor ActorRef, at time.Time)
}
```

- 新しい欄は `omitempty`。`worldpersist` の古いスナップショットはそのまま読める（テストする）。
- 人間の滞在は `Repository` のメモリに持つ（局・部屋・actor ごとの区間）。世界の事実ではなく「観測者がいた」記録で、保存しない。
  `LeaveChatRoom` がないまま、その部屋の `ChatEvents` の呼び出しが `staleViewer`（1 分）以上ない滞在は、最後の呼び出しの時刻で終わったものとみなす
  （回線断などで退室が伝わらない場合）。

## 3. 枠の計算（`internal/chatslot`、新規）

`callplan` と同じく、計算だけをする純粋な package。保存しない、LLM を使わない。係数はすべて架空の調整値とコードに書く。

**以下の頻度・確率・上限の値は暫定で、デバッグ中のため多めにしてある**（PR #359 のコメント）。具体的な値は後で決める。
値は `chatslot` の 1 か所（定数のまとまり）に置き、後で値だけを変えられるようにする。

```go
type Input struct {
	Host world.Host; Room int
	Window [2]time.Time           // 計算する範囲 (from, to]
	Viewers []Stay                 // 人間の滞在（観測される区間）
	Residents []ResidentStay       // 部屋の住民の chat 区間（spec 016 の計画）と persona
	Utterances []world.ChatUtterance // 部屋で記録された発言（人間・住民）
	Relation func(a, b world.ActorRef) int // 0 初対面、1 以前にもやり取り、2 常連
}
type Slot struct {
	Key string; Kind Kind; Room int; Speaker world.ActorRef; Handle string
	At time.Time; Trigger string /* 発言 ID、入室の鍵など */; RespondsTo string; Addressee world.ActorRef
	Depth int // 応答の連鎖の深さ
}
func Slots(in Input) []Slot // 時刻順、上限を適用済み
```

### 3.1 観測される時刻

`Viewers` の区間の和を「観測区間」とする。枠の時刻とそのきっかけの時刻の両方が観測区間に入る枠だけを返す（`talk` と `farewell` はきっかけなし）。
`Viewers` が空なら枠はない。

### 3.2 枠の種類と決め方

乱数は `unit(key, purpose)`（FNV、`callplan.unit` と同じ作り）。`p` は確率、`d` は遅れ。`soc` は `callplan.sociability` と同じ式
（`(1-LurkerTendency + ReplyTendency)/2`）を `chatslot` に写したもの。

| 種類 | きっかけ | 候補 | 確率 | 時刻 |
|---|---|---|---|---|
| `greet` | 住民の入室 | 入った住民（部屋に他の人がいるとき） | `0.5 + 0.4*soc` | 入室 + 3〜12 秒 |
| `welcome` | 誰か（人間を含む）の入室 | 入室の前から部屋にいる住民 | 初対面 `0.3 + 0.5*NewcomerOpenness`、関係あり `0.5 + 0.2*関係` | 入室 + 4〜20 秒 |
| `reply` | 他の人の発言 | 発言の時に部屋にいる、話者以外の住民 | ハンドルを含む `0.85`、その住民への応答 `0.6`、ほか `0.5*ReplyTendency*(1+0.3*関係)` | 発言 + 6〜30 秒 |
| `talk` | なし | 部屋にいる住民（他に誰かいる時刻だけ） | 区間の中で間隔 `(15 秒〜2 分)/max(soc, 0.3)` を順に引く | その時刻 |
| `farewell` | なし（退室の予定） | 退室する住民（他に誰かいるとき） | `0.7*soc` | 退室 − 5〜20 秒 |

- `welcome` と `reply` は、候補のうち `unit < p` のものから `unit/p` が最も小さい 1 人だけにする（多くとも 1 人、FR-001、不変条件）。
- ハンドルを含むかは、発言の本文に住民のハンドルが部分文字列として入っているか（大文字小文字を区別しない）。
- `reply` の `Depth` はきっかけの深さ + 1（人間の発言と `talk` は 0）。`maxDepth = 6` を超える枠は作らない。
- 住民の発言へのきっかけは、記録された発言だけ（本文が作られなかった枠には誰も応じない）。

### 3.3 上限（FR-005）

時刻順、同時刻は鍵の順に、次の条件で採否を決める。捨てた枠は返さない。

- 部屋の住民の発言どうしの最短間隔 `3 秒`。間隔が足りない枠は、`8 秒` 以内なら後ろへずらし、それ以上なら捨てる。
- 同じ住民の最短間隔 `15 秒`。
- 部屋の住民の発言は `1 分に 12 行`、`1 時間に 360 行` まで。
- すでに記録された住民の発言は、上限の計算に数える（記録済みの枠は捨てない、変えない）。

### 3.4 鍵

`Key = kind | host | room | speaker | trigger`。きっかけは、発言なら人間の発言の ID か住民の発言の枠の鍵、入室なら `actor@時刻`、
`talk` は `call 鍵#n`、`farewell` は `call 鍵`。同じ世界と同じ人間の行動なら同じ鍵になる。

## 4. 1 行の生成（`llm`）

```go
// llm/chat_line.go
type ChatLineRequest struct {
	WorldTime  time.Time
	Station    string   // 局の名前（定義から）
	Room       int
	Kind       string   // greet / welcome / reply / talk / farewell
	Speaker    ChatLinePerson // Handle, Summary（personaSummary）
	Addressee  *ChatLinePerson
	Relation   string   // 初対面 / 以前にもやり取り / 常連仲間（宛先があるとき）
	Present    []string // 部屋にいる人のハンドル
	Transcript []ChatLineTurn // Handle, Body（古い順、直近 12 行）
	MaxCells   int
}
type ChatLineWriter interface {
	WriteChatLine(ctx context.Context, req ChatLineRequest) (string, error)
}
func ValidateChatLine(line string, req ChatLineRequest) error
```

- `StructuredOpenAIProvider.WriteChatLine`: `responseTextWithJSONSchema` で `{"line": string}` を受け取る（`schemaName = "chat_line"`、
  `maxOutputTokens` は小さく）。プロンプトは `LLM_POLICY.md` の「目的 + 材料 + スキーマ」: 目的（この人物がパソコン通信のチャット室で次に打つ 1 行）、
  材料（上の欄を JSON で）、時代の枠（`withDiegeticWorldFrame`、世界の日付より後の知識・製品・語を使わない、顔文字は使いすぎない）。
  人間を特別扱いする語（「ユーザー」「お客様」など）を材料に入れない。人間も住民と同じくハンドルで出す（`LLM_POLICY.md`「Human neutrality」）。
- `ValidateChatLine`（FR-004）: 前後の空白を除いて空でない、改行・制御文字なし、`MaxCells`（共通の既定 `64` 半角幅、全角は 2）以内、
  `golang.org/x/text/encoding/japanese.ShiftJIS` で表せる、`<話者のハンドル>:` / `<話者のハンドル> :` で始まらない、`forbiddenFutureMetaTerm` に当たらない。
- 試験用の生成器（`worldrepo` のテスト）を用意する。実際の LLM はテストで呼ばない。プロンプトとスキーマは `roundTripFunc`（`llm/openai_test.go:30`）で
  要求の中身を確かめる。

## 5. リポジトリ（`worldrepo/chat_talk.go`、新規）

```go
func (r *Repository) SetChatLineWriter(w llm.ChatLineWriter)
func (r *Repository) EnterChatRoom(...) / LeaveChatRoom(...)
```

`ChatEvents(host, room, from, to)` を次のように広げる（`callplan.go` の既存の入退室の部分は変えない）。

1. 既存どおり入退室を求める（presence lock の中）。
2. 生成器があり、`residentChat` が有効なら、部屋の発言を `ListChatUtterances(host, room, from', 0)` で読み、
   `chatslot.Slots` を `Window = (from', to + lead]`（`from'` は観測区間の始まりと `to − 10 分` の遅いほう、`lead = 20 秒`）で求める。
   住民の滞在は、入退室と同じ計画（`plans`）から取る。
3. 記録されていない枠のうち、時刻が `to + lead` 以内で、きっかけの発言が記録済みのものについて、生成の仕事を始める（鍵ごとに 1 つ、
   局ごとに同時 `2` まで、待たせない）。時刻が `to − stale`（`stale = 60 秒`）より前の枠は始めずに捨てる。
4. 発言（`ChatSay`）として、`ListChatUtterances` のうち `At` が `(from, to]` のものを、入退室と合わせて時刻順に返す
   （同時刻は退室、発言、入室の順）。

生成の仕事（presence lock の外）:

- 話者の persona、宛先、関係、部屋にいる人、直近の発言（話者の入室と観測区間の始まりの遅いほうから）で `ChatLineRequest` を作り、
  `WriteChatLine` を呼ぶ（タイムアウト 20 秒、検証に落ちたら 1 回だけ再試行）。
- 記録は presence lock の中で: 同じ `Slot` の発言がまだないことを確かめ、話者がまだその部屋にいること（計画の区間）を確かめ、
  `At = max(枠の時刻, 世界の今)` として `RecordChatUtterance` の内部と同じ経路で保存し、台帳に記録する。`RespondsToEventID` は
  応じた発言の台帳の事象（`chat_utterance:<ID>`）。
- 失敗・不合格・話者がもういない・古くなった枠は記録しない。生成の仕事の結果はメモリに持ち、同じ鍵をもう一度始めない（プロセスの間だけ）。

関係の分類は、`materializationRelationshipContext` と同じ数え方（`RelationshipStore.Get` の両方向の `BoardReplyCount` の和）を、
`ActorRef` の組で求める小さな関数を新しく書く（既存の関数は変えない）。

## 6. ログ（FR-009）

`worldrepo/chat_content_log.go`（新規）: 記録したとき、`shouldLogGeneratedContent(host)` が真なら
`log.Printf("Chat generated content: %s", json)`（`host_id`、`room`、`slot`、`kind`、`speaker`、`addressee`、`responds_to`、`body`、`at`、`elapsed_ms`）。
失敗・不合格は `log.Printf("Chat line generation failed: host=%s slot=%s err=%v", ...)`。`docs/DEBUG_INSPECTION.md` に両方の形を書く。

## 7. 絵理香K版での具現化（FR-010）

- `handleChatMenu` で部屋に入ったとき `EnterChatRoom(r.Host, number, 自分, r.handle, now)`、`..` で `LeaveChatRoom`。
  電報・チャットの画面から切断・ログオフしたときの扱いは §2 の `staleViewer` に任せる（新しい切断の処理を作らない）。
- `roomEvents` で `ChatSay` を `padRunes(e.Handle, 8) + ":" + e.Body + "\r\n"` で出す。`e.Actor` が自分（`ActorKindMember` と `r.callID`）なら出さない。
- 文面は変えない。局の定義（YAML）は変えない。

## 8. 文書

- `docs/WORLD_SIMULATION.md`: 「Chat rooms」の節に、発言の枠（種類、多くとも 1 人、観測される間だけ、上限、沈黙）を足す。
- `docs/LLM_POLICY.md`: チャットの 1 行の生成が「World が選んだ枠を言葉にするだけ」であることと、材料の一覧。
- `docs/host-programs/erika-k.md`: 住民の発言の表示。頻度と上限は Fictional reconstruction。CRLF のファイルなので改行コードを保つ。
- `docs/DEBUG_INSPECTION.md`: §6 のログ。

## 9. リスクと対策

| リスク | 対策 |
|---|---|
| LLM の遅れで発言が遅れて出る | 枠の 20 秒前に始める。間に合わなければ記録時刻で出す。60 秒過ぎたら捨てる |
| 人間の発言に住民が群がる | `reply` と `welcome` は多くとも 1 人。上限 §3.3 |
| 住民どうしの応答が終わらない | 連鎖の深さ 6 まで、1 時間 360 行まで |
| 費用 | 本文は人間がいる部屋だけ。局ごとに同時 2 件 |
| 表示済みの範囲に割り込む | 記録は presence lock の中で `At = max(枠, 今)`。`ChatEvents` も同じロック |
| 退室が伝わらない | `staleViewer` で滞在を閉じる。呼び出しがなければ生成もしない |
| 枠の計算が重い | 範囲は最大 10 分 + 20 秒、部屋の住民だけ。`TestChatEventsTiming` で 1 回の `ChatEvents` を測る |

## 10. 検証

- `chatslot`: 決定性、多くとも 1 人、ハンドルで応答率が上がる、応答のない発言がある、上限、観測区間の外に枠がない、深さの上限。
- `llm`: 検証の各規則、要求の JSON スキーマと材料（`roundTripFunc`）、人間を特別扱いする語が材料にないこと。
- `worldrepo`: 試験用の生成器で、人間が入ると発言が記録される、人間がいないと生成器が 0 回、失敗・不合格で記録なし、
  同じ枠を 2 回記録しない、表示済みの範囲に割り込まない、`-race`。
- `erikak`: 実際のリポジトリ + 試験用の生成器で、発言と入退室の行が時刻順、自分の発言が二重に出ない、生成器なしで今と同じ出力。
- 金型: `git diff --stat origin/main -- apps/server/internal/hostprogram/erikak/testdata apps/server/internal/world/testdata` が空。
