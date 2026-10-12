# Plan 016: chat-foundation

## 1. 現状（読んだコード）

| 場所 | 現状 |
| --- | --- |
| `hostprogram/runtime.go:17-20` | `Runtime` は `Welcome()` と `HandleLine(line)` だけ。任意のインターフェースとして `ObservationCatalog`、`CallInfoReceiver` がある |
| `ws/session.go:84-248` | 接続ごとに 1 つの読み取りループ。`line` を受けて `HandleLine` の結果を 1 回送る。ほかに送る経路はない |
| `ws/sessions.go:20-31` | `CallSession` は `Runtime` を持つが、送信先の接続も、通話ごとのロックも持たない。再接続（`Resume`）で別の接続に付け替わる |
| `apps/web/src/modem/VirtualModem.ts:411-418` | `terminal` メッセージは、いつ届いても端末に書く。Web 側の変更は不要 |
| `world/presence.go:19-27` | `PresenceSession{HostID, Actor, Handle, Line, Baud, Start, End}`。通話の中の状態はない |
| `worldrepo/presence.go` | `presenceAt`（計算、保存なし）、`observePresence`（保存）、局ごとのロック `presenceLock` |
| `presence/compute.go` | `Compute(Input, t) Result`。300 人で 1 回 約 4 ms |
| `erikak/runtime.go:254-259, 570-583, 876-883` | 電報・チャットのスタブ |
| `erikak/runtime.go:1201-1225` | `lineUsers()`（spec 015）。メインメニューの WHO が使う。本仕様では変えない |
| `erikak/runtime.go:1135` | `padRunes` は表示幅（全角 2、半角 1）で詰める。WHO と発言の行に使える |
| `world/models.go:165-177` | `InteractionEvent`。`Channel` に `chat_utterance` の想定がある。`telegram` はまだない |
| `hostcatalog/program_detail.go:17-38` | `ErikaKDetail{Texts, Boards}`。`ErikaKTexts` は局が書く文面の名前つきの部品 |

## 2. 資料からの書式（GARAKUTA-KoBo、1996-04-30）

分類: 書式は **Likely（ソフトウェアの既定）**。資料は 1 局の記録だけで、ソフトウェアの既定か局の設定かを区別できない。
行の形とプロンプトはソフトウェア側（ホストプログラム）に置き、見出し・項目の文面・部屋と定員・注意文は局の定義に置く。

```text
【電報】C                                                    ← 局の定義: call_menu.title
------------------------------------------------------------------------------   ← 局の定義: call_menu.lines（罫線を含む）
【0】～【16】電報を送る   【*】全員に電報を送る    【P】１行ﾌﾟﾛﾌｨｰﾙ(PROF)
...
------------------------------------------------------------------------------
(空行)
#03 GUEST03   WHO    ○ 28K-...                                ← WHO 行（ホストプログラム）
#06 ring
(空行)
[番号]=電報 [*]=全員宛 [P]=PROF [ﾘﾀｰﾝ]=WHO [.]戻る [S]受け取り変更 [X]=CHAT   ← 局の定義: call_menu.hint
CALL> ->                                                      ← ホストプログラム
```

WHO 行の書式（表示幅で数える）:

```text
"#" + 2 桁の回線番号 + " " + padRunes(ハンドル, 10) + padRunes(状態, 7) + アイコン欄(3 桁) + 速度
空き回線: "#" + 2 桁 + " ring"
```

- 状態: `login`（接続から 2 分）/ `idle` / `CALL`（電報メニューにいる）/ `CHAT`。記録には `WHO`（WHO を表示中）と `SEND`（電報を入力中）もある。
  住民には `WHO` と `SEND` を使わない（短すぎて意味がない）。人間自身の行は `WHO`（記録では、WHO を表示するとき自分の行は常に `WHO`。チャットから出た直後の WHO も同じ）。
- アイコン欄: `CHAT` のときは `#` + 部屋番号 + 空白（例 `#0 `）。それ以外は記録では利用者ごとの 1 文字（○、み、火 など）だが、
  意味が確認できない（`ERIKA_K_REVERSE_ENGINEERING.md:228`）。**本仕様では空白 3 桁にする**（Fictional reconstruction として文書に書く）。
- 速度: 記録は `28K-...`、`26K-...`、`14K-...`。**`fmt.Sprintf("%dK-...", baud/1000)`**（Likely。14,400 bps 未満の表示は記録にない）。
  記録の `[52]` や `TERMINAL` は扱わない。
- 回線番号は 2 桁（記録は `#02`〜`#14`）。局の回線数が 1 桁でも 2 桁で出す。
- 行末の空白は記録にあるが、本仕様では付けない（端末の表示は同じ）。

電報（ホストプログラム）:

