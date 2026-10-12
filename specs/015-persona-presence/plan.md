# Plan 015: persona-presence

## 1. 現状（2026-10-12 の `main`、1211017 で確認）

| 対象 | 場所 | 現状 |
|---|---|---|
| サーバーの世界時計 | `apps/server/internal/worldclock/clock.go` | `MappedClock`。起動日の 0 時を `WORLD_DATE`（既定 `1996-08-26`、`internal/config/config.go`）に対応させ、経過時間を足す。 |
| クライアントの世界時計 | `apps/web/src/time/WorldClock.ts` | `Japan1996WorldClock`。ページを開いた日を `VITE_WORLD_DATE`（既定 `1996-08-26`、`App.tsx`）に対応させる。世界の壁時計を UTC 欄に入れた `Date` を返す。 |
| 1996 年の祝日 | `apps/web/src/billing/pseudoTariffs.ts` の `JAPAN_1996_HOLIDAYS` | 振替休日と初の海の日を含む 19 日。サーバーには祝日の情報がない。 |
| BUSY | `apps/server/internal/telephone/network.go` の `dialAt` / `busyProbability` | 人気度 × 時間帯係数 ÷ √回線数 の確率。電話番号 `0459999999` の固定の分岐がある。回線番号は時刻から機械的に決める。プリセットの `dial`（`hostcatalog.DialBehavior`）は `dialAt` では参照していない。 |
| 人間の通話 | `apps/server/internal/ws/sessions.go` の `SessionManager` | `Create`（CONNECT 時）と `End` / `expire`（切断・期限切れ）。`CallSession` は `Host`、`Line`、`Baud`、`ConnectedAt` を持つ。 |
| WHO | `apps/server/internal/hostprogram/erikak/runtime.go` | メインメニューの `WHO` と、チャットメニューの `WHO` の 2 か所が固定文言。前者は画面金型 `testdata/screens_golden.txt` に含まれる。 |
| 住民 | `apps/server/internal/world/models.go` の `Persona`、`population.go` の `fillActivitySkeleton` | `ActivityPattern` の 5 段階と傾向値。時刻の概念はない。 |
| 人物事実 | `world.PersonaFactStore`（`SavePersonaFact` / `ListPersonaFacts`） | 開いたキーで保存し、以後変えない。 |
| 永続化 | `apps/server/internal/worldpersist/store.go` | `debug.snapshot` の局だけ、`DevelopmentHostSnapshot` を JSON で保存する。 |

件数・関数名は実装の開始時に読み直し、違っていれば実際の値で報告する。

## 2. 全体の構成

```text
worldclock（暦連動）──> 世界時刻
                          │
worldcalendar（年ごとの暦データ + 効果の係数）
                          │
presence（純粋な計算: 局・時刻・住民・接続属性・保存済みの接続 → 接続の一覧）
                          │
world.MemoryStore / worldrepo.Repository（接続属性の保存、観測した接続の保存、人間の通話の登録）
      │                          │
telephone.Network（BUSY / 回線番号）   hostprogram/erikak（WHO の表示）
      ▲
ws.SessionManager（人間の通話の開始・終了を登録）
```

新しいパッケージは `internal/worldcalendar` と `internal/presence` の 2 つ。`presence` は保存をしない純粋な計算だけを持ち、
保存は `world` の Store が行う。

## 3. 世界時計（FR-001）

### 3.1 サーバー

`worldclock` に `CalendarClock` を追加する。

```go
// CalendarClock maps the real Japanese wall clock onto the world calendar by
// subtracting a fixed number of years. Month, day and time of day are kept, so
// the world date does not depend on when the process started.
type CalendarClock struct {
	source     WorldClock
	location   *time.Location
	yearOffset int
}

func (c *CalendarClock) Now() time.Time {
	return c.source.Now().In(c.location).AddDate(-c.yearOffset, 0, 0)
}
```

- 設定: `WORLD_YEAR_OFFSET`（既定 30）。`WORLD_DATE` が**明示的に**設定されたときだけ、従来の `MappedClock` を使う。
  `config.go` の `WorldDate` の既定値 `"1996-08-26"` は、空文字に変える（未設定なら暦連動）。
  `WorldDate` を参照している他の箇所（`worldrepo.New` の `worldDate` 引数、`/api/health` の `world_date`）は、
  暦連動のときに現在の世界の日付（`clock.Now().Format(time.DateOnly)`）を渡す。`worldrepo.worldTime` のテスト用の固定値はそのまま。
