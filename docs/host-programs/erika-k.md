# 絵理香K版 reconstruction notes

## Target

Current prototype sample: fictional **HAKATA CANAL NET**, phone `0920000196`, running an Erika-K-inspired runtime. The station itself is fictional. The runtime should increasingly follow documented 絵理香K版 interaction semantics rather than a generic BBS menu with Erika labels.

## Confirmed / strongly evidenced

Surviving late-1990s connection logs show an Erika K 1.93 station using a flow including:

- `YOUR ID:`
- `PASSWORD:`
- previous-access information
- a station-specific WELCOME/banner
- `ERIKA-K Ver1.93` identification
- dense Main Menu mixing numeric selections and mnemonic commands
- board, file, mail, telegram/chat, junk, settings, SYSOP mail, logout, enrollment, automatic-operation, board-map, unread-search, access-log and help-related entries
- hierarchical board/forum navigation with `<番号><ALIAS>` naming (e.g. `<10><SIG>`, `<11><COMP>`)
- RETURN or `.` to move up a hierarchy in the observed board flow
- `/` to return toward Main Menu in the observed flow
- current-location prompts of the form `(BJ\...) BOARD [M]=MENU [?]=HELP -->` (standard) or station-specific variations
- unread indicators and article/message counts in board menus
- board index (BX) layout showing:
  - `BD# <2-digit> <Board Name>` header and `# 最新10インデックス表示`
  - column structure: `___No. __date__ time_ _author_  ap/ref___________i n d e x_______________`
  - explicit append count under `ap/ref` column for parent articles with appends
  - commands at index prompt including `0/00/T` (unread), `W` (write), `A` / `A <n>` (append), `.` / RETURN (up), `/` (main)

Older surviving logs show a command-oriented vocabulary including families such as:

- `BM`, `BX`, `BXS`, `BR`, `BW`, `BWX`, `BKILL`, `BJ`
- `FM`, `FX`, `FXS`, `FR`, `FW`, `FWX`, `FKILL`, `FJ`
- `MX`, `MR`, `MW`, `MKILL`
- `CHAT`, `CALL`, `WHO`, `MEMB`, `PASS`, `MODE`, `GUIDE`, `BYE`

The implementation should not assume every item above behaved identically in every version or station configuration.

The Erika family is also documented as using a parent-message + appended-response model: replies (`アペ`) are appended under a root message and can be read together with the root. Preserve this semantic difference rather than flattening it into a modern forum reply UI.


Current evidence does **not** establish a generic `Re: <root subject>` reply
subject convention for Erika-K. The reconstruction therefore stores an append as
a child/append relation with **no independent append subject**, and the Erika-K
runtime renders it as `アペ 1`, `アペ 2`, etc. under the root. This is a
conservative reconstruction choice: if stronger primary evidence later shows a
separate append subject field in the target version, update this rule from that
evidence rather than borrowing syntax from another BBS package.

NMODEM support is documented in secondary protocol references and may be exposed where appropriate. Actual binary-transfer protocol implementation is a separate task.

## Account ID / handle research

The surviving Erika-K connection evidence confirms an explicit ID/password login flow.

However, the current research has not established the handle validation rules needed for the identity generator:

- whether a separate changeable handle exists in every target version
- handle charset
- punctuation
- maximum/minimum length
- case preservation/comparison
- duplicate rules

Do not borrow these rules from KTBBS, mmm or BIG-Model.

Project-wide handle policy is documented in:
docs/research/ACCOUNT_ID_HANDLE_EVIDENCE.md

## Station-specific / fictional

The following are service fiction unless separately sourced:

- HAKATA CANAL NET name and telephone number
- 福岡 local identity and welcome copy
- SYSOP/member identities
- sample article text
- exact board names and hierarchy
- hidden semantic scope / activity weighting attached to each fictional HAKATA board
- hidden `BJ 99` night-owl board
- sample online users and access records

HAKATA board semantic scopes are world/station metadata, not Erika-K host-program
defaults. They exist so a broad label such as `Ｑ＆Ａ（質問ボード）` does not let
a title generator infer an arbitrary specialist meaning. In particular, the
fictional HAKATA Q&A board is a general question/consultation board; its large
history must not collapse into PC/game questions merely because those subjects
exist elsewhere on the station.

The station catalog's `SemanticScope` is passed to the Situation proposer as
positive board-purpose data. It is not an Erika-K software-wide convention
or a prescriptive list of Situation topics. The HAKATA Q&A board additionally
fixes the World-selected root conversational act to `ask_peers`. The HAKATA
station notice board is `sysop_only` for World-originated root posts; its
interactive prototype is read-only for root posts because member passwords
are not yet verified. Appends to notices remain separately represented by
the Erika-K runtime. Do not mistake these fictional station policies for
historically verified default Erika-K access control.

