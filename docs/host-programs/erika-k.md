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
- hidden `BJ 99` night-owl board
- sample online users and access records

These are intentionally allowed to vary per station while preserving the host software's interaction grammar.

## Current implementation direction

`apps/server/internal/hostprogram/erikak/` owns the Erika K state machine. Do not fold it back into a generic profile-driven runtime.

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
- Retrospective describing Erika's append/reply model: https://kose3.wordpress.com/2000/07/05/%E7%B5%B5%E7%90%86%E9%A6%99/
- Binary transfer protocol reference mentioning Erika K / NMODEM context: https://www.wdic.org/w/WDIC/%E3%83%90%E3%82%A4%E3%83%8A%E3%83%AA%E8%BB%A2%E9%80%81%E3%83%97%E3%83%AD%E3%83%88%E3%82%B3%E3%83%AB

Future research should prioritize surviving distribution archives, manuals, help text, source, screenshots, and additional raw connection logs.