- `/api/health` に `world_clock: {"mode": "calendar" | "fixed_origin", "year_offset": 30}` を加える。
- 閏年: `AddDate` は 2028-02-29 を 1998-03-01 に正規化するため、2028 年 2 月 29 日と 3 月 1 日が同じ世界の日になる。
  本仕様では扱わず、`docs/WORLD_SIMULATION.md`「Time」に既知の制約として書く。

### 3.2 クライアント

`apps/web/src/time/WorldClock.ts` に `CalendarWorldClock` を追加し、同じ規則で計算する（日本時間の年から `yearOffset` を引き、
月日と時刻を保つ）。`App.tsx` は、`VITE_WORLD_DATE` が設定されていれば従来の `Japan1996WorldClock`、
なければ `CalendarWorldClock(VITE_WORLD_YEAR_OFFSET ?? 30)` を使う。料金計算（`PseudoTariffService`）は `WorldClock` インターフェース
越しに使っているので、呼び出し側の変更は `App.tsx` だけになる見込み。違っていれば報告する。

### 3.3 共通の検証表

サーバーとクライアントのテストで、同じ入出力の表を使う（SC-004）。

| 現実（JST） | 世界 |
|---|---|
| 2026-10-12 11:54:00 | 1996-10-12 11:54:00（土） |
| 2026-12-24 22:59:59 | 1996-12-24 22:59:59（火） |
| 2026-12-24 23:00:00 | 1996-12-24 23:00:00 |
| 2026-12-31 23:59:59 | 1996-12-31 23:59:59 |
| 2027-01-01 00:00:00 | 1997-01-01 00:00:00 |

## 4. 暦データ（FR-002、FR-003）

`internal/worldcalendar/data/1996.yaml` に事実を、`internal/worldcalendar/data/effects.yaml` に効果の係数を置き、`embed` で読む。

```yaml
# 1996.yaml（事実。値は候補であり、実装者が出典で確認してから入れる）
year: 1996
holidays:            # Confirmed: 国立天文台の暦要項、祝日法
  - {date: 1996-01-01, name: 元日}
  - {date: 1996-01-15, name: 成人の日}
  - {date: 1996-02-11, name: 建国記念の日}
  - {date: 1996-02-12, name: 振替休日}
  # ... 既存の JAPAN_1996_HOLIDAYS と同じ 19 日
periods:
  - {key: year_end_new_year, from: 1996-12-29, to: 1997-01-03, evidence: likely}   # 慣習
  - {key: obon, from: 1996-08-13, to: 1996-08-16, evidence: likely}
  - {key: school_summer, from: 1996-07-20, to: 1996-08-31, evidence: likely}      # 地域差あり
  - {key: school_winter, from: 1996-12-25, to: 1997-01-07, evidence: likely}
  - {key: school_spring, from: 1996-03-25, to: 1996-04-07, evidence: likely}
days:
  - {key: christmas_eve, date: 1996-12-24, evidence: confirmed}
  - {key: new_years_eve, date: 1996-12-31, evidence: confirmed}
events: []           # 接続に影響しうる史実の出来事。出典があるものだけ入れる
```

- 祝日はサーバーとクライアントで同じ集合であることをテストで確かめる（クライアントの `JAPAN_1996_HOLIDAYS` はそのまま残す。
  共通化はしない）。
- 出典と分類は `docs/research/CALENDAR_1996.md` に記録する。`events` は、出典（新聞縮刷版、公式記録など）で日付を確認できたものだけを
  入れ、確認できなければ空のままにする。
- 年のデータがない日付は、`Day.Known == false` を返し、曜日だけで判断する。

```go
type Day struct {
	Date     time.Time // world date, 00:00
	Known    bool      // calendar data exists for this year
	Weekday  time.Weekday
	Holiday  bool      // national holiday or substitute holiday
	Periods  []string  // keys from periods
	Days     []string  // keys from days
}

func (c *Calendar) Day(worldTime time.Time) Day
```

