# RT-BBS

Status: research baseline expanded; implementation pending.

Treat RT-BBS as an independent runtime. Gather historical documentation and connection evidence before implementing exact UI/command behavior. Record confirmed version/era, state flow, message structure, feature set and station customization separately from inferred or fictional behavior.

## Lineage

### Strong archival provenance; exact fork point pending

RT-BBS is distributed as **The resource版 Turbo-BBS**.

A surviving archive description for `rtb53bbs.zip`, **The resource edition Turbo-BBS ver5.3a**, calls it an MS-DOS source release and carries both an Eskage/ComServe 1993 copyright line and a **KUKI PC-UsersClub/Taka** copyright credit.

Vector's KTBBS manual independently identifies KTBBS as **KPUC版 Turbo BBS**. Together these are strong evidence that RT-BBS belongs to the KPUC/Turbo-BBS code lineage.

Do not yet claim an exact source fork/version. Confirm it by comparing surviving RT-BBS and KTBBS source headers/version histories.

See `turbo-bbs.md` for the upstream Maxwell TurboBBS context and disambiguation from unrelated TBBS packages.

## Confirmed feature set from the surviving source-kit description

Vector still hosts both The resource版 Turbo-BBS and **The resource版 Turbo-BBS ソースキット 5.3βb**. The source-kit description states that the host is written in **Turbo Pascal 6.0A** and that source is published.

The same description documents these RT-BBS 5.x capabilities:

- small to large NET configurations
- hierarchical boards
- password/group based CUG configuration
- up to 10 boards per forum, 16 hierarchy levels, maximum 1000 boards
- XMODEM / YMODEM / ZMODEM / M-LINK file transfer
- AT-command modems and support for MNP3–7, LAP-M, V.42bis, HST and flexible result-code handling
- gateway mechanism for bidirectional login between adjacent machines
- escape-sequence color UI, with ESC output disable option
- up to 9999 messages / 8 MB per board
- previous/next response traversal, relative/absolute number selection and read-direction switching
- signup/security and online board administration
- strengthened remote SYSOP commands
- line editor
- chat log
- public message/notice board
- extensive help
- bundled NEC PC-9800 drivers, with other MS-DOS systems possible via separate drivers
- multi-line drivers including PIO-9032C / PC-9861K arrangements

These are **RT-BBS 5.x facts**, not evidence that original 1985 TurboBBS or every KTBBS version had the same features.

## Station-specific evidence: ミンキームーンネットワーク

### Confirmed — 1993 advertisement

The station's preserved 1993 advertisement explicitly states:

- Host Program: **ミンキームーンネットワーク版RTBBS 姫ちゃんヴァージョン**
- "ホストプログラムは、RTBBSを改造しています。"
- host: NEC PC-9801 RA2 + Cx486DLC
- multiple modem channels
- Shift_JIS / 8 data bits / 1 stop bit / no parity / flow control
- guest ID `GUEST`
- four access levels with station-specific time limits

This is direct evidence of a real customized RT-BBS deployment on PC-98-era Japanese hardware.

### Confirmed — preserved 1995 log, but Station-specific

The station's 30th-anniversary article reproduces a 1995 connection log showing:

- ID/password login
- themed `Ch.n = MAIN = ... >>` prompt
- `N` for continuous unread reading
- date-based unread scan
- numeric board IDs
- `W` current-user/channel display containing ID, handle, current command, baud/protocol and memo
- `D` SYSOP call; the retrospective says it caused a host-side BEEP
- chat rooms
- `QY` logout sequence

The station explicitly says it modified RTBBS, so these prompts, command letters and themed responses are **Station-specific until verified against stock RT-BBS source/manual**.

For implementation, use this material as evidence that RT-BBS supported deep station customization and as a concrete fixture/reference for a fictional customized station—not as the stock runtime specification.

## Account ID / handle research

The published source makes RT-BBS one of the best candidates for source-level verification of:

- member/account record layout
- handle field length
- minimum length validation
- permitted characters
- upper/lowercase normalization
- duplicate comparison
- signup/update behavior

No exact RT-BBS handle validation rule is currently considered confirmed.

Project-wide handle policy is documented in:
docs/research/ACCOUNT_ID_HANDLE_EVIDENCE.md

## Open questions

- exact KTBBS/Turbo-BBS version from which RT-BBS forked
- exact login/new-user state machine in stock 5.3
- stock command letters versus configurable/station-specific commands
- board/response numbering and unread-pointer persistence
- mail/telegram/chat semantics
- exact multi-line concurrency behavior
- station customization mechanism for prompts/character theming
- compatibility of member/message databases across KTBBS and RT-BBS versions

Sources:

- https://www.vector.co.jp/vpack/filearea/dos/net/comm/host/rtbbs
- https://www.vector.co.jp/soft/dos/net/se048002.html
- https://www.vector.co.jp/soft/dos/net/se009531.html
- https://bbs.warensemble.com/?dir=bbsrelatbbsdos&page=002-files.xjs
- https://minkymoon.jp/senden-cg/
- https://minkymoon.jp/2022/10/09/minkymoonnetwork-30years/
- `turbo-bbs.md`
