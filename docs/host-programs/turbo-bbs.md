# TurboBBS / Turbo BBS (Robert H. Maxwell)

Status: historical baseline established; exact command/state reconstruction is still pending direct inspection of the surviving original distribution/source.

This document uses **TurboBBS** for Robert H. Maxwell's 1985 program. It is the upstream historical context for Japanese descendants such as KTBBS and, through the KPUC lineage, RT-BBS. Do not treat those descendants as UI skins or as evidence that every later feature existed in the 1985 original.

## Identification and disambiguation

### Confirmed

- Author: Robert H. Maxwell.
- Surviving BBS Software Directory records version 1.05 in 1985.
- Maxwell's own retrospective says he started the program after receiving Borland Turbo Pascal 2.0.
- It was designed from the beginning to run under both CP/M and MS-DOS, with platform-specific communications handling.
- TurboBBS provided its own modem/communications routines rather than depending on the common CP/M BYE layer. Maxwell says this was an intentional reliability/security choice: a BBS crash should not expose an operating-system command prompt.
- Maxwell says he announced the running BBS on Borland's CompuServe SIG and, after a request from Borland SYSOP Bela Lubkin, cleaned up and publicly released the version 1.0 Pascal source in 1985.
- Known deployments/ports mentioned by Maxwell include his Kaypro and Osborne-1 systems, an MS-DOS Compaq Portable installation in Vancouver, a Nokia MicroMikko II port in Finland, and a TI Professional PC port for a Michigan user group.
- Maxwell moved from Vancouver to the Toronto area in late 1986 and describes that as effectively ending his own TurboBBS project, although the code continued to circulate and be modified elsewhere.

### Do not confuse with

The BBS Software Directory lists these as separate packages:

- **TBBS** by Phil Becker/eSoft — a different CP/M/MS-DOS BBS family.
- **Turbo Pascal Bulletin Board System / TPBBS** by James Whorton and Eddie H. Curlin — also a different program.
- Later Japanese **KTBBS** and **The resource版 Turbo-BBS / RT-BBS** — descendants/arrangements in the Turbo-BBS lineage, not the unchanged 1985 Maxwell program.

The names are close enough that searches for "TBBS", "Turbo BBS", or "Turbo Pascal BBS" can easily mix unrelated software.

## Surviving distribution evidence

### Confirmed archival evidence

A surviving software-library catalog entry titled **Turbo BBS (& source)** lists a 1985 package containing at least:

- `BBS.PAS` — bulletin-board source
- `BBS100.DOC` — documentation
- `BBSHELP.TXT` — online help
- `BBSLIST.TXT` — BBS list
- `CLOCK.INC` — include file
- `COMMENTS.BBS` — comments/messages to SYSOP
- `FILES.BBS` — file list

This is useful provenance and shows that source, documentation, help, SYSOP-comment data and file-list data circulated together. It is **not yet** sufficient to reconstruct exact login prompts, command letters, message numbering, unread semantics, password rules, or file-transfer behavior; those must come from the actual source/manual.

## Japanese lineage

### KTBBS — Confirmed

Vector's surviving **KTBBS ユーザーズマニュアル** explicitly expands the name as **KPUC版 Turbo BBS** and says it was initially developed by 久喜PCユーザーズクラブ (Kuki PC Users Club) for its own station, then released generally after repeated version upgrades and used by many BBSs.

A later KTBBS user retrospective likewise describes KTBBS as a modification of the freeware Turbo-BBS and notes that individual stations often modified KTBBS further.

For project purposes, therefore:

```text
Robert H. Maxwell TurboBBS (1985)
        |
        +--> KPUC版 Turbo BBS / KTBBS
                 |
                 +--> many station-specific KTBBS modifications
                 |
                 +--> RT-BBS lineage (see evidence below)
```

The exact source-level fork points and version-to-version ancestry still need to be established from surviving source headers/version histories.

### RT-BBS — strong archival provenance, exact fork point pending

A surviving archive listing for **The resource edition Turbo-BBS ver5.3a** describes it as an MS-DOS source release and includes both an Eskage/ComServe copyright line dated 1993-01-02 and a **KUKI PC-UsersClub/Taka** copyright credit.

Together with KTBBS's documented expansion as KPUC版 Turbo BBS, this is strong evidence that RT-BBS belongs to the KPUC/Turbo-BBS code lineage rather than merely sharing a similar name.

However, until the archived RT-BBS source headers and KTBBS source are compared directly, keep the following as unresolved:

- which KTBBS/Turbo-BBS version RT-BBS forked from;
- which subsystems were retained versus rewritten;
- whether data-file formats stayed compatible;
- which command letters/prompts were inherited;
- when multi-line, hierarchical-board, gateway, file-transfer and enhanced SYSOP features entered the lineage.

See `rt-bbs.md` for the later RT-BBS feature set.