効果（`effects.yaml`）は、期間・日のキーと生活類型ごとの倍率、時間帯の範囲。キーは開いた文字列で、コードに期間名を分岐させない。

```yaml
# effects.yaml（架空の調整値。歴史統計ではない）
- {when: year_end_new_year, factor: 0.75}
- {when: year_end_new_year, lifestyle: [secondary_student, university_student], factor: 1.3}
- {when: new_years_eve, hours: [23, 25], factor: 1.6}     # 25 = 翌 1 時
- {when: christmas_eve, hours: [18, 24], factor: 0.75}
- {when: obon, factor: 0.8}
- {when: school_summer, lifestyle: [secondary_student, university_student], use_holiday_profile: true}
- {when: school_winter, lifestyle: [secondary_student, university_student], use_holiday_profile: true}
- {when: school_spring, lifestyle: [secondary_student, university_student], use_holiday_profile: true}
```

## 5. 住民の接続属性（FR-005）

人物事実のキー（開いたキー、`SourceKind: "presence_profile"`）:

| キー | 値 | 決め方 |
|---|---|---|
| `presence.lifestyle` | `secondary_student` / `university_student` / `office_worker` / `self_employed` / `homemaker` / `shift_worker` | `Occupation` に「中学」「高校」→ secondary、「大学」「専門学校」「院」→ university、「主婦」「主夫」→ homemaker、「自営」「店」→ self_employed、「夜勤」「看護」「警備」→ shift、それ以外の職業 → office。職業がなければ分布（0.15 / 0.25 / 0.40 / 0.08 / 0.05 / 0.07）から決める。 |
| `presence.phone_line` | `own` / `shared` | own 0.45、shared 0.55（中高生は shared 0.85） |
| `presence.night_owl` | `0.00`〜`1.00`（小数 2 桁） | 一様分布。大学生は +0.2（上限 1） |
| `presence.modem_bps` | 2400 / 9600 / 14400 / 28800 | 9600: 0.25、14400: 0.45、28800: 0.25、2400: 0.05。接続時は局の `MaxBaud` で頭打ち。 |
| `presence.teleho:<hostID>` | `yes` / `no` | 局の `Traits.TelehoFriendly` なら 0.55、そうでなければ 0.25 |

- 乱数は `presence` パッケージ内の `stableUnit(personaID, key, hostID)`（FNV、`telephone.stableUnit` と同じ方式で別実装）から取り、
  人物生成の `rand.Rand` を消費しない。
- 確率の値はすべて架空の調整値。
- 保存は「観測に使ったとき」。WHO に表示された、または人間のダイヤルの BUSY/CONNECT の判定に使われた住民だけ。
  計算だけのときは保存しない（同じ関数で同じ値が出るので、保存前後で結果は変わらない）。

## 6. 接続の計算（FR-004、FR-006）

### 6.1 開始の見込み

10 分を 1 枠とし、住民 `m` が枠 `w` に接続を始める確率を次の積で決める（上限 0.95）。

```text
p(m, w) = daily(m) × profile(lifestyle, daytype, hour) / Σ_hour profile(lifestyle, daytype, ·) / 6
        × teleho(m, host, w) × phone(m, hour) × night(m, hour) × calendar(day, lifestyle, hour) × hostHours(host, w)
```

- `daily(m)`: 1 日あたりの接続回数の期待値。`ActivityPattern` から regular 1.2、active 0.8、occasional 0.35、lurker 0.35、dormant 0.03。
  lurker は書かないが読みには来るので occasional と同じにする。
- `profile`: 生活類型ごとの 24 時間の重み。`daytype` は weekday / holiday（土日・祝日・FR-004 の `use_holiday_profile`）。
  初期値は `internal/presence/profiles.go` に表として持つ（架空の調整値）。目安:

  | 類型 | 平日 | 休日 |
  |---|---|---|
  | secondary_student | 16–18 時に小さな山、21–24 時に山、0–1 時に少し | 10–24 時に広く、21–24 時に山 |
  | university_student | 昼にも少し、22–3 時に大きな山 | 11–4 時に広く |
  | office_worker | 7 時前後に少し、20–1 時に山 | 10–1 時に広く |
  | self_employed | 12–14 時と 21–1 時 | 10–1 時 |
  | homemaker | 10–15 時に山、23 時以降は少し | 13–16 時と 22–24 時 |
  | shift_worker | 全時間帯に平らに近い | 同じ |