```text
CALL> ->5
電報をどうぞ。 (最大全角150文字  中止:ﾘﾀｰﾝｷｰ)
---------+---------+---------+---------+---------+---------+---------+---------
(本文 1 行)
#05 に電報を送りました。
(空行)
(hint)
CALL> ->
```

相手が `CHAT` 中:

```text
#02 チャットルームか会議室にちゃっかり入ってます。 CHAT終了後、電報を送ります。
(空行)
(WHO の全行)
(空行)
(hint)
CALL> ->
```

空き回線・存在しない回線への電報の文面は記録にない。**`? COMMAND ERROR` を返す**（Fictional reconstruction）。

チャット（`X`）:

```text
-----------------------------------                ← 局の定義: chat_menu.lines（見出しと部屋の一覧を含む全行）
       ☆ＣＨＡＴ ＭＥＮＵ☆
-----------------------------------
【0】 CHAT ROOM 0 に入る (定員16名)
...
-----------------------------------
ルーム番号をどうぞ  （中止はﾘﾀｰﾝｷｰ） -->           ← ホストプログラム
(空行)
  チャット終了は 「..」 または 「CTRL+B」 を入力して下さい   ← ホストプログラム
------------------------------------------------------------------------------   ← 局の定義: chat_rooms[n].notice（任意、複数行）
     午後１１時から...
------------------------------------CHAT ON-----------------------------------   ← ホストプログラム（78 桁）
## chat on  自分 ##                                  ← BEL 2 回が前に付く
```

- 入退室: `"\a\a## chat on  " + handle + " ##"`、`"\a\a## chat off " + handle + " ##"`（`on` の後は空白 2 つ、`off` の後は 1 つ）。
  記録では入退室の行の前に空行が入る場合と入らない場合がある。**本仕様では入れない**。
- 発言: `padRunes(handle, 8) + ":" + 本文`。
- 退室: 人間が `..` を入力すると、`padRunes(handle,8) + ":.."`、空行、`"\a\a## chat off " + handle + " ##"`、
  `CHAT OFF` の罫線（78 桁。`-` 36 個 + `CHAT OFF` + `-` 34 個）、空行、WHO の全行、チャットのメニュー（空行なし）、
  `ルーム番号をどうぞ…`、の順（記録どおり）。`CHAT ON` の罫線は `-` 36 個 + `CHAT ON` + `-` 35 個。
  `CTRL+B` は Web 端末から送れないので扱わない。
- チャットのメニューで空行（中止）を入力すると、**メインメニューに戻る**（記録では電報メニューではなくメインメニューが出る）。
- 部屋の定員: 局の定義の `capacity`（0 は無制限）。満員の部屋に入ろうとしたときの文面は記録にない。**`? COMMAND ERROR` を返す**。

## 3. 局の定義（YAML）

`hostcatalog.ErikaKDetail` に `Chat *ErikaKChat` を加える（`yaml:"chat"`）。なければ電報・チャットは今のスタブのまま
（既存の局の挙動を変えない）。`hakata-canal-net.yaml` には、架空の局の文面として新しく書く（GARAKUTA の文面を写さない。
局固有の文面は局ごとに違うため）。

```go
type ErikaKChat struct {
	CallMenu struct {
		Title string   `yaml:"title"`
		Lines []string `yaml:"lines"`
		Hint  string   `yaml:"hint"`
	} `yaml:"call_menu"`
	ChatMenu []string     `yaml:"chat_menu"`
	Rooms    []ErikaKRoom `yaml:"rooms"`
}

type ErikaKRoom struct {
	Number   int      `yaml:"number"`
	Capacity int      `yaml:"capacity"` // 0 = 無制限
	Notice   []string `yaml:"notice"`
	// Weight は住民が部屋を選ぶ相対的な重み（架空の調整値）。
	Weight float64 `yaml:"weight"`
}
```

検証（`hostcatalog` の読み込み時）: 部屋番号が 0〜9 で重複しない、定員が 0 以上、重みが 0 以上で合計が正。

## 4. 非同期出力（FR-001）

```go
// hostprogram/runtime.go
// AsyncOutput is optional: a host program that has something to show without
// input (chat lines, an arriving telegram) returns it from Poll. The program
// decides when interruption is appropriate; it returns "" otherwise.
type AsyncOutput interface {
	Poll(now time.Time) string
}
```

- `now` は世界時刻（`world.WorldNowOf(store)`）。`ws` は `world.WorldTimeSource` を持たない Store では `time.Now()` を渡す。
- `CallSession` に `mu sync.Mutex` を加え、`HandleLine` と `Poll` をこのロックの中で呼ぶ（`ws/session.go` の `line` の処理と、新しいポーリング）。
- `ws/session.go` の `ServeHTTP` で、CONNECT または Resume が成功したら、その接続のためのゴルーチンを 1 つ起動する:
  1 秒ごとに `sessions.IsCurrent(id, attachment)` を確かめ、現在の接続なら `Poll` を呼び、空でなければ `writeJSON(terminal)`。
  接続が終わる（`ctx.Done()`）か、通話が別の接続に付け替わる・終わると止まる。