## Station-specific case: ミンキームーンネットワーク

This is useful evidence for what a heavily customized **RT-BBS-family station** looked like in Japan, but it must not be mistaken for stock RT-BBS behavior.

### Confirmed — 1993 station advertisement

The station's preserved 1993 advertisement states:

- Host Program: **ミンキームーンネットワーク版RTBBS 姫ちゃんヴァージョン**
- "ホストプログラムは、RTBBSを改造しています。"
- host machine: NEC PC-9801 RA2 with Cx486DLC
- multiple modem channels, including 14,400 bps and 9,600 bps lines at the time of the advertisement
- Shift_JIS, 8 data bits, 1 stop bit, no parity, flow control enabled
- guest ID `GUEST`
- access levels 1–4 with different session-time limits

This is unusually good evidence that RT-BBS was not only deployed on PC-98 hardware but also treated as source-modifiable station software.

### Confirmed — preserved 1995 station log, but station-specific

The 30th-anniversary article reproduces a 1995 log showing the customized station's actual interaction style:

- ID/password login
- a themed main prompt such as `Ch.1 = MAIN = ... >>`
- `N` for "未読連続一気読み"
- date-based unread scanning and numeric board IDs
- `W` displaying current channels/users, IDs, handles, current activity, baud/protocol and memo
- `D` used for SYSOP calling; the retrospective says this caused a BEEP at the host
- chat-room use
- logout sequence `QY`

These are valuable **Station-specific** observations. Do not implement them as default RT-BBS commands/prompts until the RT-BBS source/manual confirms which parts are stock and which were the station's "姫ちゃんヴァージョン" modifications.

## Implications for ずっとパソコン通信

1. **TurboBBS itself is mainly an ancestor/context package for the 1996 world.** Maxwell's own development effectively stopped in the late 1980s; the Japanese descendants are more directly relevant to a 1996 Japanese simulation.
2. **KTBBS and RT-BBS must remain separate runtimes.** Shared ancestry is not a reason to collapse them into one generic menu system.
3. **Preserve descendant provenance without copying features backward.** RT-BBS 5.x features cannot be assumed to exist in TurboBBS 1.05.
4. **Treat station customizations as first-class evidence.** ミンキームーン demonstrates that a recognizable RT-BBS installation could be heavily themed and behaviorally customized.
5. **Source inspection is the next fidelity step.** Before implementing an original TurboBBS runtime, obtain/read the 1.05 source/manual and record exact login flow, account structure, command loop, message model, file-area behavior, SYSOP controls and modem state transitions.

## Open research tasks

- Obtain a directly inspectable copy of the Maxwell TurboBBS 1.05 distribution.
- Transcribe/inspect `BBS100.DOC` and `BBS.PAS`.
- Determine exact account/member record fields and authentication flow.
- Determine exact command letters, menus and prompts.
- Determine message storage/numbering/reply behavior and unread state, if any.
- Determine file-area and transfer behavior in the original 1.05 package.
- Compare original TurboBBS source structure with KTBBS 6.21A and RT-BBS 5.3 source headers to establish a defensible code genealogy.
- Verify the licensing/distribution terms from the original package rather than repeating later summaries.

## Sources

### Direct-author / archival

- BBS Software Directory, TurboBBS entry (includes Robert H. Maxwell's retrospective):  
  https://mirrors.archeobits.com/bbs/software.bbsdocumentary.com/expanded.html
- ProgrammaTheek volume 3005, `Turbo BBS (& source)` catalog entry:  
  https://cdnc.heyzine.com/files/uploaded/d65ccb2f1aa4b740ec948ba6ccfef2b5c2091c0b.pdf
- Vector, KTBBS ユーザーズマニュアル:  
  https://www.vector.co.jp/soft/dos/net/se009531.html
- War Ensemble BBS archive listing, `rtb53bbs.zip`:  
  https://bbs.warensemble.com/?dir=bbsrelatbbsdos&page=002-files.xjs
- Vector, The resource版 Turbo-BBS ソースキット:  
  https://www.vector.co.jp/soft/dos/net/se048002.html
- Vector, RTBBS category:  
  https://www.vector.co.jp/vpack/filearea/dos/net/comm/host/rtbbs

### Station evidence

- ミンキームーンネットワーク, 1993年当時のBBS宣伝テキスト:  
  https://minkymoon.jp/senden-cg/
- ミンキームーンネットワーク, 開局30周年記事 / 1995ログ:  
  https://minkymoon.jp/2022/10/09/minkymoonnetwork-30years/

### Secondary / retrospective

- Masami's Storiette, KTBBS explanation:  
  https://lavenderblue.jp/chair/candic/candicff.html
- 教科書には載らないニッポンのインターネットの歴史 (KTBBS chronology; use as secondary evidence only):  
  https://terrazi.hateblo.jp/entry/20020930/p2