- `teleho`: 登録あり → 23:00–07:59 は 2.5 倍、23:00–23:19 の 2 枠は追加で 3 倍（23 時の一斉接続）、08:00–22:59 は 0.5 倍。
  登録なし → 23:00–07:59 は 1.0 倍、19:00–22:59 は 0.7 倍、08:00–18:59 は 0.6 倍。
- `phone`: shared → 19:00–22:59 は 0.4 倍（家族が電話を使う）、それ以外 1.0。
- `night`: 2:00–5:59 の枠は `0.15 + 0.85 × night_owl` 倍。
- `calendar`: §4 の効果の積。データのない年は 1.0。
- `hostHours`: 局の運用時間外は 0。運用時間はプリセットの任意項目 `operating_hours`（例 `["22:00", "08:00"]`）で、
  ない局は 24 時間。既存のプリセットには追加しない。

判定は `stableUnit(hostID, personaID, windowStart.Unix()) < p(m, w)`。

### 6.2 接続時間

開始が決まったら、`stableUnit(hostID, personaID, windowStart.Unix(), "duration")` から対数正規分布で長さを決める。

- テレホ登録ありでテレホ帯に開始: 中央値 50 分、上限 240 分。08:00 を越える場合は 07:59 で切る。
- それ以外: 中央値 12 分、上限 90 分。

### 6.3 回線への割り当て

時刻 `t` の接続を求めるとき、`[t − 4 時間, t]` の枠をすべて見る（上限 240 分なので 4 時間で足りる）。

1. 保存済みの接続（住民・人間）と、登録中の人間の通話を、先に回線に載せる。
2. 計算で得た開始を、（開始時刻、住民 ID）の順に並べ、そのとき空いている最小の回線番号に載せる。空きがなければ捨てる
   （その住民の電話は BUSY だった）。同じ住民の接続が重なる場合は、先のものだけを使う。
3. `t` に接続中のもの（`start ≤ t < end`）を返す。

結果は回線番号の順。各要素:

```go
type Session struct {
	HostID    string
	Actor     world.ActorRef // persona or human member
	Handle    string
	Line      int
	Baud      int
	Start     time.Time
	End       time.Time // planned end; for a live human call, zero
	Persisted bool
}
```

計算量は 1 回あたり（住民数 × 24 枠）。300 人で 7,200 回のハッシュ計算。WHO とダイヤルのたびに計算してよい。

## 7. 保存と観測（FR-007、FR-008）

`internal/world/store.go` に任意の能力として追加する（既存の任意能力と同じ書き方）。

```go
// PresenceStore keeps observed connection sessions. A session that was shown to
// a caller (WHO) or decided a caller's dial is world history and never changes.
type PresenceStore interface {
	ListObservedSessions(hostID string, from, to time.Time) []PresenceSession
	SaveObservedSessions(hostID string, sessions []PresenceSession)
}

// LineOccupancy registers live human calls so that they hold a logical line.
type LineOccupancy interface {
	BeginHumanCall(call HumanCall) (line int, ok bool)
	SetHumanCallHandle(callID, handle string)
	EndHumanCall(callID string, at time.Time)
}

// PresenceObserver is what host runtimes use: the sessions on the line now, as
// world history.
type PresenceObserver interface {
	ObservePresenceNow(host Host) []PresenceSession
}
```

- `PresenceSession` は §6.3 の `Session` を `world` に置いたもの（`presence` が `world` を参照するので、型は `world` 側に置く）。
- `MemoryStore` が `PresenceStore` と `LineOccupancy` を実装する。`worldrepo.Repository` が `PresenceObserver` を実装し、
  `worldNow` で時刻を決め、計算 → 観測した接続と接続属性の保存 → 返却、を行う。`Repository` は Base への委譲で
  `PresenceStore` / `LineOccupancy` も満たす。