- 書き込みの順序: `conn.Write` は同時に呼ばない。接続ごとに `writeMu` を持ち、読み取りループの書き込みもポーリングの書き込みも通す。
- 間隔は `ws` の定数 `PollInterval = time.Second`。テストでは関数を直接呼ぶ（ゴルーチンのテストは `-race` で 1 本だけ）。

## 5. 通話の中の行動（FR-002〜FR-004）

### 5.1 型

```go
// world/presence.go
type CallState string

const (
	CallLogin CallState = "login"
	CallIdle  CallState = "idle"
	CallMenu  CallState = "CALL"
	CallChat  CallState = "CHAT"
)

// CallSegment is one stretch of a call in one state. Room is set for CHAT.
type CallSegment struct {
	From  time.Time
	To    time.Time
	State CallState
	Room  int
}

// CallActivity is the plan of one resident call, observed as a whole the first
// time any part of it is shown, and never rewritten afterwards.
type CallActivity struct {
	HostID   string
	Actor    ActorRef
	Start    time.Time // == PresenceSession.Start
	Segments []CallSegment
}
```

`PresenceStore` に `ListCallActivities(hostID string, from, to time.Time) []CallActivity` と
`SaveCallActivities(hostID string, acts []CallActivity)`（同じ `Actor` と `Start` がすでにあれば何もしない）を加える。
`MemoryStore`、`DevelopmentHostSnapshot`（`json:"call_activities,omitempty"`）、`worldpersist` で保存・復元する（spec 015 の接続と同じ作り）。

### 5.2 計算（`internal/callplan`、新規、保存しない）

```go
package callplan

type Input struct {
	Host     world.Host
	Rooms    []hostcatalog.ErikaKRoom // nil または空ならチャットなし
	Chat     bool                     // FR-004: false なら CHAT を作らない
	Sessions []world.PresenceSession  // 計算する通話と、部屋の混み具合に使う通話（重なるもの全部）
	Profiles map[string]presence.Profile
	Personas map[string]world.Persona
	Observed []world.CallActivity     // 保存済み。これがある通話は計算しない
}

// Plan returns the activity of every resident session in in.Sessions.
func Plan(in Input) map[string]world.CallActivity // key: actor ID + start
```

1 つの通話の計画（`(局, 住民, 通話開始)` のハッシュから。係数はすべて架空の調整値とコメントに書く）:

1. `login`: 開始から 2 分。
2. 残りを `idle` にする。そのうえで、チャットに入るかを決める:
   - **基本の意向** `intent = base(生活類型, 開始時刻) × 社交性`。テレホ帯（23:00〜08:00）は昼の 3 倍。
     社交性は `1 − LurkerTendency` と `ReplyTendency` の平均（`world.Persona` の既存の値）。
   - 入る時刻は通話の 15〜60% の位置。部屋は `Rooms` の重みで選ぶ。
   - **混み具合**: 入る時刻にその部屋にいる住民の数 `n` を、**他の通話の基本の意向だけで決めた計画**から数える（2 段にして再帰しない）。
     `n == 0` なら入る確率を 0.4 倍、`n >= 1` なら 1.5 倍。定員に達していたら入らない。
   - 入ったら、滞在は `n == 0` なら 2〜6 分（誰もいないので早く出る）、そうでなければ通話の残りの 30〜90%。
3. `CALL`: `CHAT` の直前 0.5〜2 分（メニューを経由するため）。
4. 通話の終わりまでに `CHAT` が終わらない場合は、通話の終わりで切る。

**人間のことは計算に入れない**（FR-002）。人間がチャット室にいても、住民の計画は変わらない。

### 5.3 リポジトリ（`worldrepo/callplan.go`、新規）

```go
// CallActivitiesNow returns the plans of the residents on the host's lines now,
// saving them as observed. Same lock as presence.
func (r *Repository) CallActivitiesNow(host world.Host) []world.CallActivity
// ChatEvents returns room n's join/leave events in (from, to], saving the plans
// they come from as observed.
func (r *Repository) ChatEvents(host world.Host, room int, from, to time.Time) []ChatEvent
```

- 部屋の混み具合を数えるため、`[t − Lookback, t]` で重なる通話を集める。`presence.Compute` は 1 時刻の結果を返すので、
  `t − Lookback` から 10 分ごとに `presenceAt` を呼んで通話を集める（24 回 × 約 4 ms。WHO 1 回で約 100 ms）。
  **性能の確認を tasks に入れる**（§9）。遅すぎる場合の対策（局ごとの 10 分窓のキャッシュ）は §8。