`夢工房はかた` has no verified historical subject matter. Its broad member
exchange scope is explicitly a **temporary fictional HAKATA station choice**,
not an inference that the real board was about creative work or gaming. The
two offline boards also have different fictional purposes: casual discussion/
proposals versus actual meeting logistics. No group event becomes established
in the world from a title or prose alone; future multi-person event
coordination requires canonical event/relationship state.

These are intentionally allowed to vary per station while preserving the host software's interaction grammar.

## Station configuration model

The reconstruction separates station-wide capability switches from user permissions.

`station master -> account/role permission -> resource/state ACL`

A station can disable entire services regardless of user role. Transfer protocols are independently switchable and default to all enabled. The reconstruction currently supports modern JSON configuration rather than attempting to guess the historical Erika-K config syntax.

See `apps/server/config/erika-k.example.json`.

## Current implementation direction

`apps/server/internal/hostprogram/erikak/` owns the Erika K state machine. Do not fold it back into a generic profile-driven runtime.

HAKATA CANAL NET starts with no articles. CONNECT, login, and forum navigation do not materialize headers; explicitly opening a leaf board requests only that board's headers for the index. Existing headers render without another observation job. Article bodies remain lazy and are materialized only when a caller opens the article.

The sample runtime currently demonstrates:

- ID/password login sequence
- dense mixed Main Menu
- hierarchical board tree with `<番号><ALIAS>` naming for forum categories
- `BJ`-style path navigation with `.` and RETURN to return up hierarchy
- board index (BX) layout showing `BD#` header, reverse chronological ordering, and `ap/ref` append counts
- index commands including `A` (append to thread), `W` (write new article), and `.` (parent hierarchy)
- root + アペ rendering and append writes
- file/mail/chat/mode/junk surfaces
- old command aliases where useful
- station-specific hidden board

This remains a reconstruction, not a byte-for-byte emulator of vendor binaries.

## Research sources

Preserve URLs because some sources are community archives and may disappear. Detailed analysis and captured primary sources are archived in `docs/research/ERIKA_K_PRIMARY_SOURCES.md` and `docs/research/erika-k-sources/`.

- K&Kネット (宮崎県) 1996年12月19日 実機接続キャプチャ (`kklog01.gif`, `kklog02.gif`): ボードメニュー、最新10インデックス表示（BX）、ファイルライブラリ（FM/FJ）
- 東京謎ねっと２３（文京区、03-5261-7683）「絵理香の話 １〜４」（2000年4月執筆、てんてん氏）: 電電公社九州総局/BCC、東京「BBS工事現場」によるK版開発の経緯
- 草の根BBS どんぐり倶楽部（茨城県、0294-73-2555、1994年〜）: 絵理香K版Plus! 12回線運用記録
- 通信用語の基礎知識 2000「絵理香」項目: 東京がらくた工房（1万人規模）での採用、標準版/K版価格等
- Surviving 1989/1998 Erika/Erika-K connection-log discussion: https://mixi.jp/view_bbs.pl?comm_id=386567&id=3644356
- Tokyo GARAKUTA-KoBo 1996 connection/chat log (preserved third-party archive): https://sixsamana.com/library/lib/D-00078.html
- Tokyo GARAKUTA-KoBo hidden-board log showing append controls: https://sixsamana.com/library/lib/A-00005.html
- Tokyo GARAKUTA-KoBo 1993 board log with `APPEND` interaction: https://sixsamana.com/library/lib/B-00032.html
- Tokyo GARAKUTA-KoBo 1995 board log: https://sixsamana.com/library/lib/A-00025.html
- Retrospective describing Erika's append/reply model: https://kose3.wordpress.com/2000/07/05/%E7%B5%B5%E7%90%86%E9%A6%99/
- Binary transfer protocol reference mentioning Erika K / NMODEM context: https://www.wdic.org/w/WDIC/%E3%83%90%E3%82%A4%E3%83%8A%E3%83%AA%E8%BB%A2%E9%80%81%E3%83%97%E3%83%AD%E3%83%88%E3%82%B3%E3%83%AB

Future research should prioritize surviving distribution archives, manuals, help text, source, screenshots, and additional raw connection logs.


## Lightweight initial board observation (current runtime)

CONNECT/login and forum navigation do not pre-generate titles. Opening an
unmaterialized leaf board requests at most 10 root headers for its ten-line
index; historical append headers and older archive entries are not eagerly
materialized. Prose-free board activity retains the full simulated counts,
and bodies are generated only when the corresponding article is opened.
This is the current performance policy of this fictional sample station,
not a claim about historical Erika K software.