- 同じ局の観測は、局単位のミューテックスで直列化する（同時に WHO を見た 2 人が別の結果を保存しない）。
- `worldpersist.Store` は、`debug.snapshot` の局について、観測した接続を `DevelopmentHostSnapshot` に含めて保存・復元する。
  スナップショットに新しい欄を足すだけで、古いスナップショット（欄なし）も読めること。

### 7.1 ダイヤル

`telephone.Network.dialAt`:

1. 電話番号 `0459999999` の固定分岐（`presets/busy-test.yaml` の `behavior: busy_first_n`、`busy_first_n: 5` と同じ挙動）は、
   在席の計算より先に、従来どおり評価する。`world.Host` は現在 `Dial` を持たないので、定義の値で置き換えることは本仕様では行わない
   （固定分岐の除去は別 PR）。
2. Store が `PresenceStore` と `LineOccupancy` を持たない → 従来の `busyProbability`（既存のテストのため）。
3. それ以外 → §6.3 で時刻 `at` の接続を計算し、回線に空きがあれば CONNECT（空いている最小の回線番号）、なければ BUSY。
   どちらの場合も、判定に使った住民の接続を観測として保存する。
4. 通信速度は従来の `negotiatedBaud` のまま。

### 7.2 人間の通話

`ws.SessionManager.Create` の呼び出し元（`ws/session.go` の `dial`）で CONNECT のあと `BeginHumanCall`、`End` と `expire` で `EndHumanCall`。
`SessionManager` は Store を知らないので、`NewSessionManager` に任意の `world.LineOccupancy` を渡せるようにする（nil なら何もしない）。
`Create` に渡す回線番号は `BeginHumanCall` が返した番号にする。

ダイヤルの判定と `BeginHumanCall` の間に別の通話が入る競合は、`BeginHumanCall` が空きを再確認し、空いていなければ
`ok == false` を返して、そのダイヤルを BUSY に変えることで防ぐ。

## 8. WHO（FR-009）

- `hostprogram/erikak/runtime.go` の 2 か所の `WHO` を、`world.PresenceObserver` から得た接続を書式化する関数に置き換える。
  Store が `PresenceObserver` を持たないときは、自分の行だけを表示する。
- 書式は現在の文字列と同じ: メインメニューは `LINE %d  %-9s %5d`（住民・他の人間）と `LINE %d  %-10s ONLINE`（自分）、
  チャットメニューは `LINE%d %s` を 2 空白区切りで並べる。実際の書式は現在のコードから写し、空白の数まで一致させる。
- 自分の回線番号は、`ws` が CONNECT 後にランタイムへ渡す。`hostprogram` に任意のインターフェース
  `CallInfoReceiver{ SetCallInfo(callID string, line, baud int) }` を追加し、絵理香K版が実装する。
  ログイン（ハンドルが決まった時点）で `LineOccupancy.SetHumanCallHandle` を呼ぶ。
- **画面金型**: `screens_golden.txt` の WHO の出力（`LINE 1 SYSOP 14400` / `LINE 2 MARI 9600` / `LINE 3 GUEST ONLINE`）は、
  金型テストのハーネス（`golden_test.go` の `screenTranscript`）で、この 3 行を返す試験用の `PresenceObserver` と
  `SetCallInfo("golden", 3, ...)` を与えることで、**金型ファイルを変えずに**一致させる。これが「書式を変えていない」ことの証拠になる。
  ハーネスの変更はこの注入だけにする。
- アクセス記録（`T` コマンド）の固定の名前（NORI / MARI / SYSOP）も同じ問題を持つが、本仕様の範囲外。

## 9. 検証