- 保存は `observePresence` と同じバッチの中で行う。
- `world.ChatHost` インターフェース（`CallActivitiesNow`、`ChatEvents`、`SendTelegram`、`RecordChatUtterance`）を
  `world/store.go` に置き、絵理香K版はこれを型アサーションで使う（spec 015 の `PresenceObserver` と同じ）。

### 5.4 チャットの有効化（FR-004）

`Repository.SetResidentChat(enabled bool)`。`cmd/server/main.go` で `cfg.AzureOpenAIKey != ""` のとき `true`。
既定は `false`。

## 6. 絵理香K版（FR-005、FR-006）

- 状態を足す: `"call"`（`CALL> ->`）、`"telegram_body"`（電報の本文）、`"chat_menu"`（部屋番号）、`"chat_room"`（チャット中）。
  局の定義に `chat` がなければ、今の `"chat"` のスタブを使う（既存の局と既存のテストを変えない）。
- メインメニューの `4` / `C` / `CHAT`: 定義があれば `call` へ。電報メニュー、空行、WHO、空行、hint、`CALL> ->`。
- `call` の入力: 数字 → 電報（その回線に人がいれば `telegram_body`、いなければ `? COMMAND ERROR`）、空行 → WHO、`X` → `chat_menu`、
  `.` → メインメニュー、それ以外（`*`、`P`、`H`、`A`、`B`、`S` を含む）→ `? COMMAND ERROR` と hint とプロンプト。
- `chat_room` の入力: `..` → 退室。それ以外 → 自分の発言の行を出し、`RecordChatUtterance` で台帳に保存。
- `Poll(now)`: `chat_room` のときは、前回の `Poll` から `now` までの入退室（`ChatEvents`）を時刻順に出す。
  `call` のときは何も出さない（住民からの電報は spec 018）。それ以外の状態でも何も出さない。
  入室した時刻を最初の「前回」にする（入室前の入退室は出さない）。
- 住民が通話を終えた（`CHAT` の区間が通話の終わりで切れた）場合も `## chat off ##` を出す。
- 人間自身の WHO の行は常に `WHO`（§2）。ただし他の画面（メインメニューの WHO）は spec 015 のまま。
- 台帳: 発言は `InteractionEvent{Channel: "chat_utterance", AudienceScope: "chat_room:<host>:<room>", AddressedTo: その時点で部屋にいる住民, ContentRef: "chat:<id>"}`。
  本文は新しい `world.ChatUtterance{ID, HostID, Room, Actor, Body, At}` として保存。電報は `world.Telegram{ID, HostID, From, To, Body, SentAt, DeliverAt}`
  と `InteractionEvent{Channel: "telegram", AudienceScope: "private:<host>"}`。

## 7. 文書

- `docs/host-programs/erika-k.md`: 電報・チャットの節。§2 の書式と分類（Likely / 局固有 / Fictional reconstruction の項目を明記）。
- `docs/ARCHITECTURE.md:345` 付近の「電報・チャットは固定のスタブ」を更新。
- `docs/WORLD_SIMULATION.md`「Lines and presence」の予定の段落を、実装済みの部分（入退室）と予定（発言）に分ける。

## 8. リスクと対策

| リスク | 対策 |
| --- | --- |
| WHO 1 回で 24 回の `presence.Compute`（約 100 ms） | 計測を tasks に入れる。200 ms を超えたら、10 分窓ごとの `presence.Result` を局ごとにキャッシュする（保存済みの接続が変わったら捨てる） |
| ポーリングと入力の競合 | 通話ごとのロック。`go test -race ./internal/ws` |
| 再接続で入退室が二重・欠落 | 「前回」の時刻をホストプログラムが持ち、`Poll` は時刻で区切る |
| 部屋の混み具合の計算が実行のたびに違う | 2 段の計算（基本の意向だけで数える）と、観測したら計画を丸ごと保存 |
| 人間の入室で住民の行動が変わる | 計算に人間を入れない。テストで人間の有無による計画の一致を確かめる |

## 9. 検証

- `callplan`: 決定性（同じ入力で同じ計画）、`Chat=false` で `CHAT` なし、定員を超えない、人間の有無で計画が同じ、テレホ帯で `CHAT` が多い。
- `worldrepo`: 観測した計画が住民の追加後も残る、同時の観測で保存 1 回、`ChatEvents` が時刻順で重複しない、WHO 1 回の所要時間（ログに出す）。
- 絵理香K版: 電報メニュー、WHO 行の書式（全角のハンドルを含む）、電報の 3 つの場合、入室から入退室の表示、`..` の退室、入力中に `Poll` が空、
  定義に `chat` がない局で今のスタブのまま。
- `ws`: `Poll` の出力が `terminal` で届く、入力と並行して壊れない（`-race`）。
- 金型: `screens_golden.txt`、`boards_golden.json`、人物生成の金型が変わらない。
