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
- hierarchical board/forum navigation
- RETURN to move up a hierarchy in the observed board flow
- `/` to return toward Main Menu in the observed flow
- current-location prompts of the form `(BJ\...) BOARD [M]=MENU [?]=HELP -->`
- unread indicators and article/message counts in board menus

Older surviving logs show a command-oriented vocabulary including families such as:

- `BM`, `BX`, `BXS`, `BR`, `BW`, `BWX`, `BKILL`, `BJ`
- `FM`, `FX`, `FXS`, `FR`, `FW`, `FWX`, `FKILL`, `FJ`
- `MX`, `MR`, `MW`, `MKILL`
- `CHAT`, `CALL`, `WHO`, `MEMB`, `PASS`, `MODE`, `GUIDE`, `BYE`

The implementation should not assume every item above behaved identically in every version or station configuration.

The Erika family is also documented as using a parent-message + appended-response model: replies (`アペ`) are appended under a root message and can be read together with the root. Preserve this semantic difference rather than flattening it into a modern forum reply UI.

NMODEM support is documented in secondary protocol references and may be exposed where appropriate. Actual binary-transfer protocol implementation is a separate task.

## Station-specific / fictional

The following are service fiction unless separately sourced:

- HAKATA CANAL NET name and telephone number
- 福岡 local identity and welcome copy
- SYSOP/member identities
- sample article text
- exact board names and hierarchy
- hidden `BJ 99` night-owl board
- sample online users and access records

These are intentionally allowed to vary per station while preserving the host software's interaction grammar.

## Station configuration model

The reconstruction separates station-wide capability switches from user permissions.

`station master -> account/role permission -> resource/state ACL`

A station can disable entire services regardless of user role. Transfer protocols are independently switchable and default to all enabled. The reconstruction currently supports modern JSON configuration rather than attempting to guess the historical Erika-K config syntax.

See `apps/server/config/erika-k.example.json`.

## Current implementation direction

`apps/server/internal/hostprogram/erikak/` owns the Erika K state machine. Do not fold it back into a generic profile-driven runtime.

The fictional HAKATA CANAL NET fixture now keeps at least 40 root article headers in every leaf board (including the hidden board) so board indexes feel populated. These seeded titles/authors/timestamps are **provisional fictional station content**, not historical Erika-K evidence. Their bodies remain lazy and are materialized only when a caller opens the article.

Board/forum/index navigation renders only already-committed headers and does not wait on observation/LLM barriers. A missing article body may still trigger the explicit lazy body materialization path when the article is opened.

The sample runtime currently demonstrates:

- ID/password login sequence
- dense mixed Main Menu
- hierarchical board tree
- `BJ`-style path navigation
- root + アペ rendering and append writes
- file/mail/chat/mode/junk surfaces
- old command aliases where useful
- station-specific hidden board

This remains a reconstruction, not a byte-for-byte emulator of vendor binaries.

## Research sources

Preserve URLs because some sources are community archives and may disappear.

- Surviving 1989/1998 Erika/Erika-K connection-log discussion: https://mixi.jp/view_bbs.pl?comm_id=386567&id=3644356
- Tokyo GARAKUTA-KoBo 1996 connection/chat log (preserved third-party archive): https://sixsamana.com/library/lib/D-00078.html
- Tokyo GARAKUTA-KoBo hidden-board log showing append controls: https://sixsamana.com/library/lib/A-00005.html
- Tokyo GARAKUTA-KoBo 1993 board log with `APPEND` interaction: https://sixsamana.com/library/lib/B-00032.html
- Tokyo GARAKUTA-KoBo 1995 board log: https://sixsamana.com/library/lib/A-00025.html
- Retrospective describing Erika's append/reply model: https://kose3.wordpress.com/2000/07/05/%E7%B5%B5%E7%90%86%E9%A6%99/
- Binary transfer protocol reference mentioning Erika K / NMODEM context: https://www.wdic.org/w/WDIC/%E3%83%90%E3%82%A4%E3%83%8A%E3%83%AA%E8%BB%A2%E9%80%81%E3%83%97%E3%83%AD%E3%83%88%E3%82%B3%E3%83%AB

Future research should prioritize surviving distribution archives, manuals, help text, source, screenshots, and additional raw connection logs.