| テスト | 場所 | 内容 |
|---|---|---|
| 暦連動の時計 | `worldclock/clock_test.go`、`web/src/time/WorldClock.test.ts` | §3.3 の表 |
| 暦データ | `worldcalendar/calendar_test.go` | 1996 年の 19 日の祝日（クライアントの集合と同じ）、振替休日、期間、データのない年 |
| 層ごとの係数 | `presence/factors_test.go` | 各層を単独で: 23:00 の前後、共有回線の 19–23 時、夜型度、運用時間外 |
| 決定性 | `presence/compute_test.go` | 同じ入力 → 同じ結果。住民の順序を入れ替えても同じ。 |
| 回線数 | `presence/compute_test.go` | 1,000 の時刻で、接続数 ≤ 回線数（SC-003） |
| テレホタイム | `presence/compute_test.go` | 試験用の局（300 人、4 回線ではなく十分な回線数）で、60 日分の 22:50 と 23:10 の合計を比べ、23:10 > 1.5 × 22:50 |
| 平日昼 | 同上 | 平日 14:00 の合計 < 土曜 14:00 の合計、1996-11-04（振替休日）は平日より多い |
| 観測の保存 | `world` または `worldrepo` | 観測後に住民を追加して再計算しても、保存した接続が残る。同時の観測で保存が 1 回 |
| ダイヤル | `telephone/network_test.go` | 満杯で BUSY、空きで CONNECT、`0459999999` の固定分岐の維持、Store が能力を持たないときの従来挙動 |
| 人間の通話 | `ws` | CONNECT で回線を占有、`End` と期限切れで解放 |
| WHO | `erikak/runtime_test.go`、画面金型 | 局固有の名前がコードにない（`grep -n "MARI" runtime.go` の WHO 部分が 0 件）、画面金型が無変更で通る |
| 人物生成金型 | `world/population_golden_test.go` | 無変更で通る（SC-002） |

統計的なテストの閾値は、決定論的な計算なので一度通れば常に通る。閾値は実装後に実測値を見て決めず、上の値で書く。
通らなければ、係数ではなく実装の誤りを先に疑い、2 回直して通らなければ止めて報告する。

## 10. 変更するファイル

- 追加: `internal/worldcalendar/`（`calendar.go`、`effects.go`、`data/1996.yaml`、`data/effects.yaml`、テスト）
- 追加: `internal/presence/`（`profile.go`、`profiles.go`、`factors.go`、`compute.go`、テスト）
- 追加: `internal/world/presence.go`（型と `MemoryStore` の実装）、`docs/research/CALENDAR_1996.md`
- 変更: `internal/worldclock/clock.go`、`internal/config/config.go`、`cmd/server/main.go`、`internal/world/store.go`、
  `internal/world/development_snapshot.go`、`internal/worldpersist/store.go`、`internal/worldrepo/`（`PresenceObserver`）、
  `internal/telephone/network.go`、`internal/ws/sessions.go`、`internal/ws/session.go`、`internal/hostprogram/runtime.go`（インターフェース）、
  `internal/hostprogram/erikak/runtime.go`、`internal/hostprogram/erikak/golden_test.go`（ハーネスの注入のみ）、
  `internal/hostcatalog/`（`operating_hours` の読み込みと検証）
- 変更: `apps/web/src/time/WorldClock.ts`、`apps/web/src/App.tsx`、テスト
- 変更: `docs/WORLD_SIMULATION.md`、`docs/PRODUCT_SPEC.md`、`docs/TERMINAL_AND_MODEM.md`

## 11. リスクと対策

| リスク | 対策 |
|---|---|
| 本番で `WORLD_DATE` が設定されていると、暦連動にならない | PR の説明に、デプロイ設定から `WORLD_DATE` / `VITE_WORLD_DATE` を外す必要があることを書く。`/api/health` の `world_clock.mode` で確認できる。 |
| 暦連動に切り替えた時点で、保存済みの記事の時刻より世界の時刻が前になる | 切り替え時の世界の日付は現実の月日（10 月以降）で、従来の既定 `1996-08-26` 起点の記事より後になる見込み。起動時に、保存済みの最新の記事の時刻が世界の現在より後なら警告をログに出す（止めない）。 |
| 接続属性（生活類型）と、後から LLM が決める職業が矛盾する | 接続属性は人物事実として保存されるので、人物事実を文脈に入れる既存の生成経路で参照される。矛盾の検出は後続 spec。 |
| 係数の調整で、テレホタイムの山が強すぎる・弱すぎる | 係数は表として 1 か所にまとめ、開発用に時刻ごとの接続数を出すテスト（`-run TestPresenceHistogram -v`）を用意する。値の調整は別 PR。 |
| WHO を見るたびに保存が増える | 同じ接続は 1 回だけ保存する（局・主体・開始時刻で一意）。 |
